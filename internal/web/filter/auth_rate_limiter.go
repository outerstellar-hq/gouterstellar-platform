package filter

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type authVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type authRateLimiter struct {
	visitors sync.Map
	rps      float64
	burst    int
}

func (a *authRateLimiter) getLimiter(ip string) *rate.Limiter {
	if v, ok := a.visitors.Load(ip); ok {
		av := v.(*authVisitor)
		av.lastSeen = time.Now()
		return av.limiter
	}

	v := &authVisitor{
		limiter:  rate.NewLimiter(rate.Limit(a.rps), a.burst),
		lastSeen: time.Now(),
	}
	a.visitors.Store(ip, v)
	return v.limiter
}

func (a *authRateLimiter) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		now := time.Now()
		a.visitors.Range(func(key, value any) bool {
			av := value.(*authVisitor)
			if now.Sub(av.lastSeen) > 10*time.Minute {
				a.visitors.Delete(key)
			}
			return true
		})
	}
}

// AuthRateLimiter applies stricter rate limiting to authentication-sensitive
// routes. Other routes pass through unaffected. Limiting is per-IP so a single
// high-traffic client cannot exhaust the budget for everyone (it complements
// the per-IP RateLimiter, which still applies to all routes).
func AuthRateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	a := &authRateLimiter{rps: rps, burst: burst}
	go a.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAuthRoute(r.URL.Path) {
				ip := extractAuthIP(r)
				if !a.getLimiter(ip).Allow() {
					http.Error(w, "Too many requests", http.StatusTooManyRequests)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// extractAuthIP returns the client IP from r.RemoteAddr, falling back to the
// raw value when SplitHostPort cannot parse it (e.g. unix sockets).
func extractAuthIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
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
