// t_system_config system configuration table (tenant table).
//
// Holds the runtime-tunable configuration items of the admin platform (site
// name, registration switches, upload limits, and so on), managed from the
// console rather than the YAML config file.
//
// A config key is unique within a tenant. Items with type=1 are built-in
// (system): they may be edited but not deleted, so the platform always has a
// complete set of default items. Items with type=2 are custom and deletable.
package model

// Config is a system configuration item.
type Config struct {
	BaseTenantModel
	ConfigKey   string `gorm:"size:100;not null"`     // configuration key, unique within a tenant
	ConfigName  string `gorm:"size:100;not null"`     // configuration name
	ConfigValue string `gorm:"size:1000"`             // configuration value
	Type        int8   `gorm:"not null;default:2"`    // 1 built-in 2 custom
	Visible     bool   `gorm:"not null;default:true"` // whether the frontend shows it
	Remark      string `gorm:"size:500"`
}

// TableName is the table name.
func (Config) TableName() string { return "t_system_config" }
