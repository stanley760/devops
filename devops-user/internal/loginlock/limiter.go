// Package loginlock provides C-side login failure rate limiting and temporary
// account lockout (Redis-based).
//
// ## Why it is needed
//
// bcrypt only slows down a single brute-force attempt (cost=10 is tens of
// milliseconds), but cannot stop high-speed attempts. The C side has open
// registration and enumerable usernames, so credential stuffing and
// brute-force risks are higher than for an internal admin console. Two layers
// must be combined:
//
//  1. **IP-dimension rate limiting**: stops "one IP batch-testing many
//     usernames";
//  2. **Account-dimension lockout**: stops "many IPs concentrating on one
//     account".
//
// Neither can be omitted: IP-only limiting can be bypassed with proxy pools,
// and account-only lockout mishandles "an attacker scanning all usernames from
// one IP" (those accounts are never locked, but the IP should be blocked).
//
// ## Counting and unlocking
//
// The sliding window uses fixed-window counting (Redis INCR + EXPIRE): simple
// to implement and cheap, at the cost of a possible 2x burst at window
// boundaries. Acceptable for a low-frequency scenario like login.
//
// Lockout is two-stage, "failure count reaches threshold → lock for TTL":
// failures accumulate first, and once the threshold is met a lock key is
// written with a TTL, during which requests are rejected outright; any
// successful login clears the count.
package loginlock

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrLocked means the account is temporarily locked (failure threshold
// reached).
var ErrLocked = errors.New("account is locked, please try again later")

// ErrTooManyAttempts means IP-dimension rate limiting has triggered.
var ErrTooManyAttempts = errors.New("too many attempts, please try again later")

// Redis key suffixes for the failure counter and the account lock.
const (
	// failSuffix is the failure counter key suffix.
	failSuffix = "auth:loginfail"
	// lockSuffix is the account lock key suffix.
	lockSuffix = "auth:loginlock"
)

// Config holds the rate limiting and lockout parameters.
type Config struct {
	// MaxIPFails is the max failures allowed per IP within the window; 0 means
	// unlimited.
	MaxIPFails int
	// IPWindow is the IP counter window.
	IPWindow time.Duration
	// MaxUserFails is the max failures allowed per account within the window;
	// 0 means no account lockout.
	MaxUserFails int
	// UserWindow is the account failure counter window.
	UserWindow time.Duration
	// LockTTL is the account lockout duration; how long the account stays
	// locked after the failure threshold is met.
	LockTTL time.Duration
	// Enabled is the master switch; when false, everything is allowed through
	// (can be turned off in dev/test).
	Enabled bool
}

// DefaultConfig returns a conservative set of defaults.
//
// Value trade-offs:
//   - MaxIPFails=20 / 5min: normal users almost never hit it, while scripted
//     batch attempts are blocked.
//   - MaxUserFails=5 / 15min: allows a user a few typos while stopping
//     sustained brute force.
//   - LockTTL=15min: short enough not to lock users out permanently, long
//     enough to make brute force pointless.
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		MaxIPFails:   20,
		IPWindow:     5 * time.Minute,
		MaxUserFails: 5,
		UserWindow:   15 * time.Minute,
		LockTTL:      15 * time.Minute,
	}
}

// Limiter is the login failure rate limiter and account lockout.
type Limiter struct {
	rdb    *goredis.Client
	prefix string
	cfg    Config
}

// New creates a limiter. prefix should be "t1" (the C side is single-tenant).
func New(rdb *goredis.Client, prefix string, cfg Config) *Limiter {
	return &Limiter{rdb: rdb, prefix: prefix, cfg: cfg}
}

// failKey is the IP-dimension failure counter key.
func (l *Limiter) failKey(clientIP string) string {
	return fmt.Sprintf("%s:%s:%s", l.prefix, failSuffix, clientIP)
}

// lockKey is the account-dimension lock key.
func (l *Limiter) lockKey(username string) string {
	return fmt.Sprintf("%s:%s:%s", l.prefix, lockSuffix, username)
}

// Check is called before a login attempt: allows it through when the account
// is not locked and the IP is within limits.
//
// Returning ErrLocked / ErrTooManyAttempts means rejection.
func (l *Limiter) Check(ctx context.Context, clientIP, username string) error {
	if !l.cfg.Enabled || l.rdb == nil {
		return nil
	}
	// Account lockout is checked first (more specific, giving the user an
	// actionable message).
	if l.cfg.MaxUserFails > 0 {
		locked, err := l.rdb.Exists(ctx, l.lockKey(username)).Result()
		if err != nil {
			// Redis failure: fail-closed and reject, or allow through? The
			// choice is fail-open with logging — the login endpoint being
			// unavailable is worse than letting a few attempts through, but
			// production should have Redis HA; allowing through here
			// preserves availability.
			return nil
		}
		if locked > 0 {
			return ErrLocked
		}
	}
	// IP-dimension rate limiting.
	if l.cfg.MaxIPFails > 0 {
		n, err := l.rdb.Get(ctx, l.failKey(clientIP)).Int64()
		if err != nil && !errors.Is(err, goredis.Nil) {
			return nil // same as above: fail-open
		}
		if n >= int64(l.cfg.MaxIPFails) {
			return ErrTooManyAttempts
		}
	}
	return nil
}

// Fail records a login failure; locks the account when the account threshold
// is reached.
func (l *Limiter) Fail(ctx context.Context, clientIP, username string) {
	if !l.cfg.Enabled || l.rdb == nil {
		return
	}
	if l.cfg.MaxIPFails > 0 {
		key := l.failKey(clientIP)
		pipe := l.rdb.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, l.cfg.IPWindow)
		_, _ = pipe.Exec(ctx)
	}
	if l.cfg.MaxUserFails > 0 {
		key := l.failKey("u:" + username)
		pipe := l.rdb.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, l.cfg.UserWindow)
		_, _ = pipe.Exec(ctx)

		n, err := l.rdb.Get(ctx, key).Int64()
		if err == nil && n >= int64(l.cfg.MaxUserFails) {
			// Lock as soon as the threshold is met; NX ensures an existing
			// lock is not extended, avoiding indefinite prolongation.
			l.rdb.SetNX(ctx, l.lockKey(username), "1", l.cfg.LockTTL)
		}
	}
}

// Success clears the account's failure count after a successful login (the IP
// count is not cleared, otherwise normal users who mistype a few times would
// permanently trigger IP rate limiting).
func (l *Limiter) Success(ctx context.Context, username string) {
	if !l.cfg.Enabled || l.rdb == nil {
		return
	}
	l.rdb.Del(ctx, l.failKey("u:"+username))
}

// Reset is the admin unlock (called after ban handling / a user appeal).
func (l *Limiter) Reset(ctx context.Context, username string) {
	if l.rdb == nil {
		return
	}
	pipe := l.rdb.Pipeline()
	pipe.Del(ctx, l.lockKey(username))
	pipe.Del(ctx, l.failKey("u:"+username))
	_, _ = pipe.Exec(ctx)
}
