package humakit

import (
	"github.com/danielgtaylor/huma/v2"

	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// securityJWT is the huma authentication middleware: infra decision + this suite's
// envelope presentation + optional ctx enhancement.
// claims are injected via securitymw; after successful authentication, Deps.WithClaims is
// called (for business enhancements such as the audit actor).
func (k *Kit) securityJWT() Middleware {
	authenticate := securitymw.HumaAuthenticate(k.authn, securitymw.HumaOptions{
		OnUnauthorized:        k.deps.Envelope.Unauthorized,
		OnInfrastructureError: k.deps.Envelope.AuthnInfra,
	})
	withClaims := k.deps.WithClaims
	return func(hctx huma.Context, next func(huma.Context)) {
		authenticate(hctx, func(authenticated huma.Context) {
			if withClaims != nil {
				claims := securitymw.HumaCurrentClaims(authenticated.Context())
				if claims != nil {
					ctx := withClaims(authenticated.Context(), claims)
					authenticated = huma.WithContext(authenticated, ctx)
				}
			}
			next(authenticated)
		})
	}
}
