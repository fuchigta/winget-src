package main

import (
	"context"
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

type Cache[T any] struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry[T]
	ttl     time.Duration
}

func NewCache[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		entries: make(map[string]cacheEntry[T]),
		ttl:     ttl,
	}
}

// StartCleanup starts a background goroutine that periodically removes expired entries.
// It stops when ctx is done.
func (c *Cache[T]) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				c.mu.Lock()
				for k, e := range c.entries {
					if now.After(e.expiresAt) {
						delete(c.entries, k)
					}
				}
				c.mu.Unlock()
			}
		}
	}()
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}
