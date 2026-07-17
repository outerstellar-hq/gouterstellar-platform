package filter

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	resetMaxRequests    = 5
	resetWindow         = 15 * time.Minute
	accountMaxRequests  = 20
	accountWindow       = 15 * time.Minute
	maxRateLimitBuckets = 10_000
)

type fixedWindowBucket struct {
	count       int
	windowStart time.Time
	lastSeen    time.Time
}

type fixedWindowStore struct {
	mu       sync.Mutex
	buckets  map[string]*fixedWindowBucket
	requests uint64
	now      func() time.Time
}

func newFixedWindowStore() *fixedWindowStore {
	return &fixedWindowStore{buckets: make(map[string]*fixedWindowBucket), now: time.Now}
}

func (s *fixedWindowStore) allow(key string, maxRequests int, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.requests++
	if s.requests%256 == 0 {
		s.removeIdle(now)
	}
	bucket := s.buckets[key]
	if bucket == nil {
		if len(s.buckets) >= maxRateLimitBuckets {
			s.removeOldest()
		}
		s.buckets[key] = &fixedWindowBucket{count: 1, windowStart: now, lastSeen: now}
		return true
	}
	bucket.lastSeen = now
	if now.Sub(bucket.windowStart) >= window {
		bucket.count = 1
		bucket.windowStart = now
		return true
	}
	if bucket.count >= maxRequests {
		return false
	}
	bucket.count++
	return true
}

func (s *fixedWindowStore) removeIdle(now time.Time) {
	for key, bucket := range s.buckets {
		if now.Sub(bucket.lastSeen) >= 2*accountWindow {
			delete(s.buckets, key)
		}
	}
}

func (s *fixedWindowStore) removeOldest() {
	var oldestKey string
	var oldest time.Time
	for key, bucket := range s.buckets {
		if oldestKey == "" || bucket.lastSeen.Before(oldest) {
			oldestKey = key
			oldest = bucket.lastSeen
		}
	}
	delete(s.buckets, oldestKey)
}

// AuthRateLimiter protects authentication endpoints by peer IP and account.
// Forwarding headers are honored only when the direct peer is explicitly
// configured as a trusted proxy.
func AuthRateLimiter(maxRequests int, window time.Duration, trustedProxies []string) func(http.Handler) http.Handler {
	ipBuckets := newFixedWindowStore()
	accountBuckets := newFixedWindowStore()
	trusted := make(map[string]struct{}, len(trustedProxies))
	for _, proxy := range trustedProxies {
		if proxy = strings.TrimSpace(proxy); proxy != "" {
			trusted[proxy] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit, limited := authRateLimit(r.Method, r.URL.Path, maxRequests, window)
			if !limited {
				next.ServeHTTP(w, r)
				return
			}
			ip := rateLimitClientIP(r, trusted)
			if !ipBuckets.allow(ip+":"+r.URL.Path, limit.maxRequests, limit.window) {
				http.Error(w, "Too many requests. Please try again later.", http.StatusTooManyRequests)
				return
			}
			account := extractAccountIdentifier(r)
			if account != "" && !accountBuckets.allow("account:"+account, accountMaxRequests, accountWindow) {
				http.Error(w, "Too many login attempts for this account. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type rateLimit struct {
	maxRequests int
	window      time.Duration
}

func authRateLimit(method, path string, defaultMax int, defaultWindow time.Duration) (rateLimit, bool) {
	if method != http.MethodPost {
		return rateLimit{}, false
	}
	for _, prefix := range []string{
		"/api/v1/auth/reset-request", "/api/v1/auth/reset-confirm", "/api/v1/auth/reset-password", "/api/v1/auth/confirm-reset",
		"/auth/components/reset-confirm", "/auth/reset",
	} {
		if strings.HasPrefix(path, prefix) {
			return rateLimit{maxRequests: resetMaxRequests, window: resetWindow}, true
		}
	}
	for _, prefix := range []string{
		"/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/token", "/api/v1/auth/totp/verify",
		"/auth/components/result", "/auth/components/totp-verify", "/auth/login", "/auth/register", "/auth/totp/verify",
	} {
		if strings.HasPrefix(path, prefix) {
			return rateLimit{maxRequests: defaultMax, window: defaultWindow}, true
		}
	}
	return rateLimit{}, false
}

func rateLimitClientIP(r *http.Request, trustedProxies map[string]struct{}) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		peer = host
	}
	if _, ok := trustedProxies[peer]; !ok {
		if peer == "" {
			return "unknown"
		}
		return peer
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	return peer
}

func extractAccountIdentifier(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ""
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	var account string
	switch mediaType {
	case "application/json":
		var fields struct {
			Username string `json:"username"`
			Email    string `json:"email"`
		}
		if json.Unmarshal(body, &fields) == nil {
			account = fields.Username
			if account == "" {
				account = fields.Email
			}
		}
	case "application/x-www-form-urlencoded":
		values, _ := url.ParseQuery(string(body))
		account = values.Get("email")
		if account == "" {
			account = values.Get("username")
		}
	}
	return strings.ToLower(strings.TrimSpace(account))
}
