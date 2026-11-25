// Package scheduler runs the adaptive search loop over enabled *arr instances.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/internal/settings"
)

// activityRetention is the maximum age of retained activity log entries.
const activityRetention = 30 * 24 * time.Hour

// pruneInterval controls how often the scheduler prunes old activity log entries.
const pruneInterval = 24 * time.Hour

// historyPollInterval is the minimum time between successive history polls across all instances.
const historyPollInterval = 5 * time.Minute

// historyPageSize is the number of history records fetched per poll.
const historyPageSize = 50

// Activity detail keys reused across logActivity calls.
const (
	detailInstanceName    = "instanceName"
	detailInstanceBaseURL = "instanceBaseURL"
)

// cooldownRetention is the maximum age of search cooldown records.
const cooldownRetention = settings.MaxCooldownPeriod

// firstPollLookback is the lookback window used when an instance has never been polled before.
const firstPollLookback = 24 * time.Hour

// ErrCycleRunning is returned when a search is requested for an instance whose search cycle is still running.
var ErrCycleRunning = errors.New("search cycle already running")

// ErrSearchLimitReached is returned when a search is requested after the hourly search limit is used up.
var ErrSearchLimitReached = errors.New("hourly search limit reached")

// settingsResolver loads and merges settings for a specific instance or globally.
type settingsResolver interface {
	Resolve(ctx context.Context, instanceID uuid.UUID) (settings.Resolved, error)
	ResolveGlobal(ctx context.Context) (settings.Resolved, error)
}

// cooldownTracker filters and records per-item search cooldowns.
type cooldownTracker interface {
	FilterCoolingDown(ctx context.Context, instanceID uuid.UUID,
		itemIDs []int, cooldownPeriod time.Duration) ([]int, error)
	RecordSearches(ctx context.Context, instanceID uuid.UUID,
		itemIDs []int) error
	DeleteExpired(ctx context.Context, olderThan time.Duration) (int64, error)
}

// activityLogger persists structured activity log entries and supports periodic pruning of old entries.
type activityLogger interface {
	Log(ctx context.Context, entry *activity.Entry) error
	Prune(ctx context.Context, retention time.Duration) (int64, error)
}

// arrSearcher fetches upgradeable items, triggers searches, and reads import history from *arr instances.
type arrSearcher interface {
	Upgradeable(ctx context.Context,
		instanceID uuid.UUID) (arr.UpgradeResult, error)
	Search(ctx context.Context, instanceID uuid.UUID,
		itemIDs []int) (arr.SearchResult, error)
	History(ctx context.Context, instanceID uuid.UUID,
		since time.Time, pageSize int) ([]arr.HistoryRecord, error)
}

// pollTracker persists and queries per-instance history poll timestamps.
type pollTracker interface {
	LastPolled(ctx context.Context, instanceID uuid.UUID) (time.Time, error)
	RecordPoll(ctx context.Context, instanceID uuid.UUID, polledAt time.Time) error
}

// instanceLister lists and fetches configured *arr instances.
type instanceLister interface {
	List(ctx context.Context) ([]instance.Instance, error)
	Get(ctx context.Context, id uuid.UUID) (instance.Instance, error)
}

// InstanceSchedule holds scheduling state for a single instance, exposed by the Status endpoint.
type InstanceSchedule struct {
	InstanceID   uuid.UUID     `json:"instanceId"`
	InstanceName string        `json:"instanceName"`
	NextSearchAt time.Time     `json:"nextSearchAt"`
	Interval     time.Duration `json:"interval"`
	Enabled      bool          `json:"enabled"`
}

// Status is a point-in-time snapshot of the scheduler's state.
type Status struct {
	Running          bool               `json:"running"`
	SearchesThisHour int64              `json:"searchesThisHour"`
	HourlyLimit      int                `json:"hourlyLimit"`
	Instances        []InstanceSchedule `json:"instances"`
}

// Scheduler runs each instance's search cycles in the background; Status is safe to call from any goroutine.
type Scheduler struct {
	tickInterval time.Duration
	instances    instanceLister
	settings     settingsResolver
	cooldowns    cooldownTracker
	activity     activityLogger
	arr          arrSearcher
	polls        pollTracker

	mu              sync.Mutex
	running         bool
	schedules       map[uuid.UUID]InstanceSchedule
	searchCount     int64
	hourlyLimit     int
	hourResetAt     time.Time
	lastPrunedAt    time.Time
	lastHistoryPoll time.Time

	// limitLogged and windowSkipLogged mark skips already written to the activity log for the current hour or window.
	limitLogged      bool
	windowSkipLogged map[uuid.UUID]struct{}

	// cycles tracks running search cycles; inFlight holds their instances and reserved their unspent search allowance.
	cycles   sync.WaitGroup
	inFlight map[uuid.UUID]struct{}
	reserved int64
}

// New creates a Scheduler with the given tick interval and dependencies, rejecting a non-positive tick interval.
func New(
	tickInterval time.Duration,
	instances instanceLister,
	settings settingsResolver,
	cooldowns cooldownTracker,
	activity activityLogger,
	arr arrSearcher,
	polls pollTracker,
) (*Scheduler, error) {
	if tickInterval <= 0 {
		return nil, fmt.Errorf("tick interval must be positive, got %s", tickInterval)
	}
	return &Scheduler{
		tickInterval: tickInterval,
		instances:    instances,
		settings:     settings,
		cooldowns:    cooldowns,
		activity:     activity,
		arr:          arr,
		polls:        polls,
		schedules:    make(map[uuid.UUID]InstanceSchedule),
		hourResetAt:  nextHourBoundary(time.Now()),

		windowSkipLogged: make(map[uuid.UUID]struct{}),
		inFlight:         make(map[uuid.UUID]struct{}),
	}, nil
}

// Run starts the scheduling loop and blocks until ctx is cancelled and every running search cycle has returned.
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	log.Info().Dur("tick", s.tickInterval).Msg("scheduler started")

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.cycles.Wait()
			log.Info().Msg("scheduler stopped")
			return ctx.Err()
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// Status returns a point-in-time snapshot of the scheduler's state.
func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := Status{
		Running:          s.running,
		SearchesThisHour: s.searchCount,
		HourlyLimit:      s.hourlyLimit,
	}

	instances := make([]InstanceSchedule, 0, len(s.schedules))
	for _, sched := range s.schedules {
		instances = append(instances, sched)
	}
	slices.SortFunc(instances, func(a, b InstanceSchedule) int {
		return strings.Compare(a.InstanceName, b.InstanceName)
	})
	st.Instances = instances

	return st
}

// tick runs a single scheduling pass.
func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now()

	s.mu.Lock()
	if now.After(s.hourResetAt) {
		s.searchCount = 0
		s.hourResetAt = nextHourBoundary(now)
		s.limitLogged = false
	}
	s.mu.Unlock()

	globalSettings, err := s.settings.ResolveGlobal(ctx)
	if err != nil {
		log.Error().Err(err).Msg("loading global settings")
		return
	}

	s.mu.Lock()
	s.hourlyLimit = globalSettings.SearchLimit
	s.mu.Unlock()

	insts, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("listing instances")
		return
	}

	seen := make(map[uuid.UUID]struct{}, len(insts))
	for _, inst := range insts {
		seen[inst.ID] = struct{}{}
	}

	for _, inst := range insts {
		if err := ctx.Err(); err != nil {
			return
		}
		s.startCycleIfDue(ctx, inst, now, globalSettings.SearchLimit)
	}

	s.mu.Lock()
	for id := range s.schedules {
		if _, ok := seen[id]; !ok {
			log.Debug().Str("instance", id.String()).
				Msg("removed schedule for deleted instance")
			delete(s.schedules, id)
			delete(s.windowSkipLogged, id)
		}
	}
	s.mu.Unlock()

	s.pollUpgradeHistory(ctx, insts)
	s.pruneActivityLog(ctx, now)
}

// startCycleIfDue starts a background search cycle for an instance that is enabled, idle, due, and inside its window.
func (s *Scheduler) startCycleIfDue(ctx context.Context, inst instance.Instance, now time.Time, hourlyLimit int) {
	resolved, err := s.settings.Resolve(ctx, inst.ID)
	if err != nil {
		log.Error().Err(err).Str("instance", inst.Name).
			Msg("resolving instance settings")
		return
	}

	if !resolved.Enabled {
		s.mu.Lock()
		s.schedules[inst.ID] = newInstanceSchedule(
			inst.ID, inst.Name, time.Time{}, false, 0,
		)
		delete(s.windowSkipLogged, inst.ID)
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	sched, exists := s.schedules[inst.ID]
	s.mu.Unlock()

	if exists && now.Before(sched.NextSearchAt) {
		return
	}

	if !inSearchWindow(resolved.SearchWindowStart, resolved.SearchWindowEnd, time.Now()) {
		s.mu.Lock()
		_, logged := s.windowSkipLogged[inst.ID]
		s.windowSkipLogged[inst.ID] = struct{}{}
		s.mu.Unlock()

		if !logged {
			s.logActivity(ctx, &inst.ID, activity.LevelDebug, activity.ActionSearchSkip,
				"outside search window", map[string]any{
					detailInstanceName: inst.Name,
					"windowStart":      resolved.SearchWindowStart,
					"windowEnd":        resolved.SearchWindowEnd,
				})
		}
		return
	}

	maxSearch, err := s.reserveSearches(inst.ID, hourlyLimit, resolved.BatchSize)
	if err != nil {
		if errors.Is(err, ErrSearchLimitReached) && s.claimLimitLog(hourlyLimit) {
			s.logActivity(ctx, nil, activity.LevelInfo, activity.ActionRateLimit,
				fmt.Sprintf("hourly search limit reached (%d)", hourlyLimit), nil)
		}
		return
	}

	s.mu.Lock()
	delete(s.windowSkipLogged, inst.ID)
	s.mu.Unlock()

	prevInterval := resolved.SearchInterval
	if exists && sched.Interval > 0 {
		prevInterval = sched.Interval
	}

	s.cycles.Go(func() {
		searched, totalItems, _ := s.runInstanceCycle(ctx, inst, resolved, maxSearch)
		nextInterval := computeNextInterval(
			resolved.SearchInterval, prevInterval, totalItems, resolved.BatchSize,
		)

		s.mu.Lock()
		s.schedules[inst.ID] = newInstanceSchedule(
			inst.ID, inst.Name, time.Now().Add(nextInterval), true, nextInterval,
		)
		s.mu.Unlock()
		s.releaseSearches(inst.ID, maxSearch, searched)
	})
}

// SearchNow runs one search cycle for an instance on request, capped at batchSize items and the hourly limit.
func (s *Scheduler) SearchNow(ctx context.Context, instanceID uuid.UUID, batchSize int) (int, error) {
	inst, err := s.instances.Get(ctx, instanceID)
	if err != nil {
		return 0, fmt.Errorf("fetching instance %s: %w", instanceID, err)
	}

	globalSettings, err := s.settings.ResolveGlobal(ctx)
	if err != nil {
		return 0, fmt.Errorf("loading global settings: %w", err)
	}
	resolved, err := s.settings.Resolve(ctx, instanceID)
	if err != nil {
		return 0, fmt.Errorf("resolving instance settings: %w", err)
	}

	maxSearch, err := s.reserveSearches(instanceID, globalSettings.SearchLimit, batchSize)
	if err != nil {
		return 0, err
	}

	searched, _, err := s.runInstanceCycle(ctx, inst, resolved, maxSearch)
	s.releaseSearches(instanceID, maxSearch, searched)
	return searched, err
}

// reserveSearches marks an idle instance as running and reserves up to batchSize of the hour's unused search allowance.
func (s *Scheduler) reserveSearches(id uuid.UUID, hourlyLimit, batchSize int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.inFlight[id]; running {
		return 0, ErrCycleRunning
	}

	maxSearch := int(min(int64(batchSize), int64(hourlyLimit)-s.searchCount-s.reserved))
	if maxSearch <= 0 {
		return 0, ErrSearchLimitReached
	}

	s.reserved += int64(maxSearch)
	s.inFlight[id] = struct{}{}
	return maxSearch, nil
}

// releaseSearches marks the instance as idle, counts its searched items, and returns its reservation.
func (s *Scheduler) releaseSearches(id uuid.UUID, reserved, searched int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.searchCount += int64(searched)
	s.reserved -= int64(reserved)
	delete(s.inFlight, id)
}

// claimLimitLog reports whether the reached hourly limit still needs its activity entry for this hour, claiming it.
func (s *Scheduler) claimLimitLog(hourlyLimit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.limitLogged || s.searchCount < int64(hourlyLimit) {
		return false
	}
	s.limitLogged = true
	return true
}

// pollUpgradeHistory polls each instance's history for upgrades and downloads, at most once per historyPollInterval.
func (s *Scheduler) pollUpgradeHistory(ctx context.Context, insts []instance.Instance) {
	now := time.Now()
	s.mu.Lock()
	due := now.After(s.lastHistoryPoll.Add(historyPollInterval))
	if due {
		s.lastHistoryPoll = now
	}
	s.mu.Unlock()
	if !due {
		return
	}

	for _, inst := range insts {
		if err := ctx.Err(); err != nil {
			return
		}
		s.pollInstanceHistory(ctx, inst)
	}
}

// pollInstanceHistory logs an activity entry per upgrade or download found and advances the poll time to the newest.
func (s *Scheduler) pollInstanceHistory(ctx context.Context, inst instance.Instance) {
	since, err := s.polls.LastPolled(ctx, inst.ID)
	if err != nil {
		log.Warn().Err(err).Str("instance", inst.Name).
			Msg("reading history poll state")
		return
	}
	if since.IsZero() {
		since = time.Now().Add(-firstPollLookback)
	}

	log.Debug().Str("instance", inst.Name).Time("since", since).
		Msg("polling instance history")

	records, err := s.arr.History(ctx, inst.ID, since, historyPageSize)
	if err != nil {
		log.Warn().Err(err).Str("instance", inst.Name).
			Msg("fetching instance history")
		return
	}

	log.Debug().Str("instance", inst.Name).Int("records", len(records)).
		Msg("history poll returned")

	newest := since
	for _, r := range records {
		if r.Date.After(newest) {
			newest = r.Date
		}

		action := activity.ActionDownloadDetected
		label := "new download"
		if r.IsUpgrade {
			action = activity.ActionUpgradeDetected
			label = "quality upgrade"
		}
		s.logActivity(ctx, &inst.ID, activity.LevelInfo, action,
			fmt.Sprintf("%s detected: %s", label, r.ItemLabel),
			historyDetails(inst, r))
	}

	if err := s.polls.RecordPoll(ctx, inst.ID, newest); err != nil {
		log.Warn().Err(err).Str("instance", inst.Name).
			Msg("recording history poll timestamp")
	}
}

// historyDetails builds the activity details for an import history record, omitting empty and unknown values.
func historyDetails(inst instance.Instance, r arr.HistoryRecord) map[string]any {
	details := map[string]any{
		detailInstanceName:    inst.Name,
		detailInstanceBaseURL: inst.BaseURL,
		"itemLabel":           r.ItemLabel,
		"itemDetailPath":      r.DetailPath,
		"quality":             r.Quality,
	}
	putString := func(key, value string) {
		if value != "" {
			details[key] = value
		}
	}
	putInt := func(key string, value int64) {
		if value != 0 {
			details[key] = value
		}
	}
	putScore := func(key string, value *int) {
		if value != nil {
			details[key] = *value
		}
	}

	putString("releaseTitle", r.ReleaseTitle)
	putString("releaseGroup", r.ReleaseGroup)
	putString("mediaCover", r.MediaCoverPath)
	putInt("size", r.Size)
	putScore("customFormatScore", r.CustomFormatScore)
	if r.IsUpgrade {
		putString("previousQuality", r.PreviousQuality)
		putInt("previousSize", r.PreviousSize)
		putScore("previousCustomFormatScore", r.PreviousCustomFormatScore)
	}
	if m := r.MediaInfo; m != nil {
		putString("resolution", m.Resolution)
		putString("videoCodec", m.VideoCodec)
		putInt("videoBitDepth", int64(m.VideoBitDepth))
		putInt("videoBitrate", m.VideoBitrate)
		putString("videoDynamicRange", m.VideoDynamicRangeType)
		putString("audioCodec", m.AudioCodec)
		if m.AudioChannels != 0 {
			details["audioChannels"] = m.AudioChannels
		}
		putInt("audioBitrate", m.AudioBitrate)
		putString("audioBitrateText", m.AudioBitrateText)
		putString("audioBits", m.AudioBits)
		putString("audioSampleRate", m.AudioSampleRate)
		putString("runTime", m.RunTime)
	}
	return details
}

// pruneActivityLog deletes old activity log entries and expired cooldown records once per day.
func (s *Scheduler) pruneActivityLog(ctx context.Context, now time.Time) {
	s.mu.Lock()
	due := now.After(s.lastPrunedAt.Add(pruneInterval))
	s.mu.Unlock()

	if !due {
		return
	}

	deleted, err := s.activity.Prune(ctx, activityRetention)
	if err != nil {
		log.Warn().Err(err).Msg("failed to prune activity log")
		return
	}

	cooldownDeleted, err := s.cooldowns.DeleteExpired(ctx, cooldownRetention)
	if err != nil {
		log.Warn().Err(err).Msg("failed to prune expired cooldowns")
	}

	s.mu.Lock()
	s.lastPrunedAt = now
	s.mu.Unlock()

	if deleted > 0 {
		log.Info().Int64("deleted", deleted).Msg("pruned old activity log entries")
	}
	if cooldownDeleted > 0 {
		log.Info().Int64("deleted", cooldownDeleted).
			Msg("pruned expired cooldown records")
	}
}

// runInstanceCycle searches at most maxSearch items for an instance, returning the searched and candidate counts.
func (s *Scheduler) runInstanceCycle(
	ctx context.Context,
	inst instance.Instance,
	resolved settings.Resolved,
	maxSearch int,
) (searched int, totalItems int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, 0, err
	}

	result, err := s.arr.Upgradeable(ctx, inst.ID)
	if err != nil {
		log.Error().Err(err).Str("instance", inst.Name).
			Msg("fetching upgradeable items")
		s.logActivity(ctx, &inst.ID, activity.LevelError, activity.ActionSearchCycle,
			"failed to fetch upgradeable items", map[string]any{
				detailInstanceName: inst.Name,
				"error":            err.Error(),
			})
		return 0, 0, fmt.Errorf("fetching upgradeable items: %w", err)
	}

	items := result.Items
	if resolved.SearchMissing {
		items = append(items, result.MissingItems...)
	}
	stats := result.Stats
	totalItems = len(items)
	if totalItems == 0 {
		s.logActivity(ctx, &inst.ID, activity.LevelDebug, activity.ActionSearchCycle,
			"no searchable items", map[string]any{
				detailInstanceName: inst.Name,
				"libraryTotal":     stats.LibraryTotal,
				"noFile":           stats.NoFile,
				"unmonitored":      stats.Unmonitored,
				"noProfile":        stats.NoProfile,
				"upgradeBlocked":   stats.UpgradeBlocked,
				"unknownQuality":   stats.UnknownQuality,
				"atOrAbove":        stats.AtOrAbove,
				"searchMissing":    resolved.SearchMissing,
			})
		return 0, 0, nil
	}

	itemIDs := make([]int, totalItems)
	for i, item := range items {
		itemIDs[i] = item.ID
	}

	coolingDown, err := s.cooldowns.FilterCoolingDown(ctx, inst.ID, itemIDs, resolved.CooldownPeriod)
	if err != nil {
		log.Error().Err(err).Str("instance", inst.Name).
			Msg("filtering cooldown items")
		return 0, totalItems, fmt.Errorf("filtering cooldown items: %w", err)
	}

	searchIDs := excludeIDs(itemIDs, coolingDown)
	if len(searchIDs) == 0 {
		s.logActivity(ctx, &inst.ID, activity.LevelDebug, activity.ActionSearchSkip,
			"all items in cooldown", map[string]any{
				detailInstanceName: inst.Name,
				"totalItems":       totalItems,
				"coolingDown":      len(coolingDown),
				"upgradeableAll":   totalItems,
			})
		return 0, totalItems, nil
	}

	if len(searchIDs) > maxSearch {
		searchIDs = searchIDs[:maxSearch]
	}

	itemByID := make(map[int]arr.UpgradeItem, len(items))
	for _, item := range items {
		itemByID[item.ID] = item
	}
	searchedItems := make([]map[string]any, len(searchIDs))
	for i, id := range searchIDs {
		ui := itemByID[id]
		searchedItems[i] = map[string]any{
			"label":      ui.Label,
			"detailPath": ui.DetailPath,
		}
	}

	if _, err := s.arr.Search(ctx, inst.ID, searchIDs); err != nil {
		log.Error().Err(err).Str("instance", inst.Name).
			Msg("searching items")
		s.logActivity(ctx, &inst.ID, activity.LevelError, activity.ActionSearchCycle,
			"search command failed", map[string]any{
				detailInstanceName:    inst.Name,
				detailInstanceBaseURL: inst.BaseURL,
				"error":               err.Error(),
				"searchedItems":       searchedItems,
			})
		return 0, totalItems, fmt.Errorf("searching items: %w", err)
	}

	if err := s.cooldowns.RecordSearches(ctx, inst.ID, searchIDs); err != nil {
		log.Warn().Err(err).Str("instance", inst.Name).
			Msg("recording search cooldowns")
	}

	skipped := min(totalItems, resolved.BatchSize) - len(searchIDs)
	s.logActivity(ctx, &inst.ID, activity.LevelInfo, activity.ActionSearchCycle,
		fmt.Sprintf("searched %d items for %s", len(searchIDs), inst.Name),
		map[string]any{
			detailInstanceName:    inst.Name,
			detailInstanceBaseURL: inst.BaseURL,
			"searched":            len(searchIDs),
			"skipped":             skipped,
			"coolingDown":         len(coolingDown),
			"upgradeableAll":      len(result.Items),
			"missingAll":          len(result.MissingItems),
			"candidateTotal":      totalItems,
			"libraryTotal":        stats.LibraryTotal,
			"searchedItems":       searchedItems,
		})

	return len(searchIDs), totalItems, nil
}

// logActivity writes an entry to the activity service, reporting persistence failures via zerolog.
func (s *Scheduler) logActivity(
	ctx context.Context,
	instanceID *uuid.UUID,
	level activity.Level,
	action activity.Action,
	message string,
	details map[string]any,
) {
	entry := &activity.Entry{
		InstanceID: instanceID,
		Level:      level,
		Action:     action,
		Message:    message,
		Details:    details,
	}
	if err := s.activity.Log(ctx, entry); err != nil {
		log.Warn().Err(err).Msg("failed to log scheduler activity")
	}
}

// computeNextInterval doubles previous on empty results (max 4x base), halves it on a backlog (min base/4), else base.
func computeNextInterval(base, previous time.Duration, totalItems, batchSize int) time.Duration {
	switch {
	case totalItems == 0:
		ceiling := base * 4
		doubled := previous * 2
		if doubled > ceiling {
			return ceiling
		}
		return doubled
	case totalItems > batchSize*2:
		floor := base / 4
		halved := previous / 2
		if halved < floor {
			return floor
		}
		return halved
	default:
		return base
	}
}

// inSearchWindow reports whether now falls within the HH:MM window; empty or malformed bounds mean always allowed.
func inSearchWindow(start, end string, now time.Time) bool {
	if start == "" && end == "" {
		return true
	}

	nowMins := now.Hour()*60 + now.Minute()
	startMins, startErr := parseHHMM(start)
	endMins, endErr := parseHHMM(end)

	if startErr != nil || endErr != nil {
		log.Warn().
			Str("start", start).
			Str("end", end).
			Msg("ignoring malformed search window, allowing search")
		return true
	}

	if startMins <= endMins {
		return nowMins >= startMins && nowMins < endMins
	}
	return nowMins >= startMins || nowMins < endMins
}

// parseHHMM converts "HH:MM" to minutes since midnight, rejecting an empty string.
func parseHHMM(v string) (int, error) {
	if v == "" {
		return 0, fmt.Errorf("empty time string")
	}
	return settings.ParseHHMM(v)
}

// excludeIDs returns elements of all that are not in excluded.
func excludeIDs(all, excluded []int) []int {
	excludeSet := make(map[int]struct{}, len(excluded))
	for _, id := range excluded {
		excludeSet[id] = struct{}{}
	}
	var result []int
	for _, id := range all {
		if _, ok := excludeSet[id]; !ok {
			result = append(result, id)
		}
	}
	return result
}

func newInstanceSchedule(
	id uuid.UUID, name string, nextAt time.Time, enabled bool, interval time.Duration,
) InstanceSchedule {
	return InstanceSchedule{
		InstanceID:   id,
		InstanceName: name,
		NextSearchAt: nextAt,
		Interval:     interval,
		Enabled:      enabled,
	}
}

// nextHourBoundary returns the start of the next full hour after t.
func nextHourBoundary(t time.Time) time.Time {
	return t.Truncate(time.Hour).Add(time.Hour)
}
