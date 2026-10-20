package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceHandler wraps slog.Handler, automatically attaching the current span's
// trace_id / span_id to every log record (output unchanged when ctx has no valid span).
//
// Precondition: callers (middleware/handler/service) pass c.Request.Context() or a
// request-derived ctx as slog's first argument — the existing code in this project
// already satisfies this. After log collection, records can be cross-referenced
// with Jaeger/Tempo by trace_id.
type TraceHandler struct {
	Handler slog.Handler
}

// Enabled implements slog.Handler.
func (h TraceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h TraceHandler) WithGroup(name string) slog.Handler {
	return TraceHandler{Handler: h.Handler.WithGroup(name)}
}

var _ slog.Handler = TraceHandler{}
