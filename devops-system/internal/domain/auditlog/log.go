// Package auditlog is the audit log bounded context: login logs and operation logs.
//
// Both are append-only record-shaped data with no cross-aggregate references and no business
// invariants, so they are modeled as "records" rather than "aggregate roots" — this is the
// correct trade-off in DDD for simple models: don't force a non-existent lifecycle onto logs
// just for the sake of a formal aggregate.
package auditlog

import (
	"context"
	"strings"
	"time"

	"github.com/stanley760/devops/devops-common/domain"

	"github.com/stanley760/devops/devops-system/internal/domain/iam"
)

// Login result (consistent with the existing DB column semantics).
const (
	LoginResultSuccess int8 = 0
	LoginResultFailure int8 = 1
)

// LoginLog is the login log record (t_system_login_log).
//
// Field names align with column names: result (0 success 1 failure), code (business code).
// The four geo_* fields come from the X-Higress-Geo-* headers injected by the geo-ip plugin.
type LoginLog struct {
	ID       int64
	TenantID int64
	Username string
	// UserID is filled once the user is matched; 0 for unknown users.
	UserID      int64
	IP          string
	UserAgent   string
	GeoCountry  string
	GeoProvince string
	GeoCity     string
	GeoISP      string
	Result      int8
	// Code is the business code (same as the code label of the auth_login metric).
	Code int
	// CreateTime is the login time (the table has no separate login_time column; uses the base audit time).
	CreateTime time.Time
}

// LoginLogFilter is the login log query filter (column names aligned with t_system_login_log).
type LoginLogFilter struct {
	domain.PageParam
	Username  string
	IP        string
	Result    *int8
	BeginTime *string
	EndTime   *string
}

// OperationLog is the operation log record (t_system_operation_log).
//
// Field names align with column names: user_name/user_ip/geo_isp (columns prefixed with user/geo),
// request_url/request_method, result (0 success 1 failure), code (HTTP status code).
//
// The write path is not in this service: operation auditing is persisted by the humakit
// audit-collection middleware via devops-infra/audit.Sink (preserving the existing dual-track
// approach). This domain model only carries the query-side read model and admin delete/clean.
type OperationLog struct {
	ID       int64
	TenantID int64
	TraceID  string
	UserID   int64
	UserName string
	UserType int64
	Module   string
	// SubModule is the second-level module, e.g. user.
	SubModule string
	// BizID is the business primary key, e.g. the ID of the modified user.
	BizID         int64
	Contents      string
	Extra         string
	RequestMethod string
	RequestURL    string
	UserIP        string
	UserAgent     string
	GeoCountry    string
	GeoProvince   string
	GeoCity       string
	GeoISP        string
	Result        int8
	Code          int
	CreateTime    time.Time
}

// OperationLogFilter is the operation log query filter (column names aligned with t_system_operation_log).
type OperationLogFilter struct {
	domain.PageParam
	Module    string
	SubModule string
	UserName  string
	UserIP    string
	Result    *int8
	BeginTime *string
	EndTime   *string
}

// LoginLogWriter is the login log write port (write side).
type LoginLogWriter interface {
	Append(ctx context.Context, l *LoginLog) error
}

// OperationLogWriter is the operation log write port (write side).
type OperationLogWriter interface {
	Append(ctx context.Context, l *OperationLog) error
}

// LoginLogReader is the login log read port.
type LoginLogReader interface {
	List(ctx context.Context, f LoginLogFilter) (domain.PageResult[*LoginLog], error)
	// DeleteByIDs physically deletes (audit log cleanup is a physical delete; the existing implementation uses Unscoped).
	DeleteByIDs(ctx context.Context, ids []int64) error
	// Clean clears the login logs of the current tenant.
	Clean(ctx context.Context) error
}

// OperationLogReader is the operation log read port.
type OperationLogReader interface {
	List(ctx context.Context, f OperationLogFilter) (domain.PageResult[*OperationLog], error)
	DeleteByIDs(ctx context.Context, ids []int64) error
	Clean(ctx context.Context) error
}

// IDList parses a comma-separated ID string into an ID slice.
//
// Before the refactor, service/login_log.go and service/operation_log.go each had a verbatim
// identical parseIDsCSV; it is now consolidated into a single implementation in the domain layer.
func IDList(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ids = append(ids, iam.IDsFromStrings([]string{p})...)
	}
	return ids
}

// ClipRunes truncates a string by rune (single funnel for audit log field lengths, preventing over-column truncation).
func ClipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
