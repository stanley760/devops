package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Middleware returns the gin entrypoint trace instrumentation:
// extract the upstream (Higress/frontend) W3C traceparent → create a server span →
// record route attributes at the end.
// Mounted at the very front of the business middleware chain, so the time spent in
// JWT/Tenant/Perm and the handler all falls within the same server span.
func Middleware(serviceName string, opts ...otelgin.Option) gin.HandlerFunc {
	return otelgin.Middleware(serviceName, opts...)
}

// Middleware returns the current Runtime's gin entrypoint trace instrumentation.
// It stays a pass-through when the Runtime is closed or metrics are disabled, so
// observability failures never affect business requests.
func (rt *Runtime) Middleware(opts ...otelgin.Option) gin.HandlerFunc {
	if rt == nil || rt.serviceName == "" {
		return func(c *gin.Context) { c.Next() }
	}
	return otelgin.Middleware(rt.serviceName, opts...)
}

// RED metric definitions (output via the Prometheus exporter):
//
//	http_server_requests_total{method,route,status}     count
//	http_server_request_duration_seconds{method,route}  duration histogram
//	http_server_in_flight                               in-flight requests
//
// Label cardinality control: route uses the gin route template (/users/:id) and the
// raw path never becomes a label; status is a bounded HTTP status code. Never put
// high-cardinality values such as uid/tid into metric labels.
func (rt *Runtime) initHTTPMetrics() error {
	if rt == nil || rt.meterProvider == nil {
		return nil
	}
	meter := rt.meterProvider.Meter("devops-infra/observability")

	httpRequests, err := meter.Int64Counter("http_server_requests",
		metric.WithDescription("total HTTP requests"))
	if err != nil {
		return err
	}
	httpDuration, err := meter.Float64Histogram("http_server_request_duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"))
	if err != nil {
		return err
	}
	httpInFlight, err := meter.Int64UpDownCounter("http_server_in_flight",
		metric.WithDescription("in-flight requests"))
	if err != nil {
		return err
	}

	rt.httpRequests = httpRequests
	rt.httpDuration = httpDuration
	rt.httpInFlight = httpInFlight
	return nil
}

// MetricsMiddleware RED instrumentation: mounted after the trace middleware.
// When metrics assembly fails (an abnormal MeterProvider) it silently degrades to a
// pass-through, without affecting business requests.
func (rt *Runtime) MetricsMiddleware() gin.HandlerFunc {
	if rt == nil || rt.httpRequests == nil || rt.httpDuration == nil || rt.httpInFlight == nil {
		return func(c *gin.Context) { c.Next() }
	}
	httpRequests := rt.httpRequests
	httpDuration := rt.httpDuration
	httpInFlight := rt.httpInFlight

	return func(c *gin.Context) {
		httpInFlight.Add(c.Request.Context(), 1)
		defer httpInFlight.Add(c.Request.Context(), -1)
		startedAt := time.Now()
		c.Next()

		attrs := metric.WithAttributes(
			attribute.String("method", c.Request.Method),
			attribute.String("route", routeTemplate(c)),
			attribute.Int("status", c.Writer.Status()),
		)
		httpRequests.Add(c.Request.Context(), 1, attrs)
		httpDuration.Record(c.Request.Context(), time.Since(startedAt).Seconds(), attrs)
	}
}

// routeTemplate returns the route template; unmatched routes (404) collapse to
// "unmatched" to prevent high cardinality.
func routeTemplate(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return path
	}
	return "unmatched"
}
