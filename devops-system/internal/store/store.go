// Package store provides the cache assembly entry point for devops-system.
//
// New code should prefer the cache.Cache interface returned by Cache();
// advanced framework capabilities such as the session pipeline can obtain the
// go-redis client via Get().
package store

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	"github.com/stanley760/devops/devops-infra/cache"
	rediscache "github.com/stanley760/devops/devops-infra/cache/redis"
	"github.com/stanley760/devops/devops-infra/config"
)

// Init establishes the Redis connection.
func Init(cfg config.RedisConfig) error {
	return rediscache.Init(cfg)
}

// Cache returns the general cache interface; nil when uninitialized.
func Cache() cache.Cache {
	if rediscache.Default() == nil {
		return nil
	}
	return rediscache.Default()
}

// Get returns the current go-redis client; nil before Init.
func Get() *goredis.Client {
	return rediscache.Get()
}

// Close closes the connection pool.
func Close() error {
	return rediscache.Close()
}

// Ping checks the current Redis connection.
func Ping(ctx context.Context) error {
	return rediscache.Default().Ping(ctx)
}
