package arr

import (
	"context"
	"encoding/json/v2"
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

	// Delete events carry the per-app item ID and a PascalCase reason value such as "Upgrade".
	deleteRecords := []map[string]any{
		{
			"id":                100,
			"date":              recent.Format(time.RFC3339Nano),
			"eventType":         "movieFileDeleted",
			"movieId":           42,
			"data":              map[string]string{"reason": "Upgrade", "size": "2000000000"},
			"quality":           map[string]any{"quality": map[string]any{"name": "HDTV-720p"}},
			"customFormatScore": 450,
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
			"movie": map[string]any{
				"id":        42,
				"titleSlug": "movie-2024",
				"title":     "Movie",
				"year":      2024,
				"images":    []map[string]any{{"coverType": "poster", "url": "/MediaCover/42/poster.jpg"}},
			},
			"data": map[string]string{
				"fileId":       "7",
				"size":         "8000000000",
				"releaseGroup": "GRP",
			},
			"quality":           map[string]any{"quality": map[string]any{"name": "Bluray-1080p"}},
			"customFormatScore": 1100,
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
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v3/moviefile/7" {
			//nolint:errcheck // test helper
			json.MarshalWrite(w, map[string]any{
				"id":   7,
				"size": 8000000000,
				"mediaInfo": map[string]any{
					"resolution":            "1920x1080",
					"videoCodec":            "x265",
					"videoBitDepth":         10,
					"videoBitrate":          12000000,
					"videoDynamicRangeType": "HDR10",
					"audioCodec":            "TrueHD",
					"audioChannels":         7.1,
					"audioBitrate":          4500000,
					"runTime":               "2:32:00",
				},
			})
			return
		}
		if r.URL.Path != "/api/v3/history" {
			t.Errorf("path = %q, want /api/v3/history", r.URL.Path)
		}
		if r.URL.Query().Get("includeMovie") != "true" {
			t.Errorf("includeMovie = %q, want true", r.URL.Query().Get("includeMovie"))
		}
		et := r.URL.Query().Get("eventType")
		switch et {
		case "6": // radarrFileDeleted (movieFileDeleted)
			json.MarshalWrite(w, map[string]any{"records": deleteRecords}) //nolint:errcheck // test helper
		case "3": // radarrImported (downloadFolderImported)
			json.MarshalWrite(w, map[string]any{"records": importRecords}) //nolint:errcheck // test helper
		default:
			t.Errorf("unexpected eventType = %q", et)
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(
		context.Background(), c, "v3", since, 50,
		radarrHistory,
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
	if got[0].ItemID != 42 {
		t.Errorf("got[0].ItemID = %d, want 42", got[0].ItemID)
	}
	if got[0].ItemLabel != "Movie (2024)" {
		t.Errorf("got[0].ItemLabel = %q, want %q", got[0].ItemLabel, "Movie (2024)")
	}
	if got[0].ReleaseTitle != "Movie.2024.Bluray.1080p" {
		t.Errorf("got[0].ReleaseTitle = %q, want %q", got[0].ReleaseTitle, "Movie.2024.Bluray.1080p")
	}
	if got[0].Quality != "Bluray-1080p" {
		t.Errorf("got[0].Quality = %q, want %q", got[0].Quality, "Bluray-1080p")
	}
	if got[0].PreviousQuality != "HDTV-720p" {
		t.Errorf("got[0].PreviousQuality = %q, want %q", got[0].PreviousQuality, "HDTV-720p")
	}
	if got[0].Size != 8000000000 || got[0].PreviousSize != 2000000000 {
		t.Errorf("sizes = %d/%d, want 8000000000/2000000000", got[0].Size, got[0].PreviousSize)
	}
	if got[0].CustomFormatScore == nil || *got[0].CustomFormatScore != 1100 {
		t.Errorf("got[0].CustomFormatScore = %v, want 1100", got[0].CustomFormatScore)
	}
	if got[0].PreviousCustomFormatScore == nil || *got[0].PreviousCustomFormatScore != 450 {
		t.Errorf("got[0].PreviousCustomFormatScore = %v, want 450", got[0].PreviousCustomFormatScore)
	}
	if got[0].ReleaseGroup != "GRP" {
		t.Errorf("got[0].ReleaseGroup = %q, want GRP", got[0].ReleaseGroup)
	}
	if got[0].FileID != 7 {
		t.Errorf("got[0].FileID = %d, want 7", got[0].FileID)
	}
	if got[0].DetailPath != "/movie/movie-2024" {
		t.Errorf("got[0].DetailPath = %q, want %q", got[0].DetailPath, "/movie/movie-2024")
	}
	if got[0].MediaCoverPath != "42/poster-250.jpg" {
		t.Errorf("got[0].MediaCoverPath = %q, want %q", got[0].MediaCoverPath, "42/poster-250.jpg")
	}
	if got[0].MediaInfo == nil {
		t.Fatal("got[0].MediaInfo = nil, want media info from the file endpoint")
	}
	if got[0].MediaInfo.VideoCodec != "x265" || got[0].MediaInfo.Resolution != "1920x1080" {
		t.Errorf("MediaInfo = %+v, want x265 at 1920x1080", *got[0].MediaInfo)
	}
	if got[0].MediaInfo.AudioChannels != 7.1 || got[0].MediaInfo.VideoDynamicRangeType != "HDR10" {
		t.Errorf("MediaInfo = %+v, want 7.1 channels and HDR10", *got[0].MediaInfo)
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
	if got[1].MediaInfo != nil {
		t.Error("got[1].MediaInfo != nil, want nil (file details are only fetched for upgrades)")
	}
	if got[1].MediaCoverPath != "" {
		t.Errorf("got[1].MediaCoverPath = %q, want empty (no poster image)", got[1].MediaCoverPath)
	}
}

func TestFetchArrHistoryFileFetchFailureNonFatal(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour)
	since := now.Add(-24 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/episodefile/9" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("eventType") {
		case "5": // sonarrFileDeleted
			//nolint:errcheck // test helper
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
				{
					"id":        100,
					"date":      recent.Format(time.RFC3339Nano),
					"eventType": "episodeFileDeleted",
					"episodeId": 5,
					"data":      map[string]string{"Reason": "Upgrade"},
					"quality":   map[string]any{"quality": map[string]any{"name": "WEBDL-720p"}},
				},
			}})
		case "3": // sonarrImported
			//nolint:errcheck // test helper
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
				{
					"id":          1,
					"date":        recent.Format(time.RFC3339Nano),
					"eventType":   "downloadFolderImported",
					"sourceTitle": "Show.S01E01.1080p",
					"episodeId":   5,
					"series":      map[string]any{"id": 3, "titleSlug": "show", "title": "Show"},
					"episode":     map[string]any{"seasonNumber": 1, "episodeNumber": 1, "title": "Pilot"},
					"data":        map[string]string{"FileId": "9"},
					"quality":     map[string]any{"quality": map[string]any{"name": "WEBDL-1080p"}},
				},
			}})
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(context.Background(), c, "v3", since, 50, sonarrHistory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].IsUpgrade {
		t.Error("IsUpgrade = false, want true (PascalCase data keys are matched)")
	}
	if got[0].ItemLabel != "Show S01E01" {
		t.Errorf("ItemLabel = %q, want %q", got[0].ItemLabel, "Show S01E01")
	}
	if got[0].PreviousQuality != "WEBDL-720p" {
		t.Errorf("PreviousQuality = %q, want WEBDL-720p", got[0].PreviousQuality)
	}
	if got[0].FileID != 9 {
		t.Errorf("FileID = %d, want 9", got[0].FileID)
	}
	if got[0].MediaInfo != nil {
		t.Error("MediaInfo != nil, want nil after a failed file fetch")
	}
}

func TestFetchArrHistoryCollapsesRecordsPerItem(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Hour)
	since := now.Add(-24 * time.Hour)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("eventType") {
		case "5": // lidarrFileDeleted
			w.Write([]byte(`{"records":[]}`)) //nolint:errcheck // test helper
		case "3": // lidarrImported: one record per track of the same album
			//nolint:errcheck // test helper
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
				{
					"id":          1,
					"date":        recent.Add(-time.Minute).Format(time.RFC3339Nano),
					"eventType":   "trackFileImported",
					"sourceTitle": "Artist - Album - 01",
					"albumId":     8,
					"artist":      map[string]any{"id": 2, "titleSlug": "artist", "artistName": "Artist"},
					"album":       map[string]any{"id": 8, "title": "Album", "images": []map[string]any{{"coverType": "cover"}}},
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "FLAC"}},
				},
				{
					"id":          2,
					"date":        recent.Format(time.RFC3339Nano),
					"eventType":   "trackFileImported",
					"sourceTitle": "Artist - Album - 02",
					"albumId":     8,
					"artist":      map[string]any{"id": 2, "titleSlug": "artist", "artistName": "Artist"},
					"album":       map[string]any{"id": 8, "title": "Album", "images": []map[string]any{{"coverType": "cover"}}},
					"data":        map[string]string{},
					"quality":     map[string]any{"quality": map[string]any{"name": "FLAC"}},
				},
			}})
		}
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	got, err := fetchArrHistory(context.Background(), c, "v1", since, 50, lidarrHistory)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (tracks of one album collapse to the newest record)", len(got))
	}
	if got[0].ID != 2 {
		t.Errorf("ID = %d, want 2 (newest record kept)", got[0].ID)
	}
	if got[0].ItemLabel != "Artist - Album" {
		t.Errorf("ItemLabel = %q, want %q", got[0].ItemLabel, "Artist - Album")
	}
	if got[0].DetailPath != "/artist/artist" {
		t.Errorf("DetailPath = %q, want /artist/artist", got[0].DetailPath)
	}
	if got[0].MediaCoverPath != "album/8/cover-250.jpg" {
		t.Errorf("MediaCoverPath = %q, want album/8/cover-250.jpg", got[0].MediaCoverPath)
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
			json.MarshalWrite(w, map[string]any{"records": []any{}}) //nolint:errcheck // test helper
		case "3": // sonarrImported
			//nolint:errcheck // test helper
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
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
		sonarrHistory,
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
		radarrHistory,
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
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
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
		radarrHistory,
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
		radarrHistory,
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
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
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
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
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
			json.MarshalWrite(w, map[string]any{"records": []map[string]any{
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
		whisparrV3History,
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
		whisparrV3History,
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
