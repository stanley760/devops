package authz

// Role name configuration: the platform super-admin and tenant super-admin role
// identifiers are injected by the business (or taken from the defaults), so that the
// seed policies and the IsPlatformAdmin check do not each hard-code a copy of the
// strings and drift apart.
const (
	defaultPlatformAdmin = "platform_admin"
	defaultSuperAdmin    = "super_admin"
)

// RoleConfig holds role names; zero-value fields fall back to defaults, preserving
// backward compatibility (yudao convention).
type RoleConfig struct {
	PlatformAdmin string // platform admin role (g2 pair + p,*,* wildcard), defaults to "platform_admin"
	SuperAdmin    string // tenant super-admin role (p,{domain},* + g triple), defaults to "super_admin"
}

// DefaultRoleConfig returns the default role names (identical to the historical hard-coded strings).
func DefaultRoleConfig() RoleConfig {
	return RoleConfig{
		PlatformAdmin: defaultPlatformAdmin,
		SuperAdmin:    defaultSuperAdmin,
	}
}

// withDefaults backfills zero-value fields with defaults so partial customization still works.
func (c RoleConfig) withDefaults() RoleConfig {
	if c.PlatformAdmin == "" {
		c.PlatformAdmin = defaultPlatformAdmin
	}
	if c.SuperAdmin == "" {
		c.SuperAdmin = defaultSuperAdmin
	}
	return c
}
