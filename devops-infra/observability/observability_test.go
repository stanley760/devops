package observability

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestInitDefaults(t *testing.T) {
	rt, err := Init(context.Background(), Config{
		ServiceName: "test-svc",
		Env:         "dev",
		// OTLPEndpoint left empty: no exporter, local spans still work
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	if rt.MetricsHandler == nil {
		t.Fatal("MetricsHandler should be non-nil when MetricsEnabled")
	}

	// the /metrics endpoint should output Go runtime metrics
	rec := httptest.NewRecorder()
	rt.MetricsHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "go_goroutines") {
		t.Fatal("metrics should expose go runtime metrics")
	}

	// Shutdown is idempotent-safe
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown() error = %v", err)
	}
}

func TestMetricsCarryServiceNameLabel(t *testing.T) {
	rt, err := Init(context.Background(), Config{
		ServiceName:    "label-svc",
		Env:            "dev",
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	rt.readiness.Set("mysql", true)

	rec := httptest.NewRecorder()
	rt.MetricsHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	for _, want := range []string{
		`go_goroutines{deployment_environment="dev",service_name="label-svc"}`,
		`devops_readiness{dependency="mysql",deployment_environment="dev",service_name="label-svc"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in /metrics:\n%s", want, firstLines(body, 5))
		}
	}
}

func TestCacheMetricsCollectorExportsHitAndMiss(t *testing.T) {
	rt, err := Init(context.Background(), Config{
		ServiceName:    "cache-svc",
		Env:            "test",
		MetricsEnabled: true,
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	metrics := rt.CacheMetrics()
	if metrics == nil {
		t.Fatal("CacheMetrics() should be non-nil when metrics are enabled")
	}
	metrics.RecordHit("tenant")
	metrics.RecordMiss("tenant")

	rec := httptest.NewRecorder()
	rt.MetricsHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	for _, want := range []string{
		`devops_cache_operations_total{cache="tenant",deployment_environment="test",result="hit",service_name="cache-svc"} 1`,
		`devops_cache_operations_total{cache="tenant",deployment_environment="test",result="miss",service_name="cache-svc"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in /metrics:\n%s", want, firstLines(body, 5))
		}
	}
}

func TestCacheMetricsDisabledWhenMetricsDisabled(t *testing.T) {
	rt, err := Init(context.Background(), Config{ServiceName: "no-metrics"})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	if metrics := rt.CacheMetrics(); metrics != nil {
		t.Fatalf("CacheMetrics() = %v, want nil when metrics are disabled", metrics)
	}
}

func TestSamplerRatioClamping(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 1.0}, {-1, 1.0}, {0.1, 0.1}, {1.5, 1.0},
	}
	for _, c := range cases {
		if got := samplerRatio(c.in); got != c.want {
			t.Fatalf("samplerRatio(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestTraceHandlerAttachesTraceIDs(t *testing.T) {
	var buf strings.Builder
	inner := slog.NewTextHandler(&buf, nil)
	handler := TraceHandler{Handler: inner}
	logger := slog.New(handler)

	// no span context: trace_id is not injected
	logger.InfoContext(context.Background(), "no-span")
	if strings.Contains(buf.String(), "trace_id") {
		t.Fatalf("unexpected trace_id without span: %s", buf.String())
	}

	// with a span context: trace_id/span_id are injected
	tp := sdktrace.NewTracerProvider()
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(old)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	logger.InfoContext(ctx, "with-span")
	span.End()

	out := buf.String()
	if !strings.Contains(out, "trace_id="+span.SpanContext().TraceID().String()) {
		t.Fatalf("log missing trace_id: %s", out)
	}
	if !strings.Contains(out, "span_id="+span.SpanContext().SpanID().String()) {
		t.Fatalf("log missing span_id: %s", out)
	}
}

func TestMetricsMiddlewareRecordsAndPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rt, err := Init(context.Background(), Config{ServiceName: "t", Env: "dev", MetricsEnabled: true})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer func() { _ = rt.Shutdown(context.Background()) }()

	router := gin.New()
	router.Use(rt.MetricsMiddleware())
	router.GET("/users/:id", func(c *gin.Context) { c.Status(http.StatusTeapot) })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("handler passthrough broken: %d", rec.Code)
	}

	// metrics are output via the exporter and can be scraped (the counter exists and carries the route template label)
	mrec := httptest.NewRecorder()
	rt.MetricsHandler.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := mrec.Body.String()
	if !strings.Contains(body, "http_server_requests_total") ||
		!strings.Contains(body, `route="/users/:id"`) ||
		!strings.Contains(body, `status="418"`) {
		t.Fatalf("metric with route template label missing:\n%s", firstLines(body, 5))
	}
	if !strings.Contains(body, "devops_http_request_duration_seconds_bucket") {
		t.Fatal("in-house HTTP duration histogram should use the devops_http_request_duration name")
	}
	if strings.Contains(body, "http_server_request_duration_seconds_bucket") {
		t.Fatal("in-house HTTP duration histogram should not share otelgin's metric name")
	}
	// Resource constant labels: service_name / deployment_environment are output with
	// every metric; Grafana dashboards filtering by service_name depend on these labels
	if !strings.Contains(body, `service_name="t"`) || !strings.Contains(body, `deployment_environment="dev"`) {
		t.Fatalf("resource constant labels missing:\n%s", firstLines(body, 5))
	}

	// unmatched routes collapse to unmatched (prevents high cardinality)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	mrec = httptest.NewRecorder()
	rt.MetricsHandler.ServeHTTP(mrec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(mrec.Body.String(), `route="unmatched"`) {
		t.Fatal("unmatched route should collapse to 'unmatched' label")
	}

	_ = time.Second
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
