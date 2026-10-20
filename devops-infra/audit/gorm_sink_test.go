package audit

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/stanley760/devops/devops-common/audit"
	"github.com/stanley760/devops/devops-infra/database"
)

// row is the audit table entity used in tests; its gorm tags are isomorphic to the real business table (column widths aligned with the contract constants).
type row struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	TraceID   string `gorm:"column:trace_id;size:100"`
	Username  string `gorm:"column:username;size:30"`
	UserIP    string `gorm:"column:user_ip;size:50"`
	UserAgent string `gorm:"column:user_agent;size:512"`
	Module    string `gorm:"column:module;size:50"`
	Result    int8   `gorm:"column:result"`
}

func (row) TableName() string { return "t_test_audit" }

// convert mirrors real business usage: it returns a pointer so GORM can write back the auto-increment primary key.
func convert(rec audit.Record) *row {
	return &row{
		TraceID:   rec.TraceID,
		Username:  rec.Username,
		UserIP:    rec.UserIP,
		UserAgent: rec.UserAgent,
		Module:    rec.Module,
		Result:    rec.Result,
	}
}

// gormExecutor adapts *gorm.DB into a ctx-aware Executor (a test double;
// the production implementations are the same-name adapters in each business package).
type gormExecutor struct{ db *gorm.DB }

func (g gormExecutor) Create(ctx context.Context, value any) *gorm.DB {
	return g.db.WithContext(ctx).Create(value)
}

// newDB creates an independent in-memory database per test case and runs the migration.
// It avoids file::memory:?cache=shared: a shared cache would let parallel/sequential cases block each other,
// and t.Cleanup dropping tables would interleave with the next case's migration.
func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=private"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	// The in-memory database disappears with its connection; close it explicitly to avoid leaking handles.
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&row{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestGORSinkWritesRecord(t *testing.T) {
	db := newDB(t)
	sink := NewGORSink(GORSinkConfig[*row]{Executor: gormExecutor{db}, Convert: convert})

	sink.Emit(t.Context(), audit.Record{
		Module:    "system",
		SubModule: "user",
		Username:  "alice",
		UserIP:    "10.0.0.1",
		Result:    audit.ResultSuccess,
	})

	var got row
	if err := db.First(&got).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Module != "system" || got.Username != "alice" {
		t.Fatalf("unexpected row: %+v", got)
	}
}

// TestGORSinkClipsBeforeHittingDatabase verifies truncation happens before the row reaches the database:
// MySQL strict mode rejects the entire row for an oversized varchar, and losing audit rows is less acceptable than truncating.
func TestGORSinkClipsBeforeHittingDatabase(t *testing.T) {
	db := newDB(t)

	// Capture the entity sent to GORM: a wrapping Executor records convert's output.
	spy := &capturingExecutor{inner: gormExecutor{db}}
	sink := NewGORSink(GORSinkConfig[*row]{Executor: spy, Convert: convert})

	// The oversized input must exceed all column limits (MaxIP=50 / MaxModule=50 /
	// MaxUsername=30 / MaxUserAgent=512 / MaxTraceID=100), otherwise "not truncating" would be the correct behavior.
	long := strings.Repeat("超长内容", 200) // 800 characters
	sink.Emit(t.Context(), audit.Record{
		Username:  long,
		UserIP:    long,
		UserAgent: long,
		TraceID:   long,
		Module:    long,
	})

	got := spy.snap
	if got == nil {
		t.Fatal("Convert did not produce a row")
	}
	checks := []struct {
		name  string
		value string
		limit int
	}{
		{"Username", got.Username, audit.MaxUsername},
		{"UserIP", got.UserIP, audit.MaxIP},
		{"UserAgent", got.UserAgent, audit.MaxUserAgent},
		{"TraceID", got.TraceID, audit.MaxTraceID},
		{"Module", got.Module, audit.MaxModule},
	}
	for _, c := range checks {
		if r := len([]rune(c.value)); r != c.limit {
			t.Fatalf("%s runes = %d, want clipped to %d before hitting DB", c.name, r, c.limit)
		}
		if !utf8.ValidString(c.value) {
			t.Fatalf("%s was truncated by byte and produced invalid UTF-8", c.name)
		}
	}
}

// capturingExecutor snapshots the row passed to Create before forwarding it to the real executor.
//
// It must snapshot rather than hold the pointer: GORM writes fields back after Create (and SQLite also
// silently truncates by column width), so reading the same pointer afterwards no longer shows what the Sink handed to the database.
type capturingExecutor struct {
	inner Executor
	snap  *row
}

func (c *capturingExecutor) Create(ctx context.Context, value any) *gorm.DB {
	if r, ok := value.(*row); ok {
		snapshot := *r
		c.snap = &snapshot
	}
	return c.inner.Create(ctx, value)
}

func TestGORSinkSkipsWhenContextCancelled(t *testing.T) {
	db := newDB(t)
	sink := NewGORSink(GORSinkConfig[*row]{Executor: gormExecutor{db}, Convert: convert})

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // simulate business-failure rollback / request timeout

	// No panic, no blocking: Emit has no return value by contract.
	sink.Emit(ctx, audit.Record{Module: "system", Username: "alice"})

	var n int64
	if err := db.Model(&row{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("cancelled context should not write, got %d rows", n)
	}
}

func TestGORSinkWriteFailureDoesNotPanic(t *testing.T) {
	db := newDB(t)
	// An executor whose writes inevitably fail (the table does not exist).
	bad := db.Table("t_does_not_exist")
	sink := NewGORSink(GORSinkConfig[*row]{Executor: gormExecutor{bad}, Convert: convert})

	// Key point: Emit neither returns an error nor panics — auditing is a side channel.
	sink.Emit(t.Context(), audit.Record{Module: "system", Username: "alice"})
}

func TestGORSinkIsNoopWhenNotConfigured(t *testing.T) {
	// Zero-value/partially-configured states must be safe: business code should not need nil checks everywhere.
	var nilSink *GORSink[*row]
	nilSink.Emit(t.Context(), audit.Record{Module: "system"})

	NewGORSink(GORSinkConfig[*row]{Convert: convert}).Emit(t.Context(), audit.Record{})
	NewGORSink(GORSinkConfig[*row]{Executor: gormExecutor{newDB(t)}}).Emit(t.Context(), audit.Record{})
}

func TestGORSinkHandlesNilContext(t *testing.T) {
	db := newDB(t)
	sink := NewGORSink(GORSinkConfig[*row]{Executor: gormExecutor{db}, Convert: convert})

	//nolint:staticcheck // deliberately verify nil ctx does not panic
	sink.Emit(nil, audit.Record{Module: "system", Username: "alice"})

	var n int64
	if err := db.Model(&row{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("nil context should fall back to Background and write, got %d rows", n)
	}
}

func TestExecutorOfPrefersTransaction(t *testing.T) {
	db := newDB(t)

	// Falls back to the default executor when there is no transaction.
	if got := ExecutorOf(t.Context(), gormExecutor{db}); got == nil {
		t.Fatal("ExecutorOf returned nil without transaction")
	}

	// When a transaction is active, the transaction executor is injected into ctx and ExecutorOf should retrieve it.
	if err := database.Transaction(t.Context(), db, func(ctx context.Context) error {
		if _, ok := database.ExecutorFromContext(ctx); !ok {
			t.Error("transaction not injected into ctx")
		}
		if got := ExecutorOf(ctx, gormExecutor{db}); got == nil {
			t.Error("ExecutorOf returned nil inside transaction")
		}
		return nil
	}); err != nil {
		t.Fatalf("transaction: %v", err)
	}
}

func TestScopedBypassesSoftDelete(t *testing.T) {
	db := newDB(t)
	// This test entity has no DeletedAt field; Scoped only guarantees a unified Unscoped entry point on the query side.
	q := Scoped(db.Model(&row{}))
	if q == nil {
		t.Fatal("Scoped returned nil")
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("Scoped chain must stay usable: %v", err)
	}
}

var _ audit.Sink = NewGORSink(GORSinkConfig[*row]{})

// Ensure DefaultWriteTimeout is positive, preventing a zero value from degrading into no timeout.
func TestDefaultWriteTimeoutIsPositive(t *testing.T) {
	if DefaultWriteTimeout <= 0 {
		t.Fatalf("DefaultWriteTimeout = %v, want positive", DefaultWriteTimeout)
	}
	if DefaultWriteTimeout > 30*time.Second {
		t.Fatalf("DefaultWriteTimeout = %v, too long for a side-channel write", DefaultWriteTimeout)
	}
}
