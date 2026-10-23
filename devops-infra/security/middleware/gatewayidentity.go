package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"
)

// The headers the gateway injects after verifying the JWT (Higress jwt-auth's
// claims_to_headers; override=true means client-forged headers with the same
// name get overwritten).
//
// These are the identity carrier after the "verification happens only at the
// gateway" model: the gateway verifies the signature, then hands the verified
// claims to the backend in clear headers. The backend no longer re-verifies the
// signature — it only checks that the session is still active (see
// GatewayIdentityAuthenticator).
const (
	// HeaderGatewayUID is the verified user id.
	HeaderGatewayUID = "X-Jwt-Uid"
	// HeaderGatewayUsername is the verified username.
	HeaderGatewayUsername = "X-Jwt-Username"
	// HeaderGatewaySessionID is the verified session id (sid claim).
	//
	// Required: without it the backend cannot check whether the session has been
	// revoked, and "kick offline" would only take effect once the access token
	// expires (2h by default).
	HeaderGatewaySessionID = "X-Jwt-Sid"
	// HeaderGatewayTenantID is the verified tenant id.
	HeaderGatewayTenantID = "X-Tenant-ID"
	// HeaderGatewayTenantCode is the verified tenant code.
	HeaderGatewayTenantCode = "X-Tenant-Code"
)

// ErrGatewayIdentityMissing means the request did not carry gateway-injected identity headers.
//
// It is distinct from ErrUnauthorized: the caller did not fail authentication,
// it simply did not come through the gateway.
var ErrGatewayIdentityMissing = errors.New("middleware: gateway identity missing")

// GatewayIdentity is the caller identity the gateway verified and passed along.
type GatewayIdentity struct {
	UID        int64
	Username   string
	SessionID  string
	TenantID   int64
	TenantCode string
}

type gatewayIdentityContextKey struct{}

// WithGatewayIdentity stores the identity in the context (injected by the gateway
// identity middleware, read by the authenticator).
func WithGatewayIdentity(ctx context.Context, id *GatewayIdentity) context.Context {
	return context.WithValue(ctx, gatewayIdentityContextKey{}, id)
}

// GatewayIdentityFrom reads the identity from the context.
func GatewayIdentityFrom(ctx context.Context) (*GatewayIdentity, bool) {
	id, ok := ctx.Value(gatewayIdentityContextKey{}).(*GatewayIdentity)
	return id, ok && id != nil
}

// ParseGatewayIdentity parses the identity headers injected by the gateway.
//
// All of it must be present: a half-populated identity means something was
// tampered with or the gateway config is wrong, and guessing which part to trust
// is exactly how privilege escalation happens.
func ParseGatewayIdentity(h http.Header) (*GatewayIdentity, error) {
	uid, err := strconv.ParseInt(strings.TrimSpace(h.Get(HeaderGatewayUID)), 10, 64)
	if err != nil || uid <= 0 {
		return nil, fmt.Errorf("%w: %s is missing or invalid", ErrGatewayIdentityMissing, HeaderGatewayUID)
	}
	tid, err := strconv.ParseInt(strings.TrimSpace(h.Get(HeaderGatewayTenantID)), 10, 64)
	if err != nil || tid <= 0 {
		return nil, fmt.Errorf("%w: %s is missing or invalid", ErrGatewayIdentityMissing, HeaderGatewayTenantID)
	}
	sid := strings.TrimSpace(h.Get(HeaderGatewaySessionID))
	if sid == "" {
		return nil, fmt.Errorf("%w: %s is missing", ErrGatewayIdentityMissing, HeaderGatewaySessionID)
	}
	username := strings.TrimSpace(h.Get(HeaderGatewayUsername))
	if username == "" {
		return nil, fmt.Errorf("%w: %s is missing", ErrGatewayIdentityMissing, HeaderGatewayUsername)
	}
	return &GatewayIdentity{
		UID:        uid,
		Username:   username,
		SessionID:  sid,
		TenantID:   tid,
		TenantCode: strings.TrimSpace(h.Get(HeaderGatewayTenantCode)),
	}, nil
}

// GatewayIdentityAuthenticator resolves the caller from the gateway-injected identity.
//
// It deliberately does NOT verify the JWT signature: the gateway already did that
// (that is the whole point of the "one verification" model). What it still does
// is check the session state, so a kicked-out user is rejected immediately
// rather than staying valid until the access token expires.
//
// The security premise: these headers are only trustworthy when the request
// actually came from the gateway. That is guaranteed by network isolation
// (K8s NetworkPolicy / the dedicated compose network) plus the trusted-peer
// check in the gateway identity middleware — without both, anyone able to reach
// the backend could forge an identity.
type GatewayIdentityAuthenticator struct {
	sessions SessionStore
}

// NewGatewayIdentityAuthenticator creates the authenticator.
func NewGatewayIdentityAuthenticator(sessions SessionStore) *GatewayIdentityAuthenticator {
	return &GatewayIdentityAuthenticator{sessions: sessions}
}

// Authenticate implements BearerAuthenticator.
//
// The bearer header is ignored on purpose: the token has already been verified
// by the gateway, and the raw token carries no additional information.
func (a *GatewayIdentityAuthenticator) Authenticate(ctx context.Context, _ string) (*securityjwt.Claims, error) {
	if a == nil || a.sessions == nil {
		return nil, fmt.Errorf("%w: gateway identity authenticator is not initialized", ErrInfrastructure)
	}
	id, ok := GatewayIdentityFrom(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: requests must come through the gateway", ErrGatewayIdentityMissing)
	}
	active, err := a.sessions.Active(ctx, securitysession.SessionRef{
		TenantID:  id.TenantID,
		UserID:    id.UID,
		SessionID: id.SessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInfrastructure, err)
	}
	if !active {
		return nil, ErrSessionInactive
	}
	return &securityjwt.Claims{
		UID:        id.UID,
		Username:   id.Username,
		SessionID:  id.SessionID,
		TenantID:   id.TenantID,
		TenantCode: id.TenantCode,
	}, nil
}
