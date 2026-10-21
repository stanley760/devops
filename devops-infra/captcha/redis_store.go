package captcha

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisStore is the Redis-backed captcha Store.
//
// Key shape: `{prefix}:captcha:{id}` → JSON(Answer), TTL = the challenge lifetime. The prefix
// carries the tenant/side namespace (e.g. "t1" for the C-side) so that captcha IDs issued by one
// service can never be replayed against another.
//
// One-shot read: implemented with GetDel (Redis ≥ 6.2), which is atomic. Doing GET-then-DEL in two
// round trips would leave a window where two concurrent requests both read the same answer, so an
// attacker could brute-force a single captcha with parallel guesses.
type RedisStore struct {
	rdb    *goredis.Client
	prefix string
}

// NewRedisStore creates a Store backed by Redis. prefix namespaces the keys (e.g. "t1").
func NewRedisStore(rdb *goredis.Client, prefix string) *RedisStore {
	return &RedisStore{rdb: rdb, prefix: prefix}
}

func (s *RedisStore) key(id string) string { return s.prefix + ":captcha:" + id }

// Save writes the answer with a TTL.
func (s *RedisStore) Save(ctx context.Context, id string, answer Answer, ttl time.Duration) error {
	if err := s.check(); err != nil {
		return err
	}
	if ttl <= 0 {
		// A zero TTL makes Redis delete the key immediately, making the captcha unusable —
		// failing explicitly beats a silent always-fail.
		return fmt.Errorf("captcha: TTL is not configured")
	}
	b, err := MarshalAnswer(answer)
	if err != nil {
		return fmt.Errorf("captcha: failed to encode answer: %w", err)
	}
	if err := s.rdb.Set(ctx, s.key(id), b, ttl).Err(); err != nil {
		return fmt.Errorf("captcha: failed to store captcha answer: %w", err)
	}
	return nil
}

// Read reads and deletes the answer (one-shot).
func (s *RedisStore) Read(ctx context.Context, id string) (Answer, error) {
	if err := s.check(); err != nil {
		return Answer{}, err
	}
	// GetDel is atomic: the answer is consumed exactly once even under concurrent attempts.
	b, err := s.rdb.GetDel(ctx, s.key(id)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return Answer{}, ErrNotFound
		}
		return Answer{}, fmt.Errorf("captcha: failed to read captcha answer: %w", err)
	}
	answer, err := UnmarshalAnswer(b)
	if err != nil {
		// Corrupted value: the key is already consumed by GetDel, so there is nothing to clean up.
		return Answer{}, ErrNotFound
	}
	return answer, nil
}

// check verifies the client is usable (fail explicitly rather than silently accepting everything).
func (s *RedisStore) check() error {
	if s == nil || s.rdb == nil {
		return errors.New("captcha: redis client is nil")
	}
	return nil
}
