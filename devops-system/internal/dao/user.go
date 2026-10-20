// Package dao is the data access layer.
//
// Double protection for tenant filtering: pure model queries get it injected automatically
// by TenantPlugin (tenantdb.DB(ctx)); native JOIN queries (where the plugin would inject
// ambiguous bare columns across multiple tables) explicitly carry tenant_id in the dao.
// The tenant ID is always taken from ctx (injected by middleware.Tenant; injected after
// tenant resolution in Login/Refresh).
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-infra/errs"

	infratenant "github.com/stanley760/devops/devops-infra/tenant"

	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

// tenantOf takes the tenant from ctx (for explicit filtering in native JOIN queries; a miss
// means caller misuse, so fail-fast).
func tenantOf(ctx context.Context) (int64, error) {
	tc, ok := infratenant.From(ctx)
	if !ok {
		return 0, errs.New(errs.CodeInternal, "dao is missing tenant context")
	}
	return tc.ID, nil
}

// GetUserByUsername fetches a user by username (tenant_id is injected by the plugin).
func GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := tenantdb.DB(ctx).Where("username = ?", username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeUnauthorized, "incorrect username or password")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	return &u, nil
}

// GetUserByID fetches a user by ID (tenant_id is injected by the plugin).
func GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	var u model.User
	err := tenantdb.DB(ctx).Where("id = ?", userID).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "user not found")
		}
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query user")
	}
	return &u, nil
}

// TouchLogin updates the last login info (best-effort; failure does not affect login).
func TouchLogin(ctx context.Context, userID int64, ip string) {
	tenantdb.DB(ctx).Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"login_ip":   ip,
			"login_date": gorm.Expr("CURRENT_TIMESTAMP(3)"),
		})
}

// GetUserRoles returns the user's roles within the tenant (native JOIN, explicit filtering).
func GetUserRoles(ctx context.Context, userID int64) ([]model.Role, error) {
	tid, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	var roles []model.Role
	err = tenantdb.DB(ctx).
		Table("t_system_role AS r").
		Joins("JOIN t_system_user_role ur ON ur.role_id = r.id AND ur.tenant_id = r.tenant_id").
		Where("ur.tenant_id = ? AND ur.user_id = ?", tid, userID).
		Find(&roles).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query roles")
	}
	return roles, nil
}

// GetUserMenus returns the menus visible to the user within the tenant (aggregated and
// deduplicated by role; native JOIN with explicit filtering).
// menuTypes filters menu types (1 directory 2 menu 3 button); status=1 means normal.
func GetUserMenus(ctx context.Context, userID int64, menuTypes []int8) ([]model.Menu, error) {
	tid, err := tenantOf(ctx)
	if err != nil {
		return nil, err
	}
	var menus []model.Menu
	q := tenantdb.DB(ctx).
		Table("t_system_menu AS m").
		Joins("JOIN t_system_role_menu rm ON rm.menu_id = m.id").
		Joins("JOIN t_system_user_role ur ON ur.role_id = rm.role_id AND ur.tenant_id = rm.tenant_id").
		Where("ur.tenant_id = ? AND ur.user_id = ? AND m.status = 1 AND m.deleted_at IS NULL",
			tid, userID).
		Distinct()
	if len(menuTypes) > 0 {
		q = q.Where("m.type IN ?", menuTypes)
	}
	err = q.Find(&menus).Error
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to query menus")
	}
	return menus, nil
}
