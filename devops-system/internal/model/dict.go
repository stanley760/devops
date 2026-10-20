// Dictionary tables (tenant tables).
//
// Division of labor: DictType declares the dictionary namespace (a unique
// type code); DictData holds the items (label/value pairs) under one type.
// Both are tenant-scoped, so TenantPlugin injects/filters tenant_id.
//
// A dictionary type code is unique within a tenant and cannot be changed
// after creation (the data rows reference it by code).
package model

// DictType is a dictionary type.
type DictType struct {
	BaseTenantModel
	Name   string `gorm:"size:100;not null"`  // dictionary name
	Type   string `gorm:"size:100;not null"`  // dictionary type code, unique within a tenant
	Status int8   `gorm:"not null;default:1"` // 0 disabled 1 normal
	Remark string `gorm:"size:500"`
}

// TableName is the table name.
func (DictType) TableName() string { return "t_system_dict_type" }

// DictData is a dictionary item (a label/value pair under one dictionary type).
type DictData struct {
	BaseTenantModel
	DictType string `gorm:"size:100;not null;index"` // dictionary type code
	Label    string `gorm:"size:100;not null"`       // display label
	Value    string `gorm:"size:100;not null"`       // stored value
	Sort     int
	Status   int8   `gorm:"not null;default:1"` // 0 disabled 1 normal
	CSSClass string `gorm:"size:100"`           // frontend style class, optional
	Remark   string `gorm:"size:500"`
}

// TableName is the table name.
func (DictData) TableName() string { return "t_system_dict_data" }
