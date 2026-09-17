package server

import (
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/web/templates/pages"
)

func TestAggregateStats(t *testing.T) {
	t.Parallel()

	idA := uuid.New()
	idB := uuid.New()

	instMap := map[string]instance.Instance{
		idA.String(): {ID: idA, Name: "Sonarr Main", AppType: instance.AppTypeSonarr},
		idB.String(): {ID: idB, Name: "Radarr Main", AppType: instance.AppTypeRadarr},
	}

	t.Run("empty stats", func(t *testing.T) {
		t.Parallel()
		totals, perInst := aggregateStats(nil, instMap)
		if totals.searches != 0 {
			t.Errorf("searches = %d, want 0", totals.searches)
		}
		if totals.skipped != 0 {
			t.Errorf("skipped = %d, want 0", totals.skipped)
		}
		if len(perInst) != 0 {
			t.Errorf("perInstance len = %d, want 0", len(perInst))
		}
	})

	t.Run("counts by action", func(t *testing.T) {
		t.Parallel()
		stats := []activity.ActionStats{
			{InstanceID: &idA, Action: activity.ActionSearchCycle, Count: 10},
			{InstanceID: &idA, Action: activity.ActionSearchSkip, Count: 3},
			{InstanceID: &idB, Action: activity.ActionSearchCycle, Count: 5},
		}
		totals, perInst := aggregateStats(stats, instMap)
		if totals.searches != 15 {
			t.Errorf("searches = %d, want 15", totals.searches)
		}
		if totals.skipped != 3 {
			t.Errorf("skipped = %d, want 3", totals.skipped)
		}
		if len(perInst) != 2 {
			t.Errorf("perInstance len = %d, want 2", len(perInst))
		}

		byID := make(map[string]pages.HomeInstanceStats, len(perInst))
		for _, pi := range perInst {
			byID[pi.InstanceID] = pi
		}

		a := byID[idA.String()]
		if a.SearchCount != 10 {
			t.Errorf("instance A searches = %d, want 10", a.SearchCount)
		}
		if a.SkipCount != 3 {
			t.Errorf("instance A skips = %d, want 3", a.SkipCount)
		}
		if a.InstanceName != "Sonarr Main" {
			t.Errorf("instance A name = %q, want %q", a.InstanceName, "Sonarr Main")
		}
		if a.AppType != "Sonarr" {
			t.Errorf("instance A appType = %q, want %q", a.AppType, "Sonarr")
		}

		b := byID[idB.String()]
		if b.SearchCount != 5 {
			t.Errorf("instance B searches = %d, want 5", b.SearchCount)
		}
		if b.SkipCount != 0 {
			t.Errorf("instance B skips = %d, want 0", b.SkipCount)
		}
	})

	t.Run("upgrade and download counts", func(t *testing.T) {
		t.Parallel()
		stats := []activity.ActionStats{
			{InstanceID: &idA, Action: activity.ActionUpgradeDetected, Count: 4},
			{InstanceID: &idA, Action: activity.ActionDownloadDetected, Count: 2},
			{InstanceID: &idB, Action: activity.ActionUpgradeDetected, Count: 1},
		}
		totals, perInst := aggregateStats(stats, instMap)
		if totals.upgrades != 5 {
			t.Errorf("upgrades = %d, want 5", totals.upgrades)
		}
		if totals.downloads != 2 {
			t.Errorf("downloads = %d, want 2", totals.downloads)
		}

		byID := make(map[string]pages.HomeInstanceStats, len(perInst))
		for _, pi := range perInst {
			byID[pi.InstanceID] = pi
		}

		a := byID[idA.String()]
		if a.UpgradeCount != 4 {
			t.Errorf("instance A upgrades = %d, want 4", a.UpgradeCount)
		}
		if a.DownloadCount != 2 {
			t.Errorf("instance A downloads = %d, want 2", a.DownloadCount)
		}

		b := byID[idB.String()]
		if b.UpgradeCount != 1 {
			t.Errorf("instance B upgrades = %d, want 1", b.UpgradeCount)
		}
	})

	t.Run("nil instance ID excluded from per-instance", func(t *testing.T) {
		t.Parallel()
		stats := []activity.ActionStats{
			{InstanceID: nil, Action: activity.ActionSearchCycle, Count: 7},
			{InstanceID: &idA, Action: activity.ActionSearchCycle, Count: 2},
		}
		totals, perInst := aggregateStats(stats, instMap)
		if totals.searches != 9 {
			t.Errorf("searches = %d, want 9", totals.searches)
		}
		if len(perInst) != 1 {
			t.Errorf("perInstance len = %d, want 1", len(perInst))
		}
		if len(perInst) > 0 && perInst[0].InstanceID != idA.String() {
			t.Errorf("perInstance[0].InstanceID = %q, want %q",
				perInst[0].InstanceID, idA.String())
		}
	})

	t.Run("unknown instance uses stat name", func(t *testing.T) {
		t.Parallel()
		unknownID := uuid.New()
		stats := []activity.ActionStats{
			{
				InstanceID:   &unknownID,
				InstanceName: "Deleted Instance",
				Action:       activity.ActionSearchCycle,
				Count:        4,
			},
		}
		_, perInst := aggregateStats(stats, instMap)
		if len(perInst) != 1 {
			t.Fatalf("perInstance len = %d, want 1", len(perInst))
		}
		if perInst[0].InstanceName != "Deleted Instance" {
			t.Errorf("name = %q, want %q", perInst[0].InstanceName, "Deleted Instance")
		}
		if perInst[0].AppType != "" {
			t.Errorf("appType = %q, want empty", perInst[0].AppType)
		}
	})

	t.Run("unrecognised action ignored", func(t *testing.T) {
		t.Parallel()
		stats := []activity.ActionStats{
			{InstanceID: &idA, Action: activity.Action("unknown_action"), Count: 99},
		}
		totals, _ := aggregateStats(stats, instMap)
		if totals.searches != 0 {
			t.Errorf("searches = %d, want 0", totals.searches)
		}
		if totals.skipped != 0 {
			t.Errorf("skipped = %d, want 0", totals.skipped)
		}
	})
}

func TestHomeUpgrade(t *testing.T) {
	t.Parallel()

	instID := uuid.New()
	instMap := map[string]instance.Instance{
		instID.String(): {ID: instID, Name: "Radarr Main", AppType: instance.AppTypeRadarr, BaseURL: "http://radarr:7878/"},
	}
	created := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)

	t.Run("full details", func(t *testing.T) {
		t.Parallel()
		entry := activity.Entry{
			InstanceID: &instID,
			CreatedAt:  created,
			Details: map[string]any{
				"instanceName":              "Radarr",
				"instanceBaseURL":           "http://radarr.local",
				"itemLabel":                 "Movie (2024)",
				"itemDetailPath":            "/movie/movie-2024",
				"releaseTitle":              "Movie.2024.1080p-GRP",
				"mediaCover":                "42/poster-250.jpg",
				"quality":                   "Bluray-1080p",
				"previousQuality":           "HDTV-720p",
				"size":                      float64(8_000_000_000),
				"previousSize":              float64(2_000_000_000),
				"customFormatScore":         float64(1100),
				"previousCustomFormatScore": float64(-50),
				"resolution":                "1920x1080",
				"videoCodec":                "x265",
				"videoDynamicRange":         "HDR10",
				"videoBitDepth":             float64(10),
				"videoBitrate":              float64(12_000_000),
				"audioCodec":                "TrueHD",
				"audioChannels":             7.1,
			},
		}

		got := homeUpgrade(entry, instMap)

		if got.InstanceName != "Radarr" || got.AppType != "Radarr" {
			t.Errorf("instance = %q/%q, want Radarr/Radarr", got.InstanceName, got.AppType)
		}
		if got.ItemLabel != "Movie (2024)" || got.ReleaseTitle != "Movie.2024.1080p-GRP" {
			t.Errorf("labels = %q/%q", got.ItemLabel, got.ReleaseTitle)
		}
		if got.DetailURL != "http://radarr.local/movie/movie-2024" {
			t.Errorf("DetailURL = %q", got.DetailURL)
		}
		if want := "/api/instances/" + instID.String() + "/mediacover/42/poster-250.jpg"; got.PosterURL != want {
			t.Errorf("PosterURL = %q, want %q", got.PosterURL, want)
		}
		if got.FromQuality != "HDTV-720p" || got.ToQuality != "Bluray-1080p" {
			t.Errorf("quality = %q -> %q", got.FromQuality, got.ToQuality)
		}
		if got.FromSize != 2_000_000_000 || got.ToSize != 8_000_000_000 {
			t.Errorf("size = %d -> %d", got.FromSize, got.ToSize)
		}
		if got.FromScore == nil || *got.FromScore != -50 || got.ToScore == nil || *got.ToScore != 1100 {
			t.Errorf("score = %v -> %v", got.FromScore, got.ToScore)
		}
		wantTags := []string{"1080p", "x265", "HDR10", "10-bit", "12 Mbps", "TrueHD 7.1"}
		if !slices.Equal(got.MediaTags, wantTags) {
			t.Errorf("MediaTags = %v, want %v", got.MediaTags, wantTags)
		}
		if !got.DetectedAt.Equal(created) {
			t.Errorf("DetectedAt = %v, want %v", got.DetectedAt, created)
		}
	})

	t.Run("legacy entry falls back to instance list", func(t *testing.T) {
		t.Parallel()
		entry := activity.Entry{
			InstanceID: &instID,
			CreatedAt:  created,
			Details: map[string]any{
				"itemLabel":      "Movie.2024.1080p-GRP",
				"itemDetailPath": "/movie/movie-2024",
				"quality":        "Bluray-1080p",
			},
		}

		got := homeUpgrade(entry, instMap)

		if got.InstanceName != "Radarr Main" || got.AppType != "Radarr" {
			t.Errorf("instance = %q/%q, want Radarr Main/Radarr", got.InstanceName, got.AppType)
		}
		if got.DetailURL != "http://radarr:7878/movie/movie-2024" {
			t.Errorf("DetailURL = %q", got.DetailURL)
		}
		if got.PosterURL != "" {
			t.Errorf("PosterURL = %q, want empty", got.PosterURL)
		}
		if got.FromQuality != "" || got.FromScore != nil || got.ToScore != nil {
			t.Errorf("unknown previous values should stay empty: %+v", got)
		}
		if len(got.MediaTags) != 0 {
			t.Errorf("MediaTags = %v, want none", got.MediaTags)
		}
	})

	t.Run("deleted instance", func(t *testing.T) {
		t.Parallel()
		entry := activity.Entry{
			CreatedAt: created,
			Details:   map[string]any{"releaseTitle": "Orphan.Release", "mediaCover": "1/poster-250.jpg"},
		}

		got := homeUpgrade(entry, instMap)

		if got.ItemLabel != "Orphan.Release" {
			t.Errorf("ItemLabel = %q, want release title fallback", got.ItemLabel)
		}
		if got.PosterURL != "" || got.DetailURL != "" {
			t.Errorf("links should be empty without an instance: %+v", got)
		}
	})
}
