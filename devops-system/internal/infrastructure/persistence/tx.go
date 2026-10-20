// Package persistence is the GORM implementation of domain repository ports (infrastructure layer).
//
// Dependency direction: infrastructure → domain. The domain layer does not know this package; the reverse does not hold.
package persistence

import (
	"context"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// txKey is the context key under which the transaction handle is stored.
type txKey struct{}

// gormTxManager is the GORM implementation of shared.TxManager.
//
// Problem solved: before the refactor, dao.WithTx kept *gorm.DB entirely inside the dao package,
// so the service layer could not declare transaction boundaries for cross-aggregate use cases
// and had to write manual compensation in the application layer (service/tenant.go:104 used
// HardDeleteTenant to roll back the tenant row). Now the application layer declares the
// boundary through this port, the transaction handle is passed to each repository
// implementation via ctx, and *gorm.DB no longer leaks out of the persistence package.
type gormTxManager struct{}

// NewTxManager constructs the transaction manager.
func NewTxManager() shared.TxManager { return gormTxManager{} }

// WithinTx executes fn within a single transaction.
//
// If a transaction handle already exists in ctx, it is reused directly (nested calls open no
// extra transaction), so a repository implementation calling WithinTx internally does not
// cause lock problems from transaction nesting.
func (gormTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFrom(ctx); ok {
		// already in a transaction: reuse the outer transaction.
		return fn(ctx)
	}
	return tenantdb.DB(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// txFrom takes the transaction handle from ctx.
//
// Repository implementations uniformly take the connection through this: reuse the transaction
// if present, otherwise fall back to the tenant connection (tenantdb.DB(ctx) applies the
// tenant filtering plugin).
func txFrom(ctx context.Context) (*gorm.DB, bool) {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx, true
	}
	return nil, false
}

// db takes the currently available connection: the transaction handle first, then the tenant connection.
func db(ctx context.Context) *gorm.DB {
	if tx, ok := txFrom(ctx); ok {
		return tx.WithContext(ctx)
	}
	return tenantdb.DB(ctx)
}
