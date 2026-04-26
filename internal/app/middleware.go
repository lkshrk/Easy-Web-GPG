package app

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// rateLimiter tracks request attempts per IP using a sliding window.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	max      int
}

// NewRateLimiter creates a rate limiter with the given window and max attempts.
func NewRateLimiter(window time.Duration, max int) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		max:      max,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter expired entries
	existing := rl.attempts[ip]
	valid := existing[:0]
	for _, t := range existing {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.max {
		rl.attempts[ip] = valid
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

// AuthRateLimiter is the default rate limiter for the auth endpoint.
// Allows up to 10 attempts per IP per 15-minute window.
var AuthRateLimiter = NewRateLimiter(15*time.Minute, 10)

// RateLimit wraps a handler with IP-based rate limiting.
func RateLimit(rl *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = fwd
		}
		if !rl.allow(ip) {
			slog.Warn("rate limit exceeded", "method", r.Method, "path", r.URL.Path, "ip", ip)
			http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// RequestLogger logs mutations and error responses; skips successful GETs and static assets.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)

		// Never log static asset requests.
		if strings.HasPrefix(r.URL.Path, "/static/") {
			return
		}

		isMutation := r.Method == http.MethodPost || r.Method == http.MethodDelete ||
			r.Method == http.MethodPut || r.Method == http.MethodPatch
		isError := lw.status >= 400

		if !isMutation && !isError {
			return
		}

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", lw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		switch {
		case lw.status >= 500:
			slog.Error("request", attrs...)
		case lw.status >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (lw *loggingResponseWriter) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}
