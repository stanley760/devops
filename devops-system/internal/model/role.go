// t_system_role role table (tenant table).
// Uniqueness: uk_tenant_code(tenant_id,code) is declared by the CREATE TABLE SQL in internal/dbinit/sql/schema.
package model

// Role is a role. data_scope/data_scope_dept_ids are storage-only for now; row-level data permission columns are on the Roadmap.
type Role struct {
	BaseTenantModel
	Name             string `gorm:"size:30;not null"`
	Code             string `gorm:"size:100;not null"`
	Sort             int
	DataScope        int8   `gorm:"column:data_scope;not null;default:1"` // 1 all 2 custom…
	DataScopeDeptIDs string `gorm:"size:500;column:data_scope_dept_ids"`
	Status           int8   `gorm:"not null"`           // 0 disabled 1 normal
	Type             int8   `gorm:"not null;default:2"` // 1 builtin 2 custom
	Remark           string `gorm:"size:500"`
}

// TableName is the table name.
func (Role) TableName() string { return "t_system_role" }
