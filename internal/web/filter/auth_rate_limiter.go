package filter

import (
	"net/http"
	"strings"

	"golang.org/x/time/rate"
)

// AuthRateLimiter applies stricter rate limiting to authentication-sensitive
// routes. Other routes pass through unaffected. The limiter is shared across
// all clients (it complements the per-IP RateLimiter, which still applies).
func AuthRateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAuthRoute(r.URL.Path) {
				if !limiter.Allow() {
					http.Error(w, "Too many requests", http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAuthRoute(path string) bool {
	authPrefixes := []string{
		"/auth/login", "/auth/register", "/auth/reset",
		"/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/token",
	}
	for _, prefix := range authPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
