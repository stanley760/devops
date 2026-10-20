// Association tables (tenant tables): user-role and role-menu.
// Composite primary keys, no independent IDs, no ID assignment; physical deletion (no deleted_at) —
// assigning/unassigning are idempotent operations, and soft deletion would make the composite
// primary key unable to rebuild rows with the same combination.
package model

// UserRole is the user-role association.
type UserRole struct {
	TenantID int64 `gorm:"primaryKey"`
	UserID   int64 `gorm:"primaryKey;index:idx_user_role_user"`
	RoleID   int64 `gorm:"primaryKey;index:idx_user_role_role"`
}

// TableName is the table name.
func (UserRole) TableName() string { return "t_system_user_role" }

// RoleMenu is the role-menu association (menus are global, but authorization happens per-tenant's roles).
type RoleMenu struct {
	TenantID int64 `gorm:"primaryKey"`
	RoleID   int64 `gorm:"primaryKey;index:idx_role_menu_role"`
	MenuID   int64 `gorm:"primaryKey;index:idx_role_menu_menu"`
}

// TableName is the table name.
func (RoleMenu) TableName() string { return "t_system_role_menu" }
