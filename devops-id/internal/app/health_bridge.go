package app

import (
	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"sync"
)

// HealthBridge syncs domain health state to the gRPC health server.
//
// Mapping strategy (readiness / liveness separation):
//   - "" (default) and "idgen.IdGen": SERVING only when Serving; Degraded/NotServing -> NOT_SERVING
//     -- the readiness probe removes traffic
//   - "idgen.IdGen.live": SERVING when Serving+Degraded; NOT_SERVING only when NotServing
//     -- the liveness probe, preventing degraded Pods from being restarted repeatedly
type HealthBridge struct {
	mu    sync.Mutex
	hc    *health.Server
	state iddomain.HealthState
}

// NewHealthBridge creates a health bridge. Before Bind, events only record state and do not reach gRPC.
func NewHealthBridge() *HealthBridge {
	return &HealthBridge{state: iddomain.HealthServing}
}

// Bind binds the gRPC health server and immediately applies the current state.
func (b *HealthBridge) Bind(hc *health.Server) {
	b.mu.Lock()
	b.hc = hc
	state := b.state
	b.mu.Unlock()
	b.apply(state)
}

// OnState implements the iddomain.Observer state callback.
func (b *HealthBridge) OnState(_ string, state iddomain.HealthState) {
	b.mu.Lock()
	// The overall state takes the worst value (tag="" means overall).
	if state > b.state {
		b.state = state
	}
	hc := b.hc
	cur := b.state
	b.mu.Unlock()
	if hc != nil {
		b.apply(cur)
	}
}

// OnFallbackActive implements iddomain.Observer (local fallback triggers NOT_SERVING to remove traffic).
func (b *HealthBridge) OnFallbackActive(_ string) {
	b.OnState("", iddomain.HealthNotServing)
}

// OnLeaseLost implements iddomain.Observer (lease expiry -> NOT_SERVING).
func (b *HealthBridge) OnLeaseLost(_ string) {
	b.OnState("", iddomain.HealthNotServing)
}

// OnClockRollback implements iddomain.Observer (clock rollback is transient and does not change health state).
func (b *HealthBridge) OnClockRollback(_ string, _ int64) {}

// apply writes the state to the three health service names.
func (b *HealthBridge) apply(state iddomain.HealthState) {
	b.mu.Lock()
	hc := b.hc
	b.mu.Unlock()
	if hc == nil {
		return
	}
	// readiness: SERVING only when fully healthy
	readiness := healthpb.HealthCheckResponse_SERVING
	if state != iddomain.HealthServing {
		readiness = healthpb.HealthCheckResponse_NOT_SERVING
	}
	// liveness: degraded still counts as alive; only NotServing is unhealthy
	liveness := healthpb.HealthCheckResponse_SERVING
	if state == iddomain.HealthNotServing {
		liveness = healthpb.HealthCheckResponse_NOT_SERVING
	}
	hc.SetServingStatus("", readiness)
	hc.SetServingStatus("idgen.IdGen", readiness)
	hc.SetServingStatus("idgen.IdGen.live", liveness)
}

// fanoutObserver combines multiple Observers into one.
type fanoutObserver struct {
	observers []iddomain.Observer
}

// newFanoutObserver creates a combined observer.
func newFanoutObserver(observers ...iddomain.Observer) iddomain.Observer {
	return &fanoutObserver{observers: observers}
}

func (f *fanoutObserver) OnFallbackActive(bizTag string) {
	for _, o := range f.observers {
		o.OnFallbackActive(bizTag)
	}
}

func (f *fanoutObserver) OnLeaseLost(bizTag string) {
	for _, o := range f.observers {
		o.OnLeaseLost(bizTag)
	}
}

func (f *fanoutObserver) OnClockRollback(bizTag string, rollbackMs int64) {
	for _, o := range f.observers {
		o.OnClockRollback(bizTag, rollbackMs)
	}
}

func (f *fanoutObserver) OnState(bizTag string, state iddomain.HealthState) {
	for _, o := range f.observers {
		o.OnState(bizTag, state)
	}
}
