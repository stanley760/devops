// Package auditctx carries the authenticated actor through request contexts.
//
// Cross-business contract: any module that needs to record "who triggered this change"
// reads the current actor from ctx; when to persist it is decided by each business's
// model hooks or audit middleware.
package auditctx

import "context"

type actorContextKey struct{}

// Actor identifies the authenticated user responsible for a database change.
type Actor struct {
	Username string
	// UserID primary key of the actor; 0 for unknown or system writes.
	UserID int64
	// UserType actor category (C-side member / B-side admin).
	// Semantics are defined by devops-common/audit.UserType; int64 is used here
	// to avoid a circular dependency between packages.
	UserType int64
	// TenantID tenant the actor belongs to; 0 for global-table operations.
	TenantID int64
}

// WithActor returns a context carrying the authenticated actor.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// From returns the actor attached to ctx, if any.
func From(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

// Username returns the actor's username from ctx, or an empty string if none was injected.
func Username(ctx context.Context) string {
	actor, _ := From(ctx)
	return actor.Username
}
