package redis

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/stanley760/devops/devops-infra/cache"
)

type countingMetrics struct {
	hits      int
	misses    int
	cacheName string
}

func (m *countingMetrics) RecordHit(cacheName string) {
	m.hits++
	m.cacheName = cacheName
}

func (m *countingMetrics) RecordMiss(cacheName string) {
	m.misses++
	m.cacheName = cacheName
}

func newTestClient(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	port, err := strconv.Atoi(mr.Port())
	if err != nil {
		t.Fatalf("parse miniredis port %q: %v", mr.Port(), err)
	}
	client, err := New(Config{Host: mr.Host(), Port: port})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

func TestNilClientReturnsError(t *testing.T) {
	var client *Client
	ctx := context.Background()

	if _, err := client.Get(ctx, "key"); err == nil {
		t.Fatal("Get() should return an initialization error")
	}
	if err := client.Set(ctx, "key", "value", 0); err == nil {
		t.Fatal("Set() should return an initialization error")
	}
	if err := client.Delete(ctx, "key"); err == nil {
		t.Fatal("Delete() should return an initialization error")
	}
	if _, err := client.Exists(ctx, "key"); err == nil {
		t.Fatal("Exists() should return an initialization error")
	}
	if err := client.Ping(ctx); err == nil {
		t.Fatal("Ping() should return an initialization error")
	}
}

func TestGetRecordsCacheHitAndMiss(t *testing.T) {
	client, _ := newTestClient(t)

	metrics := &countingMetrics{}
	client.SetCacheMetrics(metrics)

	ctx := context.Background()
	if _, err := client.Get(ctx, "tenant:1"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("Get() error = %v, want cache.ErrNotFound", err)
	}
	if metrics.misses != 1 || metrics.hits != 0 || metrics.cacheName != "tenant" {
		t.Fatalf("after miss: hits=%d misses=%d cache=%q, want hits=0 misses=1 cache=tenant", metrics.hits, metrics.misses, metrics.cacheName)
	}

	if err := client.Set(ctx, "tenant:1", "ok", 0); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value, err := client.Get(ctx, "tenant:1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "ok" {
		t.Fatalf("Get() value = %q, want %q", value, "ok")
	}
	if metrics.hits != 1 || metrics.misses != 1 || metrics.cacheName != "tenant" {
		t.Fatalf("after hit: hits=%d misses=%d cache=%q, want hits=1 misses=1 cache=tenant", metrics.hits, metrics.misses, metrics.cacheName)
	}
}

func TestGetWithoutMetricsSinkStillWorks(t *testing.T) {
	client, _ := newTestClient(t)

	if _, err := client.Get(context.Background(), "missing"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatalf("Get() error = %v, want cache.ErrNotFound", err)
	}
}
