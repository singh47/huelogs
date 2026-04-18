package main

import (
	"net/http"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// RequireAuth validates either a session cookie (browser) or X-API-Key header
// (programmatic access). API routes return JSON on failure; everything else
// redirects to the login page.
func (a *App) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Session auth — browser dashboard
		if sess, err := a.store.Get(r, "session"); err == nil {
			if auth, _ := sess.Values["authenticated"].(bool); auth {
				next.ServeHTTP(w, r)
				return
			}
		}
		// API key auth — programmatic ingestion / curl
		if r.Header.Get("X-API-Key") == a.apiKey {
			next.ServeHTTP(w, r)
			return
		}
		// Return JSON for /api/* routes, redirect everything else.
		if strings.HasPrefix(r.URL.Path, "/api") || r.URL.Path == "/ws" {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

// RateLimiter returns a per-IP token bucket middleware.
//
//   - requestsPerMinute: sustained request rate
//   - burst: maximum burst capacity above the sustained rate
func RateLimiter(requestsPerMinute, burst int) func(http.Handler) http.Handler {
	var mu sync.Map
	rps := rate.Limit(float64(requestsPerMinute) / 60.0)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// LoadOrStore is atomic — the "extra" limiter created in the
			// losing goroutine is simply discarded by the GC.
			actual, _ := mu.LoadOrStore(r.RemoteAddr, rate.NewLimiter(rps, burst))
			if !actual.(*rate.Limiter).Allow() {
				w.Header().Set("Retry-After", "60")
				jsonError(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
