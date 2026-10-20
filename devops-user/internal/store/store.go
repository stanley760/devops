// Package store provides devops-user's Redis wiring entrypoint.
//
// Difference from devops-system: no Cache() (the C side has no general caching
// needs such as tenant caches), only the go-redis client needed for JWT
// session state.
//
// Wiring delegates directly to devops-infra/cache/redis, without reimplementing
// connection logic.
package store

import (
	"context"

	goredis "github.com/redis/go-redis/v9"

	rediscache "github.com/stanley760/devops/devops-infra/cache/redis"
	"github.com/stanley760/devops/devops-infra/config"
)

// Init establishes the Redis connection.
func Init(cfg config.RedisConfig) error {
	return rediscache.Init(cfg)
}

// Get returns the current go-redis client; nil before Init.
func Get() *goredis.Client {
	return rediscache.Get()
}

// Close closes the connection pool.
func Close() error {
	return rediscache.Close()
}

// Ping checks the current Redis connection. The signature aligns with
// httpinfra.ReadinessCheck (ctx is not used here; go-redis's Ping uses its own
// internal retry and timeout configuration).
func Ping(_ context.Context) error {
	return rediscache.Default().Ping(context.Background())
}
