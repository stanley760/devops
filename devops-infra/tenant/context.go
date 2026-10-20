// Package tenant provides multi-tenant request context and database isolation helpers.
package tenant

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrTenantNotFound indicates the current execution context has no tenant ID.
	ErrTenantNotFound = errors.New("tenant: tenant id is not set")
	// ErrInvalidTenantID indicates the tenant ID is not a positive number.
	ErrInvalidTenantID = errors.New("tenant: tenant id must be greater than zero")
	// ErrContextMissing indicates the GORM tenant plugin did not find the tenant context.
	ErrContextMissing = errors.New("tenant: context is missing")
)

type contextKey struct{}

type Context struct {
	ID   int64
	Code string
}

type contextState struct {
	tenant Context
	skip   bool
}

// With returns a context carrying the tenant ID and tenant code.
func With(ctx context.Context, tenantID int64, tenantCode string) context.Context {
	return context.WithValue(ctx, contextKey{}, contextState{tenant: Context{ID: tenantID, Code: tenantCode}})
}

// WithTenant returns a context carrying the tenant ID.
func WithTenant(ctx context.Context, tenantID int64) (context.Context, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidTenantID, tenantID)
	}
	return With(ctx, tenantID, ""), nil
}

// From retrieves the tenant context.
func From(ctx context.Context) (Context, bool) {
	state, ok := ctx.Value(contextKey{}).(contextState)
	return state.tenant, ok && state.tenant.ID > 0
}

// TenantID returns the tenant ID in the current context.
func TenantID(ctx context.Context) (int64, bool) {
	tenant, ok := From(ctx)
	return tenant.ID, ok
}

// RequiredTenantID returns the current tenant ID, or ErrTenantNotFound when unset.
func RequiredTenantID(ctx context.Context) (int64, error) {
	tenant, ok := From(ctx)
	if !ok {
		return 0, ErrTenantNotFound
	}
	return tenant.ID, nil
}

// RequiredTenantCode returns the current tenant code, or ErrTenantNotFound when unset or empty.
func RequiredTenantCode(ctx context.Context) (string, error) {
	tenant, ok := From(ctx)
	if !ok || tenant.Code == "" {
		return "", ErrTenantNotFound
	}
	return tenant.Code, nil
}

// WithSkip returns a context that keeps tenant info but skips row-level tenant filtering.
func WithSkip(ctx context.Context) context.Context {
	state, _ := ctx.Value(contextKey{}).(contextState)
	state.skip = true
	return context.WithValue(ctx, contextKey{}, state)
}

// IsSkip reports whether the current context explicitly exempts tenant filtering.
func IsSkip(ctx context.Context) bool {
	state, ok := ctx.Value(contextKey{}).(contextState)
	return ok && state.skip
}

// Ignore is a compatibility alias for WithSkip.
func Ignore(ctx context.Context) context.Context {
	return WithSkip(ctx)
}

// IsIgnore is a compatibility alias for IsSkip.
func IsIgnore(ctx context.Context) bool {
	return IsSkip(ctx)
}

// ExecuteIgnore disables tenant isolation during fn's execution and returns fn's error.
func ExecuteIgnore(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("tenant: ignore function is nil")
	}
	return fn(WithSkip(ctx))
}

// Scope adds a tenant_id condition to a GORM query.
//
// When no tenant is set, ErrTenantNotFound is injected into db to avoid inadvertently querying cross-tenant data.
// Code that genuinely needs cross-tenant queries, such as global jobs, should explicitly pass Ignore(ctx).
func Scope(db *gorm.DB, ctx context.Context, column string) *gorm.DB {
	if db == nil {
		return nil
	}
	if IsSkip(ctx) {
		return db.WithContext(ctx)
	}
	id, err := RequiredTenantID(ctx)
	if err != nil {
		scoped := db.WithContext(ctx)
		scoped.AddError(err)
		return scoped
	}
	if column == "" {
		column = "tenant_id"
	}
	return db.WithContext(ctx).Where(clause.Eq{
		Column: clause.Column{Name: column},
		Value:  id,
	})
}
