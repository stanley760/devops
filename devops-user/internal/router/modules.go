package router

import (
	"github.com/danielgtaylor/huma/v2"

	apiv1 "github.com/stanley760/devops/devops-user/internal/api/v1"
	"github.com/stanley760/devops/devops-user/internal/middleware"
	"github.com/stanley760/devops/devops-user/internal/service"
)

// userAPIPrefix is the external prefix for C-side endpoints. Distinct from
// devops-system's /system, making it easy for the gateway to route and
// rate-limit by prefix.
const userAPIPrefix = "/user"

// Full paths of the anonymous endpoints. They must be exported by this package
// and share one definition with the huma registrations below, avoiding each of
// deploy/higress/ingresses/devops-user-public.yaml and the guard allowlist
// writing its own copy of the string and silently drifting (the consequence of
// drift is login deadlocking).
//
// These endpoints must be anonymous: the caller has no token yet. The C-side
// jwt-auth must be mounted on a separate ingress (plugins are mounted per
// Ingress resource and do not support path matching), and the application-layer
// guard must allow them through in sync.
const (
	// AuthLoginPath is the login endpoint, which issues an access token.
	AuthLoginPath = userAPIPrefix + "/api/v1/auth/login"
	// AuthRegisterPath is the registration endpoint.
	AuthRegisterPath = userAPIPrefix + "/api/v1/auth/register"
	// AuthRefreshPath is the refresh endpoint. Must be anonymous: the caller's
	// access token has expired by now, and forcing a JWT would make it
	// impossible to ever get a new token (same as login).
	AuthRefreshPath = userAPIPrefix + "/api/v1/auth/refresh"
	// JWKSPath is the verification public key, for other services'
	// self-verification and manual debugging.
	JWKSPath = "/.well-known/jwks.json"
)

// AuthRoutes holds authentication-related routes (login / register).
type AuthRoutes func(huma.API)

// ProfileRoutes holds user profile routes (read / update the current user).
type ProfileRoutes func(huma.API)

// OAuthRoutes holds third-party login routes.
type OAuthRoutes func(huma.API)

// Modules is the set of business route modules that the application layer
// explicitly composes.
//
// Using func(huma.API) instead of calling handlers directly: the router package
// does not depend on concrete Handler types, so modules can be tested and
// replaced independently.
type Modules struct {
	Auth    AuthRoutes
	Profile ProfileRoutes
	OAuth   OAuthRoutes
}

// NewAuthRoutes builds the auth routes from a service.
func NewAuthRoutes(svc *service.Auth, kit *middleware.Kit) AuthRoutes {
	h := apiv1.NewAuthHandler(svc)
	return func(api huma.API) { registerAuth(api, h, kit) }
}

// NewProfileRoutes builds the user profile routes from a service.
func NewProfileRoutes(svc *service.Profile, kit *middleware.Kit) ProfileRoutes {
	h := apiv1.NewProfileHandler(svc)
	return func(api huma.API) { registerProfile(api, h, kit) }
}

// NewOAuthRoutes builds the third-party login routes from a service.
func NewOAuthRoutes(svc *service.OAuth, kit *middleware.Kit) OAuthRoutes {
	h := apiv1.NewOAuthHandler(svc)
	return func(api huma.API) { registerOAuth(api, h, kit) }
}
