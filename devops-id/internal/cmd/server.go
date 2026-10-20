// The server subcommand: wires the dependency graph (wire) and starts the gRPC service and metrics.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	// Mounts /debug/pprof/* onto DefaultServeMux (see startMetricsServer)
	"github.com/spf13/cobra"
	"github.com/stanley760/devops/devops-id/internal/app"
	"github.com/stanley760/devops/devops-id/internal/config"
	infracfg "github.com/stanley760/devops/devops-infra/config"
	grpcinfra "github.com/stanley760/devops/devops-infra/grpc"
	"github.com/stanley760/devops/devops-infra/nacos"
	"github.com/stanley760/devops/devops-infra/observability"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	_ "net/http/pprof"
	"strconv"
	"time"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "start the gRPC snowflake ID generation service",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg config.Config
		if err := infracfg.LoadInto(&cfg); err != nil {
			return err
		}

		// Dependency graph wiring (wire): etcd client -> per-tag infrastructure factory ->
		// snowflake Manager -> application service -> gRPC handler; worker slots are
		// lazily loaded on first use of a biz_tag.
		register, cleanupGraph, err := app.InitializeGRPC(cfg)
		if err != nil {
			return err
		}
		defer cleanupGraph() // graceful shutdown: revoke all per-tag leases, release slots immediately, then close etcd

		reflection := cfg.Grpc.Reflection
		if infracfg.Env() == "dev" {
			reflection = true // force-enabled in dev, for grpcurl debugging
		}

		serviceName := cfg.Nacos.Registry.ServiceName
		if serviceName == "" {
			return fmt.Errorf("nacos.registry.service_name is not configured")
		}
		port, err := listenPort(cfg.Grpc.Listen)
		if err != nil {
			return err
		}
		if err := nacos.Register(serviceName, port, cfg.Nacos.Registry.Meta); err != nil {
			return err
		}
		defer func() {
			_ = nacos.Deregister(serviceName, port)
		}()

		grpcErr := make(chan error, 1)
		go func() {
			grpcErr <- grpcinfra.Run(grpcinfra.ServerConfig{
				Listen:       cfg.Grpc.Listen,
				Reflection:   reflection,
				StatsHandler: otelgrpc.NewServerHandler(),
			}, register)
		}()

		metricsServer, metricsErr, err := startMetricsServer(obsRuntime, cfg.Observability.MetricsListen)
		if err != nil {
			return err
		}
		defer stopMetricsServer(metricsServer)

		select {
		case err := <-metricsErr:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return <-grpcErr
		case err := <-grpcErr:
			return err
		}
	},
}

func startMetricsServer(obs *observability.Runtime, listen string) (*http.Server, <-chan error, error) {
	if listen == "" || obs == nil || obs.MetricsHandler == nil {
		return nil, make(chan error, 1), nil
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on metrics %s: %w", listen, err)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", obs.MetricsHandler)
	// Diagnostics endpoint: goroutine/block profiles help investigate handler-hang incidents
	// (2026-10-08: a suspected BatchNext hang with no evidence left behind). Only active when metrics_listen is configured.
	mux.Handle("/debug/pprof/", http.DefaultServeMux)
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()
	return server, errCh, nil
}

func stopMetricsServer(server *http.Server) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func listenPort(listen string) (int, error) {
	_, portText, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, fmt.Errorf("failed to parse gRPC listen address %q: %w", listen, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port in gRPC listen address %q", listen)
	}
	return port, nil
}
