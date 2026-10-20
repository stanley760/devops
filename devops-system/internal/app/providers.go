//go:build wireinject

// Package app owns the application dependency graph shared by server entrypoints.
package app

import (
	"github.com/google/wire"

	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/router"
	"github.com/stanley760/devops/devops-system/internal/service"
)

var serviceSet = wire.NewSet(
	service.NewAuth,
	newUserService,
	newRoleService,
	service.NewMenu,
	service.NewDept,
	service.NewPost,
	service.NewTenant,
	service.NewLoginLog,
	service.NewOperationLog,
	service.NewDict,
	service.NewConfig,
)

var routeSet = wire.NewSet(
	// The docCfg parameter of NewRegister: wire does not automatically destructure injector parameter structs, so a field provider must be supplied explicitly
	wire.FieldsOf(new(config.Config), "APIDoc"),
	router.NewAuthRoutes,
	router.NewUserRoutes,
	router.NewRoleRoutes,
	router.NewMenuRoutes,
	router.NewDeptRoutes,
	router.NewPostRoutes,
	router.NewTenantRoutes,
	router.NewLoginLogRoutes,
	router.NewOperationLogRoutes,
	router.NewDictRoutes,
	router.NewConfigRoutes,
	wire.Struct(new(router.Modules), "*"),
	router.NewRegister,
)
