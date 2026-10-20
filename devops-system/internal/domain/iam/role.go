package iam

import (
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Role types.
const (
	// RoleTypeBuiltin is a builtin role (e.g. super_admin): no p rows derived; the in-domain wildcard is granted by the seed.
	// Business rule: builtin roles cannot be deleted, and menu assignment does not override their wildcard permission.
	RoleTypeBuiltin int8 = 1
	// RoleTypeCustom is a custom role: menu assignment derives p rows.
	RoleTypeCustom int8 = 2
)

// Data scope semantics (consistent with the constants in devops-infra/datascope).
const (
	DataScopeAll        int8 = 1 // all data permission
	DataScopeCustom     int8 = 2 // custom (must carry a dept set)
	DataScopeDefaultDef int8 = 1 // default semantics for the zero value
)

// Role is the role aggregate root.
//
// Rules pulled into the rich domain model (previously scattered in service/role.go):
//   - code cannot be empty (role.go:72)
//   - when data_scope is custom, a non-empty dept set must be carried (role.go:82 → validateDataScope)
//   - code cannot be changed after creation, to avoid casbin p/g rows becoming orphaned (role.go:106)
//   - builtin roles cannot be deleted (role.go:180)
//   - builtin role menu assignment does not derive p rows (role.go:159)
type Role struct {
	shared.AggregateRoot

	ID       int64
	TenantID int64
	Name     string
	Code     string // unique within the tenant; the association key for casbin p/g rows, immutable after creation
	Sort     int
	// DataScope is the data scope semantics.
	DataScope int8
	// DataScopeDeptIDs is the dept set for the custom semantics (numeric array JSON string).
	DataScopeDeptIDs string
	Status           int8
	Type             int8
	Remark           string
	CreateTime       time.Time
	UpdateTime       time.Time

	// MenuIDs are the assigned menus (references within the aggregate).
	MenuIDs []int64
}

// IsBuiltin reports whether it is a builtin role.
func (r *Role) IsBuiltin() bool { return r.Type == RoleTypeBuiltin }

// ValidateForCreate validates before creation: code non-empty + valid data scope semantics.
func (r *Role) ValidateForCreate() error {
	if strings.TrimSpace(r.Code) == "" {
		return errs.New(errs.CodeParam, "role code cannot be empty")
	}
	if err := ValidateDataScope(r.DataScope, r.DataScopeDeptIDs); err != nil {
		return err
	}
	return nil
}

// ValidateDataScope validates the data scope semantics (domain rule).
//
// The custom semantics must carry a parseable non-empty dept set. Before the refactor this
// was the package-level private function validateDataScope at service/role.go:24; it is
// promoted to a domain rule here for easy independent unit testing.
func ValidateDataScope(dataScope int8, deptIDs string) error {
	if dataScope != DataScopeCustom {
		return nil
	}
	if len(ParseDeptIDs(deptIDs)) == 0 {
		return errs.New(errs.CodeParam, "the custom dept scope must provide a dept ID set (data_scope_dept_ids, JSON array)")
	}
	return nil
}

// RoleUpdate is the role change input (domain input).
//
// Code is not in the input: the code is immutable after creation, to avoid casbin policies becoming orphaned.
// Before the refactor, the updates map at service/role.go:126-134 implicitly ignored it by "not writing the code key";
// implicit conventions are easy to break, so the risk is eliminated here at the type level.
type RoleUpdate struct {
	Name             string
	Sort             int
	DataScope        int8
	DataScopeDeptIDs string
	Status           int8
	Remark           string
}

// Change applies the change.
func (r *Role) Change(up RoleUpdate, at time.Time) {
	r.Name = up.Name
	r.Sort = up.Sort
	r.DataScope = up.DataScope
	r.DataScopeDeptIDs = up.DataScopeDeptIDs
	r.Status = up.Status
	r.Remark = up.Remark
	r.UpdateTime = at
}

// ResolveUpdate merges the update input with existing values and returns the final effective field set.
//
// Rule: when data_scope is passed as 0, keep the existing value; when the semantics are unchanged
// and no dept set is passed, keep the existing dept set (this semantics comes from
// service/role.go:112-122; it is a business rule rather than a storage detail, so it lives in the domain layer).
func (r *Role) ResolveUpdate(up RoleUpdate, normalizeDeptIDs func(string) string) RoleUpdate {
	if up.DataScope == 0 {
		up.DataScope = r.DataScope
	}
	if up.DataScopeDeptIDs == "" && up.DataScope == r.DataScope {
		up.DataScopeDeptIDs = r.DataScopeDeptIDs
	}
	up.DataScopeDeptIDs = normalizeDeptIDs(up.DataScopeDeptIDs)
	return up
}

// AssignMenus assigns menus, triggering a p row rebuild event.
//
// Builtin roles do not derive p rows: their wildcard permission (*, {tid}, *) is granted by
// the seed and does not change with menus, otherwise the super admin would be locked out.
// This judgment was previously at service/role.go:159; it is now a domain rule.
func (r *Role) AssignMenus(menuIDs []int64, permissions []string, at time.Time) {
	r.MenuIDs = menuIDs
	if r.IsBuiltin() {
		return // builtin roles keep the in-domain wildcard
	}
	r.RecordEvent(RoleMenusAssigned{
		BaseEvent:   shared.BaseEvent{Time: at},
		RoleID:      r.ID,
		RoleCode:    r.Code,
		TenantID:    r.TenantID,
		Permissions: permissions,
	})
}

// EnsureDeletable is the pre-delete validation: builtin roles cannot be deleted.
func (r *Role) EnsureDeletable() error {
	if r.IsBuiltin() {
		return errs.New(errs.CodeParam, "builtin roles cannot be deleted")
	}
	return nil
}

// MarkDeleted soft-deletes and records a cleanup event.
func (r *Role) MarkDeleted(at time.Time) {
	r.RecordEvent(RoleDeleted{
		BaseEvent: shared.BaseEvent{Time: at},
		RoleID:    r.ID,
		RoleCode:  r.Code,
		TenantID:  r.TenantID,
	})
}

// ParseDeptIDs parses the dept ID set (numeric array JSON).
//
// Tolerance semantics consistent with the existing implementation: invalid JSON returns an empty set, with no compatibility read.
func ParseDeptIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return decodeInt64JSONArray(raw)
}
