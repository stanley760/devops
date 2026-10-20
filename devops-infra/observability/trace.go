package observability

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDFromContext returns the trace ID string of the current span in ctx.
//
// Side facilities such as audit records and log correlation use it to cross-reference
// the trace system. It returns an empty string when there is no valid span (trace not
// assembled, ctx is nil, or the span is invalid); callers must treat an empty string
// as "no trace information" rather than an error.
func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// SpanIDFromContext returns the span ID string of the current span in ctx;
// an empty string when there is no valid span.
func SpanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasSpanID() {
		return ""
	}
	return sc.SpanID().String()
}

// HasTrace reports whether ctx contains a valid trace.
func HasTrace(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	return trace.SpanContextFromContext(ctx).IsValid()
}
