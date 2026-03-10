package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLoggingPassesThrough(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello")) //nolint:errcheck // test helper
	})

	handler := withRequestLogging(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("X-Custom = %q, want %q", w.Header().Get("X-Custom"), "value")
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello")
	}
}

func TestPanicRecoveryReturns500(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("test panic")
	})

	handler := withPanicRecovery(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestBasicAuthDisabledPassesThrough(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "", "")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestBasicAuthNoCreds(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != `Basic realm="Huntarr2"` {
		t.Errorf("WWW-Authenticate = %q, want %q", got, `Basic realm="Huntarr2"`)
	}
}

func TestBasicAuthWrongCreds(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBasicAuthCorrectCreds(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok")) //nolint:errcheck // test helper
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "ok" {
		t.Errorf("body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestBasicAuthHealthExempt(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy")) //nolint:errcheck // test helper
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestBasicAuthWrongUsernameOnly(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.SetBasicAuth("wrong", "secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBasicAuthWrongPasswordOnly(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMaxBodySizeRejectsOversized(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := withMaxBodySize(inner)

	oversized := strings.Repeat("a", maxRequestBodyBytes+1)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", strings.NewReader(oversized))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMaxBodySizeAllowsNormalPayload(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := withMaxBodySize(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", strings.NewReader(`{"name":"test"}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestSecurityHeadersPresent(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withSecurityHeaders(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for name, want := range headers {
		if got := w.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if got := w.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy header is missing")
	}
}

func TestStaticCacheHeadersVersionedFile(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withStaticCacheHeaders(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/js/alpine-3.15.8.min.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	want := "public, max-age=31536000, immutable"
	if got := w.Header().Get("Cache-Control"); got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func TestStaticCacheHeadersAppFile(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withStaticCacheHeaders(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/js/logs.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != appCacheHeader {
		t.Errorf("Cache-Control = %q, want %q", got, appCacheHeader)
	}
}

func TestStaticCacheHeadersOmittedOn404(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := withStaticCacheHeaders(inner)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/js/nonexistent.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q on 404, want empty", got)
	}
}

func TestBasicAuthReturnsJSON(t *testing.T) {
	t.Parallel()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := withBasicAuth(inner, "admin", "secret")

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	want := `{"error":"unauthorised"}`
	if got := w.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
