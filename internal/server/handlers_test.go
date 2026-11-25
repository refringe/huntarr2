package server

import (
	"context"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/refringe/huntarr2/internal/config"
	"github.com/refringe/huntarr2/internal/database/testdb"
	"github.com/refringe/huntarr2/internal/instance"
)

// newTestServer returns a Server backed by a migrated temporary database, listening on an ephemeral port.
func newTestServer(t *testing.T) *Server {
	t.Helper()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating encryption key: %v", err)
	}

	srv, err := New(&config.Config{EncryptionKey: key, SchedulerTickSecs: 30}, testdb.New(t), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func TestPagesRender(t *testing.T) {
	t.Parallel()

	arrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"appName":"Sonarr","version":"4.0.9.1"}`)) //nolint:errcheck // test helper
	}))
	t.Cleanup(arrServer.Close)

	srv := newTestServer(t)
	inst := &instance.Instance{
		Name:    "Living Room Sonarr",
		AppType: instance.AppTypeSonarr,
		BaseURL: arrServer.URL,
		APIKey:  "test-api-key",
	}
	if err := srv.instances.Create(t.Context(), inst); err != nil {
		t.Fatalf("creating instance: %v", err)
	}

	tests := []struct {
		path string
		want []string
	}{
		{"/", []string{"Living Room Sonarr", "4.0.9.1"}},
		{"/connections", []string{"Living Room Sonarr", arrServer.URL}},
		{"/settings", []string{"Living Room Sonarr"}},
		{"/logs", []string{"Living Room Sonarr"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", got)
			}
			body := rr.Body.String()
			for _, want := range tt.want {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			if strings.Contains(body, "test-api-key") {
				t.Error("body contains the instance API key")
			}
		})
	}
}

func TestRoutes(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/health", http.StatusOK},
		{http.MethodGet, "/api/instances", http.StatusOK},
		{http.MethodGet, "/api/scheduler/status", http.StatusOK},
		{http.MethodGet, "/static/js/settings.js", http.StatusOK},
		{http.MethodGet, "/no-such-page", http.StatusNotFound},
		{http.MethodPost, "/", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			srv.server.Handler.ServeHTTP(rr, req)

			if rr.Code != tt.want {
				t.Errorf("status = %d, want %d", rr.Code, tt.want)
			}
		})
	}
}

func TestServeStopsWhenContextIsCancelled(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- srv.serve(ctx) }()

	cancel()

	if err := <-done; err != nil {
		t.Errorf("serve returned %v, want nil after a clean shutdown", err)
	}
	if srv.scheduler.Status().Running {
		t.Error("scheduler still running after serve returned")
	}
}

func TestServeReportsListenFailure(t *testing.T) {
	t.Parallel()

	occupied := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(occupied.Close)

	srv := newTestServer(t)
	srv.server.Addr = occupied.Listener.Addr().String()

	if err := srv.serve(t.Context()); err == nil {
		t.Error("serve returned nil, want the listen error")
	}
	if srv.scheduler.Status().Running {
		t.Error("scheduler still running after serve returned")
	}
}
