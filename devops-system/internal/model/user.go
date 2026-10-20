// t_system_users user table (tenant table).
// Uniqueness: uk_tenant_username(tenant_id,username), uk_tenant_mobile(tenant_id,mobile)
// — composite unique indexes across embedded fields are declared by the CREATE TABLE SQL in
// internal/dbinit/sql/schema (GORM tags cannot compose across embeddings).
package model

import "time"

// User is a user.
type User struct {
	BaseTenantModel
	Username  string     `gorm:"size:30;not null"`
	Password  string     `gorm:"size:100;not null"` // bcrypt
	Nickname  string     `gorm:"size:30;not null"`
	Remark    string     `gorm:"size:500"`
	DeptID    int64      `gorm:"index"`
	PostIDs   string     `gorm:"type:json;default:null"` // post ID JSON array, e.g. [4]; empty values are stored as NULL (the json_valid constraint rejects '')
	Email     string     `gorm:"size:50"`
	Mobile    *string    `gorm:"size:20"`            // pointer: unset is stored as NULL, guaranteeing uk_tenant_mobile multiple nulls do not conflict
	Gender    int8       `gorm:"not null;default:0"` // 0 unknown 1 male 2 female
	Avatar    string     `gorm:"size:512"`
	Status    int8       `gorm:"not null"` // 0 disabled 1 normal
	LoginIP   string     `gorm:"size:50;column:login_ip"`
	LoginDate *time.Time `gorm:"column:login_date"`
}

// TableName is the table name.
func (User) TableName() string { return "t_system_users" }
