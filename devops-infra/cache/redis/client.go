// Package redis provides a cache adapter based on github.com/redis/go-redis/v9.
package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/stanley760/devops/devops-infra/cache"
)

// Client wraps the go-redis client and implements cache.Cache.
var _ cache.Cache = (*Client)(nil)

type Client struct {
	client *goredis.Client
}

// New creates a client and verifies connectivity.
func New(cfg Config) (*Client, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	client := goredis.NewClient(options(cfg))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	slog.Info("connected to Redis", "addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "db", cfg.DB)
	return &Client{client: client}, nil
}

// Redis returns the underlying go-redis client for advanced capabilities such as pipelines and subscriptions.
func (client *Client) Redis() *goredis.Client {
	if client == nil {
		return nil
	}
	return client.client
}

// Get reads the cache; returns cache.ErrNotFound when the key does not exist.
func (client *Client) Get(ctx context.Context, key string) (string, error) {
	if client == nil || client.client == nil {
		return "", fmt.Errorf("Redis client not initialized")
	}

	value, err := client.client.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return "", cache.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to read Redis cache: %w", err)
	}
	return value, nil
}

// Set writes the cache; expiration <= 0 means no expiration.
func (client *Client) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	if err := client.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("failed to write Redis cache: %w", err)
	}
	return nil
}

// Delete removes one or more keys.
func (client *Client) Delete(ctx context.Context, keys ...string) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	if len(keys) == 0 {
		return nil
	}
	if err := client.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete Redis cache: %w", err)
	}
	return nil
}

// Exists reports whether the key exists.
func (client *Client) Exists(ctx context.Context, key string) (bool, error) {
	if client == nil || client.client == nil {
		return false, fmt.Errorf("Redis client not initialized")
	}
	count, err := client.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check Redis cache: %w", err)
	}
	return count > 0, nil
}

// Close closes the connection pool.
func (client *Client) Close() error {
	if client == nil || client.client == nil {
		return nil
	}
	return client.client.Close()
}

var current *Client

// Init establishes the global Redis client, compatible with existing assembly code.
func Init(cfg Config) error {
	client, err := New(cfg)
	if err != nil {
		return err
	}
	current = client
	return nil
}

// Default returns the current global client.
func Default() *Client { return current }

// Get returns the current underlying go-redis client; nil when Init has not been called.
func Get() *goredis.Client {
	return current.Redis()
}

// Close closes the global client.
func Close() error {
	client := current
	current = nil
	return client.Close()
}
