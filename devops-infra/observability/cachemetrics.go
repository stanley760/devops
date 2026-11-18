package observability

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/stanley760/devops/devops-infra/cache"
)

var _ cache.CacheMetrics = (*CacheCollector)(nil)

// CacheCollector exports application-level cache hit and miss counts.
type CacheCollector struct {
	counter *prometheus.CounterVec
}

// NewCacheCollector registers the cache counter on the provided registerer.
func NewCacheCollector(reg prometheus.Registerer) (*CacheCollector, error) {
	if reg == nil {
		return nil, nil
	}
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "devops_cache_operations_total",
		Help: "Application-level cache operations by logical cache name and result (hit/miss).",
	}, []string{"cache", "result"})
	if err := reg.Register(counter); err != nil {
		return nil, err
	}
	return &CacheCollector{counter: counter}, nil
}

// RecordHit records a cache hit for the given logical cache.
func (c *CacheCollector) RecordHit(cacheName string) {
	if c == nil || c.counter == nil {
		return
	}
	c.counter.WithLabelValues(cacheName, "hit").Inc()
}

// RecordMiss records a cache miss for the given logical cache.
func (c *CacheCollector) RecordMiss(cacheName string) {
	if c == nil || c.counter == nil {
		return
	}
	c.counter.WithLabelValues(cacheName, "miss").Inc()
}
