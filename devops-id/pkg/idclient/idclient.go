// Package idclient is the client SDK (prefetch buffer) for the devops-id ID generation service.
//
// Key design: amortize "one RPC per ID" down to 1/batch -- the process holds a local
// prefetch buffer, and one BatchNext fetches a batch (100 by default) for zero-network
// local issuance:
//   - a devops-id restart does not affect already-prefetched IDs (they were atomically allocated to this process; the resilience window = batch)
//   - when the remaining count drops below the watermark -> async single-flight refill
//   - buffer exhausted and refill failed -> fail-fast error (same philosophy as the server: refusing is preferable to duplicates)
//
// Refills use per-tag single-flight: synchronous and asynchronous refills of the same
// biz_tag share one in-flight request, eliminating local ID regressions caused by
// batch reordering.
//
// Multiple consumer replicas each hold independent buffers: within the same biz_tag IDs
// trend upward, but they interleave across replicas.
package idclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/stanley760/devops/devops-id/pkg/idgenpb"
)

// BufferConfig is the prefetch buffer config.
type BufferConfig struct {
	Batch       int           // prefetch batch size (1 = prefetching off, one RPC per ID)
	LowWater    float64       // async refill when the remaining ratio drops below this value
	CallTimeout time.Duration // per-call timeout for fetch (0 = no limit, follows the passed ctx)
}

// Client is the ID generation client: one gRPC connection + per-biz_tag local prefetch buffers.
type Client struct {
	conn   *grpc.ClientConn
	stub   idgenpb.IdGenClient
	health healthpb.HealthClient
	cfg    BufferConfig

	mu      sync.Mutex
	buffers map[string]*tagBuffer

	metrics *bufferMetrics

	// fetchFn is the test seam for BatchNext; defaults to stub call.
	fetchFn func(ctx context.Context, bizTag string) ([]int64, error)
}

// inflightFetch tracks a single in-flight refill request per tag.
type inflightFetch struct {
	done chan struct{}
}

// tagBuffer is the buffer state of a single biz_tag.
// attrs is the pre-built biz_tag attribute set: zero attribute allocation on the ID generation hot path.
type tagBuffer struct {
	mu         sync.Mutex
	ids        []int64
	inflight   *inflightFetch // single-flight: at most one refill request at a time
	lastIssued int64          // most recently issued ID, used for enqueue validation
	attrs      metric.MeasurementOption
}

// bufferMetrics is the ID generation buffer metric set. When instrument creation fails OTel returns a noop instrument (not nil),
// so observability failures do not affect ID generation.
type bufferMetrics struct {
	remaining  metric.Int64ObservableGauge
	capacity   metric.Int64ObservableGauge
	issued     metric.Int64Counter
	refillFail metric.Int64Counter
	refillDur  metric.Float64Histogram
}

// New creates the ID generation client. conn is created and lifecycled by the caller (devops-infra/grpc.Dial).
// Metrics are exported via the global MeterProvider: the service side should finish observability.Init
// before calling New; when uninitialized (probes/tests) it falls back to noop instruments with zero overhead side effects.
func New(conn *grpc.ClientConn, cfg BufferConfig) *Client {
	if cfg.Batch < 1 {
		cfg.Batch = 1
	}
	if cfg.LowWater <= 0 {
		cfg.LowWater = 0.2
	}
	c := &Client{
		conn:    conn,
		stub:    idgenpb.NewIdGenClient(conn),
		health:  healthpb.NewHealthClient(conn),
		cfg:     cfg,
		buffers: make(map[string]*tagBuffer),
	}
	c.fetchFn = c.fetchFromStub
	c.initMetrics()
	return c
}

// initMetrics creates the buffer metrics and registers the scrape-time callback (watermark gauges need no hot-path writes).
// The callback holds the lock to read each buffer's remaining count: when Next synchronously refills an empty buffer it
// blocks for at most CallTimeout, so a scrape may briefly wait on the lock -- that magnitude (seconds) is far below
// the Prometheus scrape timeout and is acceptable.
func (c *Client) initMetrics() {
	meter := otel.Meter("devops-id/idclient")
	m := &bufferMetrics{}
	var err error
	if m.remaining, err = meter.Int64ObservableGauge(
		"idgen_buffer_remaining",
		metric.WithDescription("remaining IDs in the local prefetch buffer (0 = exhausted; the next ID triggers a synchronous RPC)"),
	); err != nil {
		otel.Handle(err)
	}
	if m.capacity, err = meter.Int64ObservableGauge(
		"idgen_buffer_capacity",
		metric.WithDescription("prefetch batch capacity (BufferConfig.Batch)"),
	); err != nil {
		otel.Handle(err)
	}
	if m.issued, err = meter.Int64Counter(
		"idgen_issued",
		metric.WithDescription("cumulative count of IDs issued from the local buffer"),
	); err != nil {
		otel.Handle(err)
	}
	if m.refillFail, err = meter.Int64Counter(
		"idgen_refill_failures",
		metric.WithDescription("buffer refill failure count; trigger=sync means a business request was already blocked, async means a background prefetch failed"),
	); err != nil {
		otel.Handle(err)
	}
	if m.refillDur, err = meter.Float64Histogram(
		"idgen_refill_duration",
		metric.WithUnit("s"),
		metric.WithDescription("BatchNext refill duration"),
	); err != nil {
		otel.Handle(err)
	}
	c.metrics = m

	if _, err = meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		type entry struct {
			tag string
			tb  *tagBuffer
		}
		c.mu.Lock()
		entries := make([]entry, 0, len(c.buffers))
		for tag, tb := range c.buffers {
			entries = append(entries, entry{tag, tb})
		}
		c.mu.Unlock()
		for _, e := range entries {
			e.tb.mu.Lock()
			n := len(e.tb.ids)
			e.tb.mu.Unlock()
			attrs := metric.WithAttributes(attribute.String("biz_tag", e.tag))
			o.ObserveInt64(m.remaining, int64(n), attrs)
			o.ObserveInt64(m.capacity, int64(c.cfg.Batch), attrs)
		}
		return nil
	}, m.remaining, m.capacity); err != nil {
		otel.Handle(err)
	}
}

// Next takes one ID from the local prefetch buffer.
// bizTag selects an independent snowflake worker space.
func (c *Client) Next(ctx context.Context, bizTag string) (int64, error) {
	tb := c.get(bizTag)
	for {
		tb.mu.Lock()
		if len(tb.ids) > 0 {
			// pop from the head: the server returns monotonically increasing batches, so head-popping keeps local issuance increasing too
			id := tb.ids[0]
			tb.ids = tb.ids[1:]
			tb.lastIssued = id
			c.metrics.recordIssued(ctx, tb.attrs)
			c.asyncRefill(tb, bizTag)
			tb.mu.Unlock()
			return id, nil
		}
		tb.mu.Unlock()

		// buffer empty: synchronous refill (single-flight, never runs in parallel with async refill to cause reordering)
		if err := c.refill(ctx, tb, bizTag, "sync"); err != nil {
			return 0, fmt.Errorf("ID generation failed (biz_tag=%s): %w", bizTag, err)
		}
		// after refill completes, loop to re-check the buffer (it may have been consumed concurrently)
	}
}

// get fetches (or lazily creates) the biz_tag buffer.
func (c *Client) get(bizTag string) *tagBuffer {
	c.mu.Lock()
	defer c.mu.Unlock()
	tb, ok := c.buffers[bizTag]
	if !ok {
		tb = &tagBuffer{
			attrs: metric.WithAttributes(attribute.String("biz_tag", bizTag)),
		}
		c.buffers[bizTag] = tb
	}
	return tb
}

// refill single-flight refill: synchronous and asynchronous refills of the same biz_tag share one in-flight request.
// When one is in flight, wait for it to finish; otherwise start a new request. fetch does not hold tb.mu.
func (c *Client) refill(ctx context.Context, tb *tagBuffer, bizTag, trigger string) error {
	tb.mu.Lock()
	if tb.inflight != nil {
		// a refill is already in progress; wait for it to finish (do not start a second request in parallel)
		done := tb.inflight.done
		tb.mu.Unlock()
		select {
		case <-done:
			return nil // the first caller finished; the caller re-checks the buffer
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	call := &inflightFetch{done: make(chan struct{})}
	tb.inflight = call
	tb.mu.Unlock()

	startedAt := time.Now()
	ids, err := c.fetchFn(ctx, bizTag)
	if c.metrics != nil {
		c.metrics.recordRefill(ctx, bizTag, trigger, time.Since(startedAt), err != nil)
	}

	tb.mu.Lock()
	if err == nil {
		if mergeErr := mergeLocked(tb, ids); mergeErr != nil {
			err = mergeErr
			if c.metrics != nil {
				c.metrics.recordRefill(ctx, bizTag, "validate", 0, true)
			}
		}
	}
	if tb.inflight == call {
		tb.inflight = nil
	}
	tb.mu.Unlock()
	close(call.done)
	return err
}

// mergeLocked validates and merges a refill batch. tb.mu must be held.
// Validation rules: non-empty, strictly increasing, and the first ID greater than both the most recently issued ID and the buffer's tail ID.
func mergeLocked(tb *tagBuffer, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("empty batch from server")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			return fmt.Errorf("non-monotonic batch: ids[%d]=%d <= ids[%d]=%d", i, ids[i], i-1, ids[i-1])
		}
	}
	if ids[0] <= tb.lastIssued {
		return fmt.Errorf("stale batch: first ID %d <= last issued %d", ids[0], tb.lastIssued)
	}
	if n := len(tb.ids); n > 0 && ids[0] <= tb.ids[n-1] {
		return fmt.Errorf("stale batch: first ID %d <= buffer tail %d", ids[0], tb.ids[n-1])
	}
	tb.ids = append(tb.ids, ids...)
	return nil
}

// asyncRefill refills asynchronously when the remaining count drops below the watermark (the caller holds tb.mu).
func (c *Client) asyncRefill(tb *tagBuffer, bizTag string) {
	if c.cfg.Batch <= 1 { // prefetching off
		return
	}
	if len(tb.ids) >= int(float64(c.cfg.Batch)*c.cfg.LowWater) {
		return
	}
	if tb.inflight != nil {
		return // a refill is already in progress
	}
	// mark in-flight to prevent duplicate triggering before the goroutine starts
	call := &inflightFetch{done: make(chan struct{})}
	tb.inflight = call
	go func() {
		startedAt := time.Now()
		ids, err := c.fetchFn(context.Background(), bizTag)
		if c.metrics != nil {
			c.metrics.recordRefill(context.Background(), bizTag, "async", time.Since(startedAt), err != nil)
		}
		tb.mu.Lock()
		if err == nil {
			if mergeErr := mergeLocked(tb, ids); mergeErr != nil {
				if c.metrics != nil {
					c.metrics.recordRefill(context.Background(), bizTag, "validate", 0, true)
				}
			}
		}
		if tb.inflight == call {
			tb.inflight = nil
		}
		tb.mu.Unlock()
		close(call.done)
	}()
}

// fetchFromStub calls BatchNext to fetch a batch of IDs (bounded by CallTimeout).
func (c *Client) fetchFromStub(ctx context.Context, bizTag string) ([]int64, error) {
	if c.cfg.CallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.CallTimeout)
		defer cancel()
	}
	reply, err := c.stub.BatchNext(ctx, &idgenpb.BatchNextRequest{
		BizTag: bizTag,
		Count:  int32(c.cfg.Batch),
	})
	if err != nil {
		return nil, err
	}
	ids := reply.GetIds()
	// server response validation: non-empty + strictly increasing
	if len(ids) == 0 {
		return nil, fmt.Errorf("server returned empty batch")
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			return nil, fmt.Errorf("server returned non-monotonic batch")
		}
	}
	return ids, nil
}

// Ping liveness check: the standard gRPC health check (SERVING status).
func (c *Client) Ping(ctx context.Context) error {
	reply, err := c.health.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if reply.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("devops-id health status abnormal: %s", reply.GetStatus())
	}
	return nil
}

// recordIssued hot path: uses tagBuffer's pre-built attribute set to record the ID count with zero allocation.
func (m *bufferMetrics) recordIssued(ctx context.Context, attrs metric.MeasurementOption) {
	if m == nil {
		return
	}
	m.issued.Add(ctx, 1, attrs)
}

// recordRefill cold path (only on refill): failure count + duration histogram.
func (m *bufferMetrics) recordRefill(ctx context.Context, bizTag, trigger string, d time.Duration, failed bool) {
	if m == nil {
		return
	}
	attrs := metric.WithAttributes(
		attribute.String("biz_tag", bizTag),
		attribute.String("trigger", trigger),
	)
	m.refillDur.Record(ctx, d.Seconds(), attrs)
	if failed {
		m.refillFail.Add(ctx, 1, attrs)
	}
}
