// Integration test: verifies the idclient prefetch buffer plus the server-side ID generation chain.
// Depends on a local devops-id service (:9990);
// skip condition: t.Skip on connection failure (service not started).
package idclient_test

import (
	"context"
	"github.com/stanley760/devops/devops-id/pkg/idclient"
	grpcinfra "github.com/stanley760/devops/devops-infra/grpc"
	"testing"
	"time"
)

// dial establishes the client connection and liveness-probes it: gRPC connects lazily,
// so dial does not trigger a real connection; a Ping failure (service not started)
// is used as the skip condition.
func dial(t *testing.T) *idclient.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	conn, err := grpcinfra.Dial(ctx, "127.0.0.1:9990", 2*time.Second)
	if err != nil {
		t.Skipf("devops-id service not started, skipping integration test: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	c := idclient.New(conn, idclient.BufferConfig{
		Batch:       10,
		LowWater:    0.2,
		CallTimeout: 2 * time.Second,
	})
	if err := c.Ping(ctx); err != nil {
		t.Skipf("devops-id service not started, skipping integration test: %v", err)
	}
	return c
}

func TestNextSequential(t *testing.T) {
	c := dial(t)
	ctx := context.Background()

	var prev int64
	for i := 0; i < 50; i++ {
		id, err := c.Next(ctx, "user")
		if err != nil {
			t.Fatalf("ID generation %d failed: %v", i, err)
		}
		if id <= prev {
			t.Fatalf("ID not increasing: prev=%d cur=%d", prev, id)
		}
		prev = id
	}
	t.Logf("user worker: 50 IDs consecutively increasing, last value %d", prev)
}

func TestMultiTagIsolation(t *testing.T) {
	c := dial(t)
	ctx := context.Background()

	u1, err := c.Next(ctx, "user")
	if err != nil {
		t.Fatal(err)
	}
	r1, err := c.Next(ctx, "role")
	if err != nil {
		t.Fatal(err)
	}
	// Different biz_tags have independent worker spaces and may duplicate IDs; only verify that IDs can be generated
	t.Logf("user=%d role=%d (different biz_tags have independent worker spaces)", u1, r1)
}
