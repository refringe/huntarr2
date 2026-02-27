package server

import (
	"testing"

	"github.com/google/uuid"

	"github.com/refringe/huntarr2/internal/activity"
	"github.com/refringe/huntarr2/internal/instance"
	"github.com/refringe/huntarr2/web/templates/pages"
)

func TestCapitalise(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"sonarr", "Sonarr"},
		{"radarr", "Radarr"},
		{"whisparr", "Whisparr"},
		{"a", "A"},
		{"", ""},
		{"ALREADY", "ALREADY"},
		{"lidarr", "Lidarr"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := capitalise(tt.input)
			if got != tt.want {
				t.Errorf("capitalise(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

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
