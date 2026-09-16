package server

import (
	"crypto/subtle"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// maxRequestBodyBytes is the maximum size in bytes for request bodies.
const maxRequestBodyBytes = 1 << 20

// staticCacheHeader is the Cache-Control value for vendored static assets whose filenames contain a version number.
const staticCacheHeader = "public, max-age=31536000, immutable"

// appCacheHeader is the Cache-Control value for the application's own unversioned static assets.
const appCacheHeader = "no-cache"

// versionedFile matches filenames that contain a numeric version component such as "3.16.1".
var versionedFile = regexp.MustCompile(`\d+\.\d+`)

func withMiddleware(h http.Handler, username, password string) http.Handler {
	return withSecurityHeaders(
		withRequestLogging(withPanicRecovery(withMaxBodySize(withBasicAuth(h, username, password)))),
	)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Alpine.js needs unsafe-inline for x-on handlers and unsafe-eval for x-data/x-show/x-model expressions.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
				"style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

// withStaticCacheHeaders serves versioned filenames (e.g. alpine-3.16.1.min.js) with an immutable one-year cache
// and unversioned application files with no-cache.
func withStaticCacheHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := r.URL.Path
		if _, after, ok := strings.CutLast(base, "/"); ok {
			base = after
		}
		header := appCacheHeader
		if versionedFile.MatchString(base) {
			header = staticCacheHeader
		}
		next.ServeHTTP(&staticCacheWriter{
			ResponseWriter: w,
			cacheHeader:    header,
		}, r)
	})
}

// staticCacheWriter injects the Cache-Control header at WriteHeader time, only for responses below 400.
type staticCacheWriter struct {
	http.ResponseWriter
	cacheHeader string
	wroteHeader bool
}

func (w *staticCacheWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		if code < 400 {
			w.ResponseWriter.Header().Set("Cache-Control", w.cacheHeader)
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

func withMaxBodySize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// withBasicAuth requires HTTP Basic Authentication when a username or password is set; /api/health is always exempt.
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

// responseWriter captures the status code; Unwrap lets http.ResponseController reach the underlying writer.
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

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.wroteHeader = true
	}
	return rw.ResponseWriter.Write(b)
}
