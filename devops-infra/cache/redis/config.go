package redis

import (
	"fmt"

	goredis "github.com/redis/go-redis/v9"

	"github.com/stanley760/devops/devops-infra/config"
)

// Config is the Redis connection config. The type alias keeps service configuration loading unified.
type Config = config.RedisConfig

// normalizeConfig validates the config and fills in defaults.
func normalizeConfig(cfg Config) (Config, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		return cfg, fmt.Errorf("redis.port must be greater than 0")
	}
	if cfg.DB < 0 {
		return cfg, fmt.Errorf("redis.db cannot be negative")
	}
	if cfg.PoolSize < 0 {
		return cfg, fmt.Errorf("redis.pool_size cannot be negative")
	}
	return cfg, nil
}

// options builds the go-redis config. When PoolSize is 0, go-redis uses its built-in default.
func options(cfg Config) *goredis.Options {
	return &goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	}
}
