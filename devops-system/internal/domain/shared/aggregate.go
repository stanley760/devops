// Package shared is the shared kernel of the domain layer: aggregate base classes, domain
// events, domain errors, and repository port conventions.
//
// Layering constraint (DDD dependency direction):
//
//	api/v1 → application → domain ← infrastructure
//	                ↘ infrastructure/persistence (implements domain's ports)
//
// This package and the entire domain tree must not import GORM, huma, Redis, HTTP, or any
// other infrastructure package; only devops-infra/errs and the standard library are allowed.
// This constraint is enforced at compile time.
package shared

import (
	"context"
	"time"
)

// AggregateRoot is the aggregate root base class.
//
// An aggregate is DDD's consistency boundary: all changes inside an aggregate must complete
// within a single transaction; cross-aggregate consistency is guaranteed by domain events
// (see Event) rather than distributed transactions.
//
// Entities embedding this type hold their own domain event buffer; behavior methods only
// record events in memory, and the repository takes them out and publishes them at persistence
// time — this way "business state change" and "event publication" are naturally in the same
// transaction.
type AggregateRoot struct {
	events []Event
}

// RecordEvent records a domain event (called by entity behavior methods after a state change).
func (a *AggregateRoot) RecordEvent(e Event) { a.events = append(a.events, e) }

// PullEvents takes and clears the event buffer. Called by the repository on the transaction commit path.
func (a *AggregateRoot) PullEvents() []Event {
	if len(a.events) == 0 {
		return nil
	}
	out := make([]Event, len(a.events))
	copy(out, a.events)
	a.events = nil
	return out
}

// Events returns a read-only copy of the currently pending events (for test assertions; does not consume the buffer).
func (a *AggregateRoot) Events() []Event {
	if len(a.events) == 0 {
		return nil
	}
	out := make([]Event, len(a.events))
	copy(out, a.events)
	return out
}

// Version is the optimistic concurrency control version. Used only when the repository needs
// optimistic locking; the current implementation guarantees consistency with the tenant plugin
// + unique indexes, so this field is reserved for future evolution.
type Version int64

// AggregateID is the aggregate root identifier alias (int64 primary key, converted to a string by the API layer when sent as JSON).
type AggregateID = int64

// AuditInfo is audit trail info (value object).
//
// Corresponds to the creator/updater columns of existing tables. Extracted as a value object
// so the domain layer does not depend on the concrete context implementation of
// devops-common/auditctx and only accepts an already-resolved operator.
type AuditInfo struct {
	Creator string
	Updater string
}

// Actor is the current operator (domain input resolved from the authentication context).
type Actor struct {
	UserID   int64
	Username string
	TenantID int64
}

// NowFunc is the clock abstraction. The domain layer does not call time.Now() directly, making it easy to inject a deterministic clock in tests.
type NowFunc func() time.Time

// SystemNow is the default clock implementation.
func SystemNow() time.Time { return time.Now() }

// PageParam note: pagination input reuses devops-common/domain.PageParam.
//
// That type is already defined in the shared kernel and does not depend on GORM (standard
// library + struct tags only), and it is shared by gormx.FindPage and the API contract, so
// there is no need to redefine it in the domain layer. Each repository Filter embeds
// domain.PageParam directly.
//
// Supplementary note: devops-common/domain.PageParam carries query/form/doc tags, which only
// serve API-layer reflection (huma generating OpenAPI) and request binding. The domain and
// repository layers merely carry the type without parsing its tags, so this does not
// constitute a directional dependency from dao to the API contract (the real dependency
// direction is determined by package imports, not struct tags).

// TxManager is the transaction boundary port.
//
// Current pain point: the existing dao.WithTx keeps *gorm.DB entirely inside the dao package,
// so the service cannot orchestrate transactions across aggregate use cases and is forced to
// use manual compensation (e.g. manually HardDeleteTenant to roll back after a tenant
// creation failure). With this port extracted, the application layer can declare transaction
// boundaries, and repository implementations take the transaction handle from ctx internally,
// thus orchestrating across repositories without leaking *gorm.DB.
type TxManager interface {
	// WithinTx executes fn within a single transaction. Rolls back if fn returns an error.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
