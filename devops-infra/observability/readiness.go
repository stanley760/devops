package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ReadinessCollector exports the dependency-readiness state of the service.
//
// Why this exists: the /readyz endpoint reports whether the service's
// dependencies (MySQL / Redis / devops-id …) are reachable, and the orchestrator
// consumes that to remove the instance from traffic. Prometheus' `up` metric
// cannot express it — `up == 1` only means "the process still answers on
// /metrics", which stays true while every dependency is down. This collector
// closes that blind spot so Grafana/Alertmanager can alert on the same
// condition the orchestrator acts on.
//
// The values are set by the /readyz handler: every probe refreshes all
// dependencies, so the gauge always reflects the latest probe result.
type ReadinessCollector struct {
	gauge *prometheus.GaugeVec
}

// NewReadinessCollector registers a readiness gauge on the given registry.
//
// The metric is named devops_readiness with a "dependency" label: 1 means the
// dependency is reachable, 0 means it is not.
func NewReadinessCollector(registry *prometheus.Registry) (*ReadinessCollector, error) {
	if registry == nil {
		return nil, nil
	}
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "devops_readiness",
		Help: "Dependency readiness of the service: 1 = reachable, 0 = unreachable (the same condition the orchestrator's readiness probe acts on).",
	}, []string{"dependency"})
	if err := registry.Register(gauge); err != nil {
		return nil, err
	}
	return &ReadinessCollector{gauge: gauge}, nil
}

// Set records the readiness of a single dependency.
func (c *ReadinessCollector) Set(dependency string, ready bool) {
	if c == nil || c.gauge == nil {
		return
	}
	v := 0.0
	if ready {
		v = 1.0
	}
	c.gauge.WithLabelValues(dependency).Set(v)
}

// Reset clears all dependency states; used when the service has no
// dependencies or before the first probe so a stale value is never reported.
func (c *ReadinessCollector) Reset() {
	if c == nil || c.gauge == nil {
		return
	}
	c.gauge.Reset()
}
