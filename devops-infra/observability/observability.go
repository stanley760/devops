// Package observability assembles OpenTelemetry observability in one place:
// trace (OTLP export) + metrics (Prometheus exporter) + log trace correlation.
//
// Assembly point: each service calls Init after logging.Init and before any request
// handling, and calls Runtime.Shutdown before process exit to flush in-flight data
// (matching the cleanup semantics of logging.Close).
//
// Instrument once, consume twice: business code only sees the OTel API;
// traces go to Jaeger/Tempo via OTLP HTTP, metrics are scraped by Prometheus via
// the Prometheus exporter at /metrics — no double instrumentation required.
package observability

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config configures observability assembly.
type Config struct {
	ServiceName    string  // service name; should match the nacos registration name (cross-service trace and Grafana filtering depend on it)
	Env            string  // deployment.environment resource attribute (dev/test/prod)
	OTLPEndpoint   string  // OTLP HTTP endpoint, e.g. "http://jaeger:4318"; empty = no trace exporter
	SamplerRatio   float64 // root span sampling ratio; 1.0 recommended in dev, 0.1 in prod; <=0 means 1.0
	MetricsEnabled bool    // whether to assemble the Prometheus exporter (/metrics)
}

// Runtime is the assembly product: when MetricsHandler is non-nil it is mounted on the public route /metrics.
type Runtime struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	// registry is the self-built registry behind /metrics; nil when metrics are disabled.
	// USE metrics (e.g. the SQL connection pool) are registered through it, output alongside the OTel metrics.
	registry       *prometheus.Registry
	MetricsHandler http.Handler

	serviceName  string
	httpRequests metric.Int64Counter
	httpDuration metric.Float64Histogram
	httpInFlight metric.Int64UpDownCounter
}

// Init assembles the global OTel (TracerProvider / MeterProvider / W3C propagators)
// and correlates logs with traces.
//
// Fail-open design: observability failures must not block business startup — a failed
// OTLP exporter creation is only logged and the trace export skipped (local spans are
// still created, just not reported); only a failed Resource build is treated as an assembly error.
func Init(ctx context.Context, cfg Config) (*Runtime, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcessRuntimeVersion(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(envOrUnknown(cfg.Env)),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: failed to build resource: %w", err)
	}

	rt := &Runtime{}
	rt.serviceName = cfg.ServiceName
	traceOpts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}

	if cfg.OTLPEndpoint != "" {
		otlpOpts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.OTLPEndpoint)}
		// WithEndpointURL normalizes a URL with no path to "/", and does not append the
		// default signal path /v1/traces (otlptracehttp options.go:106-109) — sending to /
		// gets a 404 from Jaeger and traces are silently dropped.
		// Only backfill it when the business config does not carry a path explicitly;
		// Insecure is derived automatically from the scheme (http→plaintext).
		if u, perr := url.Parse(cfg.OTLPEndpoint); perr == nil && u.Path == "" {
			otlpOpts = append(otlpOpts, otlptracehttp.WithURLPath("/v1/traces"))
		}
		exp, err := otlptracehttp.New(ctx, otlpOpts...)
		if err != nil {
			// an unavailable exporter must not block startup: log it and skip the export (local spans still work)
			slog.Warn("observability: failed to create the OTLP exporter, trace export disabled", "err", err, "endpoint", cfg.OTLPEndpoint)
		} else {
			traceOpts = append(traceOpts,
				sdktrace.WithBatcher(exp,
					sdktrace.WithMaxQueueSize(8192),
					sdktrace.WithBatchTimeout(5*time.Second),
				),
				sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplerRatio(cfg.SamplerRatio)))),
			)
		}
	}
	rt.tracerProvider = sdktrace.NewTracerProvider(traceOpts...)

	if cfg.MetricsEnabled {
		// custom registry: OTel metrics + Go runtime/process metrics (GC, goroutines, memory)
		// behind a single /metrics endpoint, so Grafana dashboards can reference runtime curves directly
		registry := prometheus.NewRegistry()
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)
		rt.registry = registry
		exp, err := promexporter.New(
			promexporter.WithRegisterer(registry),
			// Promote service.name / deployment.environment from the OTel Resource to
			// constant labels so Grafana dashboards can filter/aggregate by service_name;
			// the remaining resource attributes (process/runtime versions etc.) stay out
			// of labels to control series cardinality.
			promexporter.WithResourceAsConstantLabels(func(kv attribute.KeyValue) bool {
				return kv.Key == semconv.ServiceNameKey || kv.Key == semconv.DeploymentEnvironmentKey
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("observability: failed to create the Prometheus exporter: %w", err)
		}
		rt.meterProvider = sdkmetric.NewMeterProvider(
			sdkmetric.WithResource(res),
			sdkmetric.WithReader(exp),
		)
		rt.MetricsHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
		if err := rt.initHTTPMetrics(); err != nil {
			_ = rt.meterProvider.Shutdown(ctx)
			return nil, fmt.Errorf("observability: failed to initialize HTTP metrics: %w", err)
		}
	}

	otel.SetTracerProvider(rt.tracerProvider)
	if rt.meterProvider != nil {
		otel.SetMeterProvider(rt.meterProvider)
	}
	// W3C tracecontext + baggage: aligned with the Higress gateway, the frontend, and cross-service gRPC/HTTP propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Log trace correlation: wrap the current default handler so existing slog calls
	// automatically carry trace_id/span_id.
	// Self-recursion guard: SetDefault bridges the log package's default output back to
	// slog.Default(); if the current default handler is still the built-in defaultHandler
	// (writing to log.Default()), TraceHandler → log.Default() → TraceHandler forms a
	// loop and the first log on the request path self-deadlocks on the log mutex. When
	// log's default writer is still os.Stderr it means SetDefault was never called
	// (logging.Init did not run first, typically in unit tests), so substitute an
	// equivalent handler that writes straight to stderr; in the production path
	// logging.Init's file/console handlers write directly to the writer and do not go
	// through log.Default(), so they are wrapped as-is.
	base := slog.Default().Handler()
	if log.Default().Writer() == os.Stderr {
		base = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(TraceHandler{Handler: base}))

	return rt, nil
}

// Shutdown flushes in-flight trace/metrics before releasing; call before process exit (with a timeout ctx).
func (rt *Runtime) Shutdown(ctx context.Context) error {
	if rt == nil {
		return nil
	}
	var errs []error
	if rt.meterProvider != nil {
		if err := rt.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider: %w", err))
		}
	}
	if rt.tracerProvider != nil {
		if err := rt.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider: %w", err))
		}
	}
	return errors.Join(errs...)
}

// ShutdownWithTimeout flushes in-flight data with a bounded timeout, avoiding a
// blocked process exit when an external backend is unavailable.
func (rt *Runtime) ShutdownWithTimeout(parent context.Context, timeout time.Duration) error {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return rt.Shutdown(ctx)
}

func envOrUnknown(env string) string {
	if env == "" {
		return "unknown"
	}
	return env
}

func samplerRatio(ratio float64) float64 {
	if ratio <= 0 {
		return 1.0
	}
	if ratio > 1 {
		return 1.0
	}
	return ratio
}
