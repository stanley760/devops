package persistence

import (
	"context"

	"github.com/stanley760/devops/devops-infra/tenant"
)

// tenantOf takes the tenant ID from the context.
//
// Why it is needed: queries involving JOINs or Unscoped queries bypass the tenant plugin's
// automatic injection (see the JOIN pass-through rules of the devops-infra/tenant plugin), in
// which case tenant_id must be specified explicitly, otherwise it is a cross-tenant data leak.
// Failing to get the tenant fails closed with an error.
//
// The fail-closed semantics are consistent with the existing implementation (dao/user.go:tenantOf,
// dao/menu.go:tenantOfCtx), converged into a single implementation.
func tenantOf(ctx context.Context) (int64, error) {
	return tenant.RequiredTenantID(ctx)
}
