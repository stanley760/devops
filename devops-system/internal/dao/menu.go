// Menu management dao: menus are a global table and pass through TenantPlugin.
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// ListMenus returns all menus (the tree is assembled in memory by the service; optional status filter).
func ListMenus(ctx context.Context, status *int8) ([]model.Menu, error) {
	db := tenantdb.DB(ctx).Model(&model.Menu{})
	if status != nil {
		db = db.Where("status = ?", *status)
	}
	var menus []model.Menu
	if err := db.Order("sort ASC, id ASC").Find(&menus).Error; err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query menus")
	}
	return menus, nil
}

// GetMenuByID fetches a menu by ID.
func GetMenuByID(ctx context.Context, menuID int64) (*model.Menu, error) {
	var m model.Menu
	err := tenantdb.DB(ctx).Where("id = ?", menuID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "menu not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query menu")
	}
	return &m, nil
}

// MenuHasChildren reports whether non-deleted child menus exist.
func MenuHasChildren(ctx context.Context, menuID int64) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.Menu{}).
		Where("parent_id = ?", menuID).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query child menus")
	}
	return cnt > 0, nil
}

// MenuReferencedByRoles reports whether the menu is referenced by roles (role_menu).
func MenuReferencedByRoles(ctx context.Context, menuID int64) (bool, error) {
	var cnt int64
	err := tenantdb.DB(ctx).Model(&model.RoleMenu{}).
		Where("menu_id = ?", menuID).Count(&cnt).Error
	if err != nil {
		return false, errs.Wrap(err, errs.CodeInternal, "failed to query menu references")
	}
	return cnt > 0, nil
}

// CreateMenu creates a menu (primary key is assigned).
func CreateMenu(ctx context.Context, m *model.Menu) error {
	if err := tenantdb.DB(ctx).Create(m).Error; err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to create menu")
	}
	return nil
}

// UpdateMenu updates menu fields.
func UpdateMenu(ctx context.Context, menuID int64, updates map[string]any) error {
	res := tenantdb.DB(ctx).Model(&model.Menu{}).
		Where("id = ?", menuID).Updates(updates)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to update menu")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "menu not found")
	}
	return nil
}

// DeleteMenu soft-deletes a menu (the caller guarantees no child nodes and no references).
func DeleteMenu(ctx context.Context, menuID int64) error {
	res := tenantdb.DB(ctx).Delete(&model.Menu{}, menuID)
	if res.Error != nil {
		return errs.Wrap(res.Error, errs.CodeInternal, "failed to delete menu")
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.CodeNotFound, "menu not found")
	}
	return nil
}

// MenusByIDs fetches menus by a set of IDs (validates ownership + collects permission identifiers).
func MenusByIDs(ctx context.Context, menuIDs []int64) ([]model.Menu, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var menus []model.Menu
	err := tenantdb.DB(ctx).Where("id IN ?", menuIDs).Find(&menus).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query menus")
	}
	if len(menus) != len(menuIDs) {
		return nil, errs.New(errs.CodeParam, "invalid menu included")
	}
	return menus, nil
}

// RolesByIDs fetches roles by a set of IDs (used to rebuild p rows when permission changes).
func RolesByIDs(ctx context.Context, roleIDs []int64) ([]model.Role, error) {
	if len(roleIDs) == 0 {
		return nil, nil
	}
	var roles []model.Role
	err := tenantdb.DB(ctx).Where("id IN ?", roleIDs).Find(&roles).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query roles")
	}
	return roles, nil
}
