//go:build wireinject

// Package app wires the devops-id dependency graph (each DDD layer -> gRPC register callbacks),
// isomorphic with devops-system/internal/app.
package app

import (
	"github.com/google/wire"

	idapp "github.com/stanley760/devops/devops-id/internal/application/idgen"
	idgrpc "github.com/stanley760/devops/devops-id/internal/interfaces/grpc"
)

var infraSet = wire.NewSet(
	provideEtcdClient,
	newTagInfraFactory,
)

var domainSet = wire.NewSet(
	newDomainConfig,
	provideHealthBridge,
	provideObserver,
	provideManager,
)

var serviceSet = wire.NewSet(
	idapp.NewService,
	wire.Bind(new(idapp.Generator), new(*idapp.Service)),
)

var grpcSet = wire.NewSet(
	idgrpc.NewIdGenServer,
	newGRPCRegister,
)
