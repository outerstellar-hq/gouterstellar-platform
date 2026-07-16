package filter

import (
	"net/http"
	"strings"
)

// AuthRateLimiter applies stricter per-IP rate limiting to authentication-
// sensitive routes. Other routes pass through unaffected. It complements the
// per-IP RateLimiter (which still applies to all routes).
func AuthRateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	rl := &ipRateLimiter{rps: rps, burst: burst}
	go rl.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAuthRoute(r.URL.Path) {
				if !rl.getLimiter(clientIP(r)).Allow() {
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
		"/auth/login", "/auth/register", "/auth/reset", "/auth/totp/verify",
		"/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/token", "/api/v1/auth/totp/verify",
	}
	for _, prefix := range authPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
