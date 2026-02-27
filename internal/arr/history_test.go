package arr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchArrHistory(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour)
	old := now.Add(-48 * time.Hour)
	since := now.Add(-24 * time.Hour)

	records := []map[string]any{
		{
			"id":          1,
			"date":        recent.Format(time.RFC3339Nano),
			"eventType":   "downloadFolderImported",
			"sourceTitle": "Movie.2024.Bluray.1080p",
			"data":        map[string]string{"reason": "upgrade"},
			"quality":     map[string]any{"quality": map[string]any{"name": "Bluray-1080p"}},
		},
		{
			"id":          2,
			"date":        recent.Add(-10 * time.Minute).Format(time.RFC3339Nano),
			"eventType":   "downloadFolderImported",
			"sourceTitle": "Show.S01E01.HDTV",
			"data":        map[string]string{},
			"quality":     map[string]any{"quality": map[string]any{"name": "HDTV-720p"}},
		},
		{
			"id":          3,
			"date":        old.Format(time.RFC3339Nano),
			"eventType":   "downloadFolderImported",
			"sourceTitle": "Old.Release",
			"data":        map[string]string{},
			"quality":     map[string]any{"quality": map[string]any{"name": "SDTV"}},
		},
	}

	body := map[string]any{"records": records}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/history" {
			t.Errorf("path = %q, want /api/v3/history", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("eventType") != "downloadFolderImported" {
			t.Errorf("eventType = %q, want downloadFolderImported", q.Get("eventType"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	client := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(context.Background(), client, "v3", since, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (old record should be filtered out)", len(got))
	}

	if !got[0].IsUpgrade {
		t.Error("got[0].IsUpgrade = false, want true")
	}
	if got[0].ItemLabel != "Movie.2024.Bluray.1080p" {
		t.Errorf("got[0].ItemLabel = %q, want %q", got[0].ItemLabel, "Movie.2024.Bluray.1080p")
	}
	if got[0].Quality != "Bluray-1080p" {
		t.Errorf("got[0].Quality = %q, want %q", got[0].Quality, "Bluray-1080p")
	}

	if got[1].IsUpgrade {
		t.Error("got[1].IsUpgrade = true, want false")
	}
	if got[1].ItemLabel != "Show.S01E01.HDTV" {
		t.Errorf("got[1].ItemLabel = %q, want %q", got[1].ItemLabel, "Show.S01E01.HDTV")
	}
}

func TestFetchArrHistoryEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"records":[]}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	client := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(context.Background(), client, "v3", time.Now().Add(-time.Hour), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFetchArrHistoryError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	client := newClient(srv.URL, "key", 5*time.Second)
	_, err := fetchArrHistory(context.Background(), client, "v3", time.Now().Add(-time.Hour), 50)
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}
