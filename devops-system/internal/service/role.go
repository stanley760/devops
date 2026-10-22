// Role management service: CRUD + rebuilding casbin p rows per domain.
//
// Builtin roles (type=1, e.g. super_admin) skip p row rebuilding: their
// wildcard p row (*, {tid}, *) is seed-granted domain-wide allow-all semantics
// that must not change with menu assignment, so the super admin is never
// locked out.
package service

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-infra/datascope"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// validateDataScope validates the data permission scope: a custom department
// scope must carry a parseable non-empty department set (a JSON array string,
// same storage format as post_ids).
func validateDataScope(dataScope int8, deptIDs string) error {
	if dataScope != datascope.ScopeCustom {
		return nil
	}
	if len(datascope.ParseDeptIDs(deptIDs)) == 0 {
		return errs.New(errs.CodeParam, "the custom department scope must provide a department ID set (data_scope_dept_ids, JSON array)")
	}
	return nil
}

// Role is the role management service.
type Role struct {
	invalidateScope func() // data permission cache invalidation (called when the role scope/bindings change)
}

// NewRole creates the service (invalidateScope clears the data permission cache).
func NewRole(invalidateScope func()) *Role { return &Role{invalidateScope: invalidateScope} }

// List returns a paginated query.
func (s *Role) List(ctx context.Context, q types.RolePageQuery) (domain.PageResult[types.RoleDTO], error) {
	page, err := dao.ListRoles(ctx, q)
	if err != nil {
		return domain.PageResult[types.RoleDTO]{}, err
	}
	return domain.MapPage(page, func(role model.Role) types.RoleDTO {
		return toRoleDTO(&role)
	}), nil
}

// Get returns the role details (including assigned menu IDs).
func (s *Role) Get(ctx context.Context, id types.ID) (*types.RoleDetailResp, error) {
	r, err := dao.GetRoleByID(ctx, id.Int64())
	if err != nil {
		return nil, err
	}
	menuIDs, err := dao.GetRoleMenuIDs(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	mids := make([]types.ID, 0, len(menuIDs))
	for _, mid := range menuIDs {
		mids = append(mids, types.ID(mid))
	}
	return &types.RoleDetailResp{RoleDTO: toRoleDTO(r), MenuIDs: mids}, nil
}

// Create creates a role: the code is unique within a tenant → issue an ID and
// persist (optional initial menus).
func (s *Role) Create(ctx context.Context, claims *auth.AccessClaims, req types.RoleSaveReq) error {
	if req.Code == "" {
		return errs.New(errs.CodeParam, "role code cannot be empty")
	}
	exists, err := dao.RoleCodeExists(ctx, req.Code, 0)
	if err != nil {
		return err
	}
	if exists {
		return errs.New(errs.CodeParam, "role code already exists")
	}
	if err := validateDataScope(req.DataScope, req.DataScopeDeptIDs); err != nil {
		return err
	}

	r := &model.Role{
		BaseTenantModel: model.BaseTenantModel{
			BaseModel: model.BaseModel{Creator: claims.Username, Updater: claims.Username},
		},
		Name:             req.Name,
		Code:             req.Code,
		Sort:             req.Sort,
		DataScope:        req.DataScope,
		DataScopeDeptIDs: datascope.NormalizeDeptIDs(req.DataScopeDeptIDs),
		Status:           req.Status,
		Type:             2, // custom
		Remark:           req.Remark,
	}
	if r.DataScope == 0 {
		r.DataScope = 1
	}

	return dao.CreateRole(ctx, r)
}

// Update updates a role (the code cannot be changed, to avoid losing track of casbin p/g rows).
func (s *Role) Update(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.RoleSaveReq) error {
	r, err := dao.GetRoleByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	dataScope := req.DataScope
	if dataScope == 0 {
		dataScope = r.DataScope
	}
	// When the scope is unchanged and no department set is supplied, keep the
	// existing value (data_scope_dept_ids is overwritten wholesale, so an empty
	// input clears it). NormalizeDeptIDs unifies the numeric-array form before
	// persistence, preventing multiple spellings of the same semantics in storage.
	deptIDs := req.DataScopeDeptIDs
	if deptIDs == "" && dataScope == r.DataScope {
		deptIDs = r.DataScopeDeptIDs
	}
	deptIDs = datascope.NormalizeDeptIDs(deptIDs)
	if err := validateDataScope(dataScope, deptIDs); err != nil {
		return err
	}
	updates := map[string]any{
		"name":                req.Name,
		"sort":                req.Sort,
		"data_scope":          dataScope,
		"data_scope_dept_ids": deptIDs,
		"status":              req.Status,
		"remark":              req.Remark,
		"updater":             claims.Username,
	}
	if err := dao.UpdateRole(ctx, r.ID, updates); err != nil {
		return err
	}
	// The data permission scope has changed: clear the cache (all users under
	// the role re-resolve on their next request)
	s.invalidateScope()
	return nil
}

// AssignMenus assigns menus: rebuild role_menu + rebuild the p rows from a
// (role, tenant) snapshot (effective immediately).
func (s *Role) AssignMenus(ctx context.Context, claims *auth.AccessClaims, id types.ID, req types.RoleAssignMenusReq) error {
	r, err := dao.GetRoleByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	menuIDs := idsToInt64(req.MenuIDs)
	menus, err := dao.MenusByIDs(ctx, menuIDs)
	if err != nil {
		return err
	}

	if err := dao.ReplaceRoleMenus(ctx, r.ID, menuIDs); err != nil {
		return err
	}

	if r.Type == 1 {
		return nil // builtin roles keep their domain-wide wildcard and derive no p rows
	}
	perms := make([]string, 0, len(menus))
	for _, m := range menus {
		if m.Permission != "" {
			perms = append(perms, m.Permission)
		}
	}
	if err := authz.RebuildRolePolicies(r.Code, claims.TenantID, perms); err != nil {
		return errs.Wrap(err, errs.CodeInternal, "menus saved, but the permission policy refresh failed; please retry")
	}
	return nil
}

// Delete deletes a role: builtin roles cannot be deleted → clear associations
// (user_role/role_menu) → clear p row and g row references.
func (s *Role) Delete(ctx context.Context, claims *auth.AccessClaims, id types.ID) error {
	r, err := dao.GetRoleByID(ctx, id.Int64())
	if err != nil {
		return err
	}
	if r.Type == 1 {
		return errs.New(errs.CodeParam, "builtin roles cannot be deleted")
	}

	if err := dao.DeleteRole(ctx, r); err != nil {
		return err
	}

	if err := authz.RemoveRolePolicies(r.Code, claims.TenantID); err != nil {
		return errs.Wrap(err, errs.CodeInternal, "role deleted, but the permission policy cleanup failed")
	}
	if err := authz.RemoveRoleGrouping(r.Code, claims.TenantID); err != nil {
		return errs.Wrap(err, errs.CodeInternal, "role deleted, but the user binding cleanup failed")
	}
	// Deleting the role affects the data permissions of its users: clear the cache for re-resolution
	s.invalidateScope()
	return nil
}

// toRoleDTO maps model.Role → DTO.
func toRoleDTO(r *model.Role) types.RoleDTO {
	return types.RoleDTO{
		ID:               types.ID(r.ID),
		Name:             r.Name,
		Code:             r.Code,
		Sort:             r.Sort,
		DataScope:        r.DataScope,
		DataScopeDeptIDs: r.DataScopeDeptIDs,
		Status:           r.Status,
		Type:             r.Type,
		Remark:           r.Remark,
		CreateTime:       types.NewDateTime(r.CreateTime),
	}
}
