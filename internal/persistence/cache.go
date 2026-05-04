package persistence

import (
	"time"

	"github.com/patrickmn/go-cache"
)

type MessageCache struct {
	inner *cache.Cache
}

func NewMessageCache(defaultTTL time.Duration) *MessageCache {
	return &MessageCache{inner: cache.New(defaultTTL, defaultTTL*2)}
}

func (c *MessageCache) Get(key string) (interface{}, bool) {
	return c.inner.Get(key)
}

func (c *MessageCache) Set(key string, value interface{}) {
	c.inner.Set(key, value, cache.DefaultExpiration)
}

func (c *MessageCache) GetOrSet(key string, fn func() interface{}) interface{} {
	if val, found := c.inner.Get(key); found {
		return val
	}
	val := fn()
	c.inner.Set(key, val, cache.DefaultExpiration)
	return val
}

func (c *MessageCache) Invalidate(key string) {
	c.inner.Delete(key)
}

func (c *MessageCache) InvalidateAll() {
	c.inner.Flush()
}

func (c *MessageCache) InvalidateByPrefix(prefix string) {
	for key := range c.inner.Items() {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			c.inner.Delete(key)
		}
	}
}
