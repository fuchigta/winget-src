package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

type Cache[T any] struct {
	mu         sync.RWMutex
	entries    map[string]cacheEntry[T]
	ttl        time.Duration
	maxEntries int // 0 = unlimited
}

func NewCache[T any](ttl time.Duration, maxEntries int) *Cache[T] {
	return &Cache[T]{
		entries:    make(map[string]cacheEntry[T]),
		ttl:        ttl,
		maxEntries: maxEntries,
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
		slog.Debug("cache miss", "key", key)
		var zero T
		return zero, false
	}
	slog.Debug("cache hit", "key", key)
	return entry.value, true
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, exists := c.entries[key]
	if !exists && c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		// evict the entry with the earliest expiry
		var oldestKey string
		var oldestExpiry time.Time
		for k, e := range c.entries {
			if oldestKey == "" || e.expiresAt.Before(oldestExpiry) {
				oldestKey = k
				oldestExpiry = e.expiresAt
			}
		}
		delete(c.entries, oldestKey)
	}

	c.entries[key] = cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}
