package iam

import (
	"encoding/json"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// User domain event names (keys for persistence and projection routing; changes require migrating historical data in sync).
const (
	EventUserRolesAssigned   = "iam.user.roles_assigned"
	EventUserDeactivated     = "iam.user.deactivated"
	EventUserActivated       = "iam.user.activated"
	EventUserDeleted         = "iam.user.deleted"
	EventUserPasswordChanged = "iam.user.password_changed"
)

// UserRolesAssigned: user role assignment changed → rebuild casbin g rows.
//
// Projection: casbin.RebuildUserGrouping (clear first then rebuild, naturally idempotent).
// Before the refactor this was a synchronous call at service/user.go:195; on failure it returned
// "user updated, but role authorization refresh failed, please retry the update".
type UserRolesAssigned struct {
	shared.BaseEvent
	UserID    int64
	TenantID  int64
	RoleIDs   []int64
	RoleCodes []string
}

// EventName implements Event.
func (e UserRolesAssigned) EventName() string { return EventUserRolesAssigned }

func (e UserRolesAssigned) aggregateInfo() (string, int64) { return "iam.user", e.UserID }

func (e UserRolesAssigned) eventTenantID() int64 { return e.TenantID }

// UserDeactivated: user disabled → revoke all sessions of that user (kick offline).
//
// Projection: auth.RevokeAllSessions. Before the refactor this was a synchronous call at service/user.go:202.
type UserDeactivated struct {
	shared.BaseEvent
	UserID   int64
	TenantID int64
}

// EventName implements Event.
func (e UserDeactivated) EventName() string { return EventUserDeactivated }

func (e UserDeactivated) aggregateInfo() (string, int64) { return "iam.user", e.UserID }

func (e UserDeactivated) eventTenantID() int64 { return e.TenantID }

// UserActivated: user enabled (no external action needed for now; kept for future notification/audit scenarios).
type UserActivated struct {
	shared.BaseEvent
	UserID   int64
	TenantID int64
}

// EventName implements Event.
func (e UserActivated) EventName() string { return EventUserActivated }

func (e UserActivated) aggregateInfo() (string, int64) { return "iam.user", e.UserID }

func (e UserActivated) eventTenantID() int64 { return e.TenantID }

// UserDeleted: user soft-deleted → clean up casbin g rows.
//
// Projection: casbin.RemoveUserGrouping. Before the refactor this was a synchronous call at service/user.go:250;
// on failure it returned "user deleted, but role authorization cleanup failed".
type UserDeleted struct {
	shared.BaseEvent
	UserID   int64
	TenantID int64
}

// EventName implements Event.
func (e UserDeleted) EventName() string { return EventUserDeleted }

func (e UserDeleted) aggregateInfo() (string, int64) { return "iam.user", e.UserID }

func (e UserDeleted) eventTenantID() int64 { return e.TenantID }

// UserEventPayload is the persistence payload for user domain events (the event table's Payload column).
//
// Domain events are pure value types (including BaseEvent.Time) and can round-trip through JSON directly;
// this unified deserialization entry is provided for the background retry projection task.
type UserEventPayload struct{}

// Decode restores an event instance by event name. An unknown event name returns ok=false, and the caller skips it.
func (UserEventPayload) Decode(name string, raw []byte) (shared.Event, bool, error) {
	switch name {
	case EventUserRolesAssigned:
		var e UserRolesAssigned
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventUserDeactivated:
		var e UserDeactivated
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventUserActivated:
		var e UserActivated
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventUserDeleted:
		var e UserDeleted
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventUserPasswordChanged:
		var e UserPasswordChanged
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	default:
		return nil, false, nil
	}
}

// UserPasswordChanged: password changed → force re-login (revoke all sessions).
//
// Before the refactor these were the synchronous calls authM.RevokeAllSessions in service/user.go:226
// (reset password) and service/auth.go (change password); now executed by the projector, so a
// session revocation failure no longer lets the inconsistent state of "password changed but user
// still online" happen silently.
type UserPasswordChanged struct {
	shared.BaseEvent
	UserID   int64
	TenantID int64
	// SelfChange indicates whether it is the user's own password change (a self-change should also reset other sessions).
	SelfChange bool
}

// EventName implements Event.
func (e UserPasswordChanged) EventName() string { return EventUserPasswordChanged }

func (e UserPasswordChanged) aggregateInfo() (string, int64) { return "iam.user", e.UserID }

func (e UserPasswordChanged) eventTenantID() int64 { return e.TenantID }
