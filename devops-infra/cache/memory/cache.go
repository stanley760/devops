// Package memory provides an in-process cache implementation, suitable for single-instance, test, and degradation scenarios.
package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	cacheinf "github.com/stanley760/devops/devops-infra/cache"
)

type entry struct {
	value     string
	expiresAt time.Time
}

// Cache is a concurrency-safe in-memory cache.
var _ cacheinf.Cache = (*Cache)(nil)

type Cache struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

// New creates an in-memory cache.
func New() *Cache {
	return &Cache{
		entries: make(map[string]entry),
		now:     time.Now,
	}
}

// Get reads the cache; returns cache.ErrNotFound when the key does not exist or has expired.
func (c *Cache) Get(_ context.Context, key string) (string, error) {
	c.mu.RLock()
	item, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok {
		return "", cacheinf.ErrNotFound
	}
	if !item.expiresAt.IsZero() && !c.now().Before(item.expiresAt) {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return "", cacheinf.ErrNotFound
	}
	return item.value, nil
}

// Set writes the cache; expiration <= 0 means no expiration.
func (c *Cache) Set(_ context.Context, key, value string, expiration time.Duration) error {
	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = c.now().Add(expiration)
	}

	c.mu.Lock()
	c.entries[key] = entry{value: value, expiresAt: expiresAt}
	c.mu.Unlock()
	return nil
}

// Delete removes one or more keys.
func (c *Cache) Delete(_ context.Context, keys ...string) error {
	c.mu.Lock()
	for _, key := range keys {
		delete(c.entries, key)
	}
	c.mu.Unlock()
	return nil
}

// Exists reports whether the key exists and has not expired.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	_, err := c.Get(ctx, key)
	if errors.Is(err, cacheinf.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
