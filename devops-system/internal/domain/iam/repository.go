package iam

import (
	"context"

	"github.com/stanley760/devops/devops-common/domain"
)

// Repository ports (Repository Pattern).
//
// Key design: all inputs use pure Go Filter types defined in this package, carrying no huma/GORM tags.
//
// The reverse dependency before the refactor: 6 dao files imported internal/types, with method
// signatures like ListUsers(ctx, types.UserPageQuery), causing the storage layer to be polluted
// by huma/OpenAPI form/binding/doc tags, and the repository could not evolve independently of
// the HTTP contract. Now the API contract stops at the handler layer — the handler does the
// Input → Filter conversion, and everything below the repository no longer knows OpenAPI.
//
// Ports are defined in the domain layer, implemented in infrastructure/persistence (GORM),
// and wired/injected by the app layer. This also lets the service layer inject in-memory fake
// implementations for unit tests, completely solving the "zero test coverage in the dao layer" problem.

// UserFilter is the user list query condition (domain side, pure Go).
type UserFilter struct {
	domain.PageParam
	Username string // fuzzy match
	Mobile   string // exact match
	Status   *int8
	DeptID   *int64
}

// RoleFilter is the role list query condition.
type RoleFilter struct {
	domain.PageParam
	Name   string
	Code   string
	Status *int8
}

// MenuFilter is the menu list query condition.
type MenuFilter struct {
	MenuType *int8
	Name     string
	Status   *int8
}

// DeptFilter is the dept list query condition.
type DeptFilter struct {
	Name   string
	Status *int8
}

// PostFilter is the post list query condition.
type PostFilter struct {
	domain.PageParam
	Code   string
	Name   string
	Status *int8
}

// TenantFilter is the tenant list query condition.
type TenantFilter struct {
	domain.PageParam
	Name   string
	Code   string
	Status *int8
}

// LoginLogFilter is the login log query condition.
type LoginLogFilter struct {
	domain.PageParam
	Username  string
	IP        string
	Status    *int8
	BeginTime *string
	EndTime   *string
}

// OperationLogFilter is the operation log query condition.
type OperationLogFilter struct {
	domain.PageParam
	Title    string
	OperName string
	Method   string
	Status   *int8
}

// UserRepository is the user repository port.
type UserRepository interface {
	// Save persists the aggregate (insert or update), and persists domain events in the same transaction.
	Save(ctx context.Context, u *User) error
	// Get loads the aggregate by ID (including role references).
	Get(ctx context.Context, id int64) (*User, error)
	// GetByUsername loads by username (login scenario).
	GetByUsername(ctx context.Context, username string) (*User, error)
	// List paginates (read side, directly returns a list of domain objects).
	List(ctx context.Context, f UserFilter) (domain.PageResult[*User], error)
	// Delete soft-deletes (rewrites the unique key + clears associations), and persists domain events.
	Delete(ctx context.Context, u *User, operator string) error
	// UsernameExists reports whether the username is already taken (excludeID excludes itself).
	UsernameExists(ctx context.Context, username string, excludeID int64) (bool, error)
	// MobileExists reports whether the mobile number is already taken.
	MobileExists(ctx context.Context, mobile string, excludeID int64) (bool, error)
	// ReplaceRoles rebuilds user-role associations (within a transaction).
	ReplaceRoles(ctx context.Context, userID int64, roleIDs []int64) error
	// RoleCodesByIDs looks up codes by role ID (also validates ownership; an error if not in this tenant).
	RoleCodesByIDs(ctx context.Context, roleIDs []int64) ([]string, error)
	// TouchLogin records the login IP and time.
	TouchLogin(ctx context.Context, userID int64, ip string) error
}

// RoleRepository is the role repository port.
type RoleRepository interface {
	Save(ctx context.Context, r *Role) error
	Get(ctx context.Context, id int64) (*Role, error)
	List(ctx context.Context, f RoleFilter) (domain.PageResult[*Role], error)
	Delete(ctx context.Context, r *Role, operator string) error
	CodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
	// ReplaceMenus rebuilds role-menu associations.
	ReplaceMenus(ctx context.Context, roleID int64, menuIDs []int64) error
	// MenusByIDs batch-queries menus by ID (derives permission identifiers when assigning menus).
	MenusByIDs(ctx context.Context, menuIDs []int64) ([]*Menu, error)
	// PermissionsByMenuIDs batch-fetches menu permission identifiers (avoids loading the whole menu aggregate).
	PermissionsByMenuIDs(ctx context.Context, menuIDs []int64) ([]string, error)
}

// MenuRepository is the menu repository port.
type MenuRepository interface {
	Save(ctx context.Context, m *Menu) error
	Get(ctx context.Context, id int64) (*Menu, error)
	// ListAll returns all menus (for tree construction, no pagination).
	ListAll(ctx context.Context, f MenuFilter) ([]*Menu, error)
	// ListByUserRoles returns the menus visible to the user within the tenant (frontend route tree after login).
	ListByUserRoles(ctx context.Context, userID int64, menuTypes []int8) ([]*Menu, error)
	// ListByIDs batch-queries by ID.
	ListByIDs(ctx context.Context, ids []int64) ([]*Menu, error)
	Delete(ctx context.Context, id int64) error
}

// DeptRepository is the dept repository port.
type DeptRepository interface {
	Save(ctx context.Context, d *Dept) error
	Get(ctx context.Context, id int64) (*Dept, error)
	List(ctx context.Context, f DeptFilter) ([]*Dept, error)
	Delete(ctx context.Context, d *Dept, operator string) error
	// HasChildren reports whether sub-departments exist.
	HasChildren(ctx context.Context, id int64) (bool, error)
	// HasMembers reports whether the dept has users.
	HasMembers(ctx context.Context, id int64) (bool, error)
	// LeaderNames batch-fetches dept leader names (tree display after login).
	LeaderNames(ctx context.Context, deptIDs []int64) (map[int64]string, error)
}

// PostRepository is the post repository port.
type PostRepository interface {
	Save(ctx context.Context, p *Post) error
	Get(ctx context.Context, id int64) (*Post, error)
	List(ctx context.Context, f PostFilter) (domain.PageResult[*Post], error)
	Delete(ctx context.Context, p *Post, operator string) error
	CodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
}

// TenantRepository is the tenant repository port.
type TenantRepository interface {
	Save(ctx context.Context, t *Tenant) error
	Get(ctx context.Context, id int64) (*Tenant, error)
	GetByCode(ctx context.Context, code string) (*Tenant, error)
	List(ctx context.Context, f TenantFilter) (domain.PageResult[*Tenant], error)
	Delete(ctx context.Context, t *Tenant, operator string) error
	CodeExists(ctx context.Context, code string, excludeID int64) (bool, error)
	// IsAnyUserExists reports whether the tenant already has any user (rejects taking an already-used code when creating a tenant).
	IsAnyUserExists(ctx context.Context, tenantID int64) (bool, error)
}
