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
	since := now.Add(-24 * time.Hour)

	// Delete events carry the per-app item ID and a reason field. The *arr
	// API returns PascalCase enum values (e.g. "Upgrade") via .ToString().
	deleteRecords := []map[string]any{
		{
			"id":        100,
			"date":      recent.Format(time.RFC3339Nano),
			"eventType": "movieFileDeleted",
			"movieId":   42,
			"data":      map[string]string{"reason": "Upgrade"},
		},
	}

	// Import events carry sourceTitle, quality, and the embedded entity.
	importRecords := []map[string]any{
		{
			"id":          1,
			"date":        recent.Format(time.RFC3339Nano),
			"eventType":   "downloadFolderImported",
			"sourceTitle": "Movie.2024.Bluray.1080p",
			"movieId":     42,
			"movie":       map[string]any{"titleSlug": "movie-2024"},
			"data":        map[string]string{},
			"quality":     map[string]any{"quality": map[string]any{"name": "Bluray-1080p"}},
		},
		{
			"id":          2,
			"date":        recent.Add(-10 * time.Minute).Format(time.RFC3339Nano),
			"eventType":   "downloadFolderImported",
			"sourceTitle": "Show.S01E01.HDTV",
			"movieId":     99,
			"movie":       map[string]any{"titleSlug": "show-s01e01"},
			"data":        map[string]string{},
			"quality":     map[string]any{"quality": map[string]any{"name": "HDTV-720p"}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/history" {
			t.Errorf("path = %q, want /api/v3/history", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		et := r.URL.Query().Get("eventType")
		switch et {
		case "6": // radarrFileDeleted (movieFileDeleted)
			json.NewEncoder(w).Encode(map[string]any{"records": deleteRecords}) //nolint:errcheck // test helper
		case "3": // radarrImported (downloadFolderImported)
			json.NewEncoder(w).Encode(map[string]any{"records": importRecords}) //nolint:errcheck // test helper
		default:
			t.Errorf("unexpected eventType = %q", et)
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", since, 50,
		radarrFileDeleted, []int{radarrImported}, "movieId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
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
	if got[0].DetailPath != "/movie/movie-2024" {
		t.Errorf("got[0].DetailPath = %q, want %q", got[0].DetailPath, "/movie/movie-2024")
	}

	if got[1].IsUpgrade {
		t.Error("got[1].IsUpgrade = true, want false")
	}
	if got[1].ItemLabel != "Show.S01E01.HDTV" {
		t.Errorf("got[1].ItemLabel = %q, want %q", got[1].ItemLabel, "Show.S01E01.HDTV")
	}
	if got[1].DetailPath != "/movie/show-s01e01" {
		t.Errorf("got[1].DetailPath = %q, want %q", got[1].DetailPath, "/movie/show-s01e01")
	}
}

func TestFetchArrHistoryOldRecordFiltered(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	since := now.Add(-24 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		et := r.URL.Query().Get("eventType")
		switch et {
		case "5": // sonarrFileDeleted
			json.NewEncoder(w).Encode(map[string]any{"records": []any{}}) //nolint:errcheck // test helper
		case "3": // sonarrImported
			//nolint:errcheck // test helper
			json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{
					"id":          3,
					"date":        old.Format(time.RFC3339Nano),
					"eventType":   "downloadFolderImported",
					"sourceTitle": "Old.Release",
					"episodeId":   1,
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "SDTV"}},
				},
			}})
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", since, 50,
		sonarrFileDeleted, []int{sonarrImported}, "episodeId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0 (old record should be filtered out)", len(got))
	}
}

func TestFetchArrHistoryEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"records":[]}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", time.Now().Add(-time.Hour), 50,
		radarrFileDeleted, []int{radarrImported}, "movieId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFetchArrHistoryDeleteFailureNonFatal(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour)
	since := now.Add(-24 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		et := r.URL.Query().Get("eventType")
		switch et {
		case "6": // radarrFileDeleted (movieFileDeleted)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("unsupported eventType")) //nolint:errcheck // test helper
		case "3": // radarrImported (downloadFolderImported)
			w.Header().Set("Content-Type", "application/json")
			//nolint:errcheck // test helper
			json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{
					"id":          1,
					"date":        recent.Format(time.RFC3339Nano),
					"eventType":   "downloadFolderImported",
					"sourceTitle": "Movie.2024.Bluray.1080p",
					"movieId":     42,
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "Bluray-1080p"}},
				},
			}})
		default:
			t.Errorf("unexpected eventType = %q", et)
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", since, 50,
		radarrFileDeleted, []int{radarrImported}, "movieId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (import should succeed despite delete failure)", len(got))
	}
	if got[0].IsUpgrade {
		t.Error("got[0].IsUpgrade = true, want false (no upgrade data available)")
	}
	if got[0].ItemLabel != "Movie.2024.Bluray.1080p" {
		t.Errorf("got[0].ItemLabel = %q, want %q", got[0].ItemLabel, "Movie.2024.Bluray.1080p")
	}
}

func TestFetchArrHistoryImportFailureFatal(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	_, err := fetchArrHistory(
		context.Background(), c, "v3", time.Now().Add(-time.Hour), 50,
		radarrFileDeleted, []int{radarrImported}, "movieId",
	)
	if err == nil {
		t.Fatal("expected error when import events fail, got nil")
	}
}

func TestFetchArrHistoryMultipleImportTypes(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour)
	since := now.Add(-24 * time.Hour)

	// Record 3 appears on both import pages; the dedup must keep one copy.
	overlap := map[string]any{
		"id":          3,
		"date":        recent.Add(-20 * time.Minute).Format(time.RFC3339Nano),
		"eventType":   "downloadFolderImported",
		"sourceTitle": "Overlap.Scene",
		"movieId":     7,
		"data":        map[string]string{},
		"quality":     map[string]any{"quality": map[string]any{"name": "WEBDL-1080p"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		et := r.URL.Query().Get("eventType")
		switch et {
		case "6": // whisparrV3FileDeleted (movieFileDeleted)
			//nolint:errcheck // test helper
			json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{
					"id":        100,
					"date":      recent.Format(time.RFC3339Nano),
					"eventType": "movieFileDeleted",
					"movieId":   42,
					"data":      map[string]string{"reason": "Upgrade"},
				},
			}})
		case "3": // whisparrV3Imported (downloadFolderImported)
			//nolint:errcheck // test helper
			json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{
					"id":          1,
					"date":        recent.Add(-10 * time.Minute).Format(time.RFC3339Nano),
					"eventType":   "downloadFolderImported",
					"sourceTitle": "Upgraded.Scene",
					"movieId":     42,
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "Bluray-1080p"}},
				},
				overlap,
			}})
		case "10": // whisparrV3DiskImported (diskScanImported)
			//nolint:errcheck // test helper
			json.NewEncoder(w).Encode(map[string]any{"records": []map[string]any{
				{
					"id":          2,
					"date":        recent.Format(time.RFC3339Nano),
					"eventType":   "diskScanImported",
					"sourceTitle": "DiskScan.Scene",
					"movieId":     77,
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "HDTV-720p"}},
				},
				overlap,
			}})
		default:
			t.Errorf("unexpected eventType = %q", et)
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", since, 50,
		whisparrV3FileDeleted, []int{whisparrV3Imported, whisparrV3DiskImported}, "movieId",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (overlapping record deduplicated)", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Date.After(got[i-1].Date) {
			t.Errorf("records not sorted newest first: got[%d]=%v after got[%d]=%v",
				i, got[i].Date, i-1, got[i-1].Date)
		}
	}
	byID := make(map[int]HistoryRecord, len(got))
	for _, r := range got {
		byID[r.ID] = r
	}
	if !byID[1].IsUpgrade {
		t.Error("record 1 IsUpgrade = false, want true (delete cross-reference)")
	}
	if byID[2].IsUpgrade {
		t.Error("record 2 IsUpgrade = true, want false")
	}
}

func TestFetchArrHistoryMultipleImportOnePageFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		et := r.URL.Query().Get("eventType")
		if et == "10" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error")) //nolint:errcheck // test helper
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"records":[]}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	_, err := fetchArrHistory(
		context.Background(), c, "v3", time.Now().Add(-time.Hour), 50,
		whisparrV3FileDeleted, []int{whisparrV3Imported, whisparrV3DiskImported}, "movieId",
	)
	if err == nil {
		t.Fatal("expected error when one import page fails, got nil")
	}
}

func TestHistoryRecordDetailPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  historyRecordResponse
		want string
	}{
		{
			name: "movie entity",
			rec:  historyRecordResponse{Movie: &historyEntityRef{TitleSlug: "the-dark-knight-2008"}},
			want: "/movie/the-dark-knight-2008",
		},
		{
			name: "series entity",
			rec:  historyRecordResponse{Series: &historyEntityRef{TitleSlug: "breaking-bad"}},
			want: "/series/breaking-bad",
		},
		{
			name: "artist entity",
			rec:  historyRecordResponse{Artist: &historyEntityRef{TitleSlug: "the-beatles"}},
			want: "/artist/the-beatles",
		},
		{
			name: "no entity",
			rec:  historyRecordResponse{},
			want: "",
		},
		{
			name: "empty slug",
			rec:  historyRecordResponse{Movie: &historyEntityRef{TitleSlug: ""}},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rec.detailPath()
			if got != tt.want {
				t.Errorf("detailPath() = %q, want %q", got, tt.want)
			}
		})
	}
}
