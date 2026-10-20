// Package config defines the configuration structure for devops-user (the
// C-side user service).
//
// The loading mechanism reuses devops-infra/config (bootstrap base + APP_ENV
// environment override merge + optional Nacos remote config); this package
// only declares its own root Config and dedicated sections.
//
// Differences from devops-system (deliberate, not an oversight):
//   - no tenant section: the C side is single-tenant, with no multi-tenant
//     isolation;
//   - no authz/casbin: C-side permissions are "you can only operate on your
//     own data", a row-level ownership check done by the service layer
//     filtering on claims.UID, with no RBAC;
//   - jwt.issuer is independent (this service defaults to devops-user),
//     avoiding cross-matching with devops-system tokens in the gateway
//     jwt-auth consumers.
package config

import (
	"time"

	"github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/security/gateway"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
)

// DefaultJWTKeyID is the default kid when key_id is not configured (consistent
// between the token header and JWKS).
const DefaultJWTKeyID = "devops-user-rs256-1"

// Config is the devops-user service root config, corresponding to the
// top-level structure of config/bootstrap.yaml.
type Config struct {
	Server        config.ServerConfig `mapstructure:"server"`
	Log           config.LogConfig    `mapstructure:"log"`
	Mysql         config.MysqlConfig  `mapstructure:"mysql"`
	Redis         config.RedisConfig  `mapstructure:"redis"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	IDClient      IDClientConfig      `mapstructure:"idclient"`
	Nacos         config.NacosConfig  `mapstructure:"nacos"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	APIDoc        APIDocConfig        `mapstructure:"apidoc"`
	Gateway       gateway.Config      `mapstructure:"gateway"`
	LoginLock     LoginLockConfig     `mapstructure:"login_lock"`
	OAuth         OAuthConfig         `mapstructure:"oauth"`
}

// OAuthConfig is the third-party login configuration.
//
// ⚠️ client_secret must be injected by secret management; **committing real
// values is forbidden**; leaving it empty in dev is fine (the corresponding
// provider shows as "not configured" and the frontend hides its entry).
type OAuthConfig struct {
	// StateTTL is the state lifetime; authorization requests older than 10
	// minutes are basically the user walking away, and allowing more only
	// widens the replay window.
	StateTTL time.Duration `mapstructure:"state_ttl"`
	// RedirectWhitelist holds whitelist prefixes for post-login redirect URLs.
	//
	// ⚠️ **Required**: without this check an attacker can use an open redirect
	// to phish (redirecting to their own site after a successful login to
	// steal the token).
	RedirectWhitelist []string `mapstructure:"redirect_whitelist"`
	// Per-provider config for Github / Google / Wechat.
	Github ProviderConfig `mapstructure:"github"`
	Google ProviderConfig `mapstructure:"google"`
	Wechat ProviderConfig `mapstructure:"wechat"`
}

// ProviderConfig is the configuration for a single third-party provider.
type ProviderConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
	// Scopes are the requested permission scopes.
	//
	// ⚠️ **WeChat ignores this**: website apps only allow `snsapi_login`, and
	// any other value is forcibly overridden to snsapi_login inside the
	// provider. The field is kept only for structural consistency with other
	// providers.
	Scopes []string `mapstructure:"scopes"`
	// AutoRegister controls whether to auto-create a local account when not
	// yet bound.
	//
	// ⚠️ Auto-creating accounts turns "anyone who can log into that third
	// party" into a local user. Products with open registration usually set
	// this to false: log in first, then bind, allowing consistency checks at
	// bind time.
	AutoRegister bool `mapstructure:"auto_register"`
	// NicknamePrefix is the nickname prefix used when auto-creating accounts
	// (to identify the source).
	NicknamePrefix string `mapstructure:"nickname_prefix"`
}

// LoginLockConfig is the login failure rate limiting and temporary account
// lockout configuration.
//
// bcrypt only slows down a single brute-force attempt; it cannot stop
// high-speed attempts. The C side has open registration and enumerable
// usernames, so IP rate limiting and account lockout must be combined across
// both dimensions (missing either can be bypassed).
type LoginLockConfig struct {
	// Enabled is the master switch; when false, no rate limiting or lockout
	// (can be turned off in dev/test).
	Enabled bool `mapstructure:"enabled"`
	// MaxIPFails is the max failures allowed per IP within the window; 0 means
	// no IP limit.
	MaxIPFails int `mapstructure:"max_ip_fails"`
	// IPWindow is the IP counter window.
	IPWindow time.Duration `mapstructure:"ip_window"`
	// MaxUserFails is the max failures allowed per account within the window;
	// 0 means no account lockout.
	MaxUserFails int `mapstructure:"max_user_fails"`
	// UserWindow is the account failure counter window.
	UserWindow time.Duration `mapstructure:"user_window"`
	// LockTTL is the account lockout duration.
	LockTTL time.Duration `mapstructure:"lock_ttl"`
}

// JWTConfig is the JWT dual-token configuration (RS256).
//
// Structurally identical to devops-system but **must use a different issuer
// and kid**: when both services share the same jwt-auth consumer, an identical
// issuer would make tokens mutually accepted across services — a B-side token
// could access C-side endpoints and vice versa. In production each holds its
// own private key, with only the public key exposed to the gateway via JWKS
// for verification.
type JWTConfig struct {
	SigningKey    string        `mapstructure:"signing_key"`    // RS256 private key PEM (PKCS#1/PKCS#8)
	KeyID         string        `mapstructure:"key_id"`         // kid header (rotation identifier), defaults to DefaultJWTKeyID
	Issuer        string        `mapstructure:"issuer"`         // issuer, must match the gateway jwt-auth consumers[].issuer
	AccessExpire  time.Duration `mapstructure:"access_expire"`  // access token lifetime
	RefreshExpire time.Duration `mapstructure:"refresh_expire"` // refresh token (session) lifetime
}

// EffectiveKeyID returns the effective kid.
func (c JWTConfig) EffectiveKeyID() string {
	if c.KeyID == "" {
		return DefaultJWTKeyID
	}
	return c.KeyID
}

// SharedJWTConfig maps to the shared JWT config (YAML keys unchanged,
// conversion only). keySet is injected by the caller: in this service's
// self-verification scenario, a StaticKeySet carrying its own public key is
// passed.
func (c JWTConfig) SharedJWTConfig(keySet securityjwt.KeySet) securityjwt.Config {
	return securityjwt.Config{
		PrivateKeyPEM: []byte(c.SigningKey),
		KeyID:         c.EffectiveKeyID(),
		Issuer:        c.Issuer,
		AccessExpire:  c.AccessExpire,
		RefreshExpire: c.RefreshExpire,
		KeySet:        keySet,
	}
}

// IDClientConfig is the devops-id ID generation service client configuration
// (including client prefetch buffering).
type IDClientConfig struct {
	Endpoint    string        `mapstructure:"endpoint"`     // devops-id gRPC address
	Batch       int           `mapstructure:"batch"`        // prefetch batch size (1 = prefetch off, one RPC at a time)
	LowWater    float64       `mapstructure:"low_water"`    // prefetch water level (remaining ratio)
	DialTimeout time.Duration `mapstructure:"dial_timeout"` // dial timeout
	CallTimeout time.Duration `mapstructure:"call_timeout"` // per-call timeout
}

// ObservabilityConfig is the service-level observability configuration;
// ServiceName is derived from nacos.registry.service_name.
type ObservabilityConfig struct {
	OTLPEndpoint   string  `mapstructure:"otlp_endpoint"`   // OTLP HTTP endpoint; empty means local spans are not reported
	SamplerRatio   float64 `mapstructure:"sampler_ratio"`   // root sampling ratio; <=0 is treated as 1.0
	MetricsEnabled bool    `mapstructure:"metrics_enabled"` // whether Prometheus metrics are enabled
}

// APIDocConfig is the OpenAPI documentation gating. BasePath is the doc mount
// path, not the OpenAPI servers URL.
type APIDocConfig struct {
	Enabled  bool   `mapstructure:"enabled"`   // enabled in dev/test only; must be off in prod
	BasePath string `mapstructure:"base_path"` // defaults to /apidoc
}
