package server

import (
	"crypto/subtle"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// maxRequestBodyBytes is the maximum size in bytes for request bodies. 1 MB
// is generous for JSON payloads used by this application.
const maxRequestBodyBytes = 1 << 20

// staticCacheHeader is the Cache-Control value for versioned static assets.
// One year (365 * 24 * 60 * 60 = 31536000 seconds) is safe because embedded
// files are versioned by filename (e.g. alpine-3.15.8.min.js).
const staticCacheHeader = "public, max-age=31536000, immutable"

func withMiddleware(h http.Handler, username, password string) http.Handler {
	return withSecurityHeaders(
		withRequestLogging(withPanicRecovery(withMaxBodySize(withBasicAuth(h, username, password)))),
	)
}

// withSecurityHeaders sets standard security headers on every response to
// prevent common browser-side attacks.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// unsafe-inline is required for Alpine.js inline event handlers (x-on,
		// @click). unsafe-eval is required because Alpine.js evaluates x-data,
		// x-show, and x-model expressions via AsyncFunction. Both are acceptable
		// compromises for a self-hosted management tool.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// withStaticCacheHeaders sets aggressive cache headers for static assets.
// Embedded static files are versioned by filename (e.g. alpine-3.15.8.min.js),
// making them safe to cache indefinitely. The cache header is injected only
// for successful responses to avoid browsers caching 404s for a year.
func withStaticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&staticCacheWriter{ResponseWriter: w}, r)
	})
}

// staticCacheWriter injects the Cache-Control header at WriteHeader time,
// but only for successful (2xx/3xx) responses.
type staticCacheWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *staticCacheWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if code < 400 {
			w.ResponseWriter.Header().Set("Cache-Control", staticCacheHeader)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *staticCacheWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *staticCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// withMaxBodySize wraps request bodies with http.MaxBytesReader to prevent
// clients from sending arbitrarily large payloads. Requests that exceed the
// limit receive 413 Request Entity Too Large.
func withMaxBodySize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// withBasicAuth returns a handler that requires HTTP Basic Authentication. If
// both username and password are empty, the handler is returned unmodified so
// there is zero overhead when auth is disabled. The /api/health path is always
// exempt to allow container health probes.
func withBasicAuth(next http.Handler, username, password string) http.Handler {
	if username == "" && password == "" {
		return next
	}

	user := []byte(username)
	pass := []byte(password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}

		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), user) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), pass) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Huntarr2"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorised"}`)) //nolint:errcheck // best-effort after WriteHeader
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		level := zerolog.InfoLevel
		if strings.HasPrefix(r.URL.Path, "/static/") {
			level = zerolog.DebugLevel
		}

		log.WithLevel(level).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", rw.status).
			Dur("duration", time.Since(start)).
			Msg("request handled")
	})
}

func withPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error().
					Interface("panic", v).
					Str("stack", string(debug.Stack())).
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Msg("panic recovered")
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code. It
// implements Unwrap so that http.ResponseController can reach the underlying
// writer for Flush, Hijack, and other optional interfaces.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write delegates to the underlying ResponseWriter. When no explicit
// WriteHeader call has been made, Go implicitly sends 200 OK on the
// first Write. The status field already defaults to 200 (set in the
// constructor), so no update is needed here.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
}
