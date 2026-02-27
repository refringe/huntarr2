package arr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetSendsAPIKeyHeader(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "secret" {
			t.Errorf("X-Api-Key = %q, want %q", got, "secret")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "secret", 5*time.Second)
	var dst map[string]bool
	if err := c.get(context.Background(), "/test", &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostSendsAPIKeyAndContentType(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "secret" {
			t.Errorf("X-Api-Key = %q, want %q", got, "secret")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":1}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "secret", 5*time.Second)
	var dst map[string]int
	if err := c.post(context.Background(), "/cmd", map[string]string{"name": "test"}, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst["id"] != 1 {
		t.Errorf("id = %d, want 1", dst["id"])
	}
}

func TestPostSendsJSONBody(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name string `json:"name"`
		IDs  []int  `json:"ids"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading body: %v", err)
		}
		var got payload
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshalling body: %v", err)
		}
		if got.Name != "EpisodeSearch" {
			t.Errorf("name = %q, want %q", got.Name, "EpisodeSearch")
		}
		if len(got.IDs) != 2 || got.IDs[0] != 1 || got.IDs[1] != 2 {
			t.Errorf("ids = %v, want [1 2]", got.IDs)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":42}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	var dst map[string]int
	err := c.post(context.Background(), "/api/v3/command", payload{Name: "EpisodeSearch", IDs: []int{1, 2}}, &dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetNon200ReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "bad", 5*time.Second)
	var dst map[string]any
	if err := c.get(context.Background(), "/test", &dst); err == nil {
		t.Fatal("expected error for 401 response, got nil")
	}
}

func TestPostNon200ReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	var dst map[string]any
	if err := c.post(context.Background(), "/cmd", map[string]string{}, &dst); err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestGetInvalidJSONReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not json}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	var dst map[string]any
	if err := c.get(context.Background(), "/test", &dst); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestGetTimeoutReturnsError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 50*time.Millisecond)
	var dst map[string]any
	if err := c.get(context.Background(), "/test", &dst); err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestClientRejectsRedirect(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, nil, "https://evil.example.com", http.StatusFound)
	}))
	defer srv.Close()

	c := newClient(srv.URL, "key", 5*time.Second)
	var dst map[string]any
	err := c.get(context.Background(), "/test", &dst)
	if err == nil {
		t.Fatal("expected error for redirect response, got nil")
	}
}

func TestTrailingSlashTrimmed(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/system/status" {
			t.Errorf("path = %q, want /api/v3/system/status", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`)) //nolint:errcheck // test helper
	}))
	defer srv.Close()

	c := newClient(srv.URL+"/", "key", 5*time.Second)
	var dst map[string]bool
	if err := c.get(context.Background(), "/api/v3/system/status", &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
