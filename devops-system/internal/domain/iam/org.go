package iam

import (
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Dept is the dept aggregate root (within the tenant).
//
// Note: t_system_dept has no ancestors column (aligned with yudao); the tree parent-child
// relationship is expressed only by parent_id, with path info assembled on the query side as needed.
type Dept struct {
	shared.AggregateRoot

	ID       int64
	TenantID int64
	ParentID int64
	Name     string
	Sort     int
	// LeaderUserID is the leader (user ID; column name leader_user_id).
	LeaderUserID int64
	Phone        string
	Email        string
	Status       int8
	CreateTime   time.Time
	UpdateTime   time.Time
}

// DeptUpdate is the dept change input (domain input).
type DeptUpdate struct {
	ParentID     int64
	Name         string
	Sort         int
	LeaderUserID int64
	Phone        string
	Email        string
	Status       int8
}

// ValidateForCreate validates before creation.
func (d *Dept) ValidateForCreate() error {
	if strings.TrimSpace(d.Name) == "" {
		return errs.New(errs.CodeParam, "dept name cannot be empty")
	}
	return nil
}

// Change applies the change.
func (d *Dept) Change(up DeptUpdate) {
	d.ParentID = up.ParentID
	d.Name = up.Name
	d.Sort = up.Sort
	d.LeaderUserID = up.LeaderUserID
	d.Phone = up.Phone
	d.Email = up.Email
	d.Status = up.Status
}

// EnsureNotSelf validates that a dept cannot set itself as its own parent.
func (d *Dept) EnsureNotSelf(parentID int64) error {
	if parentID == d.ID {
		return errs.New(errs.CodeParam, "parent dept cannot be itself")
	}
	return nil
}

// HasChildrenReport reports whether the dept still has child departments or members (pre-delete validation).
//
// The returned messages are kept consistent with the existing implementation to avoid frontend prompt changes.
func HasChildrenReport(hasChildren bool, hasMembers bool) error {
	if hasChildren {
		return errs.New(errs.CodeParam, "sub-departments exist, cannot delete")
	}
	if hasMembers {
		return errs.New(errs.CodeParam, "users exist under the department, cannot delete")
	}
	return nil
}

// Post is the post aggregate root (within the tenant).
type Post struct {
	shared.AggregateRoot

	ID         int64
	TenantID   int64
	Code       string // unique within the tenant
	Name       string
	Sort       int
	Status     int8
	Remark     string
	CreateTime time.Time
	UpdateTime time.Time
}

// PostUpdate is the post change input.
type PostUpdate struct {
	Code   string
	Name   string
	Sort   int
	Status int8
	Remark string
}

// ValidateForCreate validates before creation: code must be non-empty.
func (p *Post) ValidateForCreate() error {
	if strings.TrimSpace(p.Code) == "" {
		return errs.New(errs.CodeParam, "post code cannot be empty")
	}
	return nil
}

// Change applies the change.
func (p *Post) Change(up PostUpdate) {
	p.Code = up.Code
	p.Name = up.Name
	p.Sort = up.Sort
	p.Status = up.Status
	p.Remark = up.Remark
}

// Tenant is the tenant aggregate root (global table, no tenant_id — the tenant registry itself).
//
// Tenant status is coupled with user/role status: disabling a tenant invalidates all sessions and denies login.
// Before the refactor, this coupling was scattered in service/tenant.go (RevokeTenantSessions calls);
// it is now expressed via a domain event.
type Tenant struct {
	shared.AggregateRoot

	ID   int64
	Name string
	// Code is globally unique (uk_code), the frontend tenant identifier and login input.
	Code          string
	ContactName   string
	ContactMobile string
	Status        int8
	// ExpireTime nil means never expires.
	ExpireTime *time.Time
	Remark     string
	CreateTime time.Time
	UpdateTime time.Time
}

// Tenant status.
const (
	TenantStatusDisabled int8 = 0
	TenantStatusEnabled  int8 = 1
)

// TenantUpdate is the tenant change input.
type TenantUpdate struct {
	Name          string
	ContactName   string
	ContactMobile string
	Status        int8
	ExpireTime    *time.Time
	Remark        string
}

// IsEnabled reports whether the tenant is usable (exists, not disabled, not expired).
func (t *Tenant) IsEnabled(now time.Time) bool {
	if t.Status != TenantStatusEnabled {
		return false
	}
	if t.ExpireTime != nil && t.ExpireTime.Before(now) {
		return false
	}
	return true
}

// EnsureUsable validates tenant usability (login and the middleware share the same judgment semantics).
func (t *Tenant) EnsureUsable(now time.Time) error {
	if !t.IsEnabled(now) {
		return errs.New(errs.CodeForbidden, "tenant is unavailable (disabled or expired)")
	}
	return nil
}

// Change applies the change; records an event when the status turns to disabled (kick everyone offline).
func (t *Tenant) Change(up TenantUpdate, at time.Time) {
	was := t.Status
	t.Name = up.Name
	t.ContactName = up.ContactName
	t.ContactMobile = up.ContactMobile
	t.Status = up.Status
	t.ExpireTime = up.ExpireTime
	t.Remark = up.Remark
	t.UpdateTime = at

	if was == TenantStatusEnabled && up.Status == TenantStatusDisabled {
		t.RecordEvent(TenantDisabled{
			BaseEvent: shared.BaseEvent{Time: at},
			TenantID:  t.ID,
		})
	}
}

// SyncStatusWithExpiry derives the status from the expiry time (future -> enabled, past -> disabled).
//
// Business rule: when updating a tenant with an explicit expiry time, the status should stay
// consistent with it, otherwise the contradictory state of "expired but still enabled" appears.
// Before the refactor, this rule was expressed as an updates map in service/tenant.go:130-140.
func (t *Tenant) SyncStatusWithExpiry(now time.Time) {
	if t.ExpireTime == nil {
		return // never expires: do not change the status
	}
	want := TenantStatusEnabled
	if !t.ExpireTime.After(now) {
		want = TenantStatusDisabled
	}
	if t.Status != want {
		t.Change(TenantUpdate{
			Name:          t.Name,
			ContactName:   t.ContactName,
			ContactMobile: t.ContactMobile,
			ExpireTime:    t.ExpireTime,
			Remark:        t.Remark,
			Status:        want,
		}, now)
	}
}

// EnsureNotDefaultDisable forbids disabling the default tenant (to avoid locking yourself out).
//
// This rule did not exist before the refactor; it is an explicit business constraint added in this refactor.
func (t *Tenant) EnsureNotDefaultDisable(code string) error {
	if code == "default" && t.Status == TenantStatusDisabled {
		return errs.New(errs.CodeParam, "the default tenant cannot be disabled")
	}
	return nil
}
