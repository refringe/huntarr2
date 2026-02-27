package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/refringe/huntarr2/internal/instance"
)

func TestAdapterStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType instance.AppType
		apiVer  string
		resp    string
		want    SystemStatus
	}{
		{
			name:    "sonarr",
			appType: instance.AppTypeSonarr,
			apiVer:  "v3",
			resp:    `{"appName":"Sonarr","version":"4.0.0.700"}`,
			want:    SystemStatus{AppName: "Sonarr", Version: "4.0.0.700"},
		},
		{
			name:    "radarr",
			appType: instance.AppTypeRadarr,
			apiVer:  "v3",
			resp:    `{"appName":"Radarr","version":"5.2.0.8171"}`,
			want:    SystemStatus{AppName: "Radarr", Version: "5.2.0.8171"},
		},
		{
			name:    "lidarr",
			appType: instance.AppTypeLidarr,
			apiVer:  "v1",
			resp:    `{"appName":"Lidarr","version":"2.1.0.3901"}`,
			want:    SystemStatus{AppName: "Lidarr", Version: "2.1.0.3901"},
		},
		{
			name:    "whisparr",
			appType: instance.AppTypeWhisparr,
			apiVer:  "v3",
			resp:    `{"appName":"Whisparr","version":"3.0.0.100"}`,
			want:    SystemStatus{AppName: "Whisparr", Version: "3.0.0.100"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantPath := "/api/" + tc.apiVer + "/system/status"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tc.resp)) //nolint:errcheck // test helper
			}))
			defer srv.Close()

			app, err := NewApp(tc.appType, srv.URL, "key", 5*time.Second)
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			status, err := app.Status(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status != tc.want {
				t.Errorf("status = %+v, want %+v", status, tc.want)
			}
		})
	}
}

func TestAdapterQualityProfiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType instance.AppType
		apiVer  string
	}{
		{"sonarr", instance.AppTypeSonarr, "v3"},
		{"radarr", instance.AppTypeRadarr, "v3"},
		{"lidarr", instance.AppTypeLidarr, "v1"},
		{"whisparr", instance.AppTypeWhisparr, "v3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantPath := "/api/" + tc.apiVer + "/qualityprofile"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`[` + //nolint:errcheck // test helper
					`{"id":1,"name":"HD-1080p","upgradeAllowed":true,"cutoff":7,` +
					`"items":[` +
					`{"quality":{"id":1,"name":"SDTV"},"allowed":true,"items":[]},` +
					`{"quality":{"id":4,"name":"HDTV-720p"},"allowed":true,"items":[]},` +
					`{"quality":{"id":7,"name":"Bluray-1080p"},"allowed":true,"items":[]}` +
					`]},` +
					`{"id":2,"name":"Ultra-HD","upgradeAllowed":false,"cutoff":15,` +
					`"items":[` +
					`{"quality":{"id":15,"name":"Bluray-2160p"},"allowed":true,"items":[]}` +
					`]}` +
					`]`))
			}))
			defer srv.Close()

			app, err := NewApp(tc.appType, srv.URL, "key", 5*time.Second)
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			profiles, err := app.QualityProfiles(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(profiles) != 2 {
				t.Fatalf("len = %d, want 2", len(profiles))
			}
			if profiles[0].Name != "HD-1080p" {
				t.Errorf("Name = %q, want %q", profiles[0].Name, "HD-1080p")
			}
			if !profiles[0].UpgradeAllowed {
				t.Error("expected UpgradeAllowed = true for first profile")
			}
			if profiles[1].UpgradeAllowed {
				t.Error("expected UpgradeAllowed = false for second profile")
			}
		})
	}
}

func TestAdapterQualityProfileItems(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{` + //nolint:errcheck // test helper
			`"id":1,"name":"Test","upgradeAllowed":true,"cutoff":7,` +
			`"items":[` +
			`{"quality":{"id":1,"name":"SDTV"},"allowed":true,"items":[]},` +
			`{"quality":null,"name":"HD","allowed":true,"items":[` +
			`{"quality":{"id":4,"name":"HDTV-720p"},"allowed":false,"items":[]},` +
			`{"quality":{"id":9,"name":"HDTV-1080p"},"allowed":false,"items":[]}` +
			`]},` +
			`{"quality":{"id":7,"name":"Bluray-1080p"},"allowed":false,"items":[]}` +
			`]}]`))
	}))
	defer srv.Close()

	app, err := NewApp(instance.AppTypeSonarr, srv.URL, "key", 5*time.Second)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	profiles, err := app.QualityProfiles(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("len = %d, want 1", len(profiles))
	}

	items := profiles[0].Items
	if len(items) != 3 {
		t.Fatalf("len(Items) = %d, want 3", len(items))
	}

	if items[0].Quality == nil || items[0].Quality.ID != 1 {
		t.Error("items[0] should be SDTV with ID 1")
	}
	if !items[0].Allowed {
		t.Error("items[0] should be allowed")
	}

	if items[1].Quality != nil {
		t.Error("items[1] (group) should have nil Quality")
	}
	if items[1].Name != "HD" {
		t.Errorf("items[1].Name = %q, want %q", items[1].Name, "HD")
	}
	if len(items[1].Items) != 2 {
		t.Fatalf("len(items[1].Items) = %d, want 2", len(items[1].Items))
	}
	if items[1].Items[0].Quality == nil || items[1].Items[0].Quality.ID != 4 {
		t.Error("items[1].Items[0] should be HDTV-720p with ID 4")
	}

	if items[2].Quality == nil || items[2].Quality.ID != 7 {
		t.Error("items[2] should be Bluray-1080p with ID 7")
	}
}

func TestAdapterLibraryItems(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/movie" {
			w.Write([]byte(`[{"id":1,"title":"Test","year":2020,` + //nolint:errcheck // test helper
				`"qualityProfileId":1,"hasFile":true,"monitored":true,` +
				`"movieFile":{"quality":{"quality":{"id":4}}}}]`))
		}
	}))
	defer srv.Close()

	app, err := NewApp(instance.AppTypeRadarr, srv.URL, "key", 5*time.Second)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	items, err := app.LibraryItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len = %d, want 1", len(items))
	}
	if items[0].ID != 1 {
		t.Errorf("ID = %d, want 1", items[0].ID)
	}
	if len(items[0].CurrentQualityIDs) != 1 || items[0].CurrentQualityIDs[0] != 4 {
		t.Errorf("CurrentQualityIDs = %v, want [4]", items[0].CurrentQualityIDs)
	}
}

func TestAdapterSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		appType   instance.AppType
		apiVer    string
		wantCmd   string
		wantField string
	}{
		{"sonarr", instance.AppTypeSonarr, "v3", "EpisodeSearch", "episodeIds"},
		{"radarr", instance.AppTypeRadarr, "v3", "MoviesSearch", "movieIds"},
		{"lidarr", instance.AppTypeLidarr, "v1", "AlbumSearch", "albumIds"},
		{"whisparr", instance.AppTypeWhisparr, "v3", "EpisodeSearch", "episodeIds"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wantPath := "/api/" + tc.apiVer + "/command"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
				}
				if r.Method != http.MethodPost {
					t.Errorf("method = %q, want POST", r.Method)
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("reading body: %v", err)
				}
				var cmd map[string]json.RawMessage
				if err := json.Unmarshal(body, &cmd); err != nil {
					t.Fatalf("unmarshalling body: %v", err)
				}

				var cmdName string
				if err := json.Unmarshal(cmd["name"], &cmdName); err != nil {
					t.Fatalf("unmarshalling name: %v", err)
				}
				if cmdName != tc.wantCmd {
					t.Errorf("command name = %q, want %q", cmdName, tc.wantCmd)
				}

				if _, ok := cmd[tc.wantField]; !ok {
					t.Errorf("missing field %q in command body", tc.wantField)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{"id":55}`)) //nolint:errcheck // test helper
			}))
			defer srv.Close()

			app, err := NewApp(tc.appType, srv.URL, "key", 5*time.Second)
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			result, err := app.Search(context.Background(), []int{101, 102})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.CommandID != 55 {
				t.Errorf("CommandID = %d, want 55", result.CommandID)
			}
		})
	}
}

func TestAdapterSearchEmptyIDs(t *testing.T) {
	t.Parallel()

	for _, appType := range []instance.AppType{
		instance.AppTypeSonarr,
		instance.AppTypeRadarr,
		instance.AppTypeLidarr,
		instance.AppTypeWhisparr,
	} {
		t.Run(string(appType), func(t *testing.T) {
			t.Parallel()
			app, err := NewApp(appType, "http://localhost", "key", 5*time.Second)
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}
			if _, err := app.Search(context.Background(), []int{}); err == nil {
				t.Fatal("expected error for empty IDs, got nil")
			}
		})
	}
}

func TestAdapterHistory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		appType instance.AppType
		apiVer  string
	}{
		{"sonarr", instance.AppTypeSonarr, "v3"},
		{"radarr", instance.AppTypeRadarr, "v3"},
		{"lidarr", instance.AppTypeLidarr, "v1"},
		{"whisparr", instance.AppTypeWhisparr, "v3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			recent := time.Now().UTC().Add(-30 * time.Minute)
			wantPath := "/api/" + tc.apiVer + "/history"
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
				}
				resp := map[string]any{
					"records": []map[string]any{
						{
							"id":          1,
							"date":        recent.Format(time.RFC3339Nano),
							"eventType":   "downloadFolderImported",
							"sourceTitle": "Test.Release",
							"data":        map[string]string{"reason": "upgrade"},
							"quality":     map[string]any{"quality": map[string]any{"name": "HD"}},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
			}))
			defer srv.Close()

			app, err := NewApp(tc.appType, srv.URL, "key", 5*time.Second)
			if err != nil {
				t.Fatalf("NewApp: %v", err)
			}

			since := time.Now().Add(-time.Hour)
			records, err := app.History(context.Background(), since, 50)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(records) != 1 {
				t.Fatalf("len = %d, want 1", len(records))
			}
			if !records[0].IsUpgrade {
				t.Error("IsUpgrade = false, want true")
			}
			if records[0].ItemLabel != "Test.Release" {
				t.Errorf("ItemLabel = %q, want %q", records[0].ItemLabel, "Test.Release")
			}
		})
	}
}
