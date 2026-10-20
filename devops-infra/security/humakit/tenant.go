package humakit

import (
	tenantinfra "github.com/stanley760/devops/devops-infra/tenant"
)

// securityTenant tenant verification + context injection (the usability check is injected
// via Deps.TenantUsable; the business side decides the disabled/expired semantics and caching strategy).
func (k *Kit) securityTenant() Middleware {
	return tenantinfra.HumaTenantContext(k.deps.TenantUsable,
		k.deps.Envelope.Unauthorized, k.deps.Envelope.TenantForbidden)
}
