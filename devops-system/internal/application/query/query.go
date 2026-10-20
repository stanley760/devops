package query

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
	"github.com/stanley760/devops/devops-infra/datascope"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/auditlog"
	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// User read side.
//
// The key difference from the write side: the read side returns DTOs
// directly without loading aggregates or validating invariants. This lets
// the read side freely JOIN and trim fields (list pages do not need role
// details), while the write side still strictly guards aggregate boundaries.
type UserQuery struct {
	users iam.UserRepository
}

// NewUserQuery constructs the user read side.
func NewUserQuery(users iam.UserRepository) *UserQuery { return &UserQuery{users: users} }

// List returns a paginated list of users.
func (q *UserQuery) List(ctx context.Context, f iam.UserFilter) (domain.PageResult[types.UserDTO], error) {
	page, err := q.users.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.UserDTO]{}, err
	}
	return domain.MapPage(page, UserDTO), nil
}

// Get returns user details.
//
// Single point of enforcement for data permissions: after the direct
// lookup by ID, visibility is decided, and unauthorized access is always
// presented as 404 (the same response as "user does not exist", preventing
// existence probing — this security semantic must not be changed to 403).
// When no data scope is injected into ctx, it fails closed with an error
// (controlled routes already have the resolver middleware mounted; a missing
// mount is exposed immediately).
func (q *UserQuery) Get(ctx context.Context, id int64) (*types.UserDetailResp, error) {
	u, err := q.users.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := EnsureVisible(ctx, u.DeptID, u.ID); err != nil {
		return nil, err
	}
	return UserDetail(u), nil
}

// GetByUsername returns details by username (used to fetch one's own info after login).
func (q *UserQuery) GetByUsername(ctx context.Context, username string) (*types.UserDetailResp, error) {
	u, err := q.users.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	return UserDetail(u), nil
}

// EnsureVisible decides data-permission visibility (single point of enforcement).
//
// Before the refactor this was the package-level private function
// ensureUserVisible in service/user.go:24, called separately by 4 methods;
// it is now an exported function used uniformly by the read side.
func EnsureVisible(ctx context.Context, deptID, ownerID int64) error {
	scope, ok := datascope.From(ctx)
	if !ok {
		return errs.New(errs.CodeInternal, "missing data-scope context (resolver middleware not mounted on a controlled endpoint)")
	}
	if !scope.Visible(deptID, ownerID) {
		return errs.New(errs.CodeNotFound, "user does not exist")
	}
	return nil
}

// RoleQuery is the role read side.
type RoleQuery struct {
	roles iam.RoleRepository
}

// NewRoleQuery constructs the role read side.
func NewRoleQuery(roles iam.RoleRepository) *RoleQuery { return &RoleQuery{roles: roles} }

// List returns a paginated list of roles.
func (q *RoleQuery) List(ctx context.Context, f iam.RoleFilter) (domain.PageResult[types.RoleDTO], error) {
	page, err := q.roles.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.RoleDTO]{}, err
	}
	return domain.MapPage(page, RoleDTO), nil
}

// Get returns role details (including assigned menu IDs).
func (q *RoleQuery) Get(ctx context.Context, id int64) (*types.RoleDetailResp, error) {
	r, err := q.roles.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return RoleDetail(r), nil
}

// MenuQuery is the menu read side.
type MenuQuery struct {
	menus iam.MenuRepository
}

// NewMenuQuery constructs the menu read side.
func NewMenuQuery(menus iam.MenuRepository) *MenuQuery { return &MenuQuery{menus: menus} }

// List returns the menu tree.
func (q *MenuQuery) List(ctx context.Context, f iam.MenuFilter) ([]types.MenuTreeResp, error) {
	all, err := q.menus.ListAll(ctx, f)
	if err != nil {
		return nil, err
	}
	return MenuTree(all), nil
}

// Tree returns the menu tree (no filtering).
func (q *MenuQuery) Tree(ctx context.Context) ([]types.MenuTreeResp, error) {
	return q.List(ctx, iam.MenuFilter{})
}

// DeptQuery is the department read side.
type DeptQuery struct {
	depts iam.DeptRepository
}

// NewDeptQuery constructs the department read side.
func NewDeptQuery(depts iam.DeptRepository) *DeptQuery { return &DeptQuery{depts: depts} }

// List returns the department tree.
func (q *DeptQuery) List(ctx context.Context, f iam.DeptFilter) ([]types.DeptTreeResp, error) {
	all, err := q.depts.List(ctx, f)
	if err != nil {
		return nil, err
	}
	return DeptTree(all), nil
}

// PostQuery is the post read side.
type PostQuery struct {
	posts iam.PostRepository
}

// NewPostQuery constructs the post read side.
func NewPostQuery(posts iam.PostRepository) *PostQuery { return &PostQuery{posts: posts} }

// List returns a paginated list of posts.
func (q *PostQuery) List(ctx context.Context, f iam.PostFilter) (domain.PageResult[types.PostDTO], error) {
	page, err := q.posts.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.PostDTO]{}, err
	}
	return domain.MapPage(page, PostDTO), nil
}

// Get returns post details.
func (q *PostQuery) Get(ctx context.Context, id int64) (*types.PostDTO, error) {
	p, err := q.posts.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := PostDTO(p)
	return &dto, nil
}

// TenantQuery is the tenant read side.
type TenantQuery struct {
	tenants iam.TenantRepository
}

// NewTenantQuery constructs the tenant read side.
func NewTenantQuery(tenants iam.TenantRepository) *TenantQuery { return &TenantQuery{tenants: tenants} }

// List returns a paginated list of tenants.
func (q *TenantQuery) List(ctx context.Context, f iam.TenantFilter) (domain.PageResult[types.TenantDTO], error) {
	page, err := q.tenants.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.TenantDTO]{}, err
	}
	return domain.MapPage(page, TenantDTO), nil
}

// Get returns tenant details.
func (q *TenantQuery) Get(ctx context.Context, id int64) (*types.TenantDTO, error) {
	t, err := q.tenants.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := TenantDTO(t)
	return &dto, nil
}

// LoginLogQuery is the login log read side.
type LoginLogQuery struct {
	logs auditlog.LoginLogReader
}

// NewLoginLogQuery constructs the login log read side.
func NewLoginLogQuery(logs auditlog.LoginLogReader) *LoginLogQuery { return &LoginLogQuery{logs: logs} }

// List returns a paginated list of login audit records.
func (q *LoginLogQuery) List(ctx context.Context, f auditlog.LoginLogFilter) (domain.PageResult[types.LoginLogDTO], error) {
	page, err := q.logs.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.LoginLogDTO]{}, err
	}
	return domain.MapPage(page, LoginLogDTO), nil
}

// OperationLogQuery is the operation log read side.
type OperationLogQuery struct {
	logs auditlog.OperationLogReader
}

// NewOperationLogQuery constructs the operation log read side.
func NewOperationLogQuery(logs auditlog.OperationLogReader) *OperationLogQuery {
	return &OperationLogQuery{logs: logs}
}

// List returns a paginated list of operation audit records.
func (q *OperationLogQuery) List(ctx context.Context, f auditlog.OperationLogFilter) (domain.PageResult[types.OperationLogDTO], error) {
	page, err := q.logs.List(ctx, f)
	if err != nil {
		return domain.PageResult[types.OperationLogDTO]{}, err
	}
	return domain.MapPage(page, OperationLogDTO), nil
}
