package arr

import "testing"

func TestQualityRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile QualityProfile
		want    map[int]int
	}{
		{
			name: "simple flat profile",
			profile: QualityProfile{
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1, Name: "SDTV"}, Allowed: true},
					{Quality: &QualityLevel{ID: 4, Name: "HDTV-720p"}, Allowed: true},
					{Quality: &QualityLevel{ID: 7, Name: "Bluray-1080p"}, Allowed: true},
				},
			},
			want: map[int]int{1: 0, 4: 1, 7: 2},
		},
		{
			name: "group shares rank",
			profile: QualityProfile{
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1, Name: "SDTV"}, Allowed: true},
					{
						Name:    "HD",
						Allowed: true,
						Items: []ProfileEntry{
							{Quality: &QualityLevel{ID: 4, Name: "HDTV-720p"}},
							{Quality: &QualityLevel{ID: 9, Name: "HDTV-1080p"}},
						},
					},
					{Quality: &QualityLevel{ID: 7, Name: "Bluray-1080p"}, Allowed: true},
				},
			},
			want: map[int]int{1: 0, 4: 1, 9: 1, 7: 2},
		},
		{
			name: "disallowed entries excluded",
			profile: QualityProfile{
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1, Name: "SDTV"}, Allowed: false},
					{Quality: &QualityLevel{ID: 4, Name: "HDTV-720p"}, Allowed: true},
				},
			},
			want: map[int]int{4: 1},
		},
		{
			name:    "empty profile",
			profile: QualityProfile{},
			want:    map[int]int{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := qualityRank(tc.profile)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for id, wantRank := range tc.want {
				if gotRank, ok := got[id]; !ok {
					t.Errorf("missing quality ID %d", id)
				} else if gotRank != wantRank {
					t.Errorf("rank[%d] = %d, want %d", id, gotRank, wantRank)
				}
			}
		})
	}
}

func TestCutoffRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile QualityProfile
		want    int
	}{
		{
			name: "cutoff matches allowed entry",
			profile: QualityProfile{
				Cutoff: 7,
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1}, Allowed: true},
					{Quality: &QualityLevel{ID: 4}, Allowed: true},
					{Quality: &QualityLevel{ID: 7}, Allowed: true},
				},
			},
			want: 2,
		},
		{
			name: "cutoff matches middle entry",
			profile: QualityProfile{
				Cutoff: 4,
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1}, Allowed: true},
					{Quality: &QualityLevel{ID: 4}, Allowed: true},
					{Quality: &QualityLevel{ID: 7}, Allowed: true},
				},
			},
			want: 1,
		},
		{
			name: "cutoff not in allowed entries",
			profile: QualityProfile{
				Cutoff: 99,
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 1}, Allowed: true},
					{Quality: &QualityLevel{ID: 4}, Allowed: true},
				},
			},
			want: -1,
		},
		{
			name:    "empty profile",
			profile: QualityProfile{Cutoff: 7},
			want:    -1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ranks := qualityRank(tc.profile)
			got := cutoffRank(tc.profile, ranks)
			if got != tc.want {
				t.Errorf("cutoffRank = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFilterUpgradeable(t *testing.T) {
	t.Parallel()

	profile := QualityProfile{
		ID:             1,
		UpgradeAllowed: true,
		Cutoff:         7,
		Items: []ProfileEntry{
			{Quality: &QualityLevel{ID: 1, Name: "SDTV"}, Allowed: true},
			{Quality: &QualityLevel{ID: 4, Name: "HDTV-720p"}, Allowed: true},
			{Quality: &QualityLevel{ID: 7, Name: "Bluray-1080p"}, Allowed: true},
		},
	}
	profiles := map[int]QualityProfile{1: profile}

	tests := []struct {
		name  string
		items []LibraryItem
		profs map[int]QualityProfile
		want  int
	}{
		{
			name: "item below cutoff is upgradeable",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: true},
			},
			profs: profiles,
			want:  1,
		},
		{
			name: "item at cutoff is not upgradeable",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 1, CurrentQualityIDs: []int{7}, HasFile: true, Monitored: true},
			},
			profs: profiles,
			want:  0,
		},
		{
			name: "item without file skipped",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: false, Monitored: true},
			},
			profs: profiles,
			want:  0,
		},
		{
			name: "unmonitored item skipped",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: false},
			},
			profs: profiles,
			want:  0,
		},
		{
			name: "unknown quality ID skipped",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 1, CurrentQualityIDs: []int{999}, HasFile: true, Monitored: true},
			},
			profs: profiles,
			want:  0,
		},
		{
			name: "upgrade not allowed skipped",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 2, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: true},
			},
			profs: map[int]QualityProfile{
				2: {
					ID:             2,
					UpgradeAllowed: false,
					Cutoff:         7,
					Items: []ProfileEntry{
						{Quality: &QualityLevel{ID: 1}, Allowed: true},
						{Quality: &QualityLevel{ID: 7}, Allowed: true},
					},
				},
			},
			want: 0,
		},
		{
			name: "unknown profile skipped",
			items: []LibraryItem{
				{ID: 1, Label: "Movie A", QualityProfileID: 99, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: true},
			},
			profs: profiles,
			want:  0,
		},
		{
			name: "mixed items filtered correctly",
			items: []LibraryItem{
				{ID: 1, Label: "At Cutoff", QualityProfileID: 1, CurrentQualityIDs: []int{7}, HasFile: true, Monitored: true},
				{ID: 2, Label: "Below Cutoff", QualityProfileID: 1, CurrentQualityIDs: []int{4}, HasFile: true, Monitored: true},
				{ID: 3, Label: "No File", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: false, Monitored: true},
				{ID: 4, Label: "Lowest", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: true},
			},
			profs: profiles,
			want:  2,
		},
		{
			name:  "empty items returns nil",
			items: nil,
			profs: profiles,
			want:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, stats := filterUpgradeable(tc.items, tc.profs)
			if len(got) != tc.want {
				t.Errorf("len = %d, want %d", len(got), tc.want)
			}
			if stats.Upgradeable != tc.want {
				t.Errorf("stats.Upgradeable = %d, want %d",
					stats.Upgradeable, tc.want)
			}
			if stats.LibraryTotal != len(tc.items) {
				t.Errorf("stats.LibraryTotal = %d, want %d",
					stats.LibraryTotal, len(tc.items))
			}
		})
	}
}

func TestFilterUpgradeableWithGroups(t *testing.T) {
	t.Parallel()

	profile := QualityProfile{
		ID:             1,
		UpgradeAllowed: true,
		Cutoff:         7,
		Items: []ProfileEntry{
			{Quality: &QualityLevel{ID: 1, Name: "SDTV"}, Allowed: true},
			{
				Name:    "HD",
				Allowed: true,
				Items: []ProfileEntry{
					{Quality: &QualityLevel{ID: 4, Name: "HDTV-720p"}},
					{Quality: &QualityLevel{ID: 9, Name: "HDTV-1080p"}},
				},
			},
			{Quality: &QualityLevel{ID: 7, Name: "Bluray-1080p"}, Allowed: true},
		},
	}
	profiles := map[int]QualityProfile{1: profile}

	items := []LibraryItem{
		{ID: 1, Label: "At HD group", QualityProfileID: 1, CurrentQualityIDs: []int{4}, HasFile: true, Monitored: true},
		{ID: 2, Label: "At cutoff", QualityProfileID: 1, CurrentQualityIDs: []int{7}, HasFile: true, Monitored: true},
		{ID: 3, Label: "At SD", QualityProfileID: 1, CurrentQualityIDs: []int{1}, HasFile: true, Monitored: true},
	}

	got, _ := filterUpgradeable(items, profiles)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	ids := map[int]struct{}{}
	for _, item := range got {
		ids[item.ID] = struct{}{}
	}
	if _, ok := ids[1]; !ok {
		t.Error("expected item 1 (HD group, below cutoff) to be upgradeable")
	}
	if _, ok := ids[3]; !ok {
		t.Error("expected item 3 (SD, below cutoff) to be upgradeable")
	}
}
