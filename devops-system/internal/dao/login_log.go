// Login audit: write path (invoked via defer in service.Auth.login) + admin query/delete/clean.
package dao

import (
	"context"
	"log/slog"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Audit column widths (aligned with model.LoginLog gorm tags; overlong values are truncated so rows are never lost).
const (
	loginLogMaxUsername  = 30
	loginLogMaxIP        = 50
	loginLogMaxUserAgent = 512
	loginLogMaxGeo       = 64
)

// CreateLoginLog persists one login audit row. Best-effort: a failed audit write must not
// break login; it only logs a slog warning (same semantics as TouchLogin, but a lost audit
// row needs visibility).
// The caller must have injected a tenant ctx (TenantPlugin is fail-closed; a missing tenant
// ctx errors out and is swallowed here).
// Inputs come from request headers (UA/Geo) and the request body (username) with no gateway-side
// length limits: values are truncated to the column width before persistence to avoid the whole
// row being rejected under MySQL strict mode (losing the audit row is worse than truncating it).
func CreateLoginLog(ctx context.Context, log *model.LoginLog) {
	if log == nil {
		return
	}
	log.Username = clipRunes(log.Username, loginLogMaxUsername)
	log.IP = clipRunes(log.IP, loginLogMaxIP)
	log.UserAgent = clipRunes(log.UserAgent, loginLogMaxUserAgent)
	log.GeoCountry = clipRunes(log.GeoCountry, loginLogMaxGeo)
	log.GeoProvince = clipRunes(log.GeoProvince, loginLogMaxGeo)
	log.GeoCity = clipRunes(log.GeoCity, loginLogMaxGeo)
	log.GeoIsp = clipRunes(log.GeoIsp, loginLogMaxGeo)

	if err := tenantdb.DB(ctx).Create(log).Error; err != nil {
		slog.WarnContext(ctx, "failed to persist login audit", "username", log.Username, "err", err)
	}
}

// ListLoginLogs paginates login audit rows (filter by username/ip/result/time range, newest first).
func ListLoginLogs(ctx context.Context, q types.LoginLogPageQuery) (domain.PageResult[model.LoginLog], error) {
	db := tenantdb.DB(ctx).Model(&model.LoginLog{})
	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.IP != "" {
		db = db.Where("ip LIKE ?", "%"+q.IP+"%")
	}
	if q.Result != nil {
		db = db.Where("result = ?", *q.Result)
	}
	if q.BeginTime != nil {
		db = db.Where("create_time >= ?", *q.BeginTime)
	}
	if q.EndTime != nil {
		db = db.Where("create_time <= ?", *q.EndTime)
	}

	page, err := gormx.FindPage[model.LoginLog](
		db.Order("id DESC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.LoginLog]{}, errs.Wrap(err, errs.CodeInternal, "failed to query login audit list")
	}
	return page, nil
}

// DeleteLoginLogs batch-deletes audit rows (tenant_id is injected by TenantPlugin, so cross-tenant IDs are unreachable).
// Returns the number of rows actually deleted.
func DeleteLoginLogs(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := tenantdb.DB(ctx).Unscoped().
		Where("id IN ?", ids).Delete(&model.LoginLog{})
	if res.Error != nil {
		return 0, errs.Wrap(res.Error, errs.CodeInternal, "failed to delete login audit rows")
	}
	return res.RowsAffected, nil
}

// CleanLoginLogs deletes all audit rows of the current tenant (ops retention-control entry point), returning the deleted row count.
// Physical delete: audit rows have no soft-delete semantics; keeping deleted_at rows would only keep the table growing.
func CleanLoginLogs(ctx context.Context) (int64, error) {
	tid, err := tenantOf(ctx)
	if err != nil {
		return 0, err
	}
	res := tenantdb.DB(ctx).Unscoped().
		Where("tenant_id = ?", tid).Delete(&model.LoginLog{})
	if res.Error != nil {
		return 0, errs.Wrap(res.Error, errs.CodeInternal, "failed to clean login audit rows")
	}
	return res.RowsAffected, nil
}

// clipRunes truncates by rune count (MySQL varchar lengths count characters; byte-wise
// truncation would cut invalid UTF-8 — Chinese place names like "黑龙江省" (Heilongjiang Province) are exactly the
// high-risk case). If len(s) bytes is already within n, n characters is guaranteed, so the fast path returns directly.
func clipRunes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
