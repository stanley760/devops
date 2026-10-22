// Login audit query module DTOs (admin-side read side of t_system_login_log; for the write side, see service.Auth.login).
package types

import (
	"time"

	"github.com/stanley760/devops/devops-common/domain"
)

// LoginLogPageQuery holds the GET /login-logs query parameters.
type LoginLogPageQuery struct {
	domain.PageParam
	Username  string     `doc:"username, fuzzy match (failed attempts are recorded too)"`
	IP        string     `doc:"client IP, fuzzy match (supports subnet prefixes, e.g. \"192.168.\")"`
	Result    *int8      `doc:"login result 0: success 1: failure; omitting means no filter"`
	BeginTime *time.Time `doc:"begin time (inclusive), lower bound of create_time"`
	EndTime   *time.Time `doc:"end time (inclusive), upper bound of create_time"`
}

// LoginLogDTO is a login audit row.
type LoginLogDTO struct {
	ID          ID        `json:"id" doc:"record id"`
	Username    string    `json:"username" doc:"username"`
	UserID      ID        `json:"userId" doc:"user id, 0 for unknown users"`
	IP          string    `json:"ip" doc:"client IP"`
	UserAgent   string    `json:"userAgent" doc:"client User-Agent"`
	GeoCountry  string    `json:"geoCountry" doc:"country (injected by the higress geo-ip plugin header)"`
	GeoProvince string    `json:"geoProvince" doc:"province (injected by the higress geo-ip plugin header)"`
	GeoCity     string    `json:"geoCity" doc:"city (injected by the higress geo-ip plugin header)"`
	GeoIsp      string    `json:"geoIsp" doc:"ISP (injected by the higress geo-ip plugin header)"`
	Result      int8      `json:"result" doc:"login result 0: success 1: failure"`
	Code        int       `json:"code" doc:"business code, same as the auth_login metric code label"`
	CreateTime  DateTime `json:"createTime" doc:"login time"`
}
