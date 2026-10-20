// Positioning and boundaries (division of labor with the existing two layers):
//   - Casbin governs "may this interface be called" (functional permission); TenantPlugin governs
//     "inside or outside the tenant" (tenant isolation); this package governs "which rows can be
//     seen/operated" within a tenant (data permission). The three layers stack without conflict.
//   - This package is pure computation with no DB access: roles/users/departments are queried and
//     injected by the caller (the middleware resolver layer); the dao layer consumes them via
//     Apply/Visible, avoiding a dao ↔ resolver circular dependency.
//
// Semantics (aligned with yudao; t_system_role.data_scope already reserved):
//
//	1 All  2 Custom departments  3 Own department  4 Own department and below  5 Self only
//
// Multi-role merge rule: the single most permissive role wins (aligned with yudao priority semantics — deterministic and explainable);
// multiple roles that are both "custom departments" take their union. Permissiveness order: All > Own and below > Own department > Custom > Self only.
//
// Injection strategy — explicit beats implicit: tenant_id exists on every tenant table with uniform semantics, fitting TenantPlugin's
// global automatic injection; the carrier column for data permission (dept_id) and the controlled query set are business semantics, so only two
// entry points, Apply/Visible, are provided, called explicitly by dao/service; a missing wiring shows up in review and tests.
package datascope

import (
	"context"
	"encoding/json"
	"github.com/stanley760/devops/devops-infra/errs"
	"gorm.io/gorm"
	"slices"
	"sort"
	"strings"
)

// Data permission scope constants (values of t_system_role.data_scope).
const (
	ScopeAll       int8 = 1 // all data
	ScopeCustom    int8 = 2 // custom department set (data_scope_dept_ids, JSON array string)
	ScopeDept      int8 = 3 // own department only
	ScopeDeptBelow int8 = 4 // own department and below
	ScopeSelf      int8 = 5 // self only
)

// Scope is the resolved row-level visibility range (a value object, reused within a request after ctx injection).
// All bypasses filtering; SelfOnly matches the Owner column; DeptIDs matches the Dept column; the three are mutually exclusive (guaranteed by the merge rule).
type Scope struct {
	All      bool
	SelfOnly bool
	SelfID   int64   // primary key of the matching row when SelfOnly
	DeptIDs  []int64 // allowed department set; empty when All/SelfOnly
}

// DeptNode is the minimal view of the department tree (converted from model.Dept by the resolver layer; this package does not depend on model).
type DeptNode struct {
	ID       int64
	ParentID int64
}

// RoleScope is a role's data permission declaration (converted from model.Role by the resolver layer).
type RoleScope struct {
	Code             string
	DataScope        int8
	DataScopeDeptIDs string
}

// UserRef is the minimal user view needed to resolve data permission (converted from model.User by the resolver layer; this package does not depend on model).
type UserRef struct {
	ID     int64
	DeptID int64
}

// ParseDeptIDs parses data_scope_dept_ids (a JSON array string such as "[1,2,3]",
// consistent with the post_ids storage format). Invalid/empty input returns an empty set without error — validation belongs to the Role save path.
func ParseDeptIDs(s string) []int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// NormalizeDeptIDs normalizes the stored form of a department ID set: after parsing, re-marshal
// as a numeric array (empty set returns "", and the caller writes NULL). Client input may carry
// empty strings/whitespace; passing them through would let one semantics appear in multiple
// forms in the database. Unify before writing; the read side via ParseDeptIDs is unaffected.
func NormalizeDeptIDs(s string) string {
	ids := ParseDeptIDs(s)
	if len(ids) == 0 {
		return ""
	}
	b, err := json.Marshal(ids)
	if err != nil {
		return ""
	}
	return string(b)
}

// ChildrenMap maps a department list to an adjacency table from parent ID to child ID set (for subtree expansion).
func ChildrenMap(depts []DeptNode) map[int64][]int64 {
	children := make(map[int64][]int64, len(depts))
	for _, d := range depts {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	return children
}

// ExpandSubtree BFS-expands subtrees from a root set (including the roots themselves).
func ExpandSubtree(children map[int64][]int64, roots []int64) []int64 {
	if len(roots) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(children))
	queue := append([]int64(nil), roots...)
	for _, r := range roots {
		seen[r] = struct{}{}
	}
	for i := 0; i < len(queue); i++ {
		for _, child := range children[queue[i]] {
			if _, ok := seen[child]; !ok {
				seen[child] = struct{}{}
				queue = append(queue, child)
			}
		}
	}
	out := make([]int64, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Merge merges multiple roles (the single most permissive role wins; custom roles of equal permissiveness take their union).
// ownDeptID is the current user's department (0 = no department: the own-department/below semantics degrade to seeing no departments),
// selfID is the current user's primary key (the matching row under the self-only semantics), and children is the tenant department tree adjacency table.
func Merge(roles []RoleScope, ownDeptID, selfID int64, children map[int64][]int64) Scope {
	bestRank := -1 // greatest permissiveness seen so far; -1 = no roles yet
	var best int8
	var customUnion []int64
	customSeen := make(map[int64]struct{})

	for _, r := range roles {
		switch r.DataScope {
		case ScopeAll:
			return Scope{All: true}
		case ScopeDeptBelow:
			if bestRank < 3 {
				best, bestRank = ScopeDeptBelow, 3
			}
		case ScopeDept:
			if bestRank < 2 {
				best, bestRank = ScopeDept, 2
			}
		case ScopeCustom:
			for _, id := range ParseDeptIDs(r.DataScopeDeptIDs) {
				if _, ok := customSeen[id]; !ok {
					customSeen[id] = struct{}{}
					customUnion = append(customUnion, id)
				}
			}
			if bestRank < 1 {
				best, bestRank = ScopeCustom, 1
			}
		case ScopeSelf:
			if bestRank < 0 {
				best, bestRank = ScopeSelf, 0
			}
		}
	}

	switch best {
	case ScopeDeptBelow:
		if ownDeptID <= 0 {
			return Scope{} // no department: no rows visible (fail-closed, with no self fallback)
		}
		return Scope{DeptIDs: ExpandSubtree(children, []int64{ownDeptID})}
	case ScopeDept:
		if ownDeptID <= 0 {
			return Scope{}
		}
		return Scope{DeptIDs: []int64{ownDeptID}}
	case ScopeCustom:
		if len(customUnion) == 0 {
			return Scope{}
		}
		sort.Slice(customUnion, func(i, j int) bool { return customUnion[i] < customUnion[j] })
		return Scope{DeptIDs: customUnion}
	default: // no roles, or all self-only
		return Scope{SelfOnly: true, SelfID: selfID}
	}
}

// ---- ctx injection/retrieval ----

// CtxKey is the key for data permission in ctx (exported: the huma resolver middleware injects via huma.WithValue
// and must use the same key instance as From/WithScope; the type itself is exported because it crosses packages).
type CtxKey struct{}

// WithScope injects the resolution result into ctx (called by the resolver middleware; read by dao/service).
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, CtxKey{}, s)
}

// From retrieves the data permission from ctx (ok=false when not injected; callers fail closed).
func From(ctx context.Context) (Scope, bool) {
	s, ok := ctx.Value(CtxKey{}).(Scope)
	return s, ok
}

// Columns are the carrier columns for controlled queries (dept_id and the row primary key differ under different table/JOIN aliases, so declare them explicitly).
type Columns struct {
	Dept  string // department column (e.g. "dept_id", or "u.dept_id" under a JOIN)
	Owner string // row primary key column (used by the self-only semantics, e.g. "id")
}

// Apply injects the data permission from ctx into a query (the entry point for list-style queries).
//
// fail-closed: when ctx carries no data permission, attach an error to the query directly (aligned with TenantPlugin's
// fail-fast semantics for a missing tenant), preventing a controlled query from degrading to fully visible when the resolver middleware is not attached.
func Apply(ctx context.Context, db *gorm.DB, cols Columns) *gorm.DB {
	s, ok := From(ctx)
	if !ok {
		_ = db.AddError(errs.New(errs.CodeInternal, "query lacks data scope context (controlled endpoint has no resolver middleware attached)"))
		return db
	}
	switch {
	case s.All:
		return db
	case s.SelfOnly:
		return db.Where(cols.Owner+" = ?", s.SelfID)
	case len(s.DeptIDs) > 0:
		return db.Where(cols.Dept+" IN ?", s.DeptIDs)
	default:
		return db.Where("1 = 0") // empty set: no rows visible
	}
}

// Visible decides single-point visibility (the entry point for direct read/update/delete by ID; anything out of scope is invisible).
// deptID is the target row's department and rowID is the target row's primary key; the caller decides after fetching the row,
// treating invisibility as 404 (avoiding existence probing, presented the same as "resource not found").
func (s Scope) Visible(deptID, rowID int64) bool {
	switch {
	case s.All:
		return true
	case s.SelfOnly:
		return rowID == s.SelfID
	default:
		return slices.Contains(s.DeptIDs, deptID)
	}
}
