package app

import (
	"log"
	"net/http"
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

// AuthRateLimiter is a rate limiter for the auth endpoint.
// Allows max attempts per window per IP.
// AuthRateLimiter is a rate limiter for the auth endpoint.
var AuthRateLimiter = NewRateLimiter(15*time.Minute, 10)

// RateLimit wraps a handler with IP-based rate limiting.
func RateLimit(rl *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = fwd
		}
		if !rl.allow(ip) {
			log.Printf("rate limited: %s %s from %s", r.Method, r.URL.Path, ip)
			http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// RequestLogger logs each incoming request.
func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lw := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(lw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, lw.status, time.Since(start).Round(time.Millisecond))
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
