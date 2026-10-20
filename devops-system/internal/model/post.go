// t_system_post post table (tenant table).
// Uniqueness: uk_tenant_code(tenant_id,code) is declared by the CREATE TABLE SQL in internal/dbinit/sql/schema.
package model

// Post is a post.
type Post struct {
	BaseTenantModel
	Code   string `gorm:"size:64;not null"`
	Name   string `gorm:"size:50;not null"`
	Sort   int
	Status int8   `gorm:"not null"` // 0 disabled 1 normal
	Remark string `gorm:"size:500"`
}

// TableName is the table name.
func (Post) TableName() string { return "t_system_post" }
