// Package tenantdb is the single-database multi-tenant context and data access entry point.
//
// The dao layer uniformly obtains connections via DB(ctx), and TenantPlugin
// automatically injects/filters tenant_id based on the ctx; accessing tenant
// tables without a tenant context fails closed.
package tenantdb

import (
	"context"

	"gorm.io/gorm"

	database "github.com/stanley760/devops/devops-infra/database"
	client "github.com/stanley760/devops/devops-infra/database/mysql"
)

// DB is the unified dao layer entry point:
// all tenants share the same connection, and tenant isolation is implemented
// by TenantPlugin's tenant_id filtering.
func DB(ctx context.Context) *gorm.DB {
	if tx, ok := database.ExecutorFromContext(ctx); ok {
		return tx
	}
	return client.Get().WithContext(ctx)
}

// Transaction executes a transaction on the shared database.
// The transaction injects the context, so repositories inside DB(ctx) reuse the same transaction automatically.
func Transaction(ctx context.Context, fn database.TxFunc) error {
	if _, ok := database.ExecutorFromContext(ctx); ok {
		return fn(ctx)
	}
	return database.Transaction(ctx, DB(ctx), fn)
}
