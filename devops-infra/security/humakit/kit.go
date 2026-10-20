// Package humakit is the business middleware suite: assembly and chain composition of
// JWT auth → tenant → in-domain/platform authorization → data permission resolution.
//
// Layering: decisions live in security/middleware and tenant (infra decision primitives);
// this package does chain composition, data permission resolution, and fail-closed
// enforcement; message presentation and business data sources are always injected via
// Deps/Envelope. This lets each business module assemble its own messages and data
// sources without duplicating the middleware chain implementation.
package humakit

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/datascope"
	"github.com/stanley760/devops/devops-infra/http/response"
	securityauthz "github.com/stanley760/devops/devops-infra/security/authz"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// Middleware is a type alias for the huma middleware type (huma has no separate named type).
type Middleware = func(huma.Context, func(huma.Context))

// Envelope holds error presentation callbacks (messages belong to the business, injected here).
// nil fields fall back to safe defaults (HTTP status phrase envelopes), avoiding a bare
// status code with no body when injection is missed.
type Envelope struct {
	Unauthorized      func(hctx huma.Context, err error) // 401 base (including session expiry message mapping)
	AuthnInfra        func(hctx huma.Context, err error) // 401/503 authentication infrastructure error
	TenantForbidden   func(hctx huma.Context, err error) // 403 tenant unavailable
	PermForbidden     func(hctx huma.Context, err error) // 403 no permission within the domain
	PlatformForbidden func(hctx huma.Context, err error) // 403 platform (distinguishable non-admin message)
	AuthzInfra        func(hctx huma.Context, err error) // 403/503 authorization infrastructure error
}

// Deps injects all business dependencies via callbacks/interfaces; humakit makes no
// default business bindings (assembly belongs to the business side).
// nil semantics always fail closed: TenantUsable nil → always false; Authorizer nil → reject;
// a nil resolution data source → resolution error → empty Scope (no rows visible).
type Deps struct {
	Authorizer securityauthz.Authorizer // in-domain/platform authorization; nil → reject
	// SuperAdminRole is the tenant super-admin role code (data permission domain-wide bypass check);
	// "" → security/authz.DefaultRoleConfig().SuperAdmin.
	SuperAdminRole string
	// ScopeTTL is the data permission resolution result cache duration; 0 → 60s.
	ScopeTTL time.Duration
	// TenantUsable decides tenant usability (disabled/expired/nonexistent → false); nil → always false.
	TenantUsable func(ctx context.Context, tenantID int64) bool
	// IsPlatformAdmin decides platform admins (domain-wide data permission bypass); nil → false.
	IsPlatformAdmin func(ctx context.Context, userID int64) bool
	// Data permission resolution data sources; any nil → resolution failure → empty Scope.
	UserRoles func(ctx context.Context, userID int64) ([]datascope.RoleScope, error)
	User      func(ctx context.Context, userID int64) (datascope.UserRef, error)
	Depts     func(ctx context.Context) ([]datascope.DeptNode, error)
	// WithClaims is the ctx enhancement after successful authentication (e.g. audit actor); nil → pass through as-is.
	WithClaims func(ctx context.Context, claims *securityjwt.Claims) context.Context
	// Envelope holds error presentation callbacks; nil fields fall back to safe defaults.
	Envelope Envelope
}

const defaultScopeTTL = 60 * time.Second

// withDefaults backfills zero-value fields with safe defaults (no business binding).
func (d Deps) withDefaults() Deps {
	if d.SuperAdminRole == "" {
		// Same source as security/authz's RoleConfig, to avoid the two default values drifting apart.
		d.SuperAdminRole = securityauthz.DefaultRoleConfig().SuperAdmin
	}
	if d.ScopeTTL <= 0 {
		d.ScopeTTL = defaultScopeTTL
	}
	if d.TenantUsable == nil {
		d.TenantUsable = func(context.Context, int64) bool { return false }
	}
	if d.IsPlatformAdmin == nil {
		d.IsPlatformAdmin = func(context.Context, int64) bool { return false }
	}
	d.Envelope = d.Envelope.withDefaults()
	return d
}

// withDefaults falls back nil slots to HTTP status phrase envelopes (same semantics as securitymw.withHumaDefaults).
func (e Envelope) withDefaults() Envelope {
	if e.Unauthorized == nil {
		e.Unauthorized = statusPhrase(http.StatusUnauthorized)
	}
	if e.AuthnInfra == nil {
		e.AuthnInfra = statusPhrase(http.StatusServiceUnavailable)
	}
	if e.TenantForbidden == nil {
		e.TenantForbidden = statusPhrase(http.StatusForbidden)
	}
	if e.PermForbidden == nil {
		e.PermForbidden = statusPhrase(http.StatusForbidden)
	}
	if e.PlatformForbidden == nil {
		e.PlatformForbidden = statusPhrase(http.StatusForbidden)
	}
	if e.AuthzInfra == nil {
		e.AuthzInfra = statusPhrase(http.StatusServiceUnavailable)
	}
	return e
}

// statusPhrase is the envelope callback for a fixed status code + HTTP status phrase.
func statusPhrase(status int) func(huma.Context, error) {
	return func(hctx huma.Context, _ error) {
		response.WriteHumaEnvelope(hctx, status, http.StatusText(status))
	}
}

// Kit is the business middleware suite: assembly and chain composition of the
// JWT/tenant/authorization/data permission middleware.
type Kit struct {
	authn   securitymw.BearerAuthenticator
	deps    Deps
	jwtM    Middleware
	tenantM Middleware
	scopeM  Middleware
	permM   func(string) Middleware
	platM   func(string) Middleware
	scopes  *scopeCache
	// auditM is the operation audit collection middleware; nil when not assembled, and the business chain does not append that stage.
	auditM Middleware
}

// NewKit assembles the middleware suite; nil/zero-value items in deps are backfilled
// with safe defaults per fail-closed semantics.
func NewKit(authn securitymw.BearerAuthenticator, deps Deps) *Kit {
	deps = deps.withDefaults()
	k := &Kit{
		authn:  authn,
		deps:   deps,
		scopes: newScopeCache(deps.ScopeTTL),
	}
	k.jwtM = k.securityJWT()
	k.tenantM = k.securityTenant()
	k.scopeM = k.securityScope()
	k.permM = func(perm string) Middleware { return k.securityPerm(perm) }
	k.platM = func(perm string) Middleware { return k.securityPlat(perm) }
	// No audit collection by default: an explicit placeholder rather than nil, because
	// huma's chain composition does not skip nil elements.
	k.auditM = noopAudit
	return k
}

// EnableAudit assembles the operation audit collection middleware.
//
// Each business service calls this at its own assembly point (the cmd layer), passing in
// the Sink and the module whitelist; business services that do not call it keep noopAudit
// and produce no audit writes at all.
func (k *Kit) EnableAudit(deps AuditDeps) {
	k.auditM = k.NewAuditMiddleware(deps)
}

// JWT authentication middleware (Bearer → token parsing → session state → claims injected into ctx).
func (k *Kit) JWT() Middleware { return k.jwtM }

// Tenant tenant verification + context injection.
func (k *Kit) Tenant() Middleware { return k.tenantM }

// Scope row-level data permission resolution middleware (for controlled-resource routes).
func (k *Kit) Scope() Middleware { return k.scopeM }

// Perm in-domain authorization middleware (uid × tid × perm).
func (k *Kit) Perm(perm string) Middleware { return k.permM(perm) }

// Plat platform authorization middleware (platform admins).
func (k *Kit) Plat(perm string) Middleware { return k.platM(perm) }

// Logged authentication-only chain (for endpoints like logout with no tenant semantics).
func (k *Kit) Logged() huma.Middlewares { return huma.Middlewares{k.jwtM} }

// TenantBiz authentication + tenant chain (for personal-info endpoints).
func (k *Kit) TenantBiz() huma.Middlewares {
	return huma.Middlewares{k.jwtM, k.tenantM, k.auditM}
}

// Biz business route middleware chain: JWT → Tenant → Perm (in-domain) → Audit.
func (k *Kit) Biz(perm string) huma.Middlewares {
	return huma.Middlewares{k.jwtM, k.tenantM, k.permM(perm), k.auditM}
}

// ScopedBiz controlled-resource route chain: JWT → Tenant → Perm → DataScope → Audit.
// Attached only to resources that need row-level filtering; not added to uncontrolled
// resources, keeping resolution overhead and semantic boundaries explicit.
func (k *Kit) ScopedBiz(perm string) huma.Middlewares {
	return huma.Middlewares{k.jwtM, k.tenantM, k.permM(perm), k.scopeM, k.auditM}
}

// PlatChain platform route middleware chain: JWT → PlatformPerm → Audit (no Tenant, operates on global tables).
func (k *Kit) PlatChain(perm string) huma.Middlewares {
	return huma.Middlewares{k.jwtM, k.platM(perm), k.auditM}
}

// InvalidateDataScope clears the data permission cache (called on role save/delete and user role changes).
func (k *Kit) InvalidateDataScope() {
	k.scopes.invalidate()
}
