package server

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/arr"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/internal/scheduler"
	"github.com/refringe/huntarr2/web/templates/pages"
)

const latestUpgradesLimit = 20

// handleHomePage gathers data from all services and renders the dashboard.
func (s *Server) handleHomePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.fetchHomeData(ctx)
	data.AssetVersion = s.assetVersion

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Home(data).Render(ctx, w); err != nil {
		log.Error().Err(err).Msg("rendering home page")
	}
}

// fetchHomeData gathers dashboard data from all services concurrently, logging and tolerating individual failures.
func (s *Server) fetchHomeData(ctx context.Context) pages.HomeData {
	var data pages.HomeData

	// Stat aggregation below needs the instance list, which must be fetched before the concurrent calls.
	insts, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("fetching instances for home page")
	}
	instMap := make(map[string]instance.Instance, len(insts))
	for _, inst := range insts {
		instMap[inst.ID.String()] = inst
		if inst.AppType.Valid() {
			data.HasArrInstances = true
		}
	}

	var (
		allStats    []activity.ActionStats
		recentStats []activity.ActionStats
		upgrades    []activity.Entry
		arrStatuses []arr.InstanceStatus
		schedStatus scheduler.Status
	)

	var wg sync.WaitGroup
	wg.Add(5)

	go func() {
		defer wg.Done()
		entries, err := s.activity.List(ctx, activity.ListParams{
			Action: activity.ActionUpgradeDetected,
			Limit:  latestUpgradesLimit,
		})
		if err != nil {
			log.Error().Err(err).Msg("fetching latest upgrades")
			return
		}
		upgrades = entries
	}()

	go func() {
		defer wg.Done()
		stats, err := s.activity.Stats(ctx, nil)
		if err != nil {
			log.Error().Err(err).Msg("fetching all-time activity stats")
			return
		}
		allStats = stats
	}()

	go func() {
		defer wg.Done()
		since := time.Now().Add(-24 * time.Hour)
		stats, err := s.activity.Stats(ctx, &since)
		if err != nil {
			log.Error().Err(err).Msg("fetching recent activity stats")
			return
		}
		recentStats = stats
	}()

	go func() {
		defer wg.Done()
		statuses, err := s.arr.Status(ctx)
		if err != nil {
			log.Error().Err(err).Msg("fetching arr status")
			return
		}
		arrStatuses = statuses
	}()

	go func() {
		defer wg.Done()
		schedStatus = s.scheduler.Status()
	}()

	wg.Wait()

	data.SchedulerRunning = schedStatus.Running
	data.SearchesThisHour = int(schedStatus.SearchesThisHour)
	data.HourlyLimit = schedStatus.HourlyLimit

	if allStats != nil {
		allTotals, perInst := aggregateStats(allStats, instMap)
		data.AllTimeSearches = allTotals.searches
		data.AllTimeSkipped = allTotals.skipped
		data.AllTimeUpgrades = allTotals.upgrades
		data.AllTimeDownloads = allTotals.downloads
		data.PerInstance = perInst
	}
	if recentStats != nil {
		recentTotals, _ := aggregateStats(recentStats, instMap)
		data.RecentSearches = recentTotals.searches
		data.RecentSkipped = recentTotals.skipped
		data.RecentUpgrades = recentTotals.upgrades
		data.RecentDownloads = recentTotals.downloads
	}
	for _, e := range upgrades {
		data.LatestUpgrades = append(data.LatestUpgrades, homeUpgrade(e, instMap))
	}
	for _, st := range arrStatuses {
		data.ArrInstances = append(data.ArrInstances,
			pages.HomeArrInstance{
				Name:      st.Name,
				AppType:   st.AppType.Label(),
				Connected: st.Connected,
				Version:   st.Version,
			})
	}

	return data
}

// homeUpgrade converts an upgrade_detected activity entry into a dashboard row.
func homeUpgrade(e activity.Entry, instMap map[string]instance.Instance) pages.HomeUpgrade {
	d, err := activity.ParseImportDetails(e.Details)
	if err != nil {
		log.Warn().Err(err).Str("entry", e.ID.String()).Msg("reading upgrade details")
	}

	row := pages.HomeUpgrade{
		InstanceName: d.InstanceName,
		ItemLabel:    d.ItemLabel,
		ReleaseTitle: d.ReleaseTitle,
		FromQuality:  d.PreviousQuality,
		ToQuality:    d.Quality,
		FromSize:     d.PreviousSize,
		ToSize:       d.Size,
		FromScore:    d.PreviousCustomFormatScore,
		ToScore:      d.CustomFormatScore,
		MediaTags:    mediaTags(d),
		DetectedAt:   e.CreatedAt,
	}

	baseURL := d.InstanceBaseURL
	if e.InstanceID != nil {
		if inst, ok := instMap[e.InstanceID.String()]; ok {
			row.AppType = inst.AppType.Label()
			if row.InstanceName == "" {
				row.InstanceName = inst.Name
			}
			if baseURL == "" {
				baseURL = inst.BaseURL
			}
		}
		if d.MediaCover != "" {
			row.PosterURL = fmt.Sprintf("/api/instances/%s/mediacover/%s", e.InstanceID, d.MediaCover)
		}
	}
	if d.ItemDetailPath != "" && baseURL != "" {
		row.DetailURL = strings.TrimRight(baseURL, "/") + d.ItemDetailPath
	}
	if row.ItemLabel == "" {
		row.ItemLabel = row.ReleaseTitle
	}
	return row
}

// mediaTags summarises the recorded media info as short display chips, skipping values the poll did not record.
func mediaTags(d activity.ImportDetails) []string {
	candidates := []string{
		pages.ResolutionLabel(d.Resolution),
		d.VideoCodec,
		d.VideoDynamicRange,
		bitDepthLabel(d.VideoBitDepth),
		pages.FormatBitrate(d.VideoBitrate),
		pages.AudioLabel(d.AudioCodec, d.AudioChannels),
		d.AudioBitrateText,
		d.AudioBits,
		d.AudioSampleRate,
	}
	var tags []string
	for _, c := range candidates {
		if c != "" {
			tags = append(tags, c)
		}
	}
	return tags
}

// bitDepthLabel renders a video bit depth as e.g. "10-bit", or empty for zero.
func bitDepthLabel(depth int) string {
	if depth <= 0 {
		return ""
	}
	return fmt.Sprintf("%d-bit", depth)
}

// activityTotals holds the aggregate counts returned by aggregateStats.
type activityTotals struct {
	searches  int
	skipped   int
	upgrades  int
	downloads int
}

// aggregateStats sums activity counts into overall totals and a per-instance breakdown.
func aggregateStats(
	stats []activity.ActionStats,
	instMap map[string]instance.Instance,
) (totals activityTotals, perInstance []pages.HomeInstanceStats) {
	type instAcc struct {
		name      string
		appType   string
		searches  int
		skipped   int
		upgrades  int
		downloads int
	}
	byInst := make(map[string]*instAcc)

	for _, s := range stats {
		var key string
		if s.InstanceID != nil {
			key = s.InstanceID.String()
		}

		acc, ok := byInst[key]
		if !ok {
			name := s.InstanceName
			var appType string
			if s.InstanceID != nil {
				if inst, found := instMap[key]; found {
					if name == "" {
						name = inst.Name
					}
					appType = inst.AppType.Label()
				}
			}
			acc = &instAcc{name: name, appType: appType}
			byInst[key] = acc
		}

		switch s.Action {
		case activity.ActionSearchCycle:
			acc.searches += s.Count
			totals.searches += s.Count
		case activity.ActionSearchSkip:
			acc.skipped += s.Count
			totals.skipped += s.Count
		case activity.ActionUpgradeDetected:
			acc.upgrades += s.Count
			totals.upgrades += s.Count
		case activity.ActionDownloadDetected:
			acc.downloads += s.Count
			totals.downloads += s.Count
		case activity.ActionHealthCheck, activity.ActionRateLimit:
			// Logged for auditing but not aggregated into dashboard counters.
		}
	}

	for key, acc := range byInst {
		if key == "" {
			continue
		}
		perInstance = append(perInstance, pages.HomeInstanceStats{
			InstanceID:    key,
			InstanceName:  acc.name,
			AppType:       acc.appType,
			SearchCount:   acc.searches,
			SkipCount:     acc.skipped,
			UpgradeCount:  acc.upgrades,
			DownloadCount: acc.downloads,
		})
	}

	slices.SortFunc(perInstance, func(a, b pages.HomeInstanceStats) int {
		if c := cmp.Compare(a.InstanceName, b.InstanceName); c != 0 {
			return c
		}
		return cmp.Compare(a.AppType, b.AppType)
	})

	return totals, perInstance
}

func (s *Server) handleLogsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := pages.LogsData{AssetVersion: s.assetVersion}

	insts, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("fetching instances for logs page")
	} else {
		for _, inst := range insts {
			data.Instances = append(data.Instances, pages.LogsInstance{
				ID:   inst.ID.String(),
				Name: inst.Name,
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Logs(data).Render(ctx, w); err != nil {
		log.Error().Err(err).Msg("rendering logs page")
	}
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := pages.SettingsData{AssetVersion: s.assetVersion}

	insts, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("fetching instances for settings page")
	} else {
		for _, inst := range insts {
			data.Instances = append(data.Instances, pages.SettingsInstance{
				ID:      inst.ID.String(),
				Name:    inst.Name,
				AppType: inst.AppType.Label(),
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Settings(data).Render(ctx, w); err != nil {
		log.Error().Err(err).Msg("rendering settings page")
	}
}

func (s *Server) handleConnectionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := pages.ConnectionsData{AssetVersion: s.assetVersion}

	instances, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("fetching instances for connections page")
	} else {
		for _, inst := range instances {
			data.Instances = append(data.Instances, pages.ConnectionInstance{
				ID:        inst.ID.String(),
				Name:      inst.Name,
				AppType:   string(inst.AppType),
				BaseURL:   inst.BaseURL,
				HasAPIKey: inst.APIKey != "",
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Connections(data).Render(ctx, w); err != nil {
		log.Error().Err(err).Msg("rendering connections page")
	}
}
