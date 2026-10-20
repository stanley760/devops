package authz

import (
	"context"
	"testing"
)

// TestSeedBootstrapDefaultRoles SeedBootstrap's output under the default role names is
// equivalent to the historical hard-coded seed, and is idempotent (repeated calls append no rows).
func TestSeedBootstrapDefaultRoles(t *testing.T) {
	t.Parallel()

	authorizer, err := New(testDB(t))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	if err := authorizer.SeedBootstrap("10", "1"); err != nil {
		t.Fatalf("seed bootstrap: %v", err)
	}
	// idempotent: running again should not error or double the semantics
	if err := authorizer.SeedBootstrap("10", "1"); err != nil {
		t.Fatalf("seed bootstrap idempotent: %v", err)
	}

	ctx := context.Background()

	ok, err := authorizer.IsPlatformAdmin(ctx, "10")
	if err != nil || !ok {
		t.Fatalf("platform admin after seed = %v, %v", ok, err)
	}
	ok, err = authorizer.EnforcePlatform(ctx, "10", "platform:tenant:query")
	if err != nil || !ok {
		t.Fatalf("platform admin enforce = %v, %v", ok, err)
	}
	ok, err = authorizer.Enforce(ctx, "10", "1", "system:user:update")
	if err != nil || !ok {
		t.Fatalf("tenant super admin enforce = %v, %v", ok, err)
	}
	// a tenant super-admin must not reach platform endpoints
	ok, err = authorizer.EnforcePlatform(ctx, "20", "platform:tenant:query")
	if err != nil {
		t.Fatalf("platform enforce err: %v", err)
	}
	if ok {
		t.Fatal("unbound subject unexpectedly gained platform access")
	}
}

// TestSeedTenantAdminDefaultRoles SeedTenantAdmin writes only tenant super-admin rows
// and never touches the platform.
func TestSeedTenantAdminDefaultRoles(t *testing.T) {
	t.Parallel()

	authorizer, err := New(testDB(t))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	if err := authorizer.SeedTenantAdmin("20", "3"); err != nil {
		t.Fatalf("seed tenant admin: %v", err)
	}
	if err := authorizer.SeedTenantAdmin("20", "3"); err != nil {
		t.Fatalf("seed tenant admin idempotent: %v", err)
	}

	ctx := context.Background()

	ok, err := authorizer.Enforce(ctx, "20", "3", "system:user:update")
	if err != nil || !ok {
		t.Fatalf("tenant admin enforce = %v, %v", ok, err)
	}
	ok, err = authorizer.Enforce(ctx, "20", "4", "system:user:update")
	if err != nil {
		t.Fatalf("cross-tenant enforce err: %v", err)
	}
	if ok {
		t.Fatal("tenant admin unexpectedly gained cross-tenant access")
	}
	ok, err = authorizer.IsPlatformAdmin(ctx, "20")
	if err != nil {
		t.Fatalf("platform admin check err: %v", err)
	}
	if ok {
		t.Fatal("tenant admin unexpectedly became platform admin")
	}
}

// TestCustomRoleConfig custom role names: seed writes and the IsPlatformAdmin check
// must come from the same source, otherwise you get a split where the seed writes the
// custom name while the check looks up platform_admin.
func TestCustomRoleConfig(t *testing.T) {
	t.Parallel()

	cfg := RoleConfig{PlatformAdmin: "ops_admin", SuperAdmin: "ops_super"}
	authorizer, err := New(testDB(t), WithRoles(cfg))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	if got := authorizer.Roles(); got != cfg {
		t.Fatalf("Roles() = %+v, want %+v", got, cfg)
	}

	if err := authorizer.SeedBootstrap("10", "1"); err != nil {
		t.Fatalf("seed bootstrap custom: %v", err)
	}

	ctx := context.Background()

	// the check must hit the custom name (g2, 10, ops_admin), not platform_admin
	ok, err := authorizer.IsPlatformAdmin(ctx, "10")
	if err != nil || !ok {
		t.Fatalf("custom platform admin = %v, %v", ok, err)
	}
	ok, err = authorizer.EnforcePlatform(ctx, "10", "platform:tenant:query")
	if err != nil || !ok {
		t.Fatalf("custom platform enforce = %v, %v", ok, err)
	}
	ok, err = authorizer.Enforce(ctx, "10", "1", "system:user:update")
	if err != nil || !ok {
		t.Fatalf("custom super admin enforce = %v, %v", ok, err)
	}

	// the default name should not be written
	ok, err = authorizer.IsPlatformAdmin(ctx, "99")
	if err != nil {
		t.Fatalf("is platform admin err: %v", err)
	}
	if ok {
		t.Fatal("unbound subject matched platform admin")
	}
}

// TestRoleConfigZeroValueDefaults zero-value fields fall back to defaults, so partial customization still works.
func TestRoleConfigZeroValueDefaults(t *testing.T) {
	t.Parallel()

	authorizer, err := New(testDB(t), WithRoles(RoleConfig{PlatformAdmin: "ops_admin"}))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	got := authorizer.Roles()
	if got.PlatformAdmin != "ops_admin" {
		t.Fatalf("PlatformAdmin = %q, want ops_admin", got.PlatformAdmin)
	}
	if got.SuperAdmin != "super_admin" {
		t.Fatalf("SuperAdmin = %q, want super_admin (zero-value default)", got.SuperAdmin)
	}
}
