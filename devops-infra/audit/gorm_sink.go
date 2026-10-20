// Package audit provides a GORM local-persistence implementation for operation auditing.
//
// This package is a Sink implementation of devops-common/audit: the contract (what to
// record, column widths, user type enums) lives entirely in the common layer; this
// package only maps the contract onto business tables and writes.
//
// Auditing is a side-channel capability; this package strictly follows three constraints:
//  1. Write failures are only logged via slog.Warn and never block the business main path (Emit returns no error);
//  2. Truncate to the contract column widths before writing, preventing oversized fields from getting the whole row rejected under MySQL strict mode;
//  3. Audit rows are never soft-deleted — deletes on the query side use Unscoped physical deletes to avoid deleted_at rows bloating the table.
package audit

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-infra/database"
)

// Executor is the database executor required for audit writes.
//
// Accepting ctx is required: in multi-tenant scenarios connections must resolve
// the tenant from ctx (and may also reuse a transaction already started in
// ctx). *gorm.DB cannot satisfy this directly; implementors wrap it with a small adapter.
type Executor interface {
	Create(ctx context.Context, value any) *gorm.DB
}

// GORSink is a Sink implementation that writes audit records into a local business database.
//
// The generic parameter T is the audit table entity type, supplied by the business side via the convert mapping function,
// so that infra does not need to know the concrete column names and gorm tags of t_system_operation_log.
type GORSink[T any] struct {
	executor Executor
	convert  func(audit.Record) T

	// timeout caps the duration of a single audit write.
	// Auditing is a side channel; slow database queries must not pile up goroutines.
	timeout time.Duration
}

// GORSinkConfig holds assembly parameters.
type GORSinkConfig[T any] struct {
	// Executor database executor; required.
	Executor Executor
	// Convert maps Record → entity; required.
	//
	// It must return a pointer (e.g. func(rec audit.Record) *model.OperationLog),
	// otherwise GORM cannot write back the primary key and Create panics.
	Convert func(audit.Record) T
	// Timeout per-write timeout; 0 → DefaultWriteTimeout.
	Timeout time.Duration
}

// DefaultWriteTimeout is the default timeout for a single audit write.
const DefaultWriteTimeout = 3 * time.Second

// NewGORSink creates a local-persistence Sink.
func NewGORSink[T any](cfg GORSinkConfig[T]) *GORSink[T] {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultWriteTimeout
	}
	return &GORSink[T]{
		executor: cfg.Executor,
		convert:  cfg.Convert,
		timeout:  timeout,
	}
}

// Emit implements audit.Sink.
//
// It returns no error: audit failures are only logged and never affect the business main path.
func (s *GORSink[T]) Emit(ctx context.Context, rec audit.Record) {
	if s == nil || s.executor == nil || s.convert == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// Skip the write when the context is already cancelled (business failure, timeout, or transaction rollback):
	// landing in a doomed transaction is pointless and only slows the failure path.
	if err := ctx.Err(); err != nil {
		slog.WarnContext(ctx, "audit skipped: context cancelled",
			"module", rec.Module, "subModule", rec.SubModule, "err", err)
		return
	}

	rec = rec.Normalize()
	row := s.convert(rec)

	writeCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if err := s.executor.Create(writeCtx, row).Error; err != nil {
		slog.WarnContext(ctx, "audit persistence failed",
			"module", rec.Module,
			"subModule", rec.SubModule,
			"url", rec.RequestURL,
			"err", err,
		)
		return
	}
	slog.DebugContext(ctx, "audit recorded",
		"module", rec.Module,
		"subModule", rec.SubModule,
		"bizID", rec.BizID,
		"result", rec.Result,
	)
}

// ExecutorOf retrieves the transaction executor from ctx, falling back to the given default executor.
//
// When the audit table and business tables live in the same database, preferring the business transaction avoids "business succeeded but audit lost";
// when ctx carries no transaction, write via the default connection pool (audit tolerates this weak consistency).
func ExecutorOf(ctx context.Context, fallback Executor) Executor {
	if tx, ok := database.ExecutorFromContext(ctx); ok && tx != nil {
		return txExecutor{tx: tx}
	}
	return fallback
}

// txExecutor adapts *gorm.DB into a ctx-aware Executor (it ignores the incoming ctx,
// because the transaction handle is already bound to an outer ctx).
type txExecutor struct{ tx *gorm.DB }

func (t txExecutor) Create(_ context.Context, value any) *gorm.DB {
	return t.tx.Create(value)
}

// Scoped is the unified scope for audit queries: Unscoped bypasses soft deletes.
//
// Audit rows have no soft-delete semantics; queries and deletes should both see all historical rows.
// Business query sides reuse this entry point to avoid missing Unscoped in scattered places.
func Scoped(db *gorm.DB) *gorm.DB { return db.Unscoped() }

// Compile-time assertion: GORSink must satisfy the common-layer contract.
var _ audit.Sink = (*GORSink[any])(nil)
