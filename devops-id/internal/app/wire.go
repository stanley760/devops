//go:build wireinject

package app

import (
	"github.com/google/wire"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

	"github.com/stanley760/devops/devops-id/internal/config"
)

// InitializeGRPC wires the ID generation service dependency graph: etcd client -> per-tag
// infrastructure factory -> snowflake Manager -> application service -> gRPC handler -> register callbacks.
// The returned cleanup runs in reverse: revoke all per-tag leases -> close the etcd client.
// Regenerate with: go generate -tags=wireinject ./internal/app
//
//go:generate go run github.com/google/wire/cmd/wire
func InitializeGRPC(cfg config.Config) (func(*grpc.Server, *health.Server), func(), error) {
	wire.Build(infraSet, domainSet, serviceSet, grpcSet)
	return nil, nil, nil
}
