package persistence

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-system/internal/domain/shared"
	"github.com/stanley760/devops/devops-system/internal/model"
)

// Domain event table name.
const eventTable = "t_system_domain_event"

// DomainEventPO is the domain event persistence object.
//
// A new table; it does not affect existing business tables or existing data. The table
// structure is created by dbinit's SQL migration (idempotent: skipped if the table exists).
//
// Reuses model.BaseModel rather than domain.BaseModel: the latter does not exist in the
// devops-common/domain package (that package only has pagination value objects).
// Reusing the existing model.BaseModel directly lets the event table share the same
// primary key assignment (BeforeCreate hook) and audit field filling logic as business tables.
type DomainEventPO struct {
	model.BaseModel
	// EventName is the event name (projection routing key).
	EventName string `gorm:"size:64;not null;index"`
	// AggregateType is the aggregate type (e.g. iam.user).
	AggregateType string `gorm:"size:32;not null;index"`
	// AggregateID is the aggregate ID.
	AggregateID int64 `gorm:"not null;index"`
	// TenantID is the tenant ID (locates the casbin domain by tenant at projection time).
	TenantID int64 `gorm:"index;not null"`
	// Payload is the event payload JSON.
	Payload string `gorm:"type:json"`
	// OccurredAt is the event occurrence time.
	OccurredAt time.Time `gorm:"not null"`
	// ProjectedAt is the projection completion time; NULL means pending projection.
	ProjectedAt *time.Time `gorm:"index"`
	// Attempts is the projection attempt count (failure retry count).
	Attempts int `gorm:"not null;default:0"`
	// LastError is the most recent projection error.
	LastError string `gorm:"size:500"`
}

// TableName is the table name.
func (DomainEventPO) TableName() string { return eventTable }

// eventStore is the GORM implementation of shared.EventStore.
type eventStore struct{}

// NewEventStore constructs the event store.
func NewEventStore() shared.EventStore { return eventStore{} }

// Append appends domain events in the current transaction context.
func (eventStore) Append(ctx context.Context, events ...shared.Event) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]DomainEventPO, 0, len(events))
	for _, e := range events {
		payload, err := json.Marshal(e)
		if err != nil {
			return err
		}
		at, aggType, aggID := eventMeta(e)
		rows = append(rows, DomainEventPO{
			EventName:     e.EventName(),
			AggregateType: aggType,
			AggregateID:   aggID,
			TenantID:      eventTenant(e),
			Payload:       string(payload),
			OccurredAt:    at,
		})
	}
	if err := db(ctx).Create(&rows).Error; err != nil {
		return err
	}
	return nil
}

// appendEvents is a convenience call inside the repository.
func appendEvents(ctx context.Context, events ...shared.Event) error {
	if len(events) == 0 {
		return nil
	}
	return eventStore{}.Append(ctx, events...)
}

// eventMeta extracts the aggregate identifier and occurrence time from the event.
func eventMeta(e shared.Event) (time.Time, string, int64) {
	type aggInfo interface {
		aggregateInfo() (string, int64)
	}
	if a, ok := e.(aggInfo); ok {
		t, id := a.aggregateInfo()
		return e.OccurredAt(), t, id
	}
	return e.OccurredAt(), "", 0
}

// eventTenant extracts the tenant ID from the event (needed for the casbin domain at projection time).
func eventTenant(e shared.Event) int64 {
	type tenantInfo interface{ eventTenantID() int64 }
	if t, ok := e.(tenantInfo); ok {
		return t.eventTenantID()
	}
	return 0
}

// dispatcher is the domain event dispatcher: routes events to the corresponding projectors by event name.
//
// Key semantics points (this is what replaces manual compensation):
//   - projection failures do not return an error to the main flow; the business write is already committed and should not confuse the user because an authorization projection failed
//   - on failure the event keeps projected_at = NULL and accumulates attempts, retried by a background task
//   - every projector implementation must be idempotent (casbin's RebuildXxx family clears first then rebuilds, naturally idempotent)
type dispatcher struct {
	mu         sync.RWMutex
	projectors map[string][]shared.EventProjector
}

// NewDispatcher constructs the dispatcher and registers projectors.
func NewDispatcher(projectors ...shared.EventProjector) shared.EventPublisher {
	d := &dispatcher{projectors: make(map[string][]shared.EventProjector)}
	for _, p := range projectors {
		if p == nil {
			continue
		}
		for _, name := range p.Handles() {
			d.projectors[name] = append(d.projectors[name], p)
		}
	}
	return d
}

// Publish dispatches events. Projection failures are only recorded, not returned upward.
func (d *dispatcher) Publish(ctx context.Context, events ...shared.Event) error {
	for _, e := range events {
		d.mu.RLock()
		targets := d.projectors[e.EventName()]
		d.mu.RUnlock()

		for _, p := range targets {
			if err := p.Handle(ctx, e); err != nil {
				// projection failures are not propagated to the main flow: the event is already persisted and can be compensated by retry tasks.
				slog.Error("domain event projection failed",
					"event", e.EventName(),
					"projector", projectorName(p),
					"at", e.OccurredAt(),
					"err", err)
			}
		}
	}
	return nil
}

func projectorName(p shared.EventProjector) string {
	if n, ok := p.(interface{ Name() string }); ok {
		return n.Name()
	}
	return "unknown"
}

// PendingEvents takes the pending-projection events (for the background retry task).
func PendingEvents(ctx context.Context, limit int) ([]DomainEventPO, error) {
	var rows []DomainEventPO
	err := db(ctx).Where("projected_at IS NULL").
		Order("occurred_at ASC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// MarkProjected marks the event's projection as complete.
func MarkProjected(ctx context.Context, id int64) error {
	now := time.Now()
	return db(ctx).Model(&DomainEventPO{}).
		Where("id = ?", id).Updates(map[string]any{
		"projected_at": &now,
		"last_error":   "",
	}).Error
}

// MarkProjectFailed records the projection failure and accumulates the retry count.
func MarkProjectFailed(ctx context.Context, id int64, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
	}
	return db(ctx).Model(&DomainEventPO{}).Where("id = ?", id).Updates(map[string]any{
		"attempts":   gorm.Expr("attempts + 1"),
		"last_error": truncate(msg, 500),
	}).Error
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
