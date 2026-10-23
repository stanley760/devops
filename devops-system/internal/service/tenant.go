// Tenant management service: CRUD + enable/disable/expiry + tenant initial super admin.
//
// Platform route ctx carries no tenant: t_tenant is a global table and can be
// accessed directly; when creating a tenant's initial super admin, inject the
// new tenant ctx explicitly so TenantPlugin fills tenant_id on the user/role rows.
//
// Disable semantics: persist the status + revoke all of the tenant's sessions
// (RevokeTenantSessions, every access token immediately 401s) + clear the Redis
// status cache of the Tenant middleware (guarding against a 5min cache letting
// requests through).
package service

import (
	"context"
	"fmt"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"
	infratenant "github.com/stanley760/devops/devops-infra/tenant"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/store"
	"github.com/stanley760/devops/devops-system/internal/types"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// Tenant is the tenant management service.
type Tenant struct {
	authM *auth.Manager
}

// NewTenant creates the service.
func NewTenant(authM *auth.Manager) *Tenant {
	return &Tenant{authM: authM}
}

// List returns a paginated query.
func (s *Tenant) List(ctx context.Context, q types.TenantPageQuery) (domain.PageResult[types.TenantDTO], error) {
	page, err := dao.ListTenants(ctx, q)
	if err != nil {
		return domain.PageResult[types.TenantDTO]{}, err
	}
	return domain.MapPage(page, func(tenant model.Tenant) types.TenantDTO {
		return toTenantDTO(&tenant)
	}), nil
}

// Get returns the tenant details.
func (s *Tenant) Get(ctx context.Context, id types.ID) (*types.TenantDTO, error) {
	t, err := dao.GetTenantByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toTenantDTO(t)
	return &dto, nil
}

// Create creates a tenant + the tenant's initial super admin (super_admin role + menu grants +
// casbin domain-wide allow-all p/g rows).
func (s *Tenant) Create(ctx context.Context, claims *auth.AccessClaims, req types.TenantCreateReq) (*types.TenantDTO, error) {
	exists, err := dao.TenantCodeExists(ctx, req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errs.New(errs.CodeParam, "tenant code already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), 10)
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}

	t := &model.Tenant{
		BaseModel:     model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		Name:          req.Name,
		Code:          req.Code,
		ContactName:   req.ContactName,
		ContactMobile: req.ContactMobile,
		ExpireTime:    req.ExpireTime,
		Remark:        req.Remark,
		Status:        1,
	}
	if err := dao.CreateTenant(ctx, t); err != nil {
		return nil, err
	}

	// Writing the tenant's initial super admin to the shared database still
	// requires the tenant context to be injected, to fill tenant_id.
	tctx := infratenant.With(ctx, t.ID, t.Code)
	admin := &model.User{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Username: req.AdminUsername,
		Password: string(hash),
		Nickname: req.AdminUsername,
		Status:   1,
	}
	role := &model.Role{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Name: "超级管理员", Code: authz.Roles().SuperAdmin, Type: 1, DataScope: 1, Status: 1,
	}
	if err := dao.SeedTenantAdmin(tctx, role, admin); err != nil {
		// Compensation: roll back the tenant row, avoiding the intermediate state of "tenant without super admin".
		_ = dao.HardDeleteTenant(ctx, t.ID)
		return nil, err
	}

	if err := authz.SeedTenantAdmin(t.ID, admin.ID); err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "tenant created, but the super admin authorization write failed; please retry")
	}
	dto := toTenantDTO(t)
	return &dto, nil
}

// Update updates a tenant (name/contact/expire_time/remark; the code cannot be changed).
func (s *Tenant) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.TenantUpdateReq) error {
	t, err := dao.GetTenantByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":           req.Name,
		"contact_name":   req.ContactName,
		"contact_mobile": req.ContactMobile,
		"remark":         req.Remark,
		"updater":        claims.Username,
	}
	if req.ExpireTime != nil {
		updates["expire_time"] = *req.ExpireTime
		var now = time.Now()
		if req.ExpireTime.After(now) {
			updates["status"] = 1
		} else {
			updates["status"] = 0
		}
	} else {
		updates["expire_time"] = nil // never expires
	}
	return dao.UpdateTenant(ctx, t.ID, updates)
}

// UpdateStatus enables/disables a tenant: disabling kicks everyone offline (all sessions revoked + status cache cleared).
func (s *Tenant) UpdateStatus(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.TenantStatusReq) error {
	t, err := dao.GetTenantByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if t.ID == 1 && req.Status != 1 {
		return errs.New(errs.CodeParam, "the default tenant cannot be disabled")
	}
	if t.Status == req.Status {
		return nil // idempotent: an unchanged status succeeds directly
	}

	if err := dao.UpdateTenant(ctx, t.ID, map[string]any{
		"status":  req.Status,
		"updater": claims.Username,
	}); err != nil {
		return err
	}

	// Clear the Tenant middleware status cache (on disable, guard against a 5min
	// cache letting requests through; on enable, guard against reading a stale disabled state)
	if c := store.Cache(); c != nil {
		_ = c.Delete(ctx, fmt.Sprintf("tenant:%d", t.ID))
	}
	if req.Status == 0 {
		s.authM.RevokeTenantSessions(ctx, t.ID)
	}
	return nil
}

// Delete deletes a tenant: the default tenant cannot be deleted; a tenant that
// is not disabled cannot be deleted; data is retained after logical deletion.
func (s *Tenant) Delete(ctx context.Context, claims *auth.AccessClaims, id types.ID) error {
	t, err := dao.GetTenantByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if t.ID == 1 {
		return errs.New(errs.CodeParam, "the default tenant cannot be deleted")
	}
	if t.Status == 1 {
		return errs.New(errs.CodeParam, "tenant is not disabled; disable it before deleting")
	}

	if err := dao.SoftDeleteTenant(ctx, t.ID, t.Code); err != nil {
		return err
	}

	// Safety net: clear residual sessions and the status cache (already revoked on disable; this is the fallback)
	s.authM.RevokeTenantSessions(ctx, t.ID)
	if c := store.Cache(); c != nil {
		_ = c.Delete(ctx, fmt.Sprintf("tenant:%d", t.ID))
	}
	// The casbin g/p rows and business data are retained (a deleted tenant cannot log in); a cleanup tool is on the Roadmap.
	return nil
}

// toTenantDTO maps model.Tenant → DTO.
func toTenantDTO(t *model.Tenant) types.TenantDTO {
	return types.TenantDTO{
		ID:            types.ID(t.ID),
		Name:          t.Name,
		Code:          t.Code,
		ContactName:   t.ContactName,
		ContactMobile: t.ContactMobile,
		Status:        t.Status,
		ExpireTime:    types.DateTimePtr(t.ExpireTime),
		Remark:        t.Remark,
		CreateTime:    types.NewDateTime(t.CreateTime),
	}
}
