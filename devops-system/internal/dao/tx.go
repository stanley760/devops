// Transaction wrapper: *gorm.DB only appears in dao; the service orchestrates aggregate
// transactions through this package's functions and never sees transaction handles
// (layering convention: see the internal/service package comment).
package dao

import (
	"context"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// WithTx opens a transaction on the tenant-aware connection (shared by multi-step writes inside dao).
func WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return tenantdb.DB(ctx).Transaction(fn)
}
