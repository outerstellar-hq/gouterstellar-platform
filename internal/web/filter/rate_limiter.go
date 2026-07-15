package filter

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipVisitor tracks a per-IP rate limiter and the last time it was used so the
// cleanup goroutine can evict idle entries.
type ipVisitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter is a per-IP rate limiter using sync.Map. It is shared by the
// general RateLimiter and the AuthRateLimiter so the per-IP bookkeeping
// (limiter creation, idle eviction) lives in exactly one place.
type ipRateLimiter struct {
	visitors sync.Map
	rps      float64
	burst    int
}

func (rl *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	if v, ok := rl.visitors.Load(ip); ok {
		vi := v.(*ipVisitor)
		vi.lastSeen = time.Now()
		return vi.limiter
	}

	v := &ipVisitor{
		limiter:  rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
		lastSeen: time.Now(),
	}
	rl.visitors.Store(ip, v)
	return v.limiter
}

func (rl *ipRateLimiter) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		now := time.Now()
		rl.visitors.Range(func(key, value any) bool {
			vi := value.(*ipVisitor)
			if now.Sub(vi.lastSeen) > 10*time.Minute {
				rl.visitors.Delete(key)
			}
			return true
		})
	}
}

// clientIP returns the client IP from r.RemoteAddr, falling back to the raw
// value when SplitHostPort cannot parse it (e.g. unix sockets).
func clientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// RateLimiter applies per-IP rate limiting to every request it wraps.
func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	rl := &ipRateLimiter{rps: rps, burst: burst}
	go rl.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.getLimiter(clientIP(r)).Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
