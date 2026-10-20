// Package app is the application assembly layer (composition root).
//
// Dependency direction overview (where DDD's dependency inversion lands):
//
//	cmd ──→ app ──→ application ──→ domain ←── infrastructure
//	              │         (port interfaces)  (GORM/Casbin/Redis impls)
//	              └──→ api/v1, router, middleware
//
// This package is the **only** place allowed to know both the interfaces
// and their implementations: it injects the GORM repository implementations
// from infrastructure into the ports declared by application. Neither the
// domain nor the application package imports infrastructure, so domain
// rules can be tested independently of the database and HTTP.
package app

import (
	"github.com/stanley760/devops/devops-system/internal/application/command"
	"github.com/stanley760/devops/devops-system/internal/application/query"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/domain/auditlog"
	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/domain/shared"
	"github.com/stanley760/devops/devops-system/internal/infrastructure/event"
	"github.com/stanley760/devops/devops-system/internal/infrastructure/persistence"
)

// Repositories aggregates all repository implementations (injected into the ports by this package).
type Repositories struct {
	Users   iam.UserRepository
	Roles   iam.RoleRepository
	Menus   iam.MenuRepository
	Depts   iam.DeptRepository
	Posts   iam.PostRepository
	Tenants iam.TenantRepository

	LoginLogs     auditlog.LoginLogReader
	LoginLogWrite auditlog.LoginLogWriter
	OperationLogs auditlog.OperationLogReader
}

// NewRepositories constructs all repository implementations (GORM).
func NewRepositories() Repositories {
	return Repositories{
		Users:         persistence.NewUserRepository(),
		Roles:         persistence.NewRoleRepository(),
		Menus:         persistence.NewMenuRepository(),
		Depts:         persistence.NewDeptRepository(),
		Posts:         persistence.NewPostRepository(),
		Tenants:       persistence.NewTenantRepository(),
		LoginLogs:     persistence.NewLoginLogReader(),
		LoginLogWrite: persistence.NewLoginLogRepository(),
		OperationLogs: persistence.NewOperationLogReader(),
	}
}

// Commands aggregates all write-side command handlers.
type Commands struct {
	User   *command.UserCommand
	Role   *command.RoleCommand
	Org    *command.OrgCommand
	Tenant *command.TenantCommand
}

// Queries aggregates all read-side query handlers.
type Queries struct {
	User         *query.UserQuery
	Role         *query.RoleQuery
	Menu         *query.MenuQuery
	Dept         *query.DeptQuery
	Post         *query.PostQuery
	Tenant       *query.TenantQuery
	LoginLog     *query.LoginLogQuery
	OperationLog *query.OperationLogQuery
}

// NewCommands constructs the write-side handlers (injects repository ports + the transaction manager).
func NewCommands(repos Repositories, txs shared.TxManager) Commands {
	return Commands{
		User:   command.NewUserCommand(repos.Users, txs),
		Role:   command.NewRoleCommand(repos.Roles, repos.Menus, txs),
		Org:    command.NewOrgCommand(repos.Depts, repos.Posts, repos.Menus, txs),
		Tenant: command.NewTenantCommand(repos.Tenants, repos.Users, repos.Roles, txs),
	}
}

// NewQueries constructs the read-side handlers (injects repository ports).
func NewQueries(repos Repositories) Queries {
	return Queries{
		User:         query.NewUserQuery(repos.Users),
		Role:         query.NewRoleQuery(repos.Roles),
		Menu:         query.NewMenuQuery(repos.Menus),
		Dept:         query.NewDeptQuery(repos.Depts),
		Post:         query.NewPostQuery(repos.Posts),
		Tenant:       query.NewTenantQuery(repos.Tenants),
		LoginLog:     query.NewLoginLogQuery(repos.LoginLogs),
		OperationLog: query.NewOperationLogQuery(repos.OperationLogs),
	}
}

// NewTxManager constructs the transaction manager (GORM implementation).
func NewTxManager() shared.TxManager { return persistence.NewTxManager() }

// EventDispatch holds the components needed for event dispatch.
type EventDispatch struct {
	Publisher shared.EventPublisher
	Store     shared.EventStore
}

// NewEventDispatch constructs the event dispatcher and projectors.
//
// invalidateScope is the data-scope cache invalidation callback: it is
// provided by middleware.Kit, but the injection direction is app → projector,
// not the pre-refactor service → kit.InvalidateDataScope (the latter made
// the application layer depend on the middleware layer).
//
// Before the refactor, app/service_providers.go had two hand-written
// providers whose only job was to wrap kit.InvalidateDataScope into a form
// the service could consume; now that reverse dependency is gone, and the
// wire provider set is simpler as a result.
func NewEventDispatch(authM *auth.Manager, invalidateScope func()) EventDispatch {
	return EventDispatch{
		Publisher: persistence.NewDispatcher(
			event.NewCasbinProjector(),
			event.NewSessionProjector(authM),
			event.NewScopeCacheProjector(invalidateScope),
		),
		Store: persistence.NewEventStore(),
	}
}
