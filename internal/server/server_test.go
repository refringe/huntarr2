package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/refringe/huntarr2/web/static"
)

func TestMiddlewareCompositionAppliesSecurityHeaders(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withMiddleware(inner, "", "")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for key, want := range headers {
		if got := rr.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	if csp := rr.Header().Get("Content-Security-Policy"); csp == "" {
		t.Error("Content-Security-Policy header missing")
	}
}

func TestMiddlewareCompositionWithAuthRejects(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := withMiddleware(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestStaticFileServing(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.Handle("GET /static/", withStaticCacheHeaders(http.StripPrefix("/static/",
		http.FileServer(http.FS(static.FS)))))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/static/js/scheduler.js", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /static/js/scheduler.js: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if got := resp.Header.Get("Cache-Control"); got != appCacheHeader {
		t.Errorf("Cache-Control = %q, want %q", got, appCacheHeader)
	}
}

func TestStaticFileServingReturns404ForMissing(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.Handle("GET /static/", withStaticCacheHeaders(http.StripPrefix("/static/",
		http.FileServer(http.FS(static.FS)))))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/static/js/nonexistent.js", nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /static/js/nonexistent.js: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
