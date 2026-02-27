package arr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRadarrLibrary(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/movie" {
			t.Errorf("path = %q, want /api/v3/movie", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":1,"title":"Inception","year":2010,"qualityProfileId":1,"hasFile":true,"monitored":true,` +
			`"movieFile":{"quality":{"quality":{"id":7}}}},` +
			`{"id":2,"title":"The Matrix","year":1999,"qualityProfileId":2,"hasFile":false,"monitored":true}` +
			`]`))
	}))
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchRadarrLibrary(context.Background(), client, "v3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}

	if items[0].ID != 1 {
		t.Errorf("items[0].ID = %d, want 1", items[0].ID)
	}
	if items[0].Label != "Inception (2010)" {
		t.Errorf("items[0].Label = %q, want %q", items[0].Label, "Inception (2010)")
	}
	if items[0].QualityProfileID != 1 {
		t.Errorf("items[0].QualityProfileID = %d, want 1", items[0].QualityProfileID)
	}
	if len(items[0].CurrentQualityIDs) != 1 || items[0].CurrentQualityIDs[0] != 7 {
		t.Errorf("items[0].CurrentQualityIDs = %v, want [7]", items[0].CurrentQualityIDs)
	}
	if !items[0].HasFile {
		t.Error("items[0].HasFile = false, want true")
	}

	if items[1].HasFile {
		t.Error("items[1].HasFile = true, want false")
	}
	if len(items[1].CurrentQualityIDs) != 0 {
		t.Errorf("items[1].CurrentQualityIDs = %v, want empty", items[1].CurrentQualityIDs)
	}
}

func TestFetchRadarrLibraryEmpty(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchRadarrLibrary(context.Background(), client, "v3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len = %d, want 0", len(items))
	}
}

func TestFetchSonarrLibrary(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":10,"title":"Breaking Bad","qualityProfileId":1,"monitored":true},` +
			`{"id":20,"title":"Unmonitored Show","qualityProfileId":2,"monitored":false}` +
			`]`))
	})
	mux.HandleFunc("/api/v3/episode", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("seriesId") != "10" {
			t.Errorf("seriesId = %q, want 10", r.URL.Query().Get("seriesId"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":101,"episodeNumber":1,"seasonNumber":1,"hasFile":true,"monitored":true,` +
			`"episodeFile":{"quality":{"quality":{"id":4}}}},` +
			`{"id":102,"episodeNumber":2,"seasonNumber":1,"hasFile":false,"monitored":true}` +
			`]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchSonarrLibrary(context.Background(), client, "v3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}

	if items[0].Label != "Breaking Bad S01E01" {
		t.Errorf("items[0].Label = %q, want %q", items[0].Label, "Breaking Bad S01E01")
	}
	if items[0].QualityProfileID != 1 {
		t.Errorf("items[0].QualityProfileID = %d, want 1", items[0].QualityProfileID)
	}
	if len(items[0].CurrentQualityIDs) != 1 || items[0].CurrentQualityIDs[0] != 4 {
		t.Errorf("items[0].CurrentQualityIDs = %v, want [4]", items[0].CurrentQualityIDs)
	}
}

func TestFetchSonarrLibraryPerSeriesFailure(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":10,"title":"Good Show","qualityProfileId":1,"monitored":true},` +
			`{"id":20,"title":"Bad Show","qualityProfileId":1,"monitored":true}` +
			`]`))
	})
	mux.HandleFunc("/api/v3/episode", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("seriesId") == "20" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":101,"episodeNumber":1,"seasonNumber":1,"hasFile":true,"monitored":true,` + //nolint:errcheck // test helper
			`"episodeFile":{"quality":{"quality":{"id":4}}}}]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchSonarrLibrary(context.Background(), client, "v3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len = %d, want 1 (second series should be skipped)", len(items))
	}
}

func TestFetchLidarrLibrary(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/album", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":1,"title":"OK Computer","monitored":true,` +
			`"artist":{"artistName":"Radiohead","qualityProfileId":1},` +
			`"statistics":{"trackFileCount":12}},` +
			`{"id":2,"title":"No Files Album","monitored":true,` +
			`"artist":{"artistName":"Unknown","qualityProfileId":1},` +
			`"statistics":{"trackFileCount":0}}` +
			`]`))
	})
	mux.HandleFunc("/api/v1/trackfile", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("albumId") != "1" {
			t.Errorf("albumId = %q, want 1", r.URL.Query().Get("albumId"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"quality":{"quality":{"id":7}}},` +
			`{"quality":{"quality":{"id":4}}}` +
			`]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchLidarrLibrary(context.Background(), client, "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}

	if items[0].Label != "Radiohead - OK Computer" {
		t.Errorf("items[0].Label = %q, want %q", items[0].Label, "Radiohead - OK Computer")
	}
	if len(items[0].CurrentQualityIDs) != 2 {
		t.Fatalf("items[0].CurrentQualityIDs len = %d, want 2", len(items[0].CurrentQualityIDs))
	}
	if items[0].CurrentQualityIDs[0] != 7 || items[0].CurrentQualityIDs[1] != 4 {
		t.Errorf("items[0].CurrentQualityIDs = %v, want [7 4]", items[0].CurrentQualityIDs)
	}
	if !items[0].HasFile {
		t.Error("items[0].HasFile = false, want true")
	}

	if items[1].HasFile {
		t.Error("items[1].HasFile = true, want false")
	}
}

func TestFetchLidarrLibraryPerAlbumFailure(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/album", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[` + //nolint:errcheck // test helper
			`{"id":1,"title":"Good Album","monitored":true,` +
			`"artist":{"artistName":"Artist","qualityProfileId":1},` +
			`"statistics":{"trackFileCount":5}},` +
			`{"id":2,"title":"Bad Album","monitored":true,` +
			`"artist":{"artistName":"Artist","qualityProfileId":1},` +
			`"statistics":{"trackFileCount":3}}` +
			`]`))
	})
	mux.HandleFunc("/api/v1/trackfile", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("albumId") == "2" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"quality":{"quality":{"id":7}}}]`)) //nolint:errcheck // test helper
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newClient(srv.URL, "key", 0)
	items, err := fetchLidarrLibrary(context.Background(), client, "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len = %d, want 1 (second album should be skipped)", len(items))
	}
}
