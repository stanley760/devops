// Package grpc assembles the gRPC server and client.
//
// Server: recovery interceptor (panic → Internal, stack into logs), accesslog interceptor,
// standard health check (grpc.health.v1), and reflection in dev environments for grpcurl debugging.
// Client: dial wrapper with built-in Unavailable exponential backoff retry (safe for ID-generation-style idempotent calls).
package grpc

import (
	"context"
	"fmt"
	"github.com/stanley760/devops/devops-infra/grpc/interceptor"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/stats"
	"log/slog"
	"net"
	"os/signal"
	"syscall"
	"time"
)

// ServerConfig is the gRPC server config.
type ServerConfig struct {
	Listen     string // listen address, e.g. ":9090"
	Reflection bool   // whether to enable reflection (recommended in dev environments)

	// StatsHandler optional OTel stats handler (otelgrpc.NewServerHandler());
	// injected when non-nil: trace + RED metrics in one step, consistent with the HTTP side.
	StatsHandler stats.Handler
	// UnaryInterceptors custom unary interceptors appended after Recovery/AccessLog.
	UnaryInterceptors []grpc.UnaryServerInterceptor
}

// Run starts the gRPC server and shuts down gracefully on SIGINT/SIGTERM.
// register is called by each service to register its own gRPC service implementations and health status.
func Run(cfg ServerConfig, register func(s *grpc.Server, hc *health.Server)) error {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(append([]grpc.UnaryServerInterceptor{
			interceptor.Recovery(),
			interceptor.AccessLog(),
		}, cfg.UnaryInterceptors...)...),
	}
	if cfg.StatsHandler != nil {
		opts = append(opts, grpc.StatsHandler(cfg.StatsHandler))
	}
	s := grpc.NewServer(opts...)

	hc := health.NewServer()
	healthpb.RegisterHealthServer(s, hc)
	if register != nil {
		register(s, hc)
	}
	if cfg.Reflection {
		reflection.Register(s)
	}

	lis, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", cfg.Listen, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("gRPC server starting", "addr", cfg.Listen)
		if err := s.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("gRPC server error: %w", err)
	case <-ctx.Done():
	}

	slog.Info("shutdown signal received, starting graceful shutdown")
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		s.Stop() // force stop on timeout to avoid a stuck shutdown
	}
	slog.Info("gRPC server stopped")
	return nil
}
