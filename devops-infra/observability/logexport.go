package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otlploghttp "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// OTLP log export: slog records are bridged to the OTel log API and pushed to
// Loki's native OTLP endpoint (/otlp/v1/logs). Loki turns the Resource's
// service.name into the index label service_name (the "grouped by service.name"
// view in Logs Drilldown) and keeps trace_id/span_id as structured metadata, so
// a log line can jump to its trace in Tempo.
//
// Fail-open, matching the trace side: an exporter-creation failure is only
// logged and log export is skipped — file/console logging keeps working.

// slogSeverity maps a slog level onto an OTel severity number (the numeric part
// is what Loki stores as severity_number; the text form is set separately).
func slogSeverity(l slog.Level) (log.Severity, string) {
	switch {
	case l >= slog.LevelError:
		return log.SeverityError, "ERROR"
	case l >= slog.LevelWarn:
		return log.SeverityWarn, "WARN"
	case l >= slog.LevelInfo:
		return log.SeverityInfo, "INFO"
	default:
		return log.SeverityDebug, "DEBUG"
	}
}

// attrValue flattens a slog attr value into an OTel attribute value. slog.KindAny
// values are stringified (Loki stringifies non-primitives anyway); errors keep
// their message.
func attrValue(v slog.Value) attribute.Value {
	switch v.Kind() {
	case slog.KindString:
		return attribute.StringValue(v.String())
	case slog.KindInt64:
		return attribute.Int64Value(v.Int64())
	case slog.KindUint64:
		return attribute.Int64Value(int64(v.Uint64()))
	case slog.KindFloat64:
		return attribute.Float64Value(v.Float64())
	case slog.KindBool:
		return attribute.BoolValue(v.Bool())
	case slog.KindDuration:
		return attribute.StringValue(v.Duration().String())
	case slog.KindTime:
		return attribute.StringValue(v.Time().Format(time.RFC3339Nano))
	default:
		return attribute.StringValue(fmt.Sprintf("%v", v.Any()))
	}
}

// OTelLogHandler is a slog.Handler that forwards every enabled record to the
// OTel log SDK. Wrap it with TraceHandler (outer) so trace_id/span_id land on
// the record both as attributes (file/console output) and as native OTel fields
// (Loki structured metadata ↔ Tempo correlation).
type OTelLogHandler struct {
	Handler  slog.Handler // next handler in the chain (file/console); nil = terminal
	logger   log.Logger
	provider *sdklog.LoggerProvider
}

var _ slog.Handler = (*OTelLogHandler)(nil)

// otlpLogsPath derives the OTLP logs signal path from the configured endpoint.
//
// OTel convention (OTEL_EXPORTER_OTLP_LOGS_ENDPOINT): the endpoint is a base
// URL and the signal path is appended to it. A pathless base yields the
// standard /v1/logs; a base carrying a path yields that path + /v1/logs — which
// is what Loki needs, since its OTLP ingestion endpoint is /otlp/v1/logs
// (not /v1/logs). An endpoint that already ends in /v1/logs is kept as-is.
func otlpLogsPath(rawPath string) string {
	p := strings.TrimSuffix(rawPath, "/")
	switch {
	case p == "":
		return "/v1/logs"
	case strings.HasSuffix(p, "/v1/logs"):
		return p
	default:
		return p + "/v1/logs"
	}
}

// NewOTelLogHandler builds the OTLP log bridge. endpoint is the OTLP HTTP base
// URL (e.g. "http://loki:3100/otlp" or "http://collector:4318"); the logs
// signal path is derived from it via otlpLogsPath.
//
// WithEndpointURL sets the request path verbatim from the URL, so the signal
// path must be applied explicitly (WithURLPath) — the exporter otherwise posts
// to "/" and the backend 404s, silently dropping the logs.
func NewOTelLogHandler(ctx context.Context, endpoint string, res *resource.Resource) (*OTelLogHandler, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("observability: empty OTLP log endpoint")
	}
	opts := []otlploghttp.Option{otlploghttp.WithEndpointURL(endpoint)}
	logPath := ""
	if u, perr := url.Parse(endpoint); perr == nil {
		logPath = otlpLogsPath(u.Path)
	}
	if logPath != "" {
		opts = append(opts, otlploghttp.WithURLPath(logPath))
	}
	exp, err := otlploghttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("observability: failed to create the OTLP log exporter: %w", err)
	}
	provider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp,
			sdklog.WithMaxQueueSize(4096),
			sdklog.WithExportInterval(3*time.Second),
			sdklog.WithExportTimeout(5*time.Second),
		)),
	)
	return &OTelLogHandler{
		logger:   provider.Logger("github.com/stanley760/devops/devops-infra/observability"),
		provider: provider,
	}, nil
}

// Enabled implements slog.Handler: delegate to the next handler when present
// (file/console decide visibility), otherwise export everything at info+.
func (h *OTelLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.Handler != nil {
		return h.Handler.Enabled(ctx, level)
	}
	return level >= slog.LevelInfo
}

// Handle implements slog.Handler: convert the record and emit it to OTel, then
// pass through to the wrapped handler (file/console output is unchanged).
func (h *OTelLogHandler) Handle(ctx context.Context, r slog.Record) error {
	var rec log.Record
	rec.SetTimestamp(r.Time)
	rec.SetObservedTimestamp(r.Time)
	sev, sevText := slogSeverity(r.Level)
	rec.SetSeverity(sev)
	rec.SetSeverityText(sevText)
	rec.SetBody(attribute.StringValue(r.Message))

	// trace_id/span_id as attributes: the OTel SDK lifts the ctx span into the
	// record's native TraceID/SpanID fields automatically (logger.newRecord),
	// which the OTLP exporter maps to log_record.trace_id — Loki stores them as
	// structured metadata for Tempo correlation. The explicit attributes keep
	// the IDs visible in plain-text log lines too.
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttributes(
			attribute.String("trace_id", sc.TraceID().String()),
			attribute.String("span_id", sc.SpanID().String()),
		)
	}

	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "" {
			return true
		}
		rec.AddAttributes(attribute.KeyValue{Key: attribute.Key(a.Key), Value: attrValue(a.Value)})
		return true
	})

	if h.logger != nil {
		h.logger.Emit(ctx, rec)
	}

	if h.Handler != nil {
		return h.Handler.Handle(ctx, r)
	}
	return nil
}

// WithAttrs implements slog.Handler.
func (h *OTelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := h.Handler
	if next != nil {
		next = next.WithAttrs(attrs)
	}
	return &OTelLogHandler{Handler: next, logger: h.logger}
}

// WithGroup implements slog.Handler. Groups are flattened with dots (OTel has no
// group concept; Loki flattens nested attributes with underscores anyway).
func (h *OTelLogHandler) WithGroup(name string) slog.Handler {
	next := h.Handler
	if next != nil {
		next = next.WithGroup(name)
	}
	return &OTelLogHandler{Handler: next, logger: h.logger}
}

// ShutdownLogExport flushes in-flight log records; call before process exit.
func (h *OTelLogHandler) ShutdownLogExport(ctx context.Context) error {
	if h == nil || h.provider == nil {
		return nil
	}
	return h.provider.Shutdown(ctx)
}
