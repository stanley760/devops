package oauth2

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisStateStore is the Redis implementation of state storage.
//
// Key shape: t1:auth:oauth2state:{state} = JSON(StateValue), TTL = stateTTL.
// One-time use is guaranteed by Redis GETDEL (Redis 6.2+); older versions fall
// back to a Lua script, avoiding "GET passes verification but DEL fails",
// which would let the same state be used twice.
type RedisStateStore struct {
	rdb      *goredis.Client
	prefix   string
	stateTTL time.Duration
}

// NewRedisStateStore creates a store. stateTTL is recommended to be 5-10
// minutes: authorization requests older than 10 minutes are basically the user
// walking away, and allowing more only widens the replay window.
func NewRedisStateStore(rdb *goredis.Client, prefix string, stateTTL time.Duration) *RedisStateStore {
	if stateTTL <= 0 {
		stateTTL = 10 * time.Minute
	}
	return &RedisStateStore{rdb: rdb, prefix: prefix, stateTTL: stateTTL}
}

func (s *RedisStateStore) key(state string) string {
	return s.prefix + ":auth:oauth2state:" + state
}

// Save stores the state.
func (s *RedisStateStore) Save(ctx context.Context, state string, val StateValue, ttl time.Duration) error {
	if s == nil || s.rdb == nil {
		return errNoRedis
	}
	if ttl <= 0 {
		ttl = s.stateTTL
	}
	payload, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, s.key(state), payload, ttl).Err()
}

// Verify verifies and one-time retrieves the state.
//
// Lua guarantees the atomicity of "read and delete": if GET and DEL were
// separate, two concurrent requests could both read the same state and each
// complete login — the one-time nature of state would be moot.
func (s *RedisStateStore) Verify(ctx context.Context, state string) (StateValue, error) {
	var empty StateValue
	if s == nil || s.rdb == nil {
		return empty, errNoRedis
	}
	if state == "" {
		return empty, ErrStateInvalid
	}
	const getdelScript = `
local v = redis.call('GET', KEYS[1])
if v == false then return nil end
redis.call('DEL', KEYS[1])
return v`

	raw, err := s.rdb.Eval(ctx, getdelScript, []string{s.key(state)}).Result()
	if err == goredis.Nil {
		return empty, ErrStateInvalid
	}
	if err != nil {
		return empty, err
	}
	payload, ok := raw.(string)
	if !ok || payload == "" {
		return empty, ErrStateInvalid
	}
	var val StateValue
	if err := json.Unmarshal([]byte(payload), &val); err != nil {
		return empty, ErrStateInvalid
	}
	return val, nil
}

// errNoRedis means Redis is not wired up.
var errNoRedis = errNoRedisType("oauth2: redis client is nil")

type errNoRedisType string

func (e errNoRedisType) Error() string { return string(e) }
