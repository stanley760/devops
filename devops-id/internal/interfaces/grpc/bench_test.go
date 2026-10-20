// gRPC layer ID generation benchmark: bufconn in-process loopback, covering protobuf
// codec + gRPC stack overhead. Compared with domain/idgen's bench_test.go, it isolates
// the transport cost.
//
// Run:
//
//	go test ./internal/interfaces/grpc -run '^$' -bench . -benchmem
package grpc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"

	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	idapp "github.com/stanley760/devops/devops-id/internal/application/idgen"
	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
	"github.com/stanley760/devops/devops-id/pkg/idgenpb"
)

// quietLogsOnce ensures logs are silenced only once, at the first benchmark's initialization.
var quietLogsOnce sync.Once

// quietLogs silences the generator's info logs: each b.N calibration round rebuilds the
// Manager, and ready/closed messages would flood the benchmark output. Benchmark path only.
func quietLogs() {
	quietLogsOnce.Do(func() {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	})
}

// newBenchClient starts an in-process IdGen service and returns a client.
// The Manager runs in local mode (no etcd) so the benchmark only measures ID generation
// and the transport stack, excluding coordination I/O.
func newBenchClient(b *testing.B) idgenpb.IdGenClient {
	b.Helper()
	quietLogs()
	mgr, err := iddomain.NewManager(iddomain.Config{}, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(mgr.Close)

	lis := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer()
	idgenpb.RegisterIdGenServer(server, NewIdGenServer(idapp.NewService(mgr)))
	go func() { _ = server.Serve(lis) }()
	b.Cleanup(server.Stop)

	conn, err := grpcgo.NewClient(
		"passthrough:///bufnet",
		grpcgo.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpcgo.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = conn.Close() })
	return idgenpb.NewIdGenClient(conn)
}

// BenchmarkGRPCNextSerial serial Next RPCs: end-to-end cost of a single call.
func BenchmarkGRPCNextSerial(b *testing.B) {
	client := newBenchClient(b)
	ctx := context.Background()
	req := &idgenpb.NextRequest{BizTag: "bench"}
	// Warm-up: trigger biz_tag lazy initialization and establish HTTP/2 traffic.
	if _, err := client.Next(ctx, req); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := client.Next(ctx, req); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGRPCNextParallel parallel Next RPCs: synthetic throughput of the transport stack plus server-side lock contention.
func BenchmarkGRPCNextParallel(b *testing.B) {
	client := newBenchClient(b)
	ctx := context.Background()
	req := &idgenpb.NextRequest{BizTag: "bench"}
	if _, err := client.Next(ctx, req); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := client.Next(ctx, req); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkGRPCBatchNext batch RPCs: after amortizing the per-call cost, close to the domain layer's ceiling.
func BenchmarkGRPCBatchNext(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(fmt.Sprintf("size-%d", size), func(b *testing.B) {
			client := newBenchClient(b)
			ctx := context.Background()
			req := &idgenpb.BatchNextRequest{BizTag: "bench", Count: int32(size)}
			if _, err := client.BatchNext(ctx, req); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := client.BatchNext(ctx, req); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N*size)/b.Elapsed().Seconds(), "ids/s")
		})
	}
}
