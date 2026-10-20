package model

import (
	commonaudit "github.com/stanley760/devops/devops-common/audit"
)

// OperationLog is the operation audit table (tenant table).
//
// Written by the humakit audit-collection middleware via audit.Sink (contract see devops-common/audit);
// column widths correspond one-to-one to the contract constants; query/delete/clean go through the
// admin endpoint /system/api/v1/operation-logs (system:operationlog:* permission gated).
//
// Division of labor with the login audit (LoginLog): LoginLog records the attempts and results of
// the "can log in" authentication chain; OperationLog records "what the B-side admin operations did",
// covering write operations on users/roles/menus/depts/posts/tenants.
type OperationLog struct {
	BaseTenantModel

	// trace and operator
	TraceID  string               `gorm:"column:trace_id;size:100;not null;default:'';index"` // trace ID, cross-checkable with the logging/tracing system
	UserID   int64                `gorm:"not null;default:0;index"`                           // operator user ID; 0 if unknown
	UserName string               `gorm:"size:30;not null;default:'';index"`                  // operator name (snapshot, so it stays traceable after a rename)
	UserType commonaudit.UserType `gorm:"not null;default:0"`                                 // operator category: 1 member (C-side) / 2 admin (B-side)

	// operation location
	Module    string `gorm:"size:50;not null;default:'';index"` // first-level module, e.g. system
	SubModule string `gorm:"size:50;not null;default:'';index"` // second-level module, e.g. user
	BizID     int64  `gorm:"column:biz_id;not null;default:0"`  // business primary key, e.g. the ID of the modified user

	// content and source
	Contents      string `gorm:"type:text"`                    // change content summary; usually empty in phase 1
	Extra         string `gorm:"type:text"`                    // extended info (JSON), e.g. duration
	RequestMethod string `gorm:"size:16;not null;default:''"`  //
	RequestURL    string `gorm:"size:255;not null;default:''"` //
	UserIP        string `gorm:"size:50;not null;default:'';index"`
	UserAgent     string `gorm:"size:512;not null;default:''"`
	GeoCountry    string `gorm:"size:64;not null;default:''"`
	GeoProvince   string `gorm:"size:64;not null;default:''"`
	GeoCity       string `gorm:"size:64;not null;default:''"`
	GeoISP        string `gorm:"size:64;not null;default:''"`

	// result
	Result int8 `gorm:"not null;default:0"` // 0 success 1 failure
	Code   int  `gorm:"not null;default:0"` // HTTP status code, consistent with the response envelope
}

// TableName is the table name.
func (OperationLog) TableName() string { return "t_system_operation_log" }

// FromRecord maps an audit contract record to this table's entity.
//
// For use by devops-infra/audit's GORSink: the contract layer (devops-common/audit) does not
// know gorm tags and table structures; the persistence mapping is done on the business table side.
func FromRecord(rec commonaudit.Record) *OperationLog {
	return &OperationLog{
		TenantID:      rec.TenantID,
		TraceID:       rec.TraceID,
		UserID:        rec.UserID,
		UserName:      rec.Username,
		UserType:      rec.UserType,
		Module:        rec.Module,
		SubModule:     rec.SubModule,
		BizID:         rec.BizID,
		Contents:      rec.Contents,
		Extra:         rec.Extra,
		RequestMethod: rec.RequestMethod,
		RequestURL:    rec.RequestURL,
		UserIP:        rec.UserIP,
		UserAgent:     rec.UserAgent,
		GeoCountry:    rec.GeoCountry,
		GeoProvince:   rec.GeoProvince,
		GeoCity:       rec.GeoCity,
		GeoISP:        rec.GeoISP,
		Result:        rec.Result,
		Code:          rec.Code,
	}
}
