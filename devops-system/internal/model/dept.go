// t_system_dept dept table (tenant table). The tree structure is assembled in memory (no ancestors column, aligned with yudao).
package model

// Dept is a dept.
type Dept struct {
	BaseTenantModel
	Name         string `gorm:"size:30;not null"`
	ParentID     int64  `gorm:"index;default:0"`
	Sort         int
	LeaderUserID int64  `gorm:"column:leader_user_id"`
	Phone        string `gorm:"size:20"`
	Email        string `gorm:"size:50"`
	Status       int8   `gorm:"not null"` // 0 disabled 1 normal
}

// TableName is the table name.
func (Dept) TableName() string { return "t_system_dept" }
