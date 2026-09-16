package arr

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
	"uuid"

	"github.com/refringe/huntarr2/internal/instance"
)

type fakeRepository struct {
	instances []instance.Instance
}

func (f *fakeRepository) List(_ context.Context) ([]instance.Instance, error) {
	return f.instances, nil
}

func (f *fakeRepository) ListByType(_ context.Context, appType instance.AppType) ([]instance.Instance, error) {
	var out []instance.Instance
	for _, inst := range f.instances {
		if inst.AppType == appType {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (f *fakeRepository) Get(_ context.Context, id uuid.UUID) (instance.Instance, error) {
	for _, inst := range f.instances {
		if inst.ID == id {
			return inst, nil
		}
	}
	return instance.Instance{}, instance.ErrNotFound
}

func (f *fakeRepository) Create(_ context.Context, inst *instance.Instance) error {
	if inst.ID == uuid.Nil() {
		inst.ID = uuid.New()
	}
	f.instances = append(f.instances, *inst)
	return nil
}

func (f *fakeRepository) Update(_ context.Context, inst *instance.Instance) error {
	for i, existing := range f.instances {
		if existing.ID == inst.ID {
			f.instances[i] = *inst
			return nil
		}
	}
	return instance.ErrNotFound
}

func (f *fakeRepository) Delete(_ context.Context, id uuid.UUID) error {
	for i, inst := range f.instances {
		if inst.ID == id {
			f.instances = append(f.instances[:i], f.instances[i+1:]...)
			return nil
		}
	}
	return instance.ErrNotFound
}

func TestStatusNoInstances(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepository{})
	statuses, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("len = %d, want 0", len(statuses))
	}
}

func TestStatusSingleSonarr(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"appName":"Sonarr","version":"4.0.0.700"}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	id := uuid.New()
	repo := &fakeRepository{
		instances: []instance.Instance{
			{
				ID:        id,
				Name:      "My Sonarr",
				AppType:   instance.AppTypeSonarr,
				BaseURL:   srv.URL,
				APIKey:    "testkey",
				TimeoutMs: 5000,
			},
		},
	}

	svc := NewService(repo)
	statuses, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if !statuses[0].Connected {
		t.Error("expected Connected = true")
	}
	if statuses[0].Version != "4.0.0.700" {
		t.Errorf("Version = %q, want %q", statuses[0].Version, "4.0.0.700")
	}
	if statuses[0].AppType != "sonarr" {
		t.Errorf("AppType = %q, want %q", statuses[0].AppType, "sonarr")
	}
}

func TestStatusUnreachableInstance(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	repo := &fakeRepository{
		instances: []instance.Instance{
			{
				ID:        id,
				Name:      "Dead Sonarr",
				AppType:   instance.AppTypeSonarr,
				BaseURL:   "http://127.0.0.1:1",
				APIKey:    "testkey",
				TimeoutMs: 100,
			},
		},
	}

	svc := NewService(repo)
	statuses, err := svc.Status(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("len = %d, want 1", len(statuses))
	}
	if statuses[0].Connected {
		t.Error("expected Connected = false for unreachable instance")
	}
}

func TestTestConnectionSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"appName":"Sonarr","version":"4.0.0.700"}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	svc := NewService(&fakeRepository{})
	err := svc.TestConnection(context.Background(), instance.AppTypeSonarr, srv.URL, "testkey", 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepository{})
	err := svc.TestConnection(context.Background(), instance.AppTypeSonarr, "http://127.0.0.1:1", "testkey", 100)
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

func TestTestConnectionUnsupportedType(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepository{})
	err := svc.TestConnection(context.Background(), instance.AppType("unknown"), "http://localhost:9999", "key", 5000)
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
}

func TestUpgradeable(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"HD","upgradeAllowed":true,"cutoff":7,` + //nolint:errcheck // test helper
			`"items":[` +
			`{"quality":{"id":1,"name":"SDTV"},"allowed":true,"items":[]},` +
			`{"quality":{"id":4,"name":"HDTV-720p"},"allowed":true,"items":[]},` +
			`{"quality":{"id":7,"name":"Bluray-1080p"},"allowed":true,"items":[]}` +
			`]}]`))
	})
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":1,"title":"Below Max","year":2020,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":4}}}},` +
			`{"id":2,"title":"At Max","year":2019,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":7}}}}` +
			`]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	id := uuid.New()
	repo := &fakeRepository{
		instances: []instance.Instance{
			{ID: id, Name: "Radarr", AppType: instance.AppTypeRadarr, BaseURL: srv.URL, APIKey: "key", TimeoutMs: 5000},
		},
	}

	svc := NewService(repo)
	result, err := svc.Upgradeable(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("len = %d, want 1", len(result.Items))
	}
	if result.Items[0].ID != 1 {
		t.Errorf("ID = %d, want 1", result.Items[0].ID)
	}
	if result.Items[0].Label != "Below Max (2020)" {
		t.Errorf("Label = %q, want %q", result.Items[0].Label, "Below Max (2020)")
	}
	if result.Stats.LibraryTotal != 2 {
		t.Errorf("Stats.LibraryTotal = %d, want 2", result.Stats.LibraryTotal)
	}
}

func TestSearchCycleHappyPath(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"HD","upgradeAllowed":true,"cutoff":7,` + //nolint:errcheck // test helper
			`"items":[` +
			`{"quality":{"id":1,"name":"SDTV"},"allowed":true,"items":[]},` +
			`{"quality":{"id":4,"name":"HDTV-720p"},"allowed":true,"items":[]},` +
			`{"quality":{"id":7,"name":"Bluray-1080p"},"allowed":true,"items":[]}` +
			`]}]`))
	})
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":101,"title":"Movie A","year":2020,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":1}}}},` +
			`{"id":102,"title":"Movie B","year":2019,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":4}}}}` +
			`]`))
	})
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":99}`)) //nolint:errcheck // test helper
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	id := uuid.New()
	repo := &fakeRepository{
		instances: []instance.Instance{
			{ID: id, Name: "Radarr", AppType: instance.AppTypeRadarr, BaseURL: srv.URL, APIKey: "key", TimeoutMs: 5000},
		},
	}

	svc := NewService(repo)
	searched, err := svc.SearchCycle(context.Background(), id, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if searched != 2 {
		t.Errorf("searched = %d, want 2", searched)
	}
}

func TestSearchCycleZeroItems(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1,"name":"HD","upgradeAllowed":true,"cutoff":7,` + //nolint:errcheck // test helper
			`"items":[{"quality":{"id":7,"name":"Bluray-1080p"},"allowed":true,"items":[]}]}]`))
	})
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":1,"title":"Movie","year":2020,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":7}}}}` +
			`]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	id := uuid.New()
	repo := &fakeRepository{
		instances: []instance.Instance{
			{ID: id, Name: "Radarr", AppType: instance.AppTypeRadarr, BaseURL: srv.URL, APIKey: "key", TimeoutMs: 5000},
		},
	}

	svc := NewService(repo)
	searched, err := svc.SearchCycle(context.Background(), id, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if searched != 0 {
		t.Errorf("searched = %d, want 0", searched)
	}
}

func TestSearchCycleInstanceNotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(&fakeRepository{})
	_, err := svc.SearchCycle(context.Background(), uuid.New(), 50)
	if err == nil {
		t.Fatal("expected error for missing instance, got nil")
	}
}

func TestTestConnectionVersionCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		appType      instance.AppType
		version      string
		wantMismatch bool
	}{
		{"whisparr-v3 against v2 server", instance.AppTypeWhisparrV3, "2.0.0.548", true},
		{"whisparr-v3 against v3 server", instance.AppTypeWhisparrV3, "3.3.8.7878", false},
		{"whisparr-v2 against v2 server", instance.AppTypeWhisparrV2, "2.0.0.548", false},
		{"whisparr-v2 against v3 server", instance.AppTypeWhisparrV2, "3.3.8.7878", true},
		{"unparsable version skips the check", instance.AppTypeWhisparrV3, "nightly", false},
		{"sonarr accepts any version", instance.AppTypeSonarr, "99.0.0.1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"appName":"Whisparr","version":"` + tc.version + `"}`)) //nolint:errcheck // test helper
			}))
			defer srv.Close()

			svc := NewService(&fakeRepository{})
			err := svc.TestConnection(context.Background(), tc.appType, srv.URL, "key", 5000)
			if tc.wantMismatch {
				if !errors.Is(err, ErrVersionMismatch) {
					t.Fatalf("err = %v, want ErrVersionMismatch", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestMajorVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    int
	}{
		{"3.3.8.7878", 3},
		{"2.0.0.548", 2},
		{"10.1", 10},
		{"", 0},
		{"beta", 0},
		{"-1.0", 0},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()

			if got := majorVersion(tc.version); got != tc.want {
				t.Errorf("majorVersion(%q) = %d, want %d", tc.version, got, tc.want)
			}
		})
	}
}

func TestUpgradeableSlowLibraryUnderBudgetSucceeds(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/qualityprofile", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`)) //nolint:errcheck // test helper
	})
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		// Slower than the instance's per-request timeout, which must not bound the library request class.
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`)) //nolint:errcheck // test helper
	})
	synctest.Test(t, func(t *testing.T) {
		srv := httptest.NewTestServer(t, mux)
		transport := srv.Client().Transport

		id := uuid.New()
		repo := &fakeRepository{
			instances: []instance.Instance{
				{
					ID:        id,
					Name:      "Slow Radarr",
					AppType:   instance.AppTypeRadarr,
					BaseURL:   srv.URL,
					APIKey:    "testkey",
					TimeoutMs: 100,
				},
			},
		}

		svc := NewService(repo)
		svc.newApp = func(appType instance.AppType, baseURL, apiKey string, timeout time.Duration) (App, error) {
			app, err := NewApp(appType, baseURL, apiKey, timeout)
			if err == nil {
				app.(*adapter).client.httpClient.Transport = transport
			}
			return app, err
		}
		result, err := svc.Upgradeable(context.Background(), id)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Stats.LibraryTotal != 0 {
			t.Errorf("LibraryTotal = %d, want 0", result.Stats.LibraryTotal)
		}
	})
}
