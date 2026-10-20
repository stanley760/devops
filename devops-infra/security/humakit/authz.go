package humakit

import (
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// securityPerm in-domain authorization (uid × tid × perm).
func (k *Kit) securityPerm(perm string) Middleware {
	return securitymw.HumaRequirePermission(k.deps.Authorizer, perm, securitymw.HumaOptions{
		OnUnauthorized:        k.deps.Envelope.Unauthorized,
		OnForbidden:           k.deps.Envelope.PermForbidden,
		OnInfrastructureError: k.deps.Envelope.AuthzInfra,
	})
}

// securityPlat platform authorization (g2 + dom="*").
func (k *Kit) securityPlat(perm string) Middleware {
	return securitymw.HumaRequirePlatformPermission(k.deps.Authorizer, perm, securitymw.HumaOptions{
		OnUnauthorized:        k.deps.Envelope.Unauthorized,
		OnForbidden:           k.deps.Envelope.PlatformForbidden,
		OnInfrastructureError: k.deps.Envelope.AuthzInfra,
	})
}
