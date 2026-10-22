// Menu management service: CRUD + rebuilding the p rows of referencing roles when a permission changes.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-infra/errs"
	infratenant "github.com/stanley760/devops/devops-infra/tenant"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Menu is the menu management service.
type Menu struct{}

// NewMenu creates the service.
func NewMenu() *Menu { return &Menu{} }

// List returns the menu tree (status filter optional).
func (s *Menu) List(ctx context.Context, status *int8) ([]types.MenuTreeResp, error) {
	menus, err := dao.ListMenus(ctx, status)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(menus), nil
}

// Get returns the menu details.
func (s *Menu) Get(ctx context.Context, id types.ID) (*types.MenuDTO, error) {
	m, err := dao.GetMenuByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	dto := toMenuDTO(m)
	return &dto, nil
}

// Create creates a menu: validates the type/parent (a permission on a directory
// is meaningless; a button must have one).
func (s *Menu) Create(ctx context.Context, claims *auth.AccessClaims, req types.MenuSaveReq) error {
	if err := validateMenu(req.Type, req.ParentID.Int64(), req.Permission); err != nil {
		return err
	}
	if req.ParentID.Int64() > 0 {
		if _, err := dao.GetMenuByID(ctx, req.ParentID.Int64()); err != nil {
			return err
		}
	}

	m := &model.Menu{
		BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		Name:      req.Name, Permission: req.Permission, Type: req.Type, Sort: req.Sort,
		ParentID: req.ParentID.Int64(), Path: req.Path, Icon: req.Icon,
		Component: req.Component, ComponentName: req.ComponentName, Status: req.Status,
		Visible: derefBool(req.Visible, true), KeepAlive: derefBool(req.KeepAlive, false),
		AlwaysShow: derefBool(req.AlwaysShow, true),
	}
	if err := dao.CreateMenu(ctx, m); err != nil {
		return err
	}
	return nil
}

// Update updates a menu: when the permission changes, rebuild the p rows of
// referencing roles accordingly (custom roles only).
func (s *Menu) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.MenuSaveReq) error {
	old, err := dao.GetMenuByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if err := validateMenu(req.Type, req.ParentID.Int64(), req.Permission); err != nil {
		return err
	}
	if req.ParentID.Int64() == old.ID {
		return errs.New(errs.CodeParam, "parent menu cannot be itself")
	}
	if req.ParentID.Int64() > 0 {
		if _, err := dao.GetMenuByID(ctx, req.ParentID.Int64()); err != nil {
			return err
		}
	}

	updates := map[string]any{
		"name": req.Name, "permission": req.Permission, "type": req.Type, "sort": req.Sort,
		"parent_id": req.ParentID.Int64(), "path": req.Path, "icon": req.Icon,
		"component": req.Component, "component_name": req.ComponentName, "status": req.Status,
		"updater": claims.Username,
	}
	// When a bool pointer is omitted, keep the original value (a map update
	// cannot distinguish a zero value, so handle it explicitly)
	if req.Visible != nil {
		updates["visible"] = *req.Visible
	}
	if req.KeepAlive != nil {
		updates["keep_alive"] = *req.KeepAlive
	}
	if req.AlwaysShow != nil {
		updates["always_show"] = *req.AlwaysShow
	}
	if err := dao.UpdateMenu(ctx, old.ID, updates); err != nil {
		return err
	}

	if old.Permission != req.Permission {
		return s.rebuildReferencingRoles(ctx, old.ID)
	}
	return nil
}

// Delete deletes a menu: menus with children or referenced by roles cannot be deleted.
func (s *Menu) Delete(ctx context.Context, id types.ID) error {
	menuID := id.Int64()
	if _, err := dao.GetMenuByID(ctx, menuID); err != nil {
		return err
	}
	has, err := dao.MenuHasChildren(ctx, menuID)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "child menus exist, cannot delete")
	}
	has, err = dao.MenuReferencedByRoles(ctx, menuID)
	if err != nil {
		return err
	}
	if has {
		return errs.New(errs.CodeParam, "menu is referenced by roles; unassign it first")
	}
	return dao.DeleteMenu(ctx, menuID)
}

// rebuildReferencingRoles after a permission change: rebuild the p rows of the
// roles in this tenant that reference the menu.
func (s *Menu) rebuildReferencingRoles(ctx context.Context, menuID int64) error {
	roleIDs, err := dao.RoleIDsByMenuIDs(ctx, []int64{menuID})
	if err != nil {
		return err
	}
	roles, err := dao.RolesByIDs(ctx, roleIDs)
	if err != nil {
		return err
	}
	tid, ok := tenantOfCtx(ctx)
	if !ok {
		return errs.New(errs.CodeInternal, "missing tenant context")
	}
	for i := range roles {
		r := &roles[i]
		if r.Type == 1 {
			continue // builtin roles keep their wildcard
		}
		menuIDs, err := dao.GetRoleMenuIDs(ctx, r.ID)
		if err != nil {
			return err
		}
		menus, err := dao.MenusByIDs(ctx, menuIDs)
		if err != nil {
			return err
		}
		perms := make([]string, 0, len(menus))
		for _, m := range menus {
			if m.Permission != "" {
				perms = append(perms, m.Permission)
			}
		}
		if err := authz.RebuildRolePolicies(r.Code, tid, perms); err != nil {
			return errs.Wrap(err, errs.CodeInternal, "failed to rebuild role permission policies")
		}
	}
	return nil
}

// tenantOfCtx reads the tenant from the ctx (a convenience wrapper at the service layer).
func tenantOfCtx(ctx context.Context) (int64, bool) {
	tc, ok := infratenant.From(ctx)
	return tc.ID, ok
}

// validateMenu menu constraints: a valid type; buttons must carry a permission
// identifier; directories must not (leave it empty).
func validateMenu(typ int8, parentID int64, permission string) error {
	switch typ {
	case 1: // directory
		if permission != "" {
			return errs.New(errs.CodeParam, "directories do not carry permission identifiers")
		}
	case 2: // menu
		if parentID == 0 {
			return errs.New(errs.CodeParam, "a menu must be placed under a directory")
		}
	case 3: // button
		if permission == "" {
			return errs.New(errs.CodeParam, "buttons must have a permission identifier configured")
		}
		if parentID == 0 {
			return errs.New(errs.CodeParam, "a button must be placed under a menu")
		}
	default:
		return errs.New(errs.CodeParam, "invalid menu type (1 directory 2 menu 3 button)")
	}
	return nil
}

// buildMenuTree converts a list into a tree (a parent outside the set, or 0, is
// a root; sort ordering is already guaranteed by the query).
func buildMenuTree(menus []model.Menu) []types.MenuTreeResp {
	byID := make(map[int64]*types.MenuTreeResp, len(menus))
	roots := make([]*types.MenuTreeResp, 0)
	for i := range menus {
		node := &types.MenuTreeResp{MenuDTO: toMenuDTO(&menus[i])}
		byID[menus[i].ID] = node
	}
	for i := range menus {
		m := &menus[i]
		node := byID[m.ID]
		if parent, ok := byID[m.ParentID]; ok && m.ParentID != m.ID {
			parent.Children = append(parent.Children, *node)
		} else {
			roots = append(roots, node)
		}
	}
	out := make([]types.MenuTreeResp, 0, len(roots))
	for _, n := range roots {
		out = append(out, *n)
	}
	return out
}

// toMenuDTO maps model.Menu → DTO.
func toMenuDTO(m *model.Menu) types.MenuDTO {
	return types.MenuDTO{
		ID: types.ID(m.ID), Name: m.Name, Permission: m.Permission, Type: m.Type,
		Sort: m.Sort, ParentID: types.ID(m.ParentID), Path: m.Path, Icon: m.Icon,
		Component: m.Component, ComponentName: m.ComponentName, Status: m.Status,
		Visible: m.Visible, KeepAlive: m.KeepAlive, AlwaysShow: m.AlwaysShow,
		CreateTime: types.NewDateTime(m.CreateTime),
	}
}

// derefBool returns def when the pointer is nil.
func derefBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}
