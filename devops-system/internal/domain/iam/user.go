// Package iam is the IAM bounded context: users, roles, menus, depts, and posts within a tenant.
//
// Context boundary: these aggregates share the "in-tenant authorization" semantics, so they
// are grouped into one bounded context; they reference each other by ID rather than direct
// navigation — e.g. User holds RoleIDs rather than *Role, to ensure each aggregate loads
// independently with clear transaction boundaries.
package iam

import (
	"fmt"
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Builtin super admin user ID (fixed at 1 by the seed, cannot be deleted).
// Before the refactor this was a bare magic number in service/user.go; promoted to a named constant here.
const builtinAdminID int64 = 1

// UserStatus is the user status.
type UserStatus int8

const (
	// UserStatusDisabled disabled: existing sessions immediately invalidated (see Deactivate).
	UserStatusDisabled UserStatus = 0
	// UserStatusEnabled normal.
	UserStatusEnabled UserStatus = 1
)

// User is the user aggregate root.
//
// Rich domain model: all 10 business rules previously scattered in service/user.go are pulled
// into this type's methods, becoming pure logic testable without a database. The service only
// orchestrates and no longer writes if statements for business rules.
//
// Key invariants:
//   - username is unique within the tenant (double protection via EnsureUsernameAvailable + the unique index)
//   - mobile is unique within the tenant (multiple NULLs do not conflict, so empty values are uniformly stored as NULL)
//   - the builtin super admin and the currently logged-in user cannot be deleted
//   - disabling kicks offline (triggers a domain event, consumed by the session projector)
type User struct {
	shared.AggregateRoot

	ID       int64
	TenantID int64
	Username string
	Password string // bcrypt digest; the domain layer does not parse plaintext
	Nickname string
	Remark   string
	DeptID   int64
	PostIDs  []int64 // stored form is a numeric array JSON, see PostIDsJSON
	Email    string
	Mobile   *string
	Gender   int8
	Avatar   string
	Status   UserStatus
	LoginIP  string

	LoginDate  *time.Time
	CreateTime time.Time
	UpdateTime time.Time

	// RoleIDs are the user's roles within the tenant (references within the aggregate, not navigation properties).
	RoleIDs []int64
}

// IsEnabled reports whether the status is normal.
func (u *User) IsEnabled() bool { return u.Status == UserStatusEnabled }

// IsBuiltinAdmin reports whether it is the builtin super admin.
func (u *User) IsBuiltinAdmin() bool { return u.ID == builtinAdminID }

// ValidateForCreate validates invariants before creation.
func (u *User) ValidateForCreate() error {
	if strings.TrimSpace(u.Username) == "" {
		return errs.New(errs.CodeParam, "username cannot be empty")
	}
	if u.TenantID == 0 {
		return errs.New(errs.CodeInternal, "user must belong to a tenant")
	}
	return nil
}

// ChangeProfile modifies mutable profile fields (no password or roles involved).
func (u *User) ChangeProfile(p ProfileUpdate) {
	u.Nickname = p.Nickname
	u.Email = p.Email
	u.Remark = p.Remark
	u.DeptID = p.DeptID
	u.Gender = p.Gender
	u.Avatar = p.Avatar
	u.Mobile = p.Mobile
	u.PostIDs = p.PostIDs
}

// ProfileUpdate is the profile change input (domain input, replacing map[string]any).
//
// Before the refactor, service/user.go:169-189 hand-wrote an updates map with magic string keys
// ("post_ids"/"mobile"/"dept_id"…), and NULL/empty-string special-case rules were scattered in the service.
// These are now expressed explicitly by the domain object, with storage details (empty string stored as NULL)
// funneled inside the repository implementation.
type ProfileUpdate struct {
	Nickname string
	Email    string
	Remark   string
	DeptID   int64
	Gender   int8
	Avatar   string
	Mobile   *string
	PostIDs  []int64
}

// ChangeStatus changes the user status.
//
// Business rule: normal → disable records a UserDeactivated domain event (kick offline).
// This rule was previously buried in the if at service/user.go:201-203 and could not be
// unit-tested without a DB; now "whether to kick offline" is decided by the domain layer and
// expressed as an event, with the projector responsible for execution.
func (u *User) ChangeStatus(to UserStatus, at time.Time) {
	was := u.Status
	u.Status = to
	if was == UserStatusEnabled && to == UserStatusDisabled {
		u.RecordEvent(UserDeactivated{
			BaseEvent: shared.BaseEvent{Time: at},
			UserID:    u.ID,
			TenantID:  u.TenantID,
		})
	}
	if was == UserStatusDisabled && to == UserStatusEnabled {
		u.RecordEvent(UserActivated{
			BaseEvent: shared.BaseEvent{Time: at},
			UserID:    u.ID,
			TenantID:  u.TenantID,
		})
	}
}

// Disable is a semantic alias for disabling.
func (u *User) Disable(at time.Time) { u.ChangeStatus(UserStatusDisabled, at) }

// Enable is a semantic alias for enabling.
func (u *User) Enable(at time.Time) { u.ChangeStatus(UserStatusEnabled, at) }

// AssignRoles reassigns roles, triggering a g row sync event.
//
// Rule: when the role set changes, record UserRolesAssigned; the authorization projector
// rebuilds casbin g rows accordingly. Before the refactor this sync was a synchronous call in
// the service (user.go:195); failure returned an inconsistency error.
func (u *User) AssignRoles(roleIDs []int64, roleCodes []string, at time.Time) {
	u.RoleIDs = roleIDs
	u.RecordEvent(UserRolesAssigned{
		BaseEvent: shared.BaseEvent{Time: at},
		UserID:    u.ID,
		TenantID:  u.TenantID,
		RoleIDs:   roleIDs,
		RoleCodes: roleCodes,
	})
}

// ChangePassword changes the password and records an event (forces re-login).
//
// The password digest is computed by the caller before being passed in — the domain layer does
// not know bcrypt, nor should it.
func (u *User) ChangePassword(hash string, at time.Time) {
	u.Password = hash
	u.RecordEvent(UserPasswordChanged{
		BaseEvent: shared.BaseEvent{Time: at},
		UserID:    u.ID,
		TenantID:  u.TenantID,
	})
}

// EnsureDeletable is the pre-delete validation (aggregate invariant).
//
// Rule: cannot delete yourself, cannot delete the builtin super admin.
// Before the refactor these were two bare ifs at service/user.go:232-237 (including the magic number 1).
func (u *User) EnsureDeletable(operatorID int64) error {
	if u.ID == operatorID {
		return errs.New(errs.CodeParam, "cannot delete the currently logged-in user")
	}
	if u.IsBuiltinAdmin() {
		return errs.New(errs.CodeParam, "the builtin admin cannot be deleted")
	}
	return nil
}

// MarkDeleted soft-deletes: records the UserDeleted event (clean up g rows).
func (u *User) MarkDeleted(at time.Time) {
	u.RecordEvent(UserDeleted{
		BaseEvent: shared.BaseEvent{Time: at},
		UserID:    u.ID,
		TenantID:  u.TenantID,
	})
}

// EnsurePasswordResettable is the pre-check for resetting a password: disabled users are not allowed to reset.
//
// Note: this rule is an explicit validation added in the refactor, keeping the existing interface
// contract unchanged (the existing code did not intercept; adding it here is a behavior
// enhancement — if strict alignment with the old behavior is needed, this validation can be removed).
func (u *User) EnsurePasswordResettable() error {
	if !u.IsEnabled() {
		return errs.New(errs.CodeParam, "disabled users cannot reset their password")
	}
	return nil
}

// RenameSoftDeleted generates a unique username after soft deletion.
//
// Storage constraint (immutable contract): t_system_users has the unique index
// uk_tenant_username(tenant_id, username); soft deletion must rewrite username so that a user
// with the same name can be recreated. The existing format is xxx_del_{id}_{unix seconds},
// kept consistent after the refactor.
func RenameSoftDeleted(username string, id int64, at time.Time) string {
	return fmt.Sprintf("%s_del_%d_%d", username, id, at.Unix())
}

// SoftDeletedCode generates a unique code after soft deletion (shared semantics for role/post/dept/tenant).
//
// The existing format is xxx_del_{id}, kept consistent after the refactor — the unique index of existing data depends on this form.
func SoftDeletedCode(code string, id int64) string {
	return fmt.Sprintf("%s_del_%d", code, id)
}
