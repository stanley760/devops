package iam

import (
	"errors"
	"testing"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"
)

func fixedTime() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) }

// bizCode extracts the business error code (errs.BizError).
func bizCode(err error) int {
	var be *errs.BizError
	if errors.As(err, &be) {
		return be.Code
	}
	return -1
}

func newUser(id int64, status UserStatus) *User {
	return &User{ID: id, TenantID: 7, Username: "alice", Status: status}
}

// TestUserEnsureDeletable covers the delete invariants within the aggregate.
// Before the refactor these three rules were bare ifs at service/user.go:232-237, unverifiable without a DB.
func TestUserEnsureDeletable(t *testing.T) {
	cases := []struct {
		name      string
		userID    int64
		actorID   int64
		wantCode  int
		wantEmpty bool
	}{
		{name: "delete another user", userID: 42, actorID: 7, wantEmpty: true},
		{name: "deleting self rejected", userID: 7, actorID: 7, wantCode: errs.CodeParam},
		{name: "builtin super admin not deletable", userID: builtinAdminID, actorID: 7, wantCode: errs.CodeParam},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := newUser(c.userID, UserStatusEnabled).EnsureDeletable(c.actorID)
			if c.wantEmpty {
				if err != nil {
					t.Fatalf("expected pass, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected rejection, got pass")
			}
			if got := bizCode(err); got != c.wantCode {
				t.Fatalf("error code = %d, want %d", got, c.wantCode)
			}
		})
	}
}

// TestUserChangeStatusRecordsEvent: disabling/enabling should record the corresponding domain events.
// Before the refactor this rule was at service/user.go:201-203; testing required the full Redis + MySQL + HTTP stack.
func TestUserChangeStatusRecordsEvent(t *testing.T) {
	cases := []struct {
		name      string
		from      UserStatus
		to        UserStatus
		wantEvent string
	}{
		{name: "enabled to disabled", from: UserStatusEnabled, to: UserStatusDisabled, wantEvent: EventUserDeactivated},
		{name: "disabled to enabled", from: UserStatusDisabled, to: UserStatusEnabled, wantEvent: EventUserActivated},
		{name: "unchanged status records no event", from: UserStatusEnabled, to: UserStatusEnabled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newUser(42, c.from)
			u.ChangeStatus(c.to, fixedTime())
			if u.Status != c.to {
				t.Fatalf("status = %d, want %d", u.Status, c.to)
			}
			events := u.Events()
			if c.wantEvent == "" {
				if len(events) != 0 {
					t.Fatalf("expected no events, got %d", len(events))
				}
				return
			}
			if len(events) != 1 || events[0].EventName() != c.wantEvent {
				t.Fatalf("events = %v, want %s", events, c.wantEvent)
			}
		})
	}
}

// TestUserAssignRolesRecordsEvent: role assignment should record the g row sync event.
func TestUserAssignRolesRecordsEvent(t *testing.T) {
	u := newUser(42, UserStatusEnabled)
	u.AssignRoles([]int64{3, 5}, []string{"editor", "ops"}, fixedTime())

	if len(u.RoleIDs) != 2 {
		t.Fatalf("RoleIDs = %v, want [3 5]", u.RoleIDs)
	}
	events := u.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	e, ok := events[0].(UserRolesAssigned)
	if !ok {
		t.Fatalf("event type = %T, want UserRolesAssigned", events[0])
	}
	if len(e.RoleCodes) != 2 || e.RoleCodes[0] != "editor" {
		t.Fatalf("RoleCodes = %v, want [editor ops]", e.RoleCodes)
	}
}

// TestUserPullEventsConsumesBuffer: PullEvents should consume the event buffer (deliver only once).
func TestUserPullEventsConsumesBuffer(t *testing.T) {
	u := newUser(42, UserStatusEnabled)
	u.AssignRoles([]int64{1}, []string{"r"}, fixedTime())

	if got := u.PullEvents(); len(got) != 1 {
		t.Fatalf("first PullEvents = %d, want 1", len(got))
	}
	if got := u.PullEvents(); len(got) != 0 {
		t.Fatalf("second PullEvents = %d, want 0 (should already be consumed)", len(got))
	}
	// the Events read-only copy does not affect the buffer.
	if got := u.Events(); len(got) != 0 {
		t.Fatalf("Events = %d, want 0", len(got))
	}
}

// TestRoleEnsureDeletable: builtin roles cannot be deleted.
func TestRoleEnsureDeletable(t *testing.T) {
	builtin := &Role{ID: 2, Code: "super_admin", Type: RoleTypeBuiltin}
	if err := builtin.EnsureDeletable(); err == nil {
		t.Fatal("builtin roles should reject deletion")
	} else if bizCode(err) != errs.CodeParam {
		t.Fatalf("error code = %d, want CodeParam", bizCode(err))
	}

	custom := &Role{ID: 9, Code: "editor", Type: RoleTypeCustom}
	if err := custom.EnsureDeletable(); err != nil {
		t.Fatalf("custom roles should allow deletion: %v", err)
	}
}

// TestRoleAssignMenusSkipsBuiltin: builtin role menu assignment should not derive p row events.
// This is the key rule preventing the super admin from being locked out (previously service/role.go:159).
func TestRoleAssignMenusSkipsBuiltin(t *testing.T) {
	builtin := &Role{ID: 2, Code: "super_admin", Type: RoleTypeBuiltin}
	builtin.AssignMenus([]int64{1, 2}, []string{"system:user:list"}, fixedTime())
	if len(builtin.Events()) != 0 {
		t.Fatalf("builtin roles should not produce p row events, got %d", len(builtin.Events()))
	}

	custom := &Role{ID: 9, Code: "editor", Type: RoleTypeCustom}
	custom.AssignMenus([]int64{1}, []string{"system:user:list"}, fixedTime())
	events := custom.Events()
	if len(events) != 1 || events[0].EventName() != EventRoleMenusAssigned {
		t.Fatalf("events = %v, want RoleMenusAssigned", events)
	}
}

// TestValidateDataScope: the custom scope must carry a dept set.
func TestValidateDataScope(t *testing.T) {
	cases := []struct {
		name       string
		scope      int8
		deptIDs    string
		wantReject bool
	}{
		{name: "all permission needs no dept", scope: 1, deptIDs: ""},
		{name: "custom with depts", scope: 2, deptIDs: "[4,5]"},
		{name: "custom without depts rejected", scope: 2, deptIDs: "", wantReject: true},
		{name: "custom empty array rejected", scope: 2, deptIDs: "[]", wantReject: true},
		{name: "custom dirty data rejected", scope: 2, deptIDs: `["4"]`, wantReject: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateDataScope(c.scope, c.deptIDs)
			if c.wantReject && err == nil {
				t.Fatal("expected rejection, got pass")
			}
			if !c.wantReject && err != nil {
				t.Fatalf("expected pass, got: %v", err)
			}
		})
	}
}

// TestTenantIsEnabled: tenant three-state usability (normal/disabled/expired).
func TestTenantIsEnabled(t *testing.T) {
	now := fixedTime()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name       string
		status     int8
		expireTime *time.Time
		want       bool
	}{
		{name: "normal and never expires", status: TenantStatusEnabled, want: true},
		{name: "disabled", status: TenantStatusDisabled, want: false},
		{name: "expired", status: TenantStatusEnabled, expireTime: &past, want: false},
		{name: "not expired", status: TenantStatusEnabled, expireTime: &future, want: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tn := &Tenant{Status: c.status, ExpireTime: c.expireTime}
			if got := tn.IsEnabled(now); got != c.want {
				t.Fatalf("IsEnabled = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTenantChangeRecordsDisableEvent: disabling a tenant should record the everyone-offline event.
func TestTenantChangeRecordsDisableEvent(t *testing.T) {
	tn := &Tenant{ID: 7, Status: TenantStatusEnabled}
	tn.Change(TenantUpdate{Name: "t7", Status: TenantStatusDisabled}, fixedTime())

	events := tn.Events()
	if len(events) != 1 || events[0].EventName() != EventTenantDisabled {
		t.Fatalf("events = %v, want TenantDisabled", events)
	}
}

// TestMenuEnsureNotSelfOrDescendant: cycle prevention — a menu cannot be attached under its own descendant.
func TestMenuEnsureNotSelfOrDescendant(t *testing.T) {
	all := []*Menu{
		{ID: 1, ParentID: 0},
		{ID: 2, ParentID: 1},
		{ID: 3, ParentID: 2},
	}
	m := &Menu{ID: 2, ParentID: 1}

	if err := m.EnsureNotSelfOrDescendant(3, all); err == nil {
		t.Fatal("attaching under a descendant should be rejected")
	}
	if err := m.EnsureNotSelfOrDescendant(1, all); err != nil {
		t.Fatalf("attaching under the parent should be allowed: %v", err)
	}
	if err := m.EnsureNotSelfOrDescendant(0, all); err != nil {
		t.Fatalf("top level should be allowed: %v", err)
	}
}
