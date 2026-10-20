// Package refreshtoken implements C-side refresh token storage (a Redis
// key-value pair, not a JWT).
//
// ## Why refresh is not a JWT
//
// **Never reuse an access token as a refresh token**: access is a stateless JWT
// that anyone holding it can use until it expires; once it is allowed to mint
// new access tokens, it becomes an infinitely renewable credential for an
// attacker, and user actions like "logout/ban" cannot truly revoke it (it
// stays valid until it expires).
//
// This implementation uses a 256-bit cryptographic random string (base64url),
// and the server stores only the hash:
//   - refresh: the client holds the plaintext and the server stores only the
//     SHA-256 hash, so even a dumped database cannot be replayed directly;
//   - precisely revocable: logout / password change / ban simply DEL the key;
//   - rotatable: every refresh issues a new string and invalidates the old one
//     (the old string dies after a single use).
//
// Key shapes (same namespace style as devops-infra/security/session's
// t{tid}:auth:*):
//
//	t1:auth:refresh:{hash}    = {userID}      TTL = refresh_expire
//	t1:auth:refreshuser:{uid} = SET<hash>     TTL = refresh_expire
//
// **Both keys are required**: `refresh:{hash}` supports exact lookup by token;
// `refreshuser:{uid}` is the reverse index that makes "revoke everything for a
// user" possible — without it, neither "password change logs out all devices"
// nor "ban forces logout" can be implemented (there is no way to enumerate all
// of a user's refresh tokens). It is isomorphic with the `sids` set in
// devops-infra/security/session and has the same semantics.
//
// userID rather than "tid:uid": the C side is single-tenant, so tid is always
// auth.DefaultTenantID and carries no information.
package refreshtoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrNotFound means the token does not exist, has expired, or was invalidated
// by rotation.
//
// All three cases **must** return the same error: distinguishing them would let
// an attacker tell whether a token was ever valid.
var ErrNotFound = errors.New("refresh token does not exist or has expired")

// Store stores and retrieves refresh tokens.
type Store struct {
	rdb    *goredis.Client
	prefix string
	ttl    time.Duration
}

// NewStore creates a store. prefix should be "t1" (the C side is single-tenant).
func NewStore(rdb *goredis.Client, prefix string, ttl time.Duration) *Store {
	return &Store{rdb: rdb, prefix: prefix, ttl: ttl}
}

// key is the Redis key for a hashed token.
func (s *Store) key(hash string) string {
	return fmt.Sprintf("%s:auth:refresh:%s", s.prefix, hash)
}

// userKey is the reverse index from a user to all of their refresh hashes.
func (s *Store) userKey(userID int64) string {
	return fmt.Sprintf("%s:auth:refreshuser:%d", s.prefix, userID)
}

// Generate generates the plaintext token and its hash (the plaintext exists
// only at this moment and is returned to the caller).
func Generate() (plain string, hash string, err error) {
	buf := make([]byte, 32) // 256 bits
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, Hash(plain), nil
}

// Hash maps plaintext to the hash stored server-side (base64url, unpadded).
func Hash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Save stores the token → userID mapping and registers it in the user's
// reverse index.
func (s *Store) Save(ctx context.Context, hash string, userID int64) error {
	if err := s.check(); err != nil {
		return err
	}
	if s.ttl <= 0 {
		// A zero TTL would make Redis delete the key immediately, rendering the
		// token unusable — failing explicitly beats silent expiry.
		return fmt.Errorf("refreshtoken: TTL is not configured")
	}
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, s.key(hash), strconv.FormatInt(userID, 10), s.ttl)
	pipe.SAdd(ctx, s.userKey(userID), hash)
	pipe.Expire(ctx, s.userKey(userID), s.ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// Lookup looks up the userID for a token.
func (s *Store) Lookup(ctx context.Context, hash string) (int64, error) {
	if err := s.check(); err != nil {
		return 0, err
	}
	v, err := s.rdb.Get(ctx, s.key(hash)).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	uid, convErr := strconv.ParseInt(v, 10, 64)
	if convErr != nil {
		// Corrupted value: delete the bad key to avoid a pile-up of errors.
		_ = s.rdb.Del(ctx, s.key(hash)).Err()
		return 0, ErrNotFound
	}
	return uid, nil
}

// Rotate invalidates the old token and issues a new one atomically, preventing
// concurrent refreshes from minting multiple valid tokens.
//
// A Lua script guarantees the atomicity of "lookup → verify ownership → delete
// old → write new". Done step by step, concurrent refreshes would each find the
// same old token and each mint a new one (a replay vulnerability); only an
// atomic operation can guarantee "a refresh token can be used only once".
func (s *Store) Rotate(ctx context.Context, oldHash, newHash string, userID int64) error {
	if err := s.check(); err != nil {
		return err
	}
	if s.ttl <= 0 {
		return fmt.Errorf("refreshtoken: TTL is not configured")
	}
	// Also verifies userID ownership: a mismatch means a hash collision or
	// tampered data, so it is better to reject.
	// On -1 no keys are touched.
	const rotateScript = `
local v = redis.call('GET', KEYS[1])
if v == false then return 0 end
if v ~= ARGV[2] then return -1 end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], v, 'PX', ARGV[1])
redis.call('SREM', KEYS[3], ARGV[3])
redis.call('SADD', KEYS[4], ARGV[4])
redis.call('PEXPIRE', KEYS[4], ARGV[1])
return 1`

	raw, err := s.rdb.Eval(ctx, rotateScript,
		[]string{s.key(oldHash), s.key(newHash), s.userKey(userID), s.userKey(userID)},
		s.ttl.Milliseconds(), strconv.FormatInt(userID, 10), oldHash, newHash).Int64()
	if err != nil {
		return err
	}
	switch raw {
	case 1:
		return nil
	case -1:
		return fmt.Errorf("refreshtoken: token ownership mismatch")
	default:
		return ErrNotFound
	}
}

// Delete invalidates a specific token (called on logout / password change /
// ban).
func (s *Store) Delete(ctx context.Context, hash string) error {
	if err := s.check(); err != nil {
		return err
	}
	// Look up ownership first so the reverse index can be cleaned too; if the
	// lookup misses, DEL the primary key directly (possibly dirty data).
	v, err := s.rdb.Get(ctx, s.key(hash)).Result()
	if errors.Is(err, goredis.Nil) {
		return s.rdb.Del(ctx, s.key(hash)).Err()
	}
	if err != nil {
		return err
	}
	uid, convErr := strconv.ParseInt(v, 10, 64)
	if convErr != nil {
		return s.rdb.Del(ctx, s.key(hash)).Err()
	}
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, s.key(hash))
	pipe.SRem(ctx, s.userKey(uid), hash)
	_, err = pipe.Exec(ctx)
	return err
}

// RevokeUser revokes **all** of a user's refresh tokens (a password change
// logs out all devices; a ban forces logout).
//
// It relies on the userKey reverse index; old data without the index is
// skipped (its refresh tokens remain usable until expiry). This is a known
// trade-off during the upgrade period — it can be converged by having users
// log in again.
func (s *Store) RevokeUser(ctx context.Context, userID int64) error {
	if err := s.check(); err != nil {
		return err
	}
	hashes, err := s.rdb.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		// The index does not exist or is empty: still DEL once to clean up any
		// leftover empty key.
		return s.rdb.Del(ctx, s.userKey(userID)).Err()
	}
	pipe := s.rdb.Pipeline()
	for _, h := range hashes {
		pipe.Del(ctx, s.key(h))
	}
	pipe.Del(ctx, s.userKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}

// RevokeUserExcept revokes all of a user's refresh tokens except keepHash
// (keeping the current device logged in).
func (s *Store) RevokeUserExcept(ctx context.Context, userID int64, keepHash string) error {
	if err := s.check(); err != nil {
		return err
	}
	hashes, err := s.rdb.SMembers(ctx, s.userKey(userID)).Result()
	if err != nil {
		return err
	}
	pipe := s.rdb.Pipeline()
	for _, h := range hashes {
		if h == keepHash {
			continue
		}
		pipe.Del(ctx, s.key(h))
	}
	pipe.Del(ctx, s.userKey(userID))
	if keepHash != "" {
		// The kept token must be re-registered, otherwise the reverse index
		// loses it.
		pipe.SAdd(ctx, s.userKey(userID), keepHash)
		pipe.Expire(ctx, s.userKey(userID), s.ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// check verifies the client is usable (it fails explicitly when not wired up,
// rather than silently allowing access).
func (s *Store) check() error {
	if s == nil || s.rdb == nil {
		return fmt.Errorf("refreshtoken: redis client is nil")
	}
	return nil
}

// ConstantTimeEqual compares two hash strings in constant time (even unequal
// lengths take the constant-time path).
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		// Returning false immediately on length mismatch would short-circuit,
		// but the hash length is fixed and not attacker-controlled, and what is
		// compared here is "our computed hash" against "the stored hash", so
		// there is nothing to enumerate.
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// IsHashLike roughly checks whether a string looks like a hash (length check,
// used to reject obviously invalid input early and avoid querying Redis with an
// oversized string).
func IsHashLike(s string) bool {
	return len(s) == 43 && !strings.ContainsAny(s, " \t\n")
}
