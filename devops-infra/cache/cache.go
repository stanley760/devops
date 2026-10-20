// Package cache defines the generic cache interface and error semantics.
package cache

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound indicates the cache key does not exist or has expired.
var ErrNotFound = errors.New("cache: key not found")

// Cache is the minimal cache interface required by the business layer.
// expiration <= 0 means no expiration.
type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// Health is a cache implementation that supports health checks.
type Health interface {
	Ping(ctx context.Context) error
}

// Closer is a cache implementation that supports releasing connection resources.
type Closer interface {
	Close() error
}
