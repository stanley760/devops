// TenantPlugin: GORM callbacks automatically inject/filter tenant_id.
//
// Fail-closed semantics:
//   - stmt.Schema has no tenant_id field (global table) → allow
//   - Native JOIN queries (stmt.Joins non-empty) → allow: the dao filters
//     explicitly (bare columns are ambiguous across multiple tables)
//   - ctx has no tenant and no explicit Skip → error out directly, failing the
//     query (guards against cross-tenant data leakage)
//   - Query/modify/delete: inject WHERE tenant_id = ?; create: fill the TenantID field
//
// Registered only during server assembly (migrate/seed go straight to db.Get(),
// with no plugin and no collateral damage).
package tenantdb

import (
	"github.com/stanley760/devops/devops-infra/tenant"
	"gorm.io/gorm"
)

// RegisterPlugin registers the tenant filtering callbacks on the shared connection (called once during server assembly).
func RegisterPlugin(gdb *gorm.DB) error {
	return tenant.RegisterPlugin(gdb)
}
