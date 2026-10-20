package persistence

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/auditlog"
	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// tenantRepo is the GORM implementation of iam.TenantRepository.
//
// t_tenant is a global table (the tenant registry itself, no tenant_id), so the tenant plugin
// does not inject; other in-tenant tables are still filtered by the plugin.
type tenantRepo struct{}

// NewTenantRepository constructs the tenant repository.
func NewTenantRepository() iam.TenantRepository { return tenantRepo{} }

func (tenantRepo) Save(ctx context.Context, t *iam.Tenant) error {
	if t.ID == 0 {
		if err := db(ctx).Create(toTenantPO(t)).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to create tenant")
		}
		return nil
	}
	res := db(ctx).Model(&model.Tenant{}).Where("id = ?", t.ID).Updates(map[string]any{
		"name":           t.Name,
		"contact_name":   t.ContactName,
		"contact_mobile": t.ContactMobile,
		"status":         t.Status,
		"expire_time":    t.ExpireTime,
		"remark":         t.Remark,
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update tenant")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "tenant not found")
	}
	return appendEvents(ctx, t.PullEvents()...)
}

func (tenantRepo) Get(ctx context.Context, id int64) (*iam.Tenant, error) {
	var po model.Tenant
	if err := db(ctx).Where("id = ?", id).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "tenant not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query tenant")
	}
	return toTenantDomain(&po), nil
}

// GetByCode fetches a tenant by code (the resolution path for the login input tenant_code).
//
// In the login scenario ctx has not resolved a tenant yet; the tenant plugin does not inject for
// global tables anyway, so this is safe here.
func (tenantRepo) GetByCode(ctx context.Context, code string) (*iam.Tenant, error) {
	var po model.Tenant
	if err := db(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "tenant not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query tenant")
	}
	return toTenantDomain(&po), nil
}

func (tenantRepo) List(ctx context.Context, f iam.TenantFilter) (domain.PageResult[*iam.Tenant], error) {
	conn := db(ctx).Model(&model.Tenant{})
	if f.Name != "" {
		conn = conn.Where("name LIKE ?", "%"+f.Name+"%")
	}
	if f.Code != "" {
		conn = conn.Where("code = ?", f.Code)
	}
	if f.Status != nil {
		conn = conn.Where("status = ?", *f.Status)
	}
	page, err := gormx.FindPage[model.Tenant](conn.Order("id DESC"), f.PageParam)
	if err != nil {
		return domain.PageResult[*iam.Tenant]{}, errs.Wrap(err, errs.CodeInternal, "failed to query tenant list")
	}
	return domain.MapPage(page, func(po model.Tenant) *iam.Tenant { return toTenantDomain(&po) }), nil
}

// Delete soft-deletes the tenant and rewrites the code to bypass the global unique index uk_code.
func (tenantRepo) Delete(ctx context.Context, t *iam.Tenant, operator string) error {
	res := db(ctx).Model(&model.Tenant{}).Where("id = ?", t.ID).Updates(map[string]any{
		"code":       iam.SoftDeletedCode(t.Code, t.ID),
		"deleted_at": time.Now(),
		"updater":    operator,
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete tenant")
	}
	return appendEvents(ctx, t.PullEvents()...)
}

func (tenantRepo) CodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	return existsBy(ctx, &model.Tenant{}, "code", code, excludeID, "failed to check tenant code uniqueness")
}

func (tenantRepo) IsAnyUserExists(ctx context.Context, tenantID int64) (bool, error) {
	// explicitly carry tenant_id: cross-tenant table queries must express the scope themselves (the
	// tenant plugin injects by ctx, and here ctx is the tenant-management-side global ctx, not the target tenant's ctx).
	var cnt int64
	if err := db(ctx).Model(&model.User{}).
		Where("tenant_id = ?", tenantID).Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query tenant users")
	}
	return cnt > 0, nil
}

func toTenantPO(t *iam.Tenant) *model.Tenant {
	return &model.Tenant{
		BaseModel:     model.BaseModel{Updater: t.Code},
		Name:          t.Name,
		Code:          t.Code,
		ContactName:   t.ContactName,
		ContactMobile: t.ContactMobile,
		Status:        t.Status,
		ExpireTime:    t.ExpireTime,
		Remark:        t.Remark,
	}
}

func toTenantDomain(po *model.Tenant) *iam.Tenant {
	return &iam.Tenant{
		ID:            po.ID,
		Name:          po.Name,
		Code:          po.Code,
		ContactName:   po.ContactName,
		ContactMobile: po.ContactMobile,
		Status:        po.Status,
		ExpireTime:    po.ExpireTime,
		Remark:        po.Remark,
		CreateTime:    po.CreateTime,
		UpdateTime:    po.UpdateTime,
	}
}

// Audit column widths (aligned with the model gorm tags; overlong values are truncated so rows are never lost).
// Before the refactor these were scattered in dao/login_log.go:18-23; the same semantics are kept here.
const (
	loginLogMaxUsername  = 30
	loginLogMaxIP        = 50
	loginLogMaxUserAgent = 512
	loginLogMaxGeo       = 64
)

// loginLogRepo is the GORM implementation of auditlog.LoginLogWriter + LoginLogReader.
type loginLogRepo struct{}

// NewLoginLogRepository constructs the login log repository.
func NewLoginLogRepository() auditlog.LoginLogWriter { return loginLogRepo{} }

// Append persists one login audit row.
//
// Best-effort semantics preserved: a failed audit write does not affect the login result, only a warning is logged.
// Values are truncated to the column width before persistence — the gateway imposes no length limit
// on UA/Geo, and under MySQL strict mode overlong values get the whole row rejected (losing the audit
// row is worse than truncating it).
func (loginLogRepo) Append(ctx context.Context, l *auditlog.LoginLog) error {
	if l == nil {
		return nil
	}
	po := &model.LoginLog{
		Username:    auditlog.ClipRunes(l.Username, loginLogMaxUsername),
		IP:          auditlog.ClipRunes(l.IP, loginLogMaxIP),
		UserAgent:   auditlog.ClipRunes(l.UserAgent, loginLogMaxUserAgent),
		GeoCountry:  auditlog.ClipRunes(l.GeoCountry, loginLogMaxGeo),
		GeoProvince: auditlog.ClipRunes(l.GeoProvince, loginLogMaxGeo),
		GeoCity:     auditlog.ClipRunes(l.GeoCity, loginLogMaxGeo),
		GeoIsp:      auditlog.ClipRunes(l.GeoISP, loginLogMaxGeo),
		UserID:      l.UserID,
		Result:      l.Result,
		Code:        l.Code,
	}
	if err := db(ctx).Create(po).Error; err != nil {
		// a lost audit row needs visibility, hence Warn rather than silence.
		return errs.Wrap(err, errs.CodeInternal, "failed to persist login audit")
	}
	return nil
}

// NewLoginLogReader constructs the login log read port.
func NewLoginLogReader() auditlog.LoginLogReader { return loginLogRepo{} }

func (loginLogRepo) List(ctx context.Context, f auditlog.LoginLogFilter) (domain.PageResult[*auditlog.LoginLog], error) {
	conn := db(ctx).Model(&model.LoginLog{})
	if f.Username != "" {
		conn = conn.Where("username LIKE ?", "%"+f.Username+"%")
	}
	if f.IP != "" {
		conn = conn.Where("ip LIKE ?", "%"+f.IP+"%")
	}
	if f.Result != nil {
		conn = conn.Where("result = ?", *f.Result)
	}
	if f.BeginTime != nil {
		conn = conn.Where("create_time >= ?", *f.BeginTime)
	}
	if f.EndTime != nil {
		conn = conn.Where("create_time <= ?", *f.EndTime)
	}
	page, err := gormx.FindPage[model.LoginLog](conn.Order("id DESC"), f.PageParam)
	if err != nil {
		return domain.PageResult[*auditlog.LoginLog]{}, errs.Wrap(err, errs.CodeInternal, "failed to query login audit list")
	}
	out := make([]*auditlog.LoginLog, 0, len(page.List))
	for i := range page.List {
		out = append(out, toLoginLogDomain(&page.List[i]))
	}
	return domain.PageResult[*auditlog.LoginLog]{
		List: out, Total: page.Total, PageNum: page.PageNum,
		PageSize: page.PageSize, Pages: page.Pages,
	}, nil
}

// DeleteByIDs batch-deletes audit rows physically.
func (loginLogRepo) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := db(ctx).Unscoped().Where("id IN ?", ids).
		Delete(&model.LoginLog{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to delete login audit rows")
	}
	return nil
}

// Clean deletes all audit rows of the current tenant.
//
// After Unscoped the tenant plugin no longer injects conditions, so tenant_id must be specified
// explicitly — otherwise other tenants would be cleaned as well (the existing implementation
// dao/login_log.go:97 handles it explicitly the same way).
func (loginLogRepo) Clean(ctx context.Context) error {
	tid, err := tenantOf(ctx)
	if err != nil {
		return err
	}
	if err := db(ctx).Unscoped().Where("tenant_id = ?", tid).
		Delete(&model.LoginLog{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to clean login audit rows")
	}
	return nil
}

func toLoginLogDomain(po *model.LoginLog) *auditlog.LoginLog {
	return &auditlog.LoginLog{
		ID:          po.ID,
		TenantID:    po.TenantID,
		Username:    po.Username,
		UserID:      po.UserID,
		IP:          po.IP,
		UserAgent:   po.UserAgent,
		GeoCountry:  po.GeoCountry,
		GeoProvince: po.GeoProvince,
		GeoCity:     po.GeoCity,
		GeoISP:      po.GeoIsp,
		Result:      po.Result,
		Code:        po.Code,
		CreateTime:  po.CreateTime,
	}
}

// operationLogRepo is the GORM implementation of auditlog.OperationLogReader.
//
// Read-only + delete/clean: the write side is still persisted by the humakit audit-collection
// middleware via devops-infra/audit.Sink (preserving the existing dual-track approach, without
// changing the audit collection scope whitelist).
type operationLogRepo struct{}

// NewOperationLogReader constructs the operation log read port.
func NewOperationLogReader() auditlog.OperationLogReader { return operationLogRepo{} }

func (operationLogRepo) List(ctx context.Context, f auditlog.OperationLogFilter) (domain.PageResult[*auditlog.OperationLog], error) {
	conn := db(ctx).Model(&model.OperationLog{})
	if f.Module != "" {
		conn = conn.Where("module = ?", f.Module)
	}
	if f.SubModule != "" {
		conn = conn.Where("sub_module = ?", f.SubModule)
	}
	if f.UserName != "" {
		conn = conn.Where("user_name LIKE ?", "%"+f.UserName+"%")
	}
	if f.UserIP != "" {
		conn = conn.Where("user_ip LIKE ?", "%"+f.UserIP+"%")
	}
	if f.Result != nil {
		conn = conn.Where("result = ?", *f.Result)
	}
	if f.BeginTime != nil {
		conn = conn.Where("create_time >= ?", *f.BeginTime)
	}
	if f.EndTime != nil {
		conn = conn.Where("create_time <= ?", *f.EndTime)
	}
	page, err := gormx.FindPage[model.OperationLog](conn.Order("id DESC"), f.PageParam)
	if err != nil {
		return domain.PageResult[*auditlog.OperationLog]{}, errs.Wrap(err, errs.CodeInternal, "failed to query operation audit list")
	}
	return domain.MapPage(page, func(po model.OperationLog) *auditlog.OperationLog {
		return toOperationLogDomain(&po)
	}), nil
}

func (operationLogRepo) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if err := db(ctx).Unscoped().Where("id IN ?", ids).
		Delete(&model.OperationLog{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to delete operation audit rows")
	}
	return nil
}

func (operationLogRepo) Clean(ctx context.Context) error {
	tid, err := tenantOf(ctx)
	if err != nil {
		return err
	}
	if err := db(ctx).Unscoped().Where("tenant_id = ?", tid).
		Delete(&model.OperationLog{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to clean operation audit rows")
	}
	return nil
}

func toOperationLogDomain(po *model.OperationLog) *auditlog.OperationLog {
	return &auditlog.OperationLog{
		ID:            po.ID,
		TenantID:      po.TenantID,
		TraceID:       po.TraceID,
		UserID:        po.UserID,
		UserName:      po.UserName,
		UserType:      int64(po.UserType),
		Module:        po.Module,
		SubModule:     po.SubModule,
		BizID:         po.BizID,
		Contents:      po.Contents,
		Extra:         po.Extra,
		RequestMethod: po.RequestMethod,
		RequestURL:    po.RequestURL,
		UserIP:        po.UserIP,
		UserAgent:     po.UserAgent,
		GeoCountry:    po.GeoCountry,
		GeoProvince:   po.GeoProvince,
		GeoCity:       po.GeoCity,
		GeoISP:        po.GeoISP,
		Result:        po.Result,
		Code:          po.Code,
		CreateTime:    po.CreateTime,
	}
}
