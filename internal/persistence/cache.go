package persistence

import (
	"strings"
	"sync"
	"time"
)

// cacheItem holds a single cached value alongside its absolute expiry. A zero
// expiresAt means the entry never expires.
type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func (i cacheItem) expired(now time.Time) bool {
	return !i.expiresAt.IsZero() && now.After(i.expiresAt)
}

// MessageCache is a minimal TTL cache that replaces github.com/patrickmn/go-cache.
// It keeps a single TTL for every entry (matching the prior cache.DefaultExpiration
// usage) and runs a background janitor that drops expired items every ttl/2.
type MessageCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
	ttl   time.Duration
	stop  chan struct{}
}

// NewMessageCache builds a cache whose entries expire after defaultTTL. The
// janitor is stopped by calling Close.
func NewMessageCache(defaultTTL time.Duration) *MessageCache {
	c := &MessageCache{
		items: make(map[string]cacheItem),
		ttl:   defaultTTL,
		stop:  make(chan struct{}),
	}
	if defaultTTL > 0 {
		go c.janitor(defaultTTL / 2)
	}
	return c
}

func (c *MessageCache) janitor(interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.sweep(time.Now())
		case <-c.stop:
			return
		}
	}
}

func (c *MessageCache) sweep(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range c.items {
		if v.expired(now) {
			delete(c.items, k)
		}
	}
}

// Close stops the background janitor. It is safe to call multiple times.
func (c *MessageCache) Close() {
	select {
	case <-c.stop:
		// already closed
	default:
		close(c.stop)
	}
}

func (c *MessageCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if item.expired(time.Now()) {
		return nil, false
	}
	return item.value, true
}

func (c *MessageCache) Set(key string, value interface{}) {
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = time.Now().Add(c.ttl)
	}
	c.mu.Lock()
	c.items[key] = cacheItem{value: value, expiresAt: expiresAt}
	c.mu.Unlock()
}

// GetOrSet returns the existing value for key if present and unexpired,
// otherwise calls fn, stores the result, and returns it. This mirrors the
// previous go-cache-backed factory signature used by MessageService.
func (c *MessageCache) GetOrSet(key string, fn func() interface{}) interface{} {
	if val, found := c.Get(key); found {
		return val
	}
	val := fn()
	c.Set(key, val)
	return val
}

func (c *MessageCache) Invalidate(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *MessageCache) InvalidateAll() {
	c.mu.Lock()
	c.items = make(map[string]cacheItem)
	c.mu.Unlock()
}

func (c *MessageCache) InvalidateByPrefix(prefix string) {
	c.mu.Lock()
	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
		}
	}
	c.mu.Unlock()
}
