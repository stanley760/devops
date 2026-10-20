package iam

import (
	"encoding/json"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// Tenant domain event names.
const (
	EventTenantDisabled    = "iam.tenant.disabled"
	EventTenantAdminSeeded = "iam.tenant.admin_seeded"
)

// TenantDisabled: tenant disabled → revoke all sessions of that tenant (everyone offline).
//
// Projection: auth.RevokeTenantSessions.
// Before the refactor these were synchronous calls at service/tenant.go:169/192; failure was only logged.
// Now expressed as events: projection failures can be retried without loss.
type TenantDisabled struct {
	shared.BaseEvent
	TenantID int64
}

// EventName implements Event.
func (e TenantDisabled) EventName() string { return EventTenantDisabled }

func (e TenantDisabled) aggregateInfo() (string, int64) { return "iam.tenant", e.TenantID }

func (e TenantDisabled) eventTenantID() int64 { return e.TenantID }

// TenantAdminSeeded: the new tenant's initial super admin has been created → write the in-domain all-wildcard p row and g row binding.
//
// Before the refactor this was the synchronous call authz.SeedTenantAdmin at service/tenant.go:110;
// on failure it returned "tenant created, but super admin authorization write failed, please retry" —
// business data was already committed while the authorization was not in effect, leaving the client
// no way to tell whether to retry or intervene manually.
// Now expressed as an event: the main flow returns success, and projection failures are compensated
// by retry tasks without loss.
type TenantAdminSeeded struct {
	shared.BaseEvent
	TenantID    int64
	AdminUserID int64
}

// EventName implements Event.
func (e TenantAdminSeeded) EventName() string { return EventTenantAdminSeeded }

func (e TenantAdminSeeded) aggregateInfo() (string, int64) { return "iam.tenant", e.TenantID }

func (e TenantAdminSeeded) eventTenantID() int64 { return e.TenantID }

// TenantEventPayload is the persistence payload encode/decode entry for tenant domain events.
type TenantEventPayload struct{}

// Decode restores an event instance by event name. An unknown event name returns ok=false.
func (TenantEventPayload) Decode(name string, raw []byte) (shared.Event, bool, error) {
	switch name {
	case EventTenantDisabled:
		var e TenantDisabled
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	case EventTenantAdminSeeded:
		var e TenantAdminSeeded
		err := json.Unmarshal(raw, &e)
		return e, err == nil, err
	default:
		return nil, false, nil
	}
}
