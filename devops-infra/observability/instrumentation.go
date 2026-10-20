package observability

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus/collectors"
	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/semconv/v1.26.0"
	"gorm.io/gorm"
	otelform "gorm.io/plugin/opentelemetry/tracing"
)

// InstrumentGORM installs the OTel plugin for GORM, generating database client spans;
// when metrics are enabled it also registers the database/sql connection pool collector
// (the USE perspective, the go_sql_* metric family).
// database is written to the db.name attribute and the db_name label; query variables
// never enter spans, avoiding leakage of business data.
func (rt *Runtime) InstrumentGORM(db *gorm.DB, database string) error {
	if db == nil {
		return errors.New("observability: GORM DB is not initialized")
	}

	pluginOptions := []otelform.Option{
		otelform.WithDBSystem("mysql"),
		otelform.WithAttributes(semconv.DBNamespaceKey.String(database)),
		otelform.WithoutQueryVariables(),
	}
	if rt != nil && rt.tracerProvider != nil {
		pluginOptions = append(pluginOptions, otelform.WithTracerProvider(rt.tracerProvider))
	}
	if err := db.Use(otelform.NewPlugin(pluginOptions...)); err != nil {
		return fmt.Errorf("observability: failed to install the GORM OTel plugin: %w", err)
	}

	// Connection pool metrics (USE perspective): go_sql_open_connections / in_use / idle /
	// wait_count_total / wait_duration_seconds_total, with db_name as a label.
	// Skipped when metrics are disabled (registry is nil), consistent with the fail-open
	// behavior on the trace side; failing to obtain *sql.DB or registering twice only
	// warns, without blocking business operations.
	if rt == nil || rt.registry == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Warn("observability: failed to obtain the underlying *sql.DB, connection pool metrics not assembled", "err", err)
		return nil
	}
	if err := rt.registry.Register(collectors.NewDBStatsCollector(sqlDB, database)); err != nil {
		slog.Warn("observability: failed to register the SQL connection pool metrics", "db_name", database, "err", err)
	}
	return nil
}

// InstrumentRedis installs trace and connection pool metric hooks for go-redis.
// Raw Redis commands never enter spans, avoiding leakage of sensitive information in keys/values.
func (rt *Runtime) InstrumentRedis(client goredis.UniversalClient) error {
	if client == nil {
		return errors.New("observability: Redis client is not initialized")
	}

	tracingOptions := []redisotel.TracingOption{
		redisotel.WithDBStatement(false),
		redisotel.WithCallerEnabled(false),
	}
	if rt != nil && rt.tracerProvider != nil {
		tracingOptions = append(tracingOptions, redisotel.WithTracerProvider(rt.tracerProvider))
	}
	if err := redisotel.InstrumentTracing(client, tracingOptions...); err != nil {
		return fmt.Errorf("observability: failed to install Redis OTel tracing: %w", err)
	}

	metricsOptions := []redisotel.MetricsOption{}
	if rt != nil && rt.meterProvider != nil {
		metricsOptions = append(metricsOptions, redisotel.WithMeterProvider(rt.meterProvider))
	}
	if err := redisotel.InstrumentMetrics(client, metricsOptions...); err != nil {
		return fmt.Errorf("observability: failed to install Redis OTel metrics: %w", err)
	}
	return nil
}
