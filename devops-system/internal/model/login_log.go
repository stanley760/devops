// t_system_login_log login audit table (tenant table). One row per login attempt (success and failure alike),
// for audit tracing; persisted by service.Auth.login's defer (best-effort, does not affect the login result;
// dao.CreateLoginLog truncates to the column width before persistence, preventing overlong UA/Geo headers from getting the whole row rejected).
// Query/delete/clean go through the admin endpoint /system/api/v1/login-logs (system:loginlog:* permission gated).
// Attempts failing tenant validation have no tenant ctx (TenantPlugin fails closed and rejects persistence),
// visible only in the access log and the auth_login metric — the audit table only covers the tenant-resolved path.
package model

// LoginLog is the login audit.
type LoginLog struct {
	BaseTenantModel
	Username    string `gorm:"size:30;not null;index"`   // username attempted for login (recorded even on failure)
	UserID      int64  `gorm:"not null;default:0;index"` // filled once the user is matched; 0 for unknown users
	IP          string `gorm:"size:50;not null;index"`   // client's real IP (XFF chain + trusted_proxies resolution)
	UserAgent   string `gorm:"size:512"`                 // login-end UA
	GeoCountry  string `gorm:"size:64"`                  // the four fields below come from the X-Higress-Geo-* headers injected by the geo-ip plugin
	GeoProvince string `gorm:"size:64"`
	GeoCity     string `gorm:"size:64"`
	GeoIsp      string `gorm:"size:64"`
	Result      int8   `gorm:"not null;default:0"`   // 0 success 1 failure
	Code        int    `gorm:"not null;default:200"` // business code (same as the code label of the auth_login metric)
}

// TableName is the table name.
func (LoginLog) TableName() string { return "t_system_login_log" }
