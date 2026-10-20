// ID generation performance benchmarks (Go testing.B). Each of the three backpressure sources has its own benchmark:
//
//  1. g.mu serialization -- lock contention in BenchmarkNextParallel / BenchmarkBackpressure;
//  2. the 4096/ms sequence ceiling -- when the sequence is exhausted, nextLocked holds the lock waiting for the
//     next millisecond; this is the throughput plateau of BenchmarkNextSerial / BenchmarkBackpressure;
//  3. batchNext's long lock hold -- BenchmarkMixedBatchVsNext observes how batches squeeze single issuance.
//
// Run:
//
//	go test ./internal/domain/idgen -run '^$' -bench . -benchmem
//	go test ./internal/domain/idgen -run '^$' -bench Backpressure -benchtime 2s
//	go test ./internal/domain/idgen -run '^$' -bench . -cpu 1,4 -count 3
package idgen

import (
	"context"
	"fmt"
	"math/bits"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Latency histogram: 0..204.8us bucketed at 20ns (resolves the tens-of-nanoseconds hot path),
// with logarithmic buckets beyond that (covering the millisecond-scale tail latency of
// backpressure queuing, with acceptable error).
// No heap allocation throughout; even b.N in the tens of millions will not exhaust memory.
const (
	histTickNs         = 20    // exact bucket width (20ns)
	histPreciseBuckets = 10240 // exact range 10240*20ns = 204.8us
	histLogBuckets     = 32    // number of logarithmic buckets at or beyond the exact range
)

type latencyHist struct {
	fine  [histPreciseBuckets]atomic.Uint64
	logB  [histLogBuckets]atomic.Uint64
	count atomic.Uint64
	maxNs atomic.Int64
}

func (h *latencyHist) record(d time.Duration) {
	ns := d.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	h.count.Add(1)
	for {
		old := h.maxNs.Load()
		if ns <= old || h.maxNs.CompareAndSwap(old, ns) {
			break
		}
	}
	ticks := ns / histTickNs
	if ticks < histPreciseBuckets {
		h.fine[ticks].Add(1)
		return
	}
	idx := bits.Len64(uint64(ticks) / histPreciseBuckets)
	if idx >= histLogBuckets {
		idx = histLogBuckets - 1
	}
	h.logB[idx].Add(1)
}

// percentile returns the p-th percentile latency (0<p<=1). Exact buckets use the bucket midpoint; log buckets use the upper bound as an approximation.
func (h *latencyHist) percentile(p float64) time.Duration {
	total := h.count.Load()
	if total == 0 {
		return 0
	}
	want := uint64(float64(total) * p)
	if want == 0 {
		want = 1
	}
	var cum uint64
	for i := 0; i < histPreciseBuckets; i++ {
		cum += h.fine[i].Load()
		if cum >= want {
			return time.Duration(i*histTickNs + histTickNs/2)
		}
	}
	for i := 0; i < histLogBuckets; i++ {
		cum += h.logB[i].Load()
		if cum >= want {
			return time.Duration(histPreciseBuckets<<i) * histTickNs
		}
	}
	return time.Duration(histPreciseBuckets<<(histLogBuckets-1)) * histTickNs
}

// runFixedWorkers hammers with a fixed-concurrency closed loop: workers goroutines split b.N ops,
// timestamping before and after each call into the histogram. Fixed concurrency (rather than
// RunParallel's GOMAXPROCS) is what sweeps out the full "concurrency -> throughput/latency" backpressure curve.
func runFixedWorkers(b *testing.B, workers int, op func() error) *latencyHist {
	b.Helper()
	hist := &latencyHist{}
	var (
		errOnce sync.Once
		errBox  error
		failed  atomic.Bool
		wg      sync.WaitGroup
		start   = make(chan struct{})
	)
	fail := func(err error) {
		errOnce.Do(func() { errBox = err })
		failed.Store(true)
	}

	base := b.N / workers
	rem := b.N % workers
	for w := 0; w < workers; w++ {
		n := base
		if w < rem {
			n++
		}
		if n == 0 {
			continue
		}
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			for i := 0; i < n; i++ {
				if failed.Load() {
					return
				}
				startedAt := time.Now()
				if err := op(); err != nil {
					fail(err)
					return
				}
				hist.record(time.Since(startedAt))
			}
		}(n)
	}

	b.ResetTimer()
	close(start)
	wg.Wait()
	b.StopTimer()
	if errBox != nil {
		b.Fatalf("ID generation failed: %v", errBox)
	}
	return hist
}

// reportHist outputs the backpressure observation metrics: throughput (ids/s) and latency percentiles.
// Tail percentiles look at p999/max: sequence exhaustion happens about once every 4096 IDs (~0.02%),
// invisible at p99; p999 is the "hit the ceiling" signal.
// Must be called after StopTimer (runFixedWorkers already guarantees this).
func reportHist(b *testing.B, h *latencyHist) {
	b.Helper()
	if n := h.count.Load(); n > 0 {
		b.ReportMetric(float64(n)/b.Elapsed().Seconds(), "ids/s")
		b.ReportMetric(float64(h.percentile(0.50).Nanoseconds())/1000, "p50-us")
		b.ReportMetric(float64(h.percentile(0.99).Nanoseconds())/1000, "p99-us")
		b.ReportMetric(float64(h.percentile(0.999).Nanoseconds())/1000, "p999-us")
		b.ReportMetric(float64(h.maxNs.Load())/1000, "max-us")
	}
}

// BenchmarkNextSerial serial ID generation: real clock; throughput is pinned at roughly 4M ids/s
// by the 4096/ms sequence ceiling (the wait for the next millisecond is amortized into ns/op).
func BenchmarkNextSerial(b *testing.B) {
	gen := newTestGenerator(1)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := gen.next(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNextRawCPU pure hot-path cost: nowFn advances 1ms per call, so the sequence never
// runs out; measures the raw CPU cost of the mutex + timestamp comparison + bit operations
// (no waiting of any kind).
func BenchmarkNextRawCPU(b *testing.B) {
	gen := newTestGenerator(1)
	var fakeMs atomic.Int64
	fakeMs.Store(time.Now().UnixMilli())
	gen.nowFn = func() int64 { return fakeMs.Add(1) }
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := gen.next(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNextParallel parallel throughput under lock contention and the sequence ceiling (GOMAXPROCS goroutines).
// The ceiling is reached when throughput stops growing linearly with -cpu.
func BenchmarkNextParallel(b *testing.B) {
	gen := newTestGenerator(1)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := gen.next(ctx); err != nil {
				// FailNow cannot be called from a non-benchmark goroutine; record with Error and exit.
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkBackpressure main backpressure benchmark: sweeps concurrency, observing the throughput plateau and latency climb.
//
//   - ids/s stops growing after some concurrency = hit the sequence ceiling / lock bottleneck;
//   - p99-us keeps climbing with concurrency = requests are queuing on g.mu (backpressure propagating as latency).
func BenchmarkBackpressure(b *testing.B) {
	for _, workers := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers-%d", workers), func(b *testing.B) {
			gen := newTestGenerator(1)
			ctx := context.Background()
			hist := runFixedWorkers(b, workers, func() error {
				_, err := gen.next(ctx)
				return err
			})
			reportHist(b, hist)
		})
	}
}

// BenchmarkBatchNext batch amortization: takes count IDs under one lock acquisition.
// ns/op is the per-batch cost; ids/s is the throughput amortized to a single ID (the larger count is, the closer to the ceiling).
func BenchmarkBatchNext(b *testing.B) {
	for _, size := range []int{1, 10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			gen := newTestGenerator(1)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := gen.batchNext(ctx, size); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "ids/s")
		})
	}
}

// BenchmarkMixedBatchVsNext batch backpressure propagation: one goroutine continuously takes
// 10000-ID batches (batchNext holds the lock long, releasing it briefly only every 1024),
// while workers goroutines hammer Next.
// Compare p99 at the same concurrency against BenchmarkBackpressure; the difference is the
// cost of batches squeezing single issuance.
func BenchmarkMixedBatchVsNext(b *testing.B) {
	const batchSize = 10000
	for _, workers := range []int{4, 16} {
		b.Run(fmt.Sprintf("next-workers-%d", workers), func(b *testing.B) {
			gen := newTestGenerator(1)
			ctx := context.Background()
			stop := make(chan struct{})
			var batchWG sync.WaitGroup
			batchWG.Add(1)
			go func() {
				defer batchWG.Done()
				for {
					select {
					case <-stop:
						return
					default:
						if _, err := gen.batchNext(ctx, batchSize); err != nil {
							return
						}
					}
				}
			}()

			hist := runFixedWorkers(b, workers, func() error {
				_, err := gen.next(ctx)
				return err
			})
			close(stop)
			batchWG.Wait()
			reportHist(b, hist)
		})
	}
}
