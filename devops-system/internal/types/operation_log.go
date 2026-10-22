// Operation audit query module DTOs (admin-side read side of t_system_operation_log;
// the write side is persisted automatically by the humakit audit collection middleware via audit.Sink; there is no write endpoint).
package types

import (
	"time"

	commonaudit "github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-common/domain"
)

// OperationLogPageQuery holds the GET /operation-logs query parameters.
type OperationLogPageQuery struct {
	domain.PageParam
	Module    string     `doc:"first-level module, exact match; omitting means no filter"`
	SubModule string     `doc:"second-level module, exact match; omitting means no filter"`
	UserName  string     `doc:"operator name, fuzzy match"`
	UserIP    string     `doc:"client IP, fuzzy match"`
	Result    *int8      `doc:"execution result 0: success 1: failure; omitting means no filter"`
	BeginTime *time.Time `doc:"begin time (inclusive), lower bound of create_time"`
	EndTime   *time.Time `doc:"end time (inclusive), upper bound of create_time"`
}

// OperationLogDTO is an operation audit row.
type OperationLogDTO struct {
	ID            ID                   `json:"id" doc:"record id"`
	TraceID       string               `json:"traceId" doc:"trace ID"`
	UserID        ID                   `json:"userId" doc:"operator user id, 0 when unknown"`
	UserName      string               `json:"userName" doc:"operator name"`
	UserType      commonaudit.UserType `json:"userType" doc:"operator type 1: member (C-side) 2: administrator (B-side)"`
	UserTypeName  string               `json:"userTypeName" doc:"operator type name"`
	Module        string               `json:"module" doc:"first-level module"`
	SubModule     string               `json:"subModule" doc:"second-level module"`
	BizID         ID                   `json:"bizId" doc:"related business primary key"`
	Contents      string               `json:"contents" doc:"change summary"`
	Extra         string               `json:"extra" doc:"extra info (JSON), e.g. duration"`
	RequestMethod string               `json:"requestMethod" doc:"HTTP method"`
	RequestURL    string               `json:"requestUrl" doc:"request path"`
	UserIP        string               `json:"userIp" doc:"client IP"`
	UserAgent     string               `json:"userAgent" doc:"client User-Agent"`
	GeoCountry    string               `json:"geoCountry" doc:"country (injected by the higress geo-ip plugin header)"`
	GeoProvince   string               `json:"geoProvince" doc:"province (injected by the higress geo-ip plugin header)"`
	GeoCity       string               `json:"geoCity" doc:"city (injected by the higress geo-ip plugin header)"`
	GeoIsp        string               `json:"geoIsp" doc:"ISP (injected by the higress geo-ip plugin header)"`
	Result        int8                 `json:"result" doc:"execution result 0: success 1: failure"`
	Code          int                  `json:"code" doc:"HTTP status code, consistent with the response envelope"`
	CreateTime    DateTime            `json:"createTime" doc:"operation time"`
}
