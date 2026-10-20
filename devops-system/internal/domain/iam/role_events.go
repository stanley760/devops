package iam

import (
	"encoding/json"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Role domain event names.
const (
	EventRoleMenusAssigned = "iam.role.menus_assigned"
	EventRoleDeleted       = "iam.role.deleted"
)

// RoleMenusAssigned: role menu assignment changed → snapshot-rebuild casbin p rows by (role, tenant).
//
// Projection: casbin.RebuildRolePolicies (clear first then rebuild, idempotent).
// Before the refactor this was a synchronous call at service/role.go:168; on failure it returned
// "menu saved, but permission policy refresh failed, please retry". Builtin roles do not emit this event.
type RoleMenusAssigned struct {
	shared.BaseEvent
	RoleID      int64
	RoleCode    string
	TenantID    int64
	Permissions []string
}

// EventName implements Event.
func (e RoleMenusAssigned) EventName() string { return EventRoleMenusAssigned }

func (e RoleMenusAssigned) aggregateInfo() (string, int64) { return "iam.role", e.RoleID }

func (e RoleMenusAssigned) eventTenantID() int64 { return e.TenantID }

// RoleDeleted: role soft-deleted → clean up p row and g row references.
//
// Projection: casbin.RemoveRolePolicies + RemoveRoleGrouping.
// Before the refactor these were two synchronous calls at service/role.go:188-193.
type RoleDeleted struct {
	shared.BaseEvent
	RoleID   int64
	RoleCode string
	TenantID int64
}

// EventName implements Event.
func (e RoleDeleted) EventName() string { return EventRoleDeleted }

func (e RoleDeleted) aggregateInfo() (string, int64) { return "iam.role", e.RoleID }

func (e RoleDeleted) eventTenantID() int64 { return e.TenantID }

// RoleEventPayload is the persistence payload encode/decode entry for role domain events.
type RoleEventPayload struct{}

// Decode restores an event instance by event name. An unknown event name returns ok=false.
func (RoleEventPayload) Decode(name string, raw []byte) (shared.Event, bool, error) {
	switch name {
	case EventRoleMenusAssigned:
		var e RoleMenusAssigned
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventRoleDeleted:
		var e RoleDeleted
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	default:
		return nil, false, nil
	}
}
