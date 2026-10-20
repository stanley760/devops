package command

import (
	"context"

	"github.com/stanley760/devops/devops-infra/datascope"

	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// RoleCommand is the write-side role command handler.
type RoleCommand struct {
	roles iam.RoleRepository
	menus iam.MenuRepository
	txs   shared.TxManager
	now   shared.NowFunc
}

// NewRoleCommand constructs the role command handler.
func NewRoleCommand(roles iam.RoleRepository, menus iam.MenuRepository, txs shared.TxManager) *RoleCommand {
	return &RoleCommand{roles: roles, menus: menus, txs: txs, now: shared.SystemNow}
}

// CreateRoleCmd is the create-role command.
type CreateRoleCmd struct {
	TenantID         int64
	Name             string
	Code             string
	Sort             int
	DataScope        int8
	DataScopeDeptIDs string
	Status           int8
	Remark           string
}

// CreateRole creates a role.
//
// Rules: the code must be non-empty, unique within the tenant, and a valid
// data-scope value (all validated inside the aggregate).
func (c *RoleCommand) Create(ctx context.Context, cmd CreateRoleCmd) error {
	exists, err := c.roles.CodeExists(ctx, cmd.Code, 0)
	if err != nil {
		return err
	}
	if exists {
		return errParam("role code already exists")
	}

	scope := cmd.DataScope
	if scope == 0 {
		scope = iam.DataScopeDefaultDef
	}
	deptIDs := datascope.NormalizeDeptIDs(cmd.DataScopeDeptIDs)

	r := &iam.Role{
		TenantID:         cmd.TenantID,
		Name:             cmd.Name,
		Code:             cmd.Code,
		Sort:             cmd.Sort,
		DataScope:        scope,
		DataScopeDeptIDs: deptIDs,
		Status:           cmd.Status,
		Type:             iam.RoleTypeCustom,
		Remark:           cmd.Remark,
	}
	if err := r.ValidateForCreate(); err != nil {
		return err
	}

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.roles.Save(ctx, r)
	})
}

// UpdateRoleCmd is the update-role command.
//
// Note: no Code field — the code is immutable after creation, preventing
// casbin p/g rows from losing their anchor. Before the refactor,
// service/role.go:126 implicitly ignored it by omitting the code key from
// the updates map; that implicit convention was easy to break in later
// iterations, so it is now eliminated at the type level.
type UpdateRoleCmd struct {
	TenantID         int64
	RoleID           int64
	Name             string
	Sort             int
	DataScope        int8
	DataScopeDeptIDs string
	Status           int8
	Remark           string
}

// UpdateRole updates a role.
func (c *RoleCommand) Update(ctx context.Context, cmd UpdateRoleCmd) error {
	r, err := c.roles.Get(ctx, cmd.RoleID)
	if err != nil {
		return err
	}
	up := r.ResolveUpdate(iam.RoleUpdate{
		Name:             cmd.Name,
		Sort:             cmd.Sort,
		DataScope:        cmd.DataScope,
		DataScopeDeptIDs: cmd.DataScopeDeptIDs,
		Status:           cmd.Status,
		Remark:           cmd.Remark,
	}, datascope.NormalizeDeptIDs)
	if err := iam.ValidateDataScope(up.DataScope, up.DataScopeDeptIDs); err != nil {
		return err
	}
	r.Change(up, c.now())

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.roles.Save(ctx, r)
	})
}

// AssignRoleMenusCmd is the assign-menus command.
type AssignRoleMenusCmd struct {
	TenantID int64
	RoleID   int64
	MenuIDs  []int64
}

// AssignMenus assigns menus (rebuilds role_menu and derives the casbin p rows).
//
// Built-in roles derive no p rows — that decision lives inside the aggregate
// (Role.AssignMenus), ensuring the rule "the super admin is never locked
// down by menu assignment" cannot be bypassed at the application layer.
func (c *RoleCommand) AssignMenus(ctx context.Context, cmd AssignRoleMenusCmd) error {
	r, err := c.roles.Get(ctx, cmd.RoleID)
	if err != nil {
		return err
	}

	// Fetch permission identifiers instead of loading the full menu aggregates: a frequent operation on the write path.
	perms, err := c.roles.PermissionsByMenuIDs(ctx, cmd.MenuIDs)
	if err != nil {
		return err
	}

	now := c.now()
	r.AssignMenus(cmd.MenuIDs, perms, now)

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.roles.Save(ctx, r)
	})
}

// DeleteRoleCmd is the delete-role command.
type DeleteRoleCmd struct {
	TenantID int64
	RoleID   int64
}

// DeleteRole soft-deletes a role (built-in roles are rejected).
//
// The role row, user_role, and role_menu rows are cleaned up in the same
// transaction; casbin p- and g-row cleanup is done by the projector for the
// RoleDeleted event.
func (c *RoleCommand) Delete(ctx context.Context, cmd DeleteRoleCmd) error {
	r, err := c.roles.Get(ctx, cmd.RoleID)
	if err != nil {
		return err
	}
	if err := r.EnsureDeletable(); err != nil {
		return err
	}
	r.MarkDeleted(c.now())

	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.roles.Delete(ctx, r, r.Code)
	})
}

// OrgCommand is the write-side command handler for departments and posts.
type OrgCommand struct {
	depts iam.DeptRepository
	posts iam.PostRepository
	menus iam.MenuRepository
	txs   shared.TxManager
	now   shared.NowFunc
}

// NewOrgCommand constructs the org-structure command handler.
func NewOrgCommand(depts iam.DeptRepository, posts iam.PostRepository, menus iam.MenuRepository, txs shared.TxManager) *OrgCommand {
	return &OrgCommand{depts: depts, posts: posts, menus: menus, txs: txs, now: shared.SystemNow}
}

// SaveDeptCmd is the save-department command (create and update are unified: the code is immutable, so they can share one path).
type SaveDeptCmd struct {
	TenantID     int64
	DeptID       int64 // 0 means create
	ParentID     int64
	Name         string
	Sort         int
	LeaderUserID int64
	Phone        string
	Email        string
	Status       int8
}

// SaveDept creates or updates a department.
func (c *OrgCommand) SaveDept(ctx context.Context, cmd SaveDeptCmd) error {
	if cmd.DeptID == 0 {
		d := &iam.Dept{
			TenantID:     cmd.TenantID,
			ParentID:     cmd.ParentID,
			Name:         cmd.Name,
			Sort:         cmd.Sort,
			LeaderUserID: cmd.LeaderUserID,
			Phone:        cmd.Phone,
			Email:        cmd.Email,
			Status:       cmd.Status,
		}
		if err := d.ValidateForCreate(); err != nil {
			return err
		}
		return c.txs.WithinTx(ctx, func(ctx context.Context) error {
			return c.depts.Save(ctx, d)
		})
	}

	d, err := c.depts.Get(ctx, cmd.DeptID)
	if err != nil {
		return err
	}
	if err := d.EnsureNotSelf(cmd.ParentID); err != nil {
		return err
	}
	d.Change(iam.DeptUpdate{
		ParentID:     cmd.ParentID,
		Name:         cmd.Name,
		Sort:         cmd.Sort,
		LeaderUserID: cmd.LeaderUserID,
		Phone:        cmd.Phone,
		Email:        cmd.Email,
		Status:       cmd.Status,
	})
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.depts.Save(ctx, d)
	})
}

// DeleteDeptCmd is the delete-department command.
type DeleteDeptCmd struct {
	TenantID int64
	DeptID   int64
}

// DeleteDept soft-deletes a department (rejected when it has children or members).
func (c *OrgCommand) DeleteDept(ctx context.Context, cmd DeleteDeptCmd) error {
	d, err := c.depts.Get(ctx, cmd.DeptID)
	if err != nil {
		return err
	}
	hasChildren, err := c.depts.HasChildren(ctx, d.ID)
	if err != nil {
		return err
	}
	if hasChildren {
		return errParam("department has children and cannot be deleted")
	}
	hasMembers, err := c.depts.HasMembers(ctx, d.ID)
	if err != nil {
		return err
	}
	if hasMembers {
		return errParam("department has users and cannot be deleted")
	}
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.depts.Delete(ctx, d, d.Name)
	})
}

// SavePostCmd is the save-post command.
type SavePostCmd struct {
	TenantID int64
	PostID   int64 // 0 means create
	Code     string
	Name     string
	Sort     int
	Status   int8
	Remark   string
}

// SavePost creates or updates a post.
func (c *OrgCommand) SavePost(ctx context.Context, cmd SavePostCmd) error {
	if cmd.PostID == 0 {
		exists, err := c.posts.CodeExists(ctx, cmd.Code, 0)
		if err != nil {
			return err
		}
		if exists {
			return errParam("post code already exists")
		}
		p := &iam.Post{
			TenantID: cmd.TenantID,
			Code:     cmd.Code,
			Name:     cmd.Name,
			Sort:     cmd.Sort,
			Status:   cmd.Status,
			Remark:   cmd.Remark,
		}
		if err := p.ValidateForCreate(); err != nil {
			return err
		}
		return c.txs.WithinTx(ctx, func(ctx context.Context) error {
			return c.posts.Save(ctx, p)
		})
	}

	p, err := c.posts.Get(ctx, cmd.PostID)
	if err != nil {
		return err
	}
	p.Change(iam.PostUpdate{
		Code:   cmd.Code,
		Name:   cmd.Name,
		Sort:   cmd.Sort,
		Status: cmd.Status,
		Remark: cmd.Remark,
	})
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.posts.Save(ctx, p)
	})
}

// DeletePostCmd is the delete-post command.
type DeletePostCmd struct {
	TenantID int64
	PostID   int64
}

// DeletePost soft-deletes a post.
func (c *OrgCommand) DeletePost(ctx context.Context, cmd DeletePostCmd) error {
	p, err := c.posts.Get(ctx, cmd.PostID)
	if err != nil {
		return err
	}
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.posts.Delete(ctx, p, p.Code)
	})
}

// SaveMenuCmd is the save-menu command.
type SaveMenuCmd struct {
	MenuID        int64 // 0 means create
	Name          string
	Permission    string
	Type          int8
	Sort          int
	ParentID      int64
	Path          string
	Icon          string
	Component     string
	ComponentName string
	Status        int8
	Visible       bool
	KeepAlive     bool
	AlwaysShow    bool
	Remark        string
}

// SaveMenu creates or updates a menu.
func (c *OrgCommand) SaveMenu(ctx context.Context, cmd SaveMenuCmd) error {
	if cmd.MenuID == 0 {
		m := &iam.Menu{
			Name:          cmd.Name,
			Permission:    cmd.Permission,
			Type:          cmd.Type,
			Sort:          cmd.Sort,
			ParentID:      cmd.ParentID,
			Path:          cmd.Path,
			Icon:          cmd.Icon,
			Component:     cmd.Component,
			ComponentName: cmd.ComponentName,
			Status:        cmd.Status,
			Visible:       cmd.Visible,
			KeepAlive:     cmd.KeepAlive,
			AlwaysShow:    cmd.AlwaysShow,
		}
		if err := m.ValidateForCreate(); err != nil {
			return err
		}
		return c.txs.WithinTx(ctx, func(ctx context.Context) error {
			return c.menus.Save(ctx, m)
		})
	}

	m, err := c.menus.Get(ctx, cmd.MenuID)
	if err != nil {
		return err
	}
	// Cycle prevention: a menu cannot be attached under its own subtree (this check was missing before the refactor, risking an infinite loop).
	all, err := c.menus.ListAll(ctx, iam.MenuFilter{})
	if err != nil {
		return err
	}
	if err := m.EnsureNotSelfOrDescendant(cmd.ParentID, all); err != nil {
		return err
	}
	m.Change(iam.MenuUpdate{
		Name:          cmd.Name,
		Permission:    cmd.Permission,
		Type:          cmd.Type,
		Sort:          cmd.Sort,
		ParentID:      cmd.ParentID,
		Path:          cmd.Path,
		Icon:          cmd.Icon,
		Component:     cmd.Component,
		ComponentName: cmd.ComponentName,
		Status:        cmd.Status,
		Visible:       cmd.Visible,
		KeepAlive:     cmd.KeepAlive,
		AlwaysShow:    cmd.AlwaysShow,
		Remark:        cmd.Remark,
	})
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.menus.Save(ctx, m)
	})
}

// DeleteMenuCmd is the delete-menu command.
type DeleteMenuCmd struct {
	MenuID int64
}

// DeleteMenu deletes a menu.
func (c *OrgCommand) DeleteMenu(ctx context.Context, cmd DeleteMenuCmd) error {
	return c.txs.WithinTx(ctx, func(ctx context.Context) error {
		return c.menus.Delete(ctx, cmd.MenuID)
	})
}
