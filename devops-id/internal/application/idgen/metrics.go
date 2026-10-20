package idgen

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
)

// MetricsObserver implements iddomain.Observer, recording domain events as OTel metrics.
type MetricsObserver struct {
	fallbackTotal  metric.Int64Counter
	leaseLostTotal metric.Int64Counter
	rollbackTotal  metric.Int64Counter
	rejectedTotal  metric.Int64Counter
	healthState    metric.Int64ObservableGauge

	mu     sync.Mutex
	states map[string]iddomain.HealthState
}

// NewMetricsObserver creates the metrics observer. OTel returns a noop (not nil) when metric creation fails.
func NewMetricsObserver() *MetricsObserver {
	meter := otel.Meter("devops-id/idgen")
	obs := &MetricsObserver{states: make(map[string]iddomain.HealthState)}
	var err error
	if obs.fallbackTotal, err = meter.Int64Counter(
		"idgen_fallback_total",
		metric.WithDescription("number of times local fallback mode was entered"),
	); err != nil {
		otel.Handle(err)
	}
	if obs.leaseLostTotal, err = meter.Int64Counter(
		"idgen_lease_lost_total",
		metric.WithDescription("number of etcd lease expiries"),
	); err != nil {
		otel.Handle(err)
	}
	if obs.rollbackTotal, err = meter.Int64Counter(
		"idgen_clock_rollback_total",
		metric.WithDescription("number of ID generations rejected due to clock rollback"),
	); err != nil {
		otel.Handle(err)
	}
	if obs.rejectedTotal, err = meter.Int64Counter(
		"idgen_requests_rejected_total",
		metric.WithDescription("number of ID generation requests rejected"),
	); err != nil {
		otel.Handle(err)
	}
	if obs.healthState, err = meter.Int64ObservableGauge(
		"idgen_health_state",
		metric.WithDescription("ID generation health state (0=serving, 1=degraded, 2=not_serving)"),
	); err != nil {
		otel.Handle(err)
	}

	if _, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		obs.mu.Lock()
		defer obs.mu.Unlock()
		for tag, st := range obs.states {
			attrs := metric.WithAttributes(attribute.String("biz_tag", tag))
			o.ObserveInt64(obs.healthState, int64(st), attrs)
		}
		return nil
	}, obs.healthState); err != nil {
		otel.Handle(err)
	}
	return obs
}

func (o *MetricsObserver) OnFallbackActive(bizTag string) {
	if o == nil || o.fallbackTotal == nil {
		return
	}
	o.fallbackTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("biz_tag", bizTag)))
}

func (o *MetricsObserver) OnLeaseLost(bizTag string) {
	if o == nil || o.leaseLostTotal == nil {
		return
	}
	o.leaseLostTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("biz_tag", bizTag)))
}

func (o *MetricsObserver) OnClockRollback(bizTag string, _ int64) {
	if o == nil || o.rollbackTotal == nil {
		return
	}
	o.rollbackTotal.Add(context.Background(), 1, metric.WithAttributes(attribute.String("biz_tag", bizTag)))
}

func (o *MetricsObserver) OnState(bizTag string, state iddomain.HealthState) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.states[bizTag] = state
	o.mu.Unlock()
}
