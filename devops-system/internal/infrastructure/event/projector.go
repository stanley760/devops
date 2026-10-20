// Package event contains the projector implementations for domain events (syncing write-model changes to the casbin read model).
//
// This is the landing of the "write model / read model separation" in CQRS: aggregates record
// domain events within the transaction, authorization policy sync is done by the projector,
// projection failures do not affect the main-flow response, and because events are persisted
// failures can be retried without loss.
//
// The pre-refactor problem (3 manual compensations):
//
//	service/user.go:130  "user created, but role authorization write failed, please retry the update"
//	service/user.go:195  "user updated, but role authorization refresh failed, please retry the update"
//	service/user.go:250  "user deleted, but role authorization cleanup failed"
//	service/role.go:168  "menu saved, but permission policy refresh failed, please retry"
//
// The common point of these errors: the business row was already committed but the
// authorization was not in effect, leaving the client with a semantically vague prompt —
// no way to tell whether to retry or intervene manually, and no way to tell from the data
// whether the authorization has been synced. Now these scenarios are unified: the main flow
// returns success + the event is stored + the projector syncs, and on failure the event stays
// in the pending-projection state, compensated by retry tasks.
package event

import (
	"context"
	"log/slog"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/domain/iam"
	"github.com/stanley760/devops/devops-system/internal/domain/shared"
)

// casbinProjector projects IAM domain events onto casbin policies.
//
// Idempotency: casbin's RebuildXxx family are all "clear first then rebuild", and RemoveXxx is
// conditional deletion, so repeated executions yield the same result, satisfying the idempotency
// requirement under at-least-once delivery.
type casbinProjector struct{}

// NewCasbinProjector constructs the casbin projector.
func NewCasbinProjector() shared.EventProjector { return casbinProjector{} }

// Name implements naming (for dispatcher logs).
func (casbinProjector) Name() string { return "casbin" }

// Handles declares the subscribed events.
func (casbinProjector) Handles() []string {
	return []string{
		iam.EventUserRolesAssigned,
		iam.EventUserDeactivated,
		iam.EventUserDeleted,
		iam.EventRoleMenusAssigned,
		iam.EventRoleDeleted,
		iam.EventTenantAdminSeeded,
	}
}

// Handle dispatches a single event to the corresponding sync action.
func (casbinProjector) Handle(ctx context.Context, e shared.Event) error {
	switch ev := e.(type) {
	case iam.UserRolesAssigned:
		return authz.RebuildUserGrouping(ev.UserID, ev.TenantID, ev.RoleCodes)
	case iam.UserDeactivated:
		return nil // session revocation is handled by sessionProjector
	case iam.UserDeleted:
		return authz.RemoveUserGrouping(ev.UserID, ev.TenantID)
	case iam.RoleMenusAssigned:
		return authz.RebuildRolePolicies(ev.RoleCode, ev.TenantID, ev.Permissions)
	case iam.RoleDeleted:
		if err := authz.RemoveRolePolicies(ev.RoleCode, ev.TenantID); err != nil {
			return err
		}
		return authz.RemoveRoleGrouping(ev.RoleCode, ev.TenantID)
	case iam.TenantAdminSeeded:
		// new tenant's initial super admin: in-domain all-wildcard p row + g row binding (the existing SeedTenantAdmin semantics).
		// Idempotent: p/g writes go through the casbin adapter's add, so repeated execution produces no duplicate rows.
		return authz.SeedTenantAdmin(ev.TenantID, ev.AdminUserID)
	default:
		slog.WarnContext(ctx, "casbin projector received an unknown event", "event", e.EventName())
		return nil
	}
}

// sessionProjector projects "user/tenant unavailable" events as session revocation (kick offline).
type sessionProjector struct {
	authM *auth.Manager
}

// NewSessionProjector constructs the session projector.
func NewSessionProjector(authM *auth.Manager) shared.EventProjector {
	return sessionProjector{authM: authM}
}

// Name implements naming.
func (sessionProjector) Name() string { return "session" }

// Handles declares the subscribed events.
func (sessionProjector) Handles() []string {
	return []string{iam.EventUserDeactivated, iam.EventTenantDisabled}
}

// Handle revokes sessions in the corresponding scope.
func (p sessionProjector) Handle(_ context.Context, e shared.Event) error {
	if p.authM == nil {
		slog.Error("session projector has no auth.Manager assembled, skipping session revocation", "event", e.EventName())
		return nil
	}
	switch ev := e.(type) {
	case iam.UserDeactivated:
		p.authM.RevokeAllSessions(context.Background(), ev.TenantID, ev.UserID)
	case iam.TenantDisabled:
		p.authM.RevokeTenantSessions(context.Background(), ev.TenantID)
	}
	return nil
}

// cacheProjector cleans the data-permission cache.
//
// Before the refactor this was the direct invalidateScope() calls at service/user.go:199 and
// service/role.go:139/195 — the application layer thus depended on a middleware-layer callback
// (app/service_providers.go passed kit.InvalidateDataScope to the service, a reverse design of
// "service depends on middleware").
// Now event-driven: the application layer no longer holds middleware callbacks, and the dependency direction is corrected.
type cacheProjector struct {
	invalidate func()
}

// NewScopeCacheProjector constructs the data-permission cache projector.
func NewScopeCacheProjector(invalidate func()) shared.EventProjector {
	return cacheProjector{invalidate: invalidate}
}

// Name implements naming.
func (cacheProjector) Name() string { return "scope-cache" }

// Handles declares the subscribed events: role assignment and data-scope changes both alter the visible scope.
func (cacheProjector) Handles() []string {
	return []string{iam.EventUserRolesAssigned, iam.EventRoleMenusAssigned, iam.EventRoleDeleted}
}

// Handle clears the cache.
func (p cacheProjector) Handle(_ context.Context, _ shared.Event) error {
	if p.invalidate != nil {
		p.invalidate()
	}
	return nil
}
