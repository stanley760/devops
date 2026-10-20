// Package authz is the admin-plane authorization facade: Enforce and policy
// maintenance are uniformly routed to the shared authorizer
// (devops-infra/security/authz, which instantiates Casbin + gorm-adapter;
// there is no package-level enforcer).
//
// Model highlights (carried by the shared package, behavior unchanged):
//   - sub = user_id, dom = tenant_id, obj = permission identifier (e.g. system:user:create)
//   - g(user, role, tenant) triple: in-tenant roles; the same user_id is a
//     different g row in each tenant, giving natural isolation
//   - g2(user, platform_admin) pair: platform roles (not domain-bound) —
//     in-domain * and platform capabilities are fully separated
//   - p(role, tenant, perm): a role's permission snapshot in a tenant;
//     p, platform_admin, *, * grants platform admins full cross-domain access
//
// Privilege-escalation boundary (6.3): the platform:tenant:* permission
// identifier only appears on platform routes; a tenant super admin's
// p, super_admin, {tid}, * cannot reach platform endpoints because its dom
// does not match "*".
//
// This package keeps only admin-plane semantics: the role-name config
// (RoleConfig), seed IDs, and numeric-primary-key/domain conversion happen
// on the system side; generic persistence and authorization decisions live
// in the shared package.
package authz

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"

	"gorm.io/gorm"

	securityauthz "github.com/stanley760/devops/devops-infra/security/authz"
)

// holder is an explicitly initialized instance holder (assembly chain:
// Init/SetAuthorizer → the package-level functions). Tests can swap it via
// SetAuthorizer, and instances do not leak into each other.
var holder atomic.Pointer[securityauthz.CasbinAuthorizer]

// Init wires up the shared authorizer: gorm-adapter persistence
// (casbin_rule table) + loading existing policies. opts can inject the
// role-name config (e.g. securityauthz.WithRoles(...)); omitting it uses the
// default yudao semantics.
func Init(gdb *gorm.DB, opts ...securityauthz.Option) error {
	a, err := securityauthz.New(gdb, opts...)
	if err != nil {
		return fmt.Errorf("failed to initialize casbin: %w", err)
	}
	holder.Store(a)
	slog.Info("Casbin enforcer initialized", "table", "casbin_rule")
	return nil
}

// Roles returns the currently effective role-name config (seeding, checks,
// and tenant role-row creation all read from the same source). When not
// wired up, the default config is returned.
func Roles() securityauthz.RoleConfig {
	if a := loaded(); a != nil {
		return a.Roles()
	}
	return securityauthz.DefaultRoleConfig()
}

// SetAuthorizer explicitly injects the instance (used by tests or alternative assembly paths).
func SetAuthorizer(a *securityauthz.CasbinAuthorizer) { holder.Store(a) }

// Authorizer exports the shared authorizer (for injection into the huma
// authorization middleware, see D7). When not wired up it returns nil, and
// the middleware layer fails closed (consistent with the gin Enforce
// behavior).
func Authorizer() securityauthz.Authorizer {
	if a := loaded(); a != nil {
		return a
	}
	return nil
}

// Close releases the instance (keeping the assembly chain symmetric; the shared SyncedEnforcer has no explicit Close).
func Close() { holder.Store(nil) }

func loaded() *securityauthz.CasbinAuthorizer { return holder.Load() }

// Enforce is in-domain authorization: does the user hold perm in tenant tid?
// Internal errors are treated as denial and logged (fail-closed).
func Enforce(userID, tenantID int64, perm string) bool {
	a := loaded()
	if a == nil {
		return false
	}
	ok, err := a.Enforce(context.Background(),
		strconv.FormatInt(userID, 10), strconv.FormatInt(tenantID, 10), perm)
	if err != nil {
		slog.Error("casbin Enforce failed", "err", err, "uid", userID, "tid", tenantID, "perm", perm)
		return false
	}
	return ok
}

// EnforcePlatform is platform-route authorization (dom = "*"; only platform_admin wildcard p rows can match).
func EnforcePlatform(userID int64, perm string) bool {
	a := loaded()
	if a == nil {
		return false
	}
	ok, err := a.EnforcePlatform(context.Background(), strconv.FormatInt(userID, 10), perm)
	if err != nil {
		slog.Error("casbin EnforcePlatform failed", "err", err, "uid", userID, "perm", perm)
		return false
	}
	return ok
}

// IsPlatformAdmin reports whether the user is a platform admin (g2 check, used by the Platform middleware).
func IsPlatformAdmin(userID int64) bool {
	a := loaded()
	if a == nil {
		return false
	}
	ok, err := a.IsPlatformAdmin(context.Background(), strconv.FormatInt(userID, 10))
	if err != nil {
		slog.Error("casbin platform-role query failed", "err", err, "uid", userID)
		return false
	}
	return ok
}

// RebuildRolePolicies rebuilds the p rows from the (role, tenant) snapshot:
// when saving role menus it first clears the old rows, then batch-adds rows
// whose permission is non-empty. An empty perms is equivalent to clearing
// all of that role's authorizations in the domain.
func RebuildRolePolicies(roleCode string, tenantID int64, perms []string) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.RebuildRolePolicies(roleCode, strconv.FormatInt(tenantID, 10), perms)
}

// RemoveRolePolicies removes all of a role's p rows in the domain (called when deleting a role).
func RemoveRolePolicies(roleCode string, tenantID int64) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.RemoveRolePolicies(roleCode, strconv.FormatInt(tenantID, 10))
}

// RebuildUserGrouping rebuilds the g rows by (user, tenant) (called when saving user roles).
func RebuildUserGrouping(userID, tenantID int64, roleCodes []string) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.RebuildUserGrouping(strconv.FormatInt(userID, 10), strconv.FormatInt(tenantID, 10), roleCodes)
}

// RemoveUserGrouping removes all of a user's g rows in the domain (called when deleting a user).
func RemoveUserGrouping(userID, tenantID int64) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.RemoveUserGrouping(strconv.FormatInt(userID, 10), strconv.FormatInt(tenantID, 10))
}

// RemoveRoleGrouping removes all g rows in the domain that reference a role
// (called when deleting a role). The g triples are filtered by the role field.
func RemoveRoleGrouping(roleCode string, tenantID int64) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.RemoveRoleGrouping(roleCode, strconv.FormatInt(tenantID, 10))
}

// SeedPolicies seeds the initial policies (idempotent, reused by
// migrate/tenant creation): delegates to the shared package's SeedBootstrap,
// with role names taken from RoleConfig (default platform_admin / super_admin).
//
//	p, {pa}, *, *             platform admin full access across tenants (g2 path)
//	g2, {adminUID}, {pa}
//	p, {sa}, {tid}, *         tenant super admin full access in-domain
//	g, {adminUID}, {sa}, {tid}
func SeedPolicies(tenantID, adminUID int64) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.SeedBootstrap(strconv.FormatInt(adminUID, 10), strconv.FormatInt(tenantID, 10))
}

// SeedTenantAdmin seeds the tenant's initial super-admin policies (called
// when creating a tenant, idempotent):
//
//	p, {sa}, {tid}, *         tenant super admin full access in-domain
//	g, {adminUID}, {sa}, {tid}
func SeedTenantAdmin(tenantID, adminUID int64) error {
	a := loaded()
	if a == nil {
		return fmt.Errorf("authz not initialized")
	}
	return a.SeedTenantAdmin(strconv.FormatInt(adminUID, 10), strconv.FormatInt(tenantID, 10))
}
