package shared

import (
	"context"
	"time"
)

// Event is a domain event.
//
// Purpose: decouple "aggregate state has changed" from "external system projections need syncing".
//
// Pain point before the refactor: casbin policy sync was a synchronous call in the service;
// on write failure it could only return a compensating error like "user created, but role
// authorization write failed, please retry the update" — the business row was already
// committed but the authorization was not in effect, so the client could not tell whether to
// retry or intervene manually (3 places: user.Create / user.Update / role.AssignMenus, plus
// tenant's manual HardDeleteTenant compensation).
//
// After the refactor: the aggregate records events within the transaction, the repository
// persists them in the same transaction as the business write, and projection views are
// consumed by the projector. Authorization sync failures no longer pollute the main-flow
// response, and because events are persisted, failures can be retried without loss — this is
// exactly the value of the "write model / read model separation" in CQRS.
type Event interface {
	// EventName is the event name (used as the key for persistence and projection routing).
	EventName() string
	// OccurredAt is the event occurrence time.
	OccurredAt() time.Time
}

// BaseEvent is the event base type; embedding it automatically implements Event's OccurredAt.
type BaseEvent struct {
	Time time.Time
}

// OccurredAt implements Event.
func (e BaseEvent) OccurredAt() time.Time { return e.Time }

// EventStore is the event storage port (write side).
//
// Events and business data are written in the same transaction, guaranteeing that "state change"
// and "pending sync facts" are persisted atomically.
// Implementation: infrastructure/persistence/gorm_store.
type EventStore interface {
	// Append appends events in the current transaction context.
	Append(ctx context.Context, events ...Event) error
}

// EventPublisher is the event dispatch port (implemented on the infrastructure side).
type EventPublisher interface {
	// Publish dispatches events to all matching projection handlers.
	Publish(ctx context.Context, events ...Event) error
}

// EventProjector is the projector: subscribes to a class of events and projects write-model
// changes onto an external read model (casbin).
//
// Idempotency requirement: projectors may be invoked repeatedly (retry on failure), so every
// Handle implementation must be idempotent — for casbin, the RebuildXxx family of functions
// is itself idempotent (clear first, then rebuild).
type EventProjector interface {
	// Handles returns the event names this projector cares about.
	Handles() []string
	// Handle processes a single event. Returned errors are retried by the dispatcher per policy, without affecting the main-flow response.
	Handle(ctx context.Context, e Event) error
}
