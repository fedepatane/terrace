package cache

import (
	"sync"
	"time"
)

type item[T any] struct {
	value     T
	expiresAt time.Time
}

// Cache is a generic thread-safe in-memory TTL cache.
type Cache[T any] struct {
	mu    sync.RWMutex
	items map[string]item[T]
}

func New[T any]() *Cache[T] {
	c := &Cache[T]{items: make(map[string]item[T])}
	go c.evictLoop()
	return c
}

func (c *Cache[T]) Set(key string, val T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = item[T]{value: val, expiresAt: time.Now().Add(ttl)}
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	it, ok := c.items[key]
	if !ok || time.Now().After(it.expiresAt) {
		var zero T
		return zero, false
	}
	return it.value, true
}

func (c *Cache[T]) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, v := range c.items {
			if now.After(v.expiresAt) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
	}
}
