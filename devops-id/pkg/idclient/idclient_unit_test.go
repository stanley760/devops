package idclient

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient creates a Client with a stubbed fetchFn (no gRPC).
func newTestClient(batch int, fetchFn func(ctx context.Context, bizTag string) ([]int64, error)) *Client {
	c := &Client{
		cfg:     BufferConfig{Batch: batch, LowWater: 0.2},
		buffers: make(map[string]*tagBuffer),
		fetchFn: fetchFn,
	}
	c.initMetrics()
	return c
}

func TestSingleFlightOneRPCPerTag(t *testing.T) {
	// P1-3: concurrent Next on an empty buffer issues only one RPC
	var calls atomic.Int32
	start := make(chan struct{})
	c := newTestClient(100, func(ctx context.Context, bizTag string) ([]int64, error) {
		calls.Add(1)
		<-start // returns only after all goroutines have arrived
		ids := make([]int64, 100)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		return ids, nil
	})

	const n = 10
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Next(context.Background(), "tag")
		}()
	}
	time.Sleep(50 * time.Millisecond) // wait for all goroutines to enter
	close(start)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1 (single-flight)", got)
	}
}

func TestSyncWaitsForInflightAsyncNoReorder(t *testing.T) {
	// P1-3: while an async refill is in flight, a sync refill must wait rather than start in parallel
	var calls atomic.Int32
	var mu sync.Mutex
	batches := [][]int64{}
	asyncStarted := make(chan struct{})
	asyncProceed := make(chan struct{})

	c := newTestClient(10, func(ctx context.Context, bizTag string) ([]int64, error) {
		n := calls.Add(1)
		if n == 1 {
			// first call: return a slow async batch [100..109]
			close(asyncStarted)
			<-asyncProceed
			ids := make([]int64, 10)
			for i := range ids {
				ids[i] = int64(100 + i)
			}
			mu.Lock()
			batches = append(batches, ids)
			mu.Unlock()
			return ids, nil
		}
		// second call (should not happen)
		ids := []int64{1, 2, 3}
		mu.Lock()
		batches = append(batches, ids)
		mu.Unlock()
		return ids, nil
	})

	// first take one ID to trigger the async refill
	c.cfg.Batch = 10
	tb := c.get("tag")
	// manually trigger asyncRefill (must be below the watermark: 1 < 10*0.2=2)
	tb.mu.Lock()
	tb.ids = []int64{50}
	c.asyncRefill(tb, "tag")
	tb.mu.Unlock()

	<-asyncStarted // wait for the async refill to start

	// now clear the buffer and call Next (sync path) -- it should wait rather than issue another request
	tb.mu.Lock()
	tb.ids = nil
	tb.mu.Unlock()

	// call Next before the async refill finishes -- it should wait for the in-flight request
	go func() {
		time.Sleep(20 * time.Millisecond)
		close(asyncProceed) // release the async refill
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	id, err := c.Next(ctx, "tag")
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	mu.Lock()
	totalCalls := len(batches)
	mu.Unlock()
	if totalCalls != 1 {
		t.Fatalf("total fetch calls = %d, want 1 (no reorder)", totalCalls)
	}
	// the ID should come from the async batch [100..109]
	if id < 100 || id > 109 {
		t.Fatalf("id = %d, want in [100..109]", id)
	}
}

func TestMergeRejectsNonMonotonicBatch(t *testing.T) {
	tb := &tagBuffer{}
	err := mergeLocked(tb, []int64{5, 3, 7})
	if err == nil {
		t.Fatal("non-monotonic batch should be rejected")
	}
}

func TestMergeRejectsEmptyBatch(t *testing.T) {
	tb := &tagBuffer{}
	err := mergeLocked(tb, []int64{})
	if err == nil {
		t.Fatal("empty batch should be rejected")
	}
}

func TestMergeRejectsStaleBatch(t *testing.T) {
	tb := &tagBuffer{lastIssued: 100}
	err := mergeLocked(tb, []int64{50, 60})
	if err == nil {
		t.Fatal("stale batch (first ID <= lastIssued) should be rejected")
	}
}

func TestMergeAcceptsValidBatch(t *testing.T) {
	tb := &tagBuffer{lastIssued: 10}
	err := mergeLocked(tb, []int64{11, 12, 13})
	if err != nil {
		t.Fatalf("valid batch rejected: %v", err)
	}
	if len(tb.ids) != 3 {
		t.Fatalf("buffer len = %d, want 3", len(tb.ids))
	}
}

func TestNextStrictlyIncreasing(t *testing.T) {
	// end-to-end: IDs issued by Next must be strictly increasing
	var next int64
	c := newTestClient(10, func(ctx context.Context, bizTag string) ([]int64, error) {
		base := next
		ids := make([]int64, 10)
		for i := range ids {
			ids[i] = base + int64(i) + 1
		}
		next = base + 10
		return ids, nil
	})

	var prev int64
	for i := 0; i < 50; i++ {
		id, err := c.Next(context.Background(), "tag")
		if err != nil {
			t.Fatalf("Next #%d error = %v", i, err)
		}
		if id <= prev {
			t.Fatalf("ID not increasing: %d <= %d (i=%d)", id, prev, i)
		}
		prev = id
	}
}

func TestWaiterRetriesAfterFailedFlight(t *testing.T) {
	// the first refill fails, the second retry succeeds
	var calls atomic.Int32
	c := newTestClient(5, func(ctx context.Context, bizTag string) ([]int64, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("transient error")
		}
		return []int64{1, 2, 3, 4, 5}, nil
	})

	tb := c.get("tag")

	// the first refill fails
	err := c.refill(context.Background(), tb, "tag", "sync")
	if err == nil {
		t.Fatal("the first refill should fail")
	}

	// the second refill should succeed
	err = c.refill(context.Background(), tb, "tag", "sync")
	if err != nil {
		t.Fatalf("second refill error = %v", err)
	}
	tb.mu.Lock()
	n := len(tb.ids)
	tb.mu.Unlock()
	if n != 5 {
		t.Fatalf("buffer len = %d, want 5", n)
	}
}
