package authz

import "fmt"

// Initial policy seeds (idempotent): role names come from RoleConfig, and the rule
// shapes are word-for-word identical to the historical versions.
//
//	SeedBootstrap:  p,  {pa}, *,          *      platform admins bypass domains (g2 path)
//	                g2, {subject}, {pa}           bind the platform admin
//	                p,  {sa}, {domain},   *      tenant super-admins bypass within the domain
//	                g,  {subject}, {sa},  {domain}
//
//	SeedTenantAdmin: p, {sa}, {domain},   *      tenant super-admins bypass within the domain
//	                 g, {subject}, {sa},  {domain}

// SeedBootstrap seeds the initial policies (idempotent, reused by migrate/first startup).
// subject is the initial admin primary key (its string form is converted by the caller), domain is the tenant domain.
func (a *CasbinAuthorizer) SeedBootstrap(subject, domain string) error {
	if _, err := a.getEnforcer(); err != nil {
		return err
	}

	pa, sa := a.roles.PlatformAdmin, a.roles.SuperAdmin
	for _, rule := range []struct {
		ptype string
		vals  []string
	}{
		{"p", []string{pa, "*", "*"}},
		{"g2", []string{subject, pa}},
		{"p", []string{sa, domain, "*"}},
		{"g", []string{subject, sa, domain}},
	} {
		if err := a.AddPolicyIfAbsent(rule.ptype, rule.vals...); err != nil {
			return fmt.Errorf("failed to write seed %s row: %w", rule.ptype, err)
		}
	}
	return nil
}

// SeedTenantAdmin seeds the tenant's initial super-admin policy (called when creating
// a tenant, idempotent).
func (a *CasbinAuthorizer) SeedTenantAdmin(subject, domain string) error {
	if _, err := a.getEnforcer(); err != nil {
		return err
	}

	sa := a.roles.SuperAdmin
	if err := a.AddPolicyIfAbsent("p", sa, domain, "*"); err != nil {
		return fmt.Errorf("failed to write the tenant super-admin policy: %w", err)
	}
	if err := a.AddPolicyIfAbsent("g", subject, sa, domain); err != nil {
		return fmt.Errorf("failed to write the tenant super-admin binding: %w", err)
	}
	return nil
}
