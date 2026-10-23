// Tenant management dao (P7): t_tenant is a global table (no tenant_id; TenantPlugin lets it through),
// and the platform route ctx can access it directly without a tenant.
package dao

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ListTenants paginates tenants (filter by name/code/status).
func ListTenants(ctx context.Context, q types.TenantPageQuery) (domain.PageResult[model.Tenant], error) {
	db := tenantdb.DB(ctx).Model(&model.Tenant{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}

	page, err := gormx.FindPage[model.Tenant](
		db.Order("id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.Tenant]{}, errs.Wrap(err, errs.CodeInternal, "failed to query tenant list")
	}
	return page, nil
}

// GetTenantByCode fetches a tenant by code (for login: a miss maps to a 401 "tenant not found" semantics).
func GetTenantByCode(ctx context.Context, code string) (*model.Tenant, error) {
	var t model.Tenant
	err := tenantdb.DB(ctx).Where("code = ?", code).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeUnauthorized, "tenant not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query tenant")
	}
	return &t, nil
}

// GetTenantByID fetches a tenant by ID.
func GetTenantByID(ctx context.Context, tenantID int64) (*model.Tenant, error) {
	var t model.Tenant
	err := tenantdb.DB(ctx).Where("id = ?", tenantID).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "tenant not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query tenant")
	}
	return &t, nil
}

// TenantCodeExists reports whether the tenant code is already taken (globally unique).
func TenantCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.Tenant{}).Where("code = ?", code)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check tenant code uniqueness")
	}
	return cnt > 0, nil
}

// CreateTenant creates a tenant (primary key is assigned, biz_tag=tenant).
func CreateTenant(ctx context.Context, t *model.Tenant) error {
	if err := tenantdb.DB(ctx).Create(t).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create tenant")
	}
	return nil
}

// UpdateTenant updates tenant fields.
func UpdateTenant(ctx context.Context, tenantID int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.Tenant{}).
		Where("id = ?", tenantID).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update tenant")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "tenant not found")
	}
	return nil
}

// SoftDeleteTenantTx soft-deletes a tenant within a transaction: rewrites the code to bypass
// the global unique index (the same code can be recreated).
func SoftDeleteTenantTx(tx *gorm.DB, tenantID int64, code string) error {
	res := tx.Model(&model.Tenant{}).Where("id = ?", tenantID).Updates(map[string]any{
		"code":       code + "_del_" + strconv.FormatInt(tenantID, 10),
		"deleted_at": time.Now(),
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete tenant")
	}
	return nil
}

// SoftDeleteTenant soft-deletes a tenant (single transaction, see SoftDeleteTenantTx).
func SoftDeleteTenant(ctx context.Context, tenantID int64, code string) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		return SoftDeleteTenantTx(tx, tenantID, code)
	})
}

// HardDeleteTenant physically deletes the tenant row: for compensating rollback in the creation flow.
func HardDeleteTenant(ctx context.Context, tenantID int64) error {
	return tenantdb.DB(ctx).Unscoped().Delete(&model.Tenant{}, tenantID).Error
}

// SeedTenantAdmin creates the initial super admin for a new tenant (super_admin role + user +
// user_role + the role's menu grants, single transaction).
// ctx must be the new tenant's ctx, used to populate tenant_id.
// The super admin is seeded with the full global menu catalog minus the platform-only subtree:
// menus whose component lives under "platform/" and their buttons belong to platform routes and
// must not appear in a tenant admin's sidebar (the platform gate would 403 them anyway).
func SeedTenantAdmin(ctx context.Context, role *model.Role, admin *model.User) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(role).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to create tenant super admin role")
		}
		if err := tx.Create(admin).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to create tenant super admin user")
		}
		if err := tx.Create(&model.UserRole{UserID: admin.ID, RoleID: role.ID}).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to bind tenant super admin role")
		}
		menuIDs, err := seedableMenuIDs(tx)
		if err != nil {
			return err
		}
		if len(menuIDs) == 0 {
			return nil
		}
		rows := make([]model.RoleMenu, 0, len(menuIDs))
		for _, id := range menuIDs {
			rows = append(rows, model.RoleMenu{RoleID: role.ID, MenuID: id})
		}
		// TenantPlugin fills tenant_id from ctx on create, scoping the grants to the new tenant.
		if err := tx.Create(&rows).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to seed tenant super admin menus")
		}
		return nil
	})
}

// seedableMenuIDs returns the menu IDs a new tenant's super admin starts with: the full global
// catalog (t_system_menu is a global table, the query is tenant-agnostic) minus the platform-only
// subtree — menus whose component lives under "platform/" and all their descendants.
func seedableMenuIDs(tx *gorm.DB) ([]int64, error) {
	var menus []model.Menu
	if err := tx.Find(&menus).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to list menus")
	}
	excluded := make(map[int64]bool)
	for _, m := range menus {
		if strings.HasPrefix(m.Component, "platform/") {
			excluded[m.ID] = true
		}
	}
	// propagate the exclusion down the tree (buttons of a platform menu)
	for grew := true; grew; {
		grew = false
		for _, m := range menus {
			if !excluded[m.ID] && excluded[m.ParentID] {
				excluded[m.ID] = true
				grew = true
			}
		}
	}
	ids := make([]int64, 0, len(menus))
	for _, m := range menus {
		if !excluded[m.ID] {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}
