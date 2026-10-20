package humakit

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/datascope"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// errNoScopeSource resolution data source not injected (fail-closed: resolution failure → empty Scope).
var errNoScopeSource = errors.New("humakit: data scope source not configured")

// scopeCache entry: (tenantID, userID) → resolution result + expiry time (held by the Kit instance).
// In-process TTL cache: change invalidation is covered by the caller's InvalidateDataScope;
// in multi-instance deployments this should become a shared cache with a version number.
type scopeCache struct {
	mu      sync.RWMutex
	entries map[[2]int64]scopeEntry
	ttl     time.Duration
}

type scopeEntry struct {
	scope   datascope.Scope
	expires time.Time
}

// newScopeCache constructs an empty cache.
func newScopeCache(ttl time.Duration) *scopeCache {
	return &scopeCache{entries: make(map[[2]int64]scopeEntry), ttl: ttl}
}

// invalidate clears all entries.
func (c *scopeCache) invalidate() {
	c.mu.Lock()
	c.entries = make(map[[2]int64]scopeEntry)
	c.mu.Unlock()
}

// securityScope data permission resolution middleware (mounted on controlled-resource routes).
// Fail-closed on resolution failure: pass through with an empty Scope (no rows visible) —
// the functional permission (Perm) has already gated the request once, so a failure here
// only affects the data scope and is not escalated to full visibility.
func (k *Kit) securityScope() Middleware {
	return func(hctx huma.Context, next func(huma.Context)) {
		claims := securitymw.HumaCurrentClaims(hctx.Context())
		if claims == nil || claims.UID == 0 {
			next(huma.WithValue(hctx, datascope.CtxKey{}, datascope.Scope{}))
			return
		}
		scope, err := k.resolveDataScope(hctx.Context(), claims.UID, claims.TenantID)
		if err != nil {
			// fail-closed: pass through with an empty set, the request continues
			// (the list gets a 1=0 filter or empty visibility)
			next(huma.WithValue(hctx, datascope.CtxKey{}, datascope.Scope{}))
			return
		}
		next(huma.WithValue(hctx, datascope.CtxKey{}, scope))
	}
}

// resolveDataScope resolution with TTL caching: platform admins/tenant super-admins
// bypass, otherwise role scopes are merged.
func (k *Kit) resolveDataScope(ctx context.Context, userID, tenantID int64) (datascope.Scope, error) {
	key := [2]int64{tenantID, userID}

	k.scopes.mu.RLock()
	if e, ok := k.scopes.entries[key]; ok && time.Now().Before(e.expires) {
		k.scopes.mu.RUnlock()
		return e.scope, nil
	}
	k.scopes.mu.RUnlock()

	scope, err := k.computeDataScope(ctx, userID)
	if err != nil {
		return datascope.Scope{}, err
	}

	k.scopes.mu.Lock()
	k.scopes.entries[key] = scopeEntry{scope: scope, expires: time.Now().Add(k.scopes.ttl)}
	k.scopes.mu.Unlock()
	return scope, nil
}

// computeDataScope a single resolution (both the data source and the platform check are
// injected via Deps, always going through infra value objects).
func (k *Kit) computeDataScope(ctx context.Context, userID int64) (datascope.Scope, error) {
	deps := k.deps

	// platform admins: bypass across tenants (g2 check, same source as the authorization middleware)
	if deps.IsPlatformAdmin(ctx, userID) {
		return datascope.Scope{All: true}, nil
	}

	if deps.UserRoles == nil || deps.User == nil || deps.Depts == nil {
		return datascope.Scope{}, errNoScopeSource
	}

	roleScopes, err := deps.UserRoles(ctx, userID)
	if err != nil {
		return datascope.Scope{}, err
	}
	for _, r := range roleScopes {
		if r.Code == deps.SuperAdminRole {
			return datascope.Scope{All: true}, nil // tenant super-admin: bypass within the domain
		}
	}

	u, err := deps.User(ctx, userID)
	if err != nil {
		return datascope.Scope{}, err
	}

	deptNodes, err := deps.Depts(ctx)
	if err != nil {
		return datascope.Scope{}, err
	}

	return datascope.Merge(roleScopes, u.DeptID, u.ID, datascope.ChildrenMap(deptNodes)), nil
}
