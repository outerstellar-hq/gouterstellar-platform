package filter

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	visitors sync.Map
	rps      float64
	burst    int
}

func (rl *rateLimiter) getLimiter(ip string) *rate.Limiter {
	if v, ok := rl.visitors.Load(ip); ok {
		vi := v.(*visitor)
		vi.lastSeen = time.Now()
		return vi.limiter
	}

	v := &visitor{
		limiter:  rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
		lastSeen: time.Now(),
	}
	rl.visitors.Store(ip, v)
	return v.limiter
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		now := time.Now()
		rl.visitors.Range(func(key, value any) bool {
			vi := value.(*visitor)
			if now.Sub(vi.lastSeen) > 10*time.Minute {
				rl.visitors.Delete(key)
			}
			return true
		})
	}
}

func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	rl := &rateLimiter{rps: rps, burst: burst}
	go rl.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			limiter := rl.getLimiter(ip)
			if !limiter.Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
