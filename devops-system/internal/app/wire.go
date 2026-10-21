//go:build wireinject

package app

import (
	goredis "github.com/redis/go-redis/v9"

	httpinfra "github.com/stanley760/devops/devops-infra/http"

	"github.com/google/wire"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/middleware"
)

// InitializeHTTP assembles services, handlers, and the business route callback.
// Regenerate the injector with: go generate -tags=wireinject ./internal/app
//
//go:generate go run github.com/google/wire/cmd/wire
func InitializeHTTP(cfg config.Config, kit *middleware.Kit, authM *auth.Manager, rdb *goredis.Client) httpinfra.Register {
	wire.Build(serviceSet, routeSet)
	return nil
}
