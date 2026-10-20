package idgen

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stanley760/devops/devops-id/internal/infrastructure/workerfile"
)

type fakeLeaseLifecycle struct {
	grantErr     error
	releaseErr   error
	lost         chan struct{}
	grantCalled  bool
	releaseCalls int
}

func (f *fakeLeaseLifecycle) Grant(context.Context) error {
	f.grantCalled = true
	return f.grantErr
}

func (f *fakeLeaseLifecycle) Lost() <-chan struct{} {
	return f.lost
}

func (f *fakeLeaseLifecycle) Release(context.Context) error {
	f.releaseCalls++
	return f.releaseErr
}

type fakeSlotCoordinator struct {
	acquireSlot  int
	acquireErr   error
	refreshErr   error
	acquireCalls int
	refreshCalls int
	prefixes     []string
}

func (f *fakeSlotCoordinator) Acquire(_ context.Context, keyPrefix string, _ string, _ int) (int, error) {
	f.acquireCalls++
	f.prefixes = append(f.prefixes, keyPrefix)
	return f.acquireSlot, f.acquireErr
}

func (f *fakeSlotCoordinator) Refresh(context.Context, string, string, int64, int64) error {
	f.refreshCalls++
	return f.refreshErr
}

func workerOf(id int64) int64 {
	return (id >> seqBits) & maxWorker
}

func TestManagerAcquiresSlotAndCachesWorker(t *testing.T) {
	cache := t.TempDir()
	lease := &fakeLeaseLifecycle{lost: make(chan struct{})}
	slots := &fakeSlotCoordinator{acquireSlot: 7}

	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: slots, Cache: workerfile.New(cache, "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	id, err := manager.Next(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if workerOf(id) != 7 || !lease.grantCalled || slots.acquireCalls != 1 {
		t.Fatalf("worker=%d grant=%v acquire=%d", workerOf(id), lease.grantCalled, slots.acquireCalls)
	}
	if len(slots.prefixes) != 1 || slots.prefixes[0] != "idgen/snowflake/leaf/user/" {
		t.Fatalf("slot prefixes = %v", slots.prefixes)
	}
	data, err := os.ReadFile(filepath.Join(cache, "leaf", "user", "worker_id"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != strconv.Itoa(7) {
		t.Fatalf("worker cache = %q, want 7", data)
	}
}

func TestManagerUsesIndependentWorkerSpacePerBizTag(t *testing.T) {
	cache := t.TempDir()
	var leases []*fakeLeaseLifecycle
	var slots []*fakeSlotCoordinator
	call := 0

	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		call++
		lease := &fakeLeaseLifecycle{lost: make(chan struct{})}
		coordinator := &fakeSlotCoordinator{acquireSlot: 6 + call}
		leases = append(leases, lease)
		slots = append(slots, coordinator)
		return TagInfra{Lease: lease, Slots: coordinator, Cache: workerfile.New(cache, "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	userID, err := manager.Next(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	roleID, err := manager.Next(context.Background(), "role")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Next(context.Background(), "user"); err != nil {
		t.Fatal(err)
	}

	if workerOf(userID) != 7 || workerOf(roleID) != 8 {
		t.Fatalf("workers = (%d, %d), want (7, 8)", workerOf(userID), workerOf(roleID))
	}
	wantPrefixes := []string{
		"idgen/snowflake/leaf/user/",
		"idgen/snowflake/leaf/role/",
	}
	if len(slots) != len(wantPrefixes) {
		t.Fatalf("generator count = %d, want %d", len(slots), len(wantPrefixes))
	}
	for i, want := range wantPrefixes {
		if slots[i].acquireCalls != 1 || slots[i].prefixes[0] != want {
			t.Fatalf("slot[%d] calls=%d prefixes=%v, want %q", i, slots[i].acquireCalls, slots[i].prefixes, want)
		}
	}

	userCache := filepath.Join(cache, "leaf", "user", "worker_id")
	roleCache := filepath.Join(cache, "leaf", "role", "worker_id")
	for path, want := range map[string]string{userCache: "7", roleCache: "8"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("worker cache %s = %q, want %q", path, data, want)
		}
	}

	manager.Close()
	for i, lease := range leases {
		if lease.releaseCalls != 1 {
			t.Fatalf("lease[%d] release calls = %d, want 1", i, lease.releaseCalls)
		}
	}
}

func TestManagerRejectsInvalidBizTag(t *testing.T) {
	manager, err := NewManager(Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	if _, err := manager.Next(context.Background(), "user/role"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("Next() error = %v, want ErrInvalidName", err)
	}
}

func TestManagerClosedRejectsNewRequests(t *testing.T) {
	manager, err := NewManager(Config{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager.Close()

	if _, err := manager.Next(context.Background(), "user"); !errors.Is(err, ErrManagerClosed) {
		t.Fatalf("Next() error = %v, want ErrManagerClosed", err)
	}
}

func TestManagerUsesLocalModeSlot(t *testing.T) {
	// Local mode (nil Lease/Slots = no coordination infrastructure): use the cached slot
	cache := t.TempDir()
	workerFile := filepath.Join(cache, "leaf", "user", "worker_id")
	if err := os.MkdirAll(filepath.Dir(workerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workerFile, []byte("5"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		return TagInfra{Cache: workerfile.New(cache, "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	id, err := manager.Next(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if workerOf(id) != 5 {
		t.Fatalf("local mode worker = %d, want 5", workerOf(id))
	}
}

func TestNoFallbackWithoutOptIn(t *testing.T) {
	// P0-1 fail-closed: etcd unavailable and AllowLocalFallback not enabled -> refuse ID generation
	lease := &fakeLeaseLifecycle{lost: make(chan struct{}), grantErr: errors.New("etcd down")}
	slots := &fakeSlotCoordinator{}

	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: slots, Cache: workerfile.New(t.TempDir(), "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	_, err = manager.Next(context.Background(), "user")
	if !errors.Is(err, ErrInfraUnavailable) {
		t.Fatalf("fail-closed should return ErrInfraUnavailable, got %v", err)
	}
}

func TestFallbackWithoutCachedSlotFails(t *testing.T) {
	// P0-1: AllowLocalFallback enabled but no cached slot -> ErrNoCachedSlot (never slot 0)
	lease := &fakeLeaseLifecycle{lost: make(chan struct{}), grantErr: errors.New("etcd down")}

	manager, err := NewManager(Config{LeafName: "leaf", AllowLocalFallback: true}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: &fakeSlotCoordinator{}, Cache: workerfile.New(t.TempDir(), "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	_, err = manager.Next(context.Background(), "user")
	if !errors.Is(err, ErrNoCachedSlot) {
		t.Fatalf("no cached slot should return ErrNoCachedSlot, got %v", err)
	}
}

func TestFallbackWithCachedSlotUsesCached(t *testing.T) {
	// P0-1: AllowLocalFallback enabled and a cached slot exists -> use the cache
	cache := t.TempDir()
	workerFile := filepath.Join(cache, "leaf", "user", "worker_id")
	if err := os.MkdirAll(filepath.Dir(workerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workerFile, []byte("5"), 0o644); err != nil {
		t.Fatal(err)
	}
	lease := &fakeLeaseLifecycle{lost: make(chan struct{}), grantErr: errors.New("etcd down")}

	manager, err := NewManager(Config{LeafName: "leaf", AllowLocalFallback: true}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: &fakeSlotCoordinator{}, Cache: workerfile.New(cache, "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	id, err := manager.Next(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	if workerOf(id) != 5 {
		t.Fatalf("fallback worker = %d, want 5", workerOf(id))
	}
}

func TestNoFallbackOnContextCancel(t *testing.T) {
	// P1-6: ctx cancellation does not trigger fallback
	lease := &fakeLeaseLifecycle{lost: make(chan struct{}), grantErr: context.Canceled}

	manager, err := NewManager(Config{LeafName: "leaf", AllowLocalFallback: true}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: &fakeSlotCoordinator{}, Cache: workerfile.New(t.TempDir(), "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	_, err = manager.Next(context.Background(), "user")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx cancel should be returned as-is, got %v", err)
	}
}

func TestManagerDoesNotFallbackWhenSlotOccupied(t *testing.T) {
	lease := &fakeLeaseLifecycle{lost: make(chan struct{})}
	slots := &fakeSlotCoordinator{acquireErr: ErrSlotOccupied}

	manager, err := NewManager(Config{}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: slots, Cache: workerfile.New(t.TempDir(), "devops-id", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Next(context.Background(), "user"); !errors.Is(err, ErrSlotOccupied) {
		t.Fatalf("Next() error = %v, want ErrSlotOccupied", err)
	}
	if lease.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", lease.releaseCalls)
	}
}

func TestInfraLeaseLostRejectsNewIDs(t *testing.T) {
	lease := &fakeLeaseLifecycle{lost: make(chan struct{})}
	manager, err := NewManager(Config{}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: &fakeSlotCoordinator{}, Cache: workerfile.New(t.TempDir(), "devops-id", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	if _, err := manager.Next(context.Background(), "user"); err != nil {
		t.Fatal(err)
	}
	close(lease.lost)
	deadline := time.Now().Add(time.Second)
	for {
		_, err := manager.Next(context.Background(), "user")
		if errors.Is(err, ErrLeaseLost) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("Next() error = %v, want ErrLeaseLost", err)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCloseWithInfraReleasesLease(t *testing.T) {
	lease := &fakeLeaseLifecycle{lost: make(chan struct{})}
	manager, err := NewManager(Config{}, func(bizTag string) TagInfra {
		return TagInfra{Lease: lease, Slots: &fakeSlotCoordinator{}, Cache: workerfile.New(t.TempDir(), "devops-id", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Next(context.Background(), "user"); err != nil {
		t.Fatal(err)
	}

	manager.Close()
	if lease.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", lease.releaseCalls)
	}
}

func TestBatchNextRespectsMaxBatchCount(t *testing.T) {
	manager, err := NewManager(Config{MaxBatchCount: 10}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	_, err = manager.BatchNext(context.Background(), "user", 11)
	if !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("BatchNext error = %v, want ErrBatchTooLarge", err)
	}
}

func TestMaxGeneratorsLimit(t *testing.T) {
	manager, err := NewManager(Config{MaxGenerators: 2}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	if _, err := manager.Next(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Next(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	_, err = manager.Next(context.Background(), "c")
	if !errors.Is(err, ErrTooManyTags) {
		t.Fatalf("third tag error = %v, want ErrTooManyTags", err)
	}
}

func TestConcurrentFirstUseInitializesOnce(t *testing.T) {
	// P1-5: concurrent first use of the same tag initializes it only once
	slots := &fakeSlotCoordinator{acquireSlot: 3}
	lease := &fakeLeaseLifecycle{lost: make(chan struct{})}

	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		time.Sleep(50 * time.Millisecond) // simulate slow I/O
		return TagInfra{Lease: lease, Slots: slots, Cache: workerfile.New(t.TempDir(), "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	const n = 20
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			_, err := manager.Next(context.Background(), "user")
			errs <- err
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("goroutine %d error: %v", i, err)
		}
	}
	if slots.acquireCalls != 1 {
		t.Fatalf("acquire calls = %d, want 1", slots.acquireCalls)
	}
}

func TestWatermarkStartupWait(t *testing.T) {
	// P0-2: wait for the watermark window to pass at startup
	cache := t.TempDir()
	// preset the watermark to now+100ms
	wmPath := filepath.Join(cache, "leaf", "user", "watermark")
	if err := os.MkdirAll(filepath.Dir(wmPath), 0o755); err != nil {
		t.Fatal(err)
	}
	wm := time.Now().UnixMilli() + 100
	if err := os.WriteFile(wmPath, []byte(strconv.FormatInt(wm, 10)), 0o644); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	manager, err := NewManager(Config{LeafName: "leaf"}, func(bizTag string) TagInfra {
		return TagInfra{Cache: workerfile.New(cache, "leaf", bizTag)}
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	id, err := manager.Next(context.Background(), "user")
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(started)
	_ = id
	if elapsed < 50*time.Millisecond {
		t.Fatalf("startup wait %v is too short; the watermark window may not have been awaited", elapsed)
	}
}

func TestValidateNameLengthLimit(t *testing.T) {
	manager, err := NewManager(Config{MaxBizTagLen: 10}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()

	longTag := "this_is_a_very_long_biz_tag_name"
	_, err = manager.Next(context.Background(), longTag)
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("over-long tag error = %v, want ErrInvalidName", err)
	}
}
