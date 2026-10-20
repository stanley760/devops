package humakit

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/stanley760/devops/devops-infra/datascope"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// fakeAuthn is an authenticator returning fixed claims (avoiding JWT/session assembly).
type fakeAuthn struct {
	claims *securityjwt.Claims
	err    error
}

func (f fakeAuthn) Authenticate(context.Context, string) (*securityjwt.Claims, error) {
	return f.claims, f.err
}

// probeScope assembles a probe route with JWT + Scope attached, returning the ctx Scope obtained.
func probeScope(t *testing.T, authn fakeAuthn, deps Deps) (datascope.Scope, bool, int) {
	t.Helper()
	kit := NewKit(authn, deps)

	cfg := huma.DefaultConfig("humakit-test", "v1")
	cfg.CreateHooks = nil
	_, api := humatest.New(t, cfg)

	var got datascope.Scope
	var ok bool
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/",
		Middlewares: huma.Middlewares{kit.JWT(), kit.Scope()},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		got, ok = datascope.From(ctx)
		return nil, nil
	})
	return got, ok, api.Get("/").Code
}

// TestDefaultEnvelope401 without injected messages, authentication failure falls back to
// the safe default (HTTP status phrase envelope).
func TestDefaultEnvelope401(t *testing.T) {
	t.Parallel()

	kit := NewKit(fakeAuthn{err: securitymw.ErrUnauthorized}, Deps{})

	cfg := huma.DefaultConfig("humakit-envelope", "v1")
	cfg.CreateHooks = nil
	_, api := humatest.New(t, cfg)
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/",
		Middlewares: kit.Logged(),
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) { return nil, nil })

	resp := api.Get("/")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), http.StatusText(http.StatusUnauthorized)) {
		t.Fatalf("body = %s, want status-phrase envelope", resp.Body.String())
	}
}

// TestScopeSuperAdmin tenant super-admins bypass all data.
func TestScopeSuperAdmin(t *testing.T) {
	t.Parallel()

	got, ok, code := probeScope(t, fakeAuthn{claims: &securityjwt.Claims{UID: 20, TenantID: 1}}, Deps{
		SuperAdminRole: "super_admin",
		UserRoles: func(context.Context, int64) ([]datascope.RoleScope, error) {
			return []datascope.RoleScope{{Code: "super_admin", DataScope: datascope.ScopeSelf}}, nil
		},
		User: func(context.Context, int64) (datascope.UserRef, error) {
			return datascope.UserRef{ID: 20, DeptID: 9}, nil
		},
		Depts: func(context.Context) ([]datascope.DeptNode, error) { return nil, nil },
	})
	if code != http.StatusNoContent {
		t.Fatalf("probe status = %d", code)
	}
	if !ok || !got.All {
		t.Fatalf("scope = %+v ok=%v, want All", got, ok)
	}
}

// TestScopeFailClosedWithoutSource when the data source is missing, resolution fails →
// empty Scope, and must not escalate to full visibility.
func TestScopeFailClosedWithoutSource(t *testing.T) {
	t.Parallel()

	got, ok, code := probeScope(t, fakeAuthn{claims: &securityjwt.Claims{UID: 20, TenantID: 1}}, Deps{})
	if code != http.StatusNoContent {
		t.Fatalf("probe status = %d", code)
	}
	if !ok {
		t.Fatal("scope key not injected")
	}
	if got.All || got.SelfOnly || len(got.DeptIDs) > 0 {
		t.Fatalf("scope = %+v, want empty (fail-closed)", got)
	}
}

// TestScopeMergeDeptBelow multi-role merge: this department and below are expanded along the department tree.
func TestScopeMergeDeptBelow(t *testing.T) {
	t.Parallel()

	got, ok, code := probeScope(t, fakeAuthn{claims: &securityjwt.Claims{UID: 30, TenantID: 1}}, Deps{
		SuperAdminRole:  "super_admin",
		IsPlatformAdmin: func(context.Context, int64) bool { return false },
		UserRoles: func(context.Context, int64) ([]datascope.RoleScope, error) {
			return []datascope.RoleScope{{Code: "ops", DataScope: datascope.ScopeDeptBelow}}, nil
		},
		User: func(context.Context, int64) (datascope.UserRef, error) {
			return datascope.UserRef{ID: 30, DeptID: 2}, nil
		},
		Depts: func(context.Context) ([]datascope.DeptNode, error) {
			return []datascope.DeptNode{{ID: 2, ParentID: 1}, {ID: 3, ParentID: 2}, {ID: 4, ParentID: 1}}, nil
		},
	})
	if code != http.StatusNoContent {
		t.Fatalf("probe status = %d", code)
	}
	if !ok {
		t.Fatal("scope key not injected")
	}
	// department 2 and its subtree {2,3}, excluding sibling department 4
	want := map[int64]bool{2: true, 3: true}
	if len(got.DeptIDs) != len(want) {
		t.Fatalf("DeptIDs = %v, want keys %v", got.DeptIDs, want)
	}
	for _, id := range got.DeptIDs {
		if !want[id] {
			t.Fatalf("DeptIDs = %v, contains unexpected %d", got.DeptIDs, id)
		}
	}
}

// TestPlatformAdminScope platform admins bypass everything.
func TestPlatformAdminScope(t *testing.T) {
	t.Parallel()

	got, ok, code := probeScope(t, fakeAuthn{claims: &securityjwt.Claims{UID: 1, TenantID: 1}}, Deps{
		IsPlatformAdmin: func(context.Context, int64) bool { return true },
	})
	if code != http.StatusNoContent {
		t.Fatalf("probe status = %d", code)
	}
	if !ok || !got.All {
		t.Fatalf("scope = %+v ok=%v, want All", got, ok)
	}
}

// TestInvalidateDataScope the invalidation hook clears the cache.
func TestInvalidateDataScope(t *testing.T) {
	t.Parallel()

	kit := NewKit(fakeAuthn{claims: &securityjwt.Claims{UID: 1, TenantID: 1}}, Deps{})
	kit.InvalidateDataScope() // must not panic
}
