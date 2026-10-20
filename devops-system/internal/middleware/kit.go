// Package middleware is this service's business assembly layer: it wires the middleware chain
// from devops-infra/security/humakit with this service's copy, data sources, audit, and tenant semantics.
//
// Layering: decision logic and chain composition live in humakit (infra); this package only does
// business default binding — envelope copy (huma.go), tenant usability judgment (tenant.go),
// the dao adapter (dao/scope_source.go), and auditctx injection. The assembly point is cmd/root.go.
package middleware

import (
	"context"

	"github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-common/auditctx"
	client "github.com/stanley760/devops/devops-infra/database/mysql"
	"github.com/stanley760/devops/devops-infra/security/humakit"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/store"
)

// Type aliases: the *middleware.Kit signature in router/app/wire stays unchanged, so wire needs no regeneration.
type (
	Kit  = humakit.Kit
	Deps = humakit.Deps
)

// NewKit assembles the business middleware suite: nil/zero entries get production default
// bindings (business semantics); envelope copy and data source adapters stay in this layer,
// with decision logic and chain composition in devops-infra/security/humakit.
func NewKit(authn *auth.Manager, deps Deps) *Kit {
	if deps.Authorizer == nil {
		deps.Authorizer = authz.Authorizer()
	}
	if deps.SuperAdminRole == "" {
		deps.SuperAdminRole = authz.Roles().SuperAdmin
	}
	if deps.TenantUsable == nil {
		deps.TenantUsable = newTenantChecker(client.Get(), store.Cache())
	}
	if deps.IsPlatformAdmin == nil {
		deps.IsPlatformAdmin = func(_ context.Context, userID int64) bool { return authz.IsPlatformAdmin(userID) }
	}
	if deps.UserRoles == nil {
		deps.UserRoles = dao.UserScopes
	}
	if deps.User == nil {
		deps.User = dao.UserRef
	}
	if deps.Depts == nil {
		deps.Depts = dao.DeptNodes
	}
	if deps.WithClaims == nil {
		deps.WithClaims = injectAuditActor
	}
	deps.Envelope = systemEnvelope()
	return humakit.NewKit(authn, deps)
}

// injectAuditActor injects the identity into the audit context after successful authentication:
// model.BeforeCreate/BeforeUpdate consumes the username in it to fill creator/updater,
// and the humakit audit-collection middleware consumes the full identity to write audit records.
// Operators of the B-side admin console are uniformly marked as audit.UserTypeAdmin.
func injectAuditActor(ctx context.Context, claims *securityjwt.Claims) context.Context {
	if claims == nil || claims.Username == "" {
		return ctx
	}
	return auditctx.WithActor(ctx, auditctx.Actor{
		UserID:   claims.UID,
		Username: claims.Username,
		UserType: int64(audit.UserTypeAdmin),
		TenantID: claims.TenantID,
	})
}

// systemEnvelope is this service's envelope copy (six slots verbatim aligned with the historical gin version).
func systemEnvelope() humakit.Envelope {
	return humakit.Envelope{
		Unauthorized:      writeHuma401,
		AuthnInfra:        writeHumaAuthInfraErr,
		TenantForbidden:   writeHuma403Tenant,
		PermForbidden:     writeHuma403Perm,
		PlatformForbidden: writeHuma403Platform,
		AuthzInfra:        writeHumaAuthzInfraErr,
	}
}
