// Package auth is the authentication facade: JWT dual tokens and Redis session
// management (C-side form).
//
// Issuance/verification/sessions are all delegated to the shared
// implementation; this package only adapts C-side semantics:
//
//   - devops-infra/security/jwt: RS256 + kid issuance/parsing, short-lived
//     access token;
//   - devops-infra/security/session: Redis session state, verified on every
//     request, so old access tokens die immediately after logout/password
//     change/ban;
//   - devops-infra/security/middleware: framework-agnostic authentication
//     decisions (Bearer parsing → token parsing → session state), with
//     Authenticate consumed by middleware.
//
// ## Differences from devops-system (deliberate on the C side, not an
// oversight)
//
// The shared session.Store partitions the Redis keyspace by TenantID
// (t{tid}:auth:*) and enforces TenantID > 0, while the C-side devops-user is
// single-tenant. To reuse that implementation, this package introduces a
// **fixed sentinel tenant ID** (DefaultTenantID = 1) to carry the keyspace and
// claims.tid:
//
//   - it does not represent a real tenant, only keyspace isolation and an
//     interface placeholder;
//   - business code must not do tenant filtering based on claims.TenantID (the
//     C side has no tenant concept);
//   - if the C side ever adopts multi-tenancy, a new independent
//     session.Store implementation should be created rather than continuing to
//     reuse the sentinel value.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitymiddleware "github.com/stanley760/devops/devops-infra/security/middleware"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"

	"github.com/stanley760/devops/devops-user/internal/config"
	"github.com/stanley760/devops/devops-user/internal/refreshtoken"
)

// DefaultTenantID is the positive tenant ID required by the shared
// session.Store (a C-side single-tenant sentinel value).
//
// Used only as a placeholder for the Redis keyspace and claims.tid; **it does
// not represent a real tenant**. Business code must not make tenant isolation
// decisions based on TenantID.
const DefaultTenantID int64 = 1

// AccessClaims is the access token payload (JSON tags aligned with the shared
// JWT implementation).
type AccessClaims struct {
	UserID     int64  `json:"uid"`
	Username   string `json:"username"`
	SID        string `json:"sid"`
	TenantID   int64  `json:"tid"`
	TenantCode string `json:"tcode"`
	securityjwt.RegisteredClaims
}

// FromShared converts shared claims to this service's AccessClaims (a one-time
// conversion at the middleware boundary, so business handlers do not depend on
// infrastructure package types).
func FromShared(c *securityjwt.Claims) *AccessClaims {
	if c == nil {
		return nil
	}
	return &AccessClaims{
		UserID:           c.UID,
		Username:         c.Username,
		SID:              c.SessionID,
		TenantID:         c.TenantID,
		TenantCode:       c.TenantCode,
		RegisteredClaims: c.RegisteredClaims,
	}
}

// ClaimsFrom gets the JWT payload from the context (available after the JWT
// middleware).
func ClaimsFrom(ctx context.Context) *AccessClaims {
	return FromShared(securitymiddleware.HumaCurrentClaims(ctx))
}

// Manager is the JWT + session manager (a facade over the shared
// implementation).
type Manager struct {
	cfg      config.JWTConfig
	tokens   *securityjwt.Manager
	sessions securitysession.Store
	// refreshTokens stores refresh tokens (opaque random strings).
	refreshTokens *refreshtoken.Store
	authn         *securitymiddleware.Authenticator
}

// New creates a manager. Wiring is in cmd/root.go.
//
// When refreshTokens is nil, Refresh returns ErrRefreshInvalid (no nil
// dereference), while access token issuance and verification still work
// normally — making it easy to toggle the refresh feature per environment.
func New(cfg config.JWTConfig, tokens *securityjwt.Manager, sessions securitysession.Store, refresh *refreshtoken.Store) *Manager {
	return &Manager{
		cfg:           cfg,
		tokens:        tokens,
		sessions:      sessions,
		refreshTokens: refresh,
		authn:         securitymiddleware.NewAuthenticator(tokens, sessions),
	}
}

// JWKS returns the JWKS document carrying the verification public key, for
// Higress jwt-auth and other services to verify signatures.
func (m *Manager) JWKS() (map[string]any, error) {
	return m.tokens.JWKS()
}

// Authenticate is the shared authentication decision: Bearer parsing → token
// parsing → session state verification. Middleware consumes this method, so it
// must satisfy securitymw.BearerAuthenticator.
func (m *Manager) Authenticate(ctx context.Context, bearerHeader string) (*securityjwt.Claims, error) {
	return m.authn.Authenticate(ctx, bearerHeader)
}

// Issue issues an access + refresh token pair.
//
// refresh is a **256-bit random opaque string** (the server stores only the
// hash), **not the access token** — reusing access as refresh would let an
// attacker renew indefinitely, and logout/ban could not truly revoke it.
//
// The access lifetime is governed by jwt.access_expire; the refresh lifetime
// is governed by the refreshtoken.Store TTL (= jwt.refresh_expire); the two
// are independent.
func (m *Manager) Issue(ctx context.Context, userID int64, username string) (TokenPair, error) {
	sid := uuid.NewString()
	pair, err := m.tokens.Issue(ctx, securityjwt.Subject{
		UserID:     userID,
		Username:   username,
		TenantID:   DefaultTenantID,
		TenantCode: "c-end",
	}, sid)
	if err != nil {
		return TokenPair{}, err
	}
	if err := m.sessions.Create(ctx, securitysession.Session{
		TenantID:  DefaultTenantID,
		UserID:    userID,
		SessionID: sid,
		TTL:       m.cfg.RefreshExpire,
	}); err != nil {
		return TokenPair{}, err
	}

	plain, hash, err := refreshtoken.Generate()
	if err != nil {
		return TokenPair{}, err
	}
	if err := m.refreshTokens.Save(ctx, hash, userID); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  pair.AccessToken,
		RefreshToken: plain,
		ExpiresIn:    pair.AccessExpiresIn,
		IssuedAt:     time.Now().UnixMilli(),
	}, nil
}

// TokenPair is the token pair issuance result.
type TokenPair struct {
	AccessToken  string `json:"access_token" doc:"access token"`
	RefreshToken string `json:"refresh_token" doc:"refresh token (opaque random string, not JWT)"`
	ExpiresIn    int64  `json:"expires_in" doc:"access token lifetime (seconds)"`
	IssuedAt     int64  `json:"issued_at" doc:"issued at (Unix milliseconds)"`
}

// Refresh exchanges a refresh token for a new token pair (rotating: the old
// refresh token dies immediately).
//
// username is looked up from the database by the caller and passed in — if the
// username is missing on refresh, the new access token's username claim would
// be empty, and any downstream audit/display relying on that claim would be
// wrong.
//
// Semantics:
//   - refresh does not exist / expired / rotated / malformed → ErrRefreshInvalid
//     (four cases merged to prevent probing);
//   - rotation is atomic; only one concurrent refresh can succeed (see
//     refreshtoken.Store.Rotate).
func (m *Manager) Refresh(ctx context.Context, refreshPlain, username string) (TokenPair, error) {
	if m.refreshTokens == nil {
		return TokenPair{}, ErrRefreshInvalid
	}
	oldHash := refreshtoken.Hash(refreshPlain)
	userID, err := m.refreshTokens.Lookup(ctx, oldHash)
	if err != nil {
		return TokenPair{}, ErrRefreshInvalid
	}

	// access and refresh share the same sid after rotation: this way rotating
	// a refresh token does not need to rebuild the session, and the session
	// the user is currently using is not kicked offline by a refresh.
	sid := uuid.NewString()
	pair, err := m.tokens.Issue(ctx, securityjwt.Subject{
		UserID:     userID,
		Username:   username,
		TenantID:   DefaultTenantID,
		TenantCode: "c-end",
	}, sid)
	if err != nil {
		return TokenPair{}, err
	}

	newPlain, newHash, err := refreshtoken.Generate()
	if err != nil {
		return TokenPair{}, err
	}
	// Atomic rotation: look up old → delete old → write new. Failure means it
	// was already used concurrently.
	if err := m.refreshTokens.Rotate(ctx, oldHash, newHash, userID); err != nil {
		return TokenPair{}, ErrRefreshInvalid
	}
	if err := m.sessions.Create(ctx, securitysession.Session{
		TenantID:  DefaultTenantID,
		UserID:    userID,
		SessionID: sid,
		TTL:       m.cfg.RefreshExpire,
	}); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  pair.AccessToken,
		RefreshToken: newPlain,
		ExpiresIn:    pair.AccessExpiresIn,
		IssuedAt:     time.Now().UnixMilli(),
	}, nil
}

// RevokeRefresh invalidates a specific refresh token (used on password change
// / logout).
func (m *Manager) RevokeRefresh(ctx context.Context, refreshPlain string) error {
	if m.refreshTokens == nil || refreshPlain == "" {
		return nil
	}
	return m.refreshTokens.Delete(ctx, refreshtoken.Hash(refreshPlain))
}

// RevokeUserSessions forces the user **offline on all devices**: revoking all
// of their refresh tokens and all access sessions.
//
// It combines revocation on both sides:
//   - refresh side: refreshtoken.RevokeUser (relies on the user → hashes
//     reverse index);
//   - access side: session.Store.RevokeUser (enumerates the sids set).
//
// Both are "all devices" — every session of the user on phone/tablet/PC dies
// at once.
//
// ⚠️ An issued access JWT remains usable until natural expiry even after
// revocation: every request in this service goes through HumaAuthenticate,
// which verifies the Redis session state (infra/security/middleware), so after
// revocation **the next request gets 401**, with no need to wait for JWT
// expiry. That is the benefit of "stateful verification".
func (m *Manager) RevokeUserSessions(ctx context.Context, userID int64) error {
	var errs []error
	if m.refreshTokens != nil {
		if err := m.refreshTokens.RevokeUser(ctx, userID); err != nil {
			errs = append(errs, err)
		}
	}
	if err := m.sessions.RevokeUser(ctx, DefaultTenantID, userID); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// RevokeUserSessionsExcept forces offline on all devices but keeps the device
// corresponding to a given refresh token (the current caller's device), for
// the "kick all other devices after a password change" scenario.
//
// When keepPlain is empty it is equivalent to RevokeUserSessions (kick
// everything).
func (m *Manager) RevokeUserSessionsExcept(ctx context.Context, userID int64, keepPlain string) error {
	var errs []error
	if m.refreshTokens != nil {
		var keepHash string
		if keepPlain != "" {
			keepHash = refreshtoken.Hash(keepPlain)
		}
		if err := m.refreshTokens.RevokeUserExcept(ctx, userID, keepHash); err != nil {
			errs = append(errs, err)
		}
	}
	// The access side cannot reverse-lookup sid from a refresh token (the
	// session side only has a user → sids set), so there is no Except here —
	// the kept device is kicked too and needs to log in once more. If the
	// product requires "keep the current device", the session side needs a
	// sid → refreshHash reverse index.
	if err := m.sessions.RevokeUser(ctx, DefaultTenantID, userID); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// ErrRefreshInvalid means the refresh token is invalid (does not exist /
// expired / rotated / malformed).
//
// The cases are merged into a single error: distinguishing them would let an
// attacker tell whether a token was ever valid.
var ErrRefreshInvalid = errors.New("refresh token is invalid or has expired")

// ErrAccountDisabled means the account has been banned.
var ErrAccountDisabled = errors.New("account has been banned")

// Parse parses an access token (no session state verification; for internal
// logic that only needs the payload).
func (m *Manager) Parse(token string) (*AccessClaims, error) {
	claims, err := m.tokens.ParseAccess(token)
	if err != nil {
		return nil, err
	}
	return FromShared(claims), nil
}

// SessionActive reports whether the session is still valid (invalidated
// immediately after kick/password change/ban).
func (m *Manager) SessionActive(ctx context.Context, claims *AccessClaims) bool {
	if claims == nil {
		return false
	}
	active, err := m.sessions.Active(ctx, securitysession.SessionRef{
		TenantID:  DefaultTenantID,
		UserID:    claims.UserID,
		SessionID: claims.SID,
	})
	return err == nil && active
}

// Logout revokes the current session.
func (m *Manager) Logout(ctx context.Context, claims *AccessClaims) error {
	if claims == nil {
		return nil
	}
	return m.sessions.Revoke(ctx, securitysession.SessionRef{
		TenantID:  DefaultTenantID,
		UserID:    claims.UserID,
		SessionID: claims.SID,
	})
}
