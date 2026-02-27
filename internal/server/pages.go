package server

import (
	"cmp"
	"context"
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

// capitalise returns s with the first byte upper-cased. This is suitable for
// ASCII identifiers such as application type names (e.g. "sonarr" to
// "Sonarr"). It is not safe for arbitrary multi-byte Unicode input.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// handleHomePage gathers data from all services and renders the dashboard.
func (s *Server) handleHomePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := s.fetchHomeData(ctx)
	data.AssetVersion = s.assetVersion

	// Templ streams directly to the ResponseWriter, so the status header is
	// sent before rendering begins. If Render fails partway through, the
	// client receives partial HTML with no way to signal the error in the
	// HTTP status. Logging is the best recovery available here.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages.Home(data).Render(ctx, w); err != nil {
		log.Error().Err(err).Msg("rendering home page")
	}
}

// fetchHomeData gathers dashboard data from all services concurrently.
// Individual service errors are logged but do not abort the collection so
// that partial data is still available rather than a blank error screen.
func (s *Server) fetchHomeData(ctx context.Context) pages.HomeData {
	var data pages.HomeData

	// The instance list is a dependency for activity stat aggregation, so
	// it must complete before the concurrent calls below.
	insts, err := s.instances.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("fetching instances for home page")
	}
	instMap := make(map[string]instance.Instance, len(insts))
	for _, inst := range insts {
		instMap[inst.ID.String()] = inst
		switch inst.AppType {
		case instance.AppTypeSonarr, instance.AppTypeRadarr,
			instance.AppTypeLidarr, instance.AppTypeWhisparr:
			data.HasArrInstances = true
		}
	}

	// The remaining service calls are independent; run them concurrently
	// to reduce dashboard latency. Each goroutine writes to a dedicated
	// variable; wg.Wait provides the happens-before guarantee. If a
	// service call panics, the deferred wg.Done still executes, and the
	// panic propagates to the recovery middleware via the HTTP handler.
	var (
		allStats    []activity.ActionStats
		recentStats []activity.ActionStats
		arrStatuses []arr.InstanceStatus
		schedStatus scheduler.Status
	)

	var wg sync.WaitGroup
	wg.Add(4)

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

	// Assemble results that depend on instMap resolution.
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
	for _, st := range arrStatuses {
		data.ArrInstances = append(data.ArrInstances,
			pages.HomeArrInstance{
				Name:      st.Name,
				AppType:   capitalise(string(st.AppType)),
				Connected: st.Connected,
				Version:   st.Version,
			})
	}

	return data
}

// activityTotals holds the aggregate counts returned by aggregateStats.
type activityTotals struct {
	searches  int
	skipped   int
	upgrades  int
	downloads int
}

// aggregateStats sums activity counts across ActionStats entries, building
// a per-instance breakdown and overall totals.
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
					appType = capitalise(string(inst.AppType))
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
			// Health checks and rate limit events are logged for
			// auditing but not aggregated into dashboard counters.
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
				AppType: capitalise(string(inst.AppType)),
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
