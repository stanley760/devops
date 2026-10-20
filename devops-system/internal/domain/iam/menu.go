package iam

import (
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Menu is the menu aggregate root (global table, no tenant_id).
//
// Note: t_system_menu carries no tenant_id; it is a global table and the tenant plugin does
// not inject for it. Menu tree construction (parent-child assembly) is in buildTree — before
// the refactor there were 3 nearly verbatim copies of the tree-assembly code
// (service/dept.go:143, service/menu.go:204, service/auth.go:267), now unified into the
// generic buildTree.
type Menu struct {
	shared.AggregateRoot

	ID            int64
	ParentID      int64
	Name          string
	Type          int8
	Path          string
	Component     string
	ComponentName string
	// Permission is the permission identifier, e.g. system:user:create; empty for directories and tenant management.
	// The name aligns with the column name permission (to avoid confusion with the standard
	// library Permission constants, so the abbreviation Perm is not used).
	Permission string
	Icon       string
	Sort       int
	Visible    bool
	KeepAlive  bool
	AlwaysShow bool
	Status     int8
	// Remark is not a table column: the menu table has no remark column; this field only carries input and is ignored by the
	// repository, kept to align with the existing API contract (types.MenuSaveReq includes remark).
	CreateTime time.Time
	UpdateTime time.Time

	// Children are child menus (references within the aggregate).
	Children []*Menu
}

// MenuType is the menu type.
const (
	MenuTypeDir    int8 = 1 // directory
	MenuTypeMenu   int8 = 2 // menu
	MenuTypeButton int8 = 3 // button
)

// MenuUpdate is the menu change input (domain input).
type MenuUpdate struct {
	Name          string
	Type          int8
	ParentID      int64
	Path          string
	Component     string
	ComponentName string
	Permission    string
	Icon          string
	Sort          int
	Visible       bool
	KeepAlive     bool
	AlwaysShow    bool
	Status        int8
	Remark        string
}

// ValidateForCreate validates before creation: name must be non-empty, parent must not be self-referencing.
func (m *Menu) ValidateForCreate() error {
	if strings.TrimSpace(m.Name) == "" {
		return errs.New(errs.CodeParam, "menu name cannot be empty")
	}
	if m.ParentID == m.ID {
		return errs.New(errs.CodeParam, "parent menu cannot be itself")
	}
	return nil
}

// Change applies the change (menu type changes excluded, as a type change affects child node semantics).
func (m *Menu) Change(up MenuUpdate) {
	m.Name = up.Name
	m.Type = up.Type
	m.ParentID = up.ParentID
	m.Path = up.Path
	m.Component = up.Component
	m.ComponentName = up.ComponentName
	m.Permission = up.Permission
	m.Icon = up.Icon
	m.Sort = up.Sort
	m.Visible = up.Visible
	m.KeepAlive = up.KeepAlive
	m.AlwaysShow = up.AlwaysShow
	m.Status = up.Status
}

// EnsureNotSelfOrDescendant validates that a menu cannot be attached under its own subtree (cycle prevention).
//
// This rule did not exist before the refactor — the original service/menu.go only had a
// fallback when aggregate loading failed, leaving a risk of attaching a menu under its own
// descendant and causing an infinite loop in tree construction. It is now explicitly modeled
// as a domain rule.
func (m *Menu) EnsureNotSelfOrDescendant(parentID int64, all []*Menu) error {
	if parentID == 0 || parentID == m.ID {
		return nil
	}
	byID := make(map[int64]*Menu, len(all))
	for _, x := range all {
		byID[x.ID] = x
	}
	// walk up from the target parent; passing through itself on the way means a cycle.
	seen := make(map[int64]bool, len(all))
	for cur := parentID; cur != 0; {
		if cur == m.ID {
			return errs.New(errs.CodeParam, "parent menu cannot be a descendant of itself")
		}
		if seen[cur] {
			break // existing data already has a cycle; stop backtracking to avoid an infinite loop
		}
		seen[cur] = true
		parent, ok := byID[cur]
		if !ok {
			return nil
		}
		cur = parent.ParentID
	}
	return nil
}

// CollectPermissions collects the permission identifiers of itself and all descendants (used to derive p rows when assigning role menus).
func (m *Menu) CollectPermissions() []string {
	out := make([]string, 0)
	var walk func(*Menu)
	walk = func(x *Menu) {
		if p := strings.TrimSpace(x.Permission); p != "" {
			out = append(out, p)
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(m)
	return out
}

// IsButton reports whether it is the button type.
func (m *Menu) IsButton() bool { return m.Type == MenuTypeButton }
