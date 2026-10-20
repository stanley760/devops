// t_system_menu menu table (global table, no tenant_id, aligned with yudao).
// No api_path/api_method: API authorization goes through the "permission identifier" mode (Casbin judges by permission).
package model

// Menu is a menu. type: 1 directory 2 menu 3 button.
type Menu struct {
	BaseModel
	Name          string `gorm:"size:50;not null"`
	Permission    string `gorm:"size:100"` // permission identifier, e.g. system:user:create; empty for directories/tenant management
	Type          int8   `gorm:"not null"`
	Sort          int
	ParentID      int64  `gorm:"index;default:0"`
	Path          string `gorm:"size:200"`
	Icon          string `gorm:"size:100"`
	Component     string `gorm:"size:255"`
	ComponentName string `gorm:"size:255;column:component_name"`
	Status        int8   `gorm:"not null"`               // 0 disabled 1 normal
	Visible       bool   `gorm:"not null;default:true"`  // whether to show
	KeepAlive     bool   `gorm:"not null;default:false"` // whether to cache
	AlwaysShow    bool   `gorm:"not null;default:true"`  // directories always show
}

// TableName is the table name.
func (Menu) TableName() string { return "t_system_menu" }
