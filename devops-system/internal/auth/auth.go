// Package auth is the authentication facade: JWT dual tokens and Redis
// session management.
//
// Issuance/verification and session state are delegated to shared
// implementations (this package no longer holds security algorithm details):
//
//   - devops-infra/security/jwt: RS256 + kid issuance/parsing, with claims
//     carrying uid/username/sid/tid/tcode and a short lifetime
//     (access_expire). Existing HS256 tokens all become invalid on upgrade;
//     refresh sessions are unaffected.
//
//   - devops-infra/security/session: Redis session state (tenant-partitioned
//     keyspace, key shapes unchanged):
//     t{tid}:auth:refresh:{sid} = user_id, TTL = refresh_expire
//     t{tid}:auth:sids:{uid}    = SET<sid> (used by RevokeAllSessions)
//     auth:sid2uid:{sid}        = "tid:uid" (the refresh endpoint locates the
//     session owner by sid)
//
//   - devops-infra/security/middleware: framework-agnostic authentication
//     decisions (Bearer parsing → token parsing → session state), with
//     Authenticate consumed by the Gin middleware.
//
//   - Every request verifies the session exists — after a forced logout
//     (logout/password change/disable), old access tokens 401 immediately
//
//   - The refresh endpoint rotates the sid (creates the new session before
//     revoking the old sid), in line with rotation best practices
package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/stanley760/devops/devops-infra/errs"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitymiddleware "github.com/stanley760/devops/devops-infra/security/middleware"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"

	"github.com/stanley760/devops/devops-system/internal/config"
)

// AccessClaims is the access token payload (field names stay compatible with the JSON tags, so callers are unaffected).
type AccessClaims struct {
	UserID     int64  `json:"uid"`
	Username   string `json:"username"`
	SID        string `json:"sid"`
	TenantID   int64  `json:"tid"`
	TenantCode string `json:"tcode"`
	securityjwt.RegisteredClaims
}

// FromShared converts shared claims → system AccessClaims (a one-time
// conversion at the middleware boundary, so business handlers do not depend
// on infrastructure packages).
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

// ClaimsFrom reads the JWT payload from the context (available after
// HumaJWT). Business handlers only ever see AccessClaims and do not depend
// on the infrastructure claims type.
func ClaimsFrom(ctx context.Context) *AccessClaims {
	return FromShared(securitymiddleware.HumaCurrentClaims(ctx))
}

// Manager is the JWT + session manager (a facade over the shared implementations).
type Manager struct {
	cfg        config.JWTConfig
	tokens     *securityjwt.Manager
	sessions   securitysession.Store
	authn      *securitymiddleware.Authenticator
	refreshTTL time.Duration
}

// New creates the manager: injects the shared JWT manager and session store (wiring is in cmd/root.go).
func New(cfg config.JWTConfig, tokens *securityjwt.Manager, sessions securitysession.Store) *Manager {
	return &Manager{
		cfg:        cfg,
		tokens:     tokens,
		sessions:   sessions,
		authn:      securitymiddleware.NewAuthenticator(tokens, sessions),
		refreshTTL: cfg.RefreshExpire,
	}
}

// JWKS returns the JWKS document of the verification public keys ({"keys":[...]}) for Higress/services to fetch.
func (m *Manager) JWKS() (map[string]any, error) {
	return m.tokens.JWKS()
}

// Authenticate is the shared authentication decision: Bearer parsing →
// token parsing → session-state validation. It returns typed errors
// (ErrUnauthorized/ErrSessionInactive/ErrInfrastructure).
func (m *Manager) Authenticate(ctx context.Context, bearerHeader string) (*securityjwt.Claims, error) {
	return m.authn.Authenticate(ctx, bearerHeader)
}

// Issue signs the dual tokens (shared by login and refresh rotation).
// It returns the access token, the refresh token (i.e. the sid), and the
// access token lifetime in seconds.
func (m *Manager) Issue(ctx context.Context, userID int64, username string, tenantID int64, tenantCode string) (access, refresh string, expiresIn int64, err error) {
	sid := uuid.NewString()
	pair, err := m.tokens.Issue(ctx, securityjwt.Subject{
		UserID: userID, Username: username, TenantID: tenantID, TenantCode: tenantCode,
	}, sid)
	if err != nil {
		return "", "", 0, errs.Wrap(err, errs.CodeInternal, "failed to sign token")
	}
	if err := m.sessions.Create(ctx, securitysession.Session{
		TenantID: tenantID, UserID: userID, SessionID: sid, TTL: m.refreshTTL,
	}); err != nil {
		return "", "", 0, errs.Wrap(err, errs.CodeInternal, "failed to write session")
	}
	return pair.AccessToken, sid, pair.AccessExpiresIn, nil
}

// Parse validates and parses the access token (no Redis lookup; session state is validated separately by the middleware).
func (m *Manager) Parse(token string) (*AccessClaims, error) {
	shared, err := m.tokens.ParseAccess(token)
	if err != nil {
		return nil, errs.New(errs.CodeUnauthorized, "not logged in or session expired")
	}
	return FromShared(shared), nil
}

// SessionActive reports whether the session is still valid (the key to immediate invalidation after logout/forced logout/password change).
func (m *Manager) SessionActive(ctx context.Context, claims *AccessClaims) bool {
	active, err := m.sessions.Active(ctx, securitysession.SessionRef{
		TenantID: claims.TenantID, UserID: claims.UserID, SessionID: claims.SID,
	})
	return err == nil && active
}

// LookupSID locates the session owner from the refresh token (sid) (used by the refresh endpoint).
func (m *Manager) LookupSID(ctx context.Context, sid string) (tenantID, userID int64, ok bool) {
	return m.sessions.Lookup(ctx, sid)
}

// SIDMatch verifies that the sid session really belongs to the user.
func (m *Manager) SIDMatch(ctx context.Context, tenantID int64, sid string, userID int64) bool {
	matched, err := m.sessions.Matches(ctx, securitysession.SessionRef{
		TenantID: tenantID, UserID: userID, SessionID: sid,
	}, userID)
	return err == nil && matched
}

// Rotate rotates the session: signs a new dual token and revokes the old sid
// (called after service.Refresh validation passes). The new session is
// created before the old one is revoked; a revocation failure is only logged
// (the old session expires naturally and does not block the new one).
func (m *Manager) Rotate(ctx context.Context, oldSID string, userID int64, username string, tenantID int64, tenantCode string) (access, refresh string, expiresIn int64, err error) {
	access, refresh, expiresIn, err = m.Issue(ctx, userID, username, tenantID, tenantCode)
	if err != nil {
		return "", "", 0, err
	}
	if err := m.sessions.Revoke(ctx, securitysession.SessionRef{
		TenantID: tenantID, UserID: userID, SessionID: oldSID,
	}); err != nil {
		slog.Error("failed to revoke old session during rotation", "err", err, "tid", tenantID, "uid", userID)
	}
	return access, refresh, expiresIn, nil
}

// Logout revokes the current session.
func (m *Manager) Logout(ctx context.Context, claims *AccessClaims) {
	if err := m.sessions.Revoke(ctx, securitysession.SessionRef{
		TenantID: claims.TenantID, UserID: claims.UserID, SessionID: claims.SID,
	}); err != nil {
		slog.Error("failed to revoke session on logout", "err", err, "tid", claims.TenantID, "uid", claims.UserID)
	}
}

// RevokeAllSessions revokes all of a user's sessions (password change/password reset/disable/delete user, disable tenant).
func (m *Manager) RevokeAllSessions(ctx context.Context, tenantID, userID int64) {
	if err := m.sessions.RevokeUser(ctx, tenantID, userID); err != nil {
		slog.Error("failed to revoke user sessions", "err", err, "tid", tenantID, "uid", userID)
	}
}

// RevokeTenantSessions revokes all of a tenant's sessions (disable tenant → everyone offline).
func (m *Manager) RevokeTenantSessions(ctx context.Context, tenantID int64) {
	if err := m.sessions.RevokeTenant(ctx, tenantID); err != nil {
		slog.Error("failed to revoke tenant sessions", "err", err, "tid", tenantID)
	}
}
