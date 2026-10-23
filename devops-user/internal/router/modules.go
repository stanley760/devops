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

// The allowlist of anonymous endpoints is no longer declared here: it is
// maintained as a single policy in infra
// (devops-infra/security/gateway/exposed-endpoints.yaml). The application-layer
// guard reads that policy, the Higress public ingress is generated from it, and
// the router's consistency test compares it with the actually registered
// endpoints — so deploy/higress/ingresses/devops-user-public.yaml, the guard
// allowlist and the huma registrations can no longer each keep their own copy
// and drift apart (the consequence of drift is login deadlocking).
//
// JWKSPath is the verification public key, for other services'
// self-verification and manual debugging.
const JWKSPath = "/.well-known/jwks.json"

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
func NewAuthRoutes(svc *service.Auth, captchaSvc *service.Captcha, kit *middleware.Kit) AuthRoutes {
	h := apiv1.NewAuthHandler(svc, captchaSvc)
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
