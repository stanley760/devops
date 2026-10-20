// t_tenant tenant registry table (global table, no tenant_id).
// code is globally unique: the frontend tenant identifier and login input.
package model

import "time"

// Tenant is a tenant.
type Tenant struct {
	BaseModel
	Name          string     `gorm:"size:30;not null"`
	Code          string     `gorm:"size:30;not null;uniqueIndex:uk_code"`
	ContactName   string     `gorm:"size:30;column:contact_name"`
	ContactMobile string     `gorm:"size:500;column:contact_mobile"`
	Status        int8       `gorm:"not null"` // 0 disabled 1 normal
	ExpireTime    *time.Time // NULL never expires
	Remark        string     `gorm:"size:500"`
}

// TableName is the table name.
func (Tenant) TableName() string { return "t_tenant" }
