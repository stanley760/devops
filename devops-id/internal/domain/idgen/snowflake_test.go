// Unit tests for the ID generation domain: bit layout, monotonicity, and clock rollback protection (no etcd needed).
// etcd slot claiming / lease release are runtime acceptance items (dual-instance startup has no slot conflicts; reclaimable within 10s after kill).
package idgen

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestGenerator(workerID int64) *tagGenerator {
	return &tagGenerator{
		workerID:      workerID,
		lastTimestamp: time.Now().UnixMilli(),
		nowFn:         func() int64 { return time.Now().UnixMilli() },
		obs:           noopObserver{},
		cfg: Config{
			WatermarkSafetyMargin:    5 * time.Second,
			WatermarkPersistInterval: 3 * time.Second,
			WatermarkMaxWait:         30 * time.Second,
		},
	}
}

func TestNextBitLayoutAndMonotonic(t *testing.T) {
	const worker = int64(777)
	generator := newTestGenerator(worker)

	var prev int64
	for i := 0; i < 1000; i++ {
		id, err := generator.next(context.Background())
		if err != nil {
			t.Fatalf("ID generation %d failed: %v", i, err)
		}
		if id <= prev {
			t.Fatalf("ID not increasing: %d <= %d", id, prev)
		}
		if got := (id >> seqBits) & maxWorker; got != worker {
			t.Fatalf("worker bits wrong: got %d want %d", got, worker)
		}
		if got := id & maxSeq; got < 0 || got > maxSeq {
			t.Fatalf("sequence bits out of range: %d", got)
		}
		if ms := (id >> (workerBits + seqBits)) + epoch; ms > time.Now().UnixMilli() {
			t.Fatalf("timestamp bits ahead of current time: %d", ms)
		}
		prev = id
	}
}

func TestBatchNextStrictlyIncreasing(t *testing.T) {
	manager, err := NewManager(Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := manager.BatchNext(context.Background(), "any", 500)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Fatalf("batch IDs not strictly increasing: ids[%d]=%d <= ids[%d]=%d", i, ids[i], i-1, ids[i-1])
		}
	}
}

func TestClockBackwardSmallSpin(t *testing.T) {
	generator := newTestGenerator(1)
	generator.lastTimestamp = time.Now().UnixMilli() + 3 // roll back 3ms (< 5ms threshold)
	if _, err := generator.next(context.Background()); err != nil {
		t.Fatalf("small rollback should spin-wait to catch up, got %v", err)
	}
}

func TestClockBackwardLargeReject(t *testing.T) {
	generator := newTestGenerator(1)
	generator.lastTimestamp = time.Now().UnixMilli() + 100 // roll back 100ms (>= 5ms threshold)
	if _, err := generator.next(context.Background()); !errors.Is(err, ErrClockBackward) {
		t.Fatalf("large rollback should reject ID generation, got %v", err)
	}
}

func TestLeaseLostReject(t *testing.T) {
	// after lease expiry (the slot may have been taken by another instance) ID generation must be cut off
	generator := newTestGenerator(1)
	generator.leaseLost.Store(true)
	if _, err := generator.next(context.Background()); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("lease loss should reject ID generation, got %v", err)
	}
}
