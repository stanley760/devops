// Role management dao (P6): p row rebuild logic lives in the service layer (casbin and DB transaction boundaries).
package dao

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-common/domain/gormx"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// ListRoles paginates roles (filter by name/code/status).
func ListRoles(ctx context.Context, q types.RolePageQuery) (domain.PageResult[model.Role], error) {
	db := tenantdb.DB(ctx).Model(&model.Role{})
	if q.Name != "" {
		db = db.Where("name LIKE ?", "%"+q.Name+"%")
	}
	if q.Code != "" {
		db = db.Where("code = ?", q.Code)
	}
	if q.Status != nil {
		db = db.Where("status = ?", *q.Status)
	}

	page, err := gormx.FindPage[model.Role](
		db.Order("sort ASC, id ASC"),
		domain.PageParam{PageNum: q.PageNum, PageSize: q.PageSize},
	)
	if err != nil {
		return domain.PageResult[model.Role]{}, errs.Wrap(err, errs.CodeInternal, "failed to query role list")
	}
	return page, nil
}

// UpdateRole updates role fields (map form, non-zero fields only).
func UpdateRole(ctx context.Context, roleID int64, updates map[string]any) error {
	if err := tenantdb.DB(ctx).Model(&model.Role{}).
		Where("id = ?", roleID).Updates(updates).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to update role")
	}
	return nil
}

// GetRoleByID fetches a role by ID (within the tenant).
func GetRoleByID(ctx context.Context, roleID int64) (*model.Role, error) {
	var r model.Role
	err := tenantdb.DB(ctx).Where("id = ?", roleID).First(&r).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "role not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query role")
	}
	return &r, nil
}

// RoleCodeExists reports whether the role code is already taken within the tenant (excludeID excludes itself).
func RoleCodeExists(ctx context.Context, code string, excludeID int64) (bool, error) {
	db := tenantdb.DB(ctx).Model(&model.Role{}).Where("code = ?", code)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	var cnt int64
	if err := db.Count(&cnt).Error; err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to check role code uniqueness")
	}
	return cnt > 0, nil
}

// CreateRole creates a role (primary key is assigned).
func CreateRole(ctx context.Context, r *model.Role) error {
	if err := tenantdb.DB(ctx).Create(r).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create role")
	}
	return nil
}

// SoftDeleteRoleTx soft-deletes a role within a transaction: rewrites the code to bypass
// the tenant-scoped unique index (a role with the same name can be recreated).
func SoftDeleteRoleTx(tx *gorm.DB, roleID int64, code string) error {
	res := tx.Model(&model.Role{}).Where("id = ?", roleID).Updates(map[string]any{
		"code":       code + "_del_" + strconv.FormatInt(roleID, 10),
		"deleted_at": time.Now(),
	})
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete role")
	}
	return nil
}

// GetRoleMenuIDs returns the role's assigned menu IDs (tenant_id is injected by the plugin).
func GetRoleMenuIDs(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := tenantdb.DB(ctx).Model(&model.RoleMenu{}).
		Where("role_id = ?", roleID).Pluck("menu_id", &ids).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query role menus")
	}
	return ids, nil
}

// SyncRoleMenusTx rebuilds role-menu associations within a transaction (physical delete old + batch insert new).
func SyncRoleMenusTx(tx *gorm.DB, roleID int64, menuIDs []int64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenu{}).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to clean role menus")
	}
	if len(menuIDs) == 0 {
		return nil
	}
	rows := make([]model.RoleMenu, 0, len(menuIDs))
	for _, mid := range menuIDs {
		rows = append(rows, model.RoleMenu{RoleID: roleID, MenuID: mid})
	}
	return tx.Create(&rows).Error
}

// RoleIDsByMenuIDs returns the role IDs referencing the given menu set (used to rebuild p rows).
func RoleIDsByMenuIDs(ctx context.Context, menuIDs []int64) ([]int64, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var ids []int64
	err := tenantdb.DB(ctx).Model(&model.RoleMenu{}).
		Where("menu_id IN ?", menuIDs).Distinct().Pluck("role_id", &ids).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query roles referencing menus")
	}
	return ids, nil
}

// ClearRoleBindingsTx cleans up a role's user associations within a transaction (on role deletion).
func ClearRoleBindingsTx(tx *gorm.DB, roleID int64) error {
	return tx.Where("role_id = ?", roleID).Delete(&model.UserRole{}).Error
}

// ReplaceRoleMenus rebuilds a role's menu associations (single transaction).
func ReplaceRoleMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		return SyncRoleMenusTx(tx, roleID, menuIDs)
	})
}

// DeleteRole deletes a role (single transaction): soft delete (rewrite code) -> clear user associations -> clear menu associations.
func DeleteRole(ctx context.Context, r *model.Role) error {
	return WithTx(ctx, func(tx *gorm.DB) error {
		if err := SoftDeleteRoleTx(tx, r.ID, r.Code); err != nil {
			return err
		}
		if err := ClearRoleBindingsTx(tx, r.ID); err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to clean user roles")
		}
		if err := tx.Where("role_id = ?", r.ID).Delete(&model.RoleMenu{}).Error; err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to delete role")
		}
		return nil
	})
}
