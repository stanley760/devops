// Package audit provides the operation-audit wiring for devops-system.
//
// Layering: the contract (what is recorded, column widths, user-type enums)
// lives in devops-common/audit; the generic GORM write implementation lives
// in devops-infra/audit; this package only wires up this service's seat —
// connecting the tenant-aware connection (tenantdb.DB) with the business
// table model (model.OperationLog).
package audit

import (
	"context"

	"gorm.io/gorm"

	infraaudit "github.com/stanley760/devops/devops-infra/audit"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// Executor resolves the database connection from the request context.
//
// It goes through tenantdb.DB(ctx) rather than a bare connection: this both
// reuses the transaction already open in ctx (business write and audit write
// commit in the same transaction) and lets TenantPlugin inject tenant_id
// from ctx — audit rows naturally land under the correct tenant, with no
// need to fill in TenantID manually inside the conversion function.
type Executor struct{}

// Create implements infraaudit.Executor.
func (Executor) Create(ctx context.Context, value any) *gorm.DB {
	return tenantdb.DB(ctx).Create(value)
}

// NewSink wires up this service's operation-audit Sink.
func NewSink() *infraaudit.GORSink[*model.OperationLog] {
	return infraaudit.NewGORSink(infraaudit.GORSinkConfig[*model.OperationLog]{
		Executor: Executor{},
		Convert:  model.FromRecord,
	})
}
