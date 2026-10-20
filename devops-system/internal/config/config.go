// Package config defines the configuration structures for the devops-system
// service.
//
// The loading mechanism reuses devops-infra/config (base + APP_ENV
// environment override merging); this package only defines its own root
// Config and service-specific sections.
package config

import (
	"time"

	"github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/security/gateway"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
)

// DefaultJWTKeyID is the default kid when key_id is not configured (consistent between the token header and JWKS).
const DefaultJWTKeyID = "devops-system-rs256-1"

// Config is the devops-system service root config, corresponding to the top-level structure of config/config.yaml
type Config struct {
	Server        config.ServerConfig `mapstructure:"server"`
	Log           config.LogConfig    `mapstructure:"log"`
	Mysql         config.MysqlConfig  `mapstructure:"mysql"`
	Redis         config.RedisConfig  `mapstructure:"redis"`
	JWT           JWTConfig           `mapstructure:"jwt"`
	Tenant        TenantConfig        `mapstructure:"tenant"`
	IDClient      IDClientConfig      `mapstructure:"idclient"`
	Nacos         config.NacosConfig  `mapstructure:"nacos"`
	Observability ObservabilityConfig `mapstructure:"observability"`
	APIDoc        APIDocConfig        `mapstructure:"apidoc"`
	Gateway       gateway.Config      `mapstructure:"gateway"`
}

// JWTConfig is the JWT dual-token config.
//
// The access token has been upgraded from HS256 to RS256 + kid (existing
// HS256 tokens all become invalid on upgrade; refresh sessions are
// unaffected). The signing_key semantics changed accordingly to an RSA
// private key PEM (PKCS#1/PKCS#8), held by configuration/secret
// management; the public key is exposed via JWKS to Higress and the
// services for verification.
type JWTConfig struct {
	SigningKey    string        `mapstructure:"signing_key"`    // RS256 private key PEM
	KeyID         string        `mapstructure:"key_id"`         // kid header (rotation identifier); defaults to DefaultJWTKeyID
	Issuer        string        `mapstructure:"issuer"`         // issuer
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

// SharedJWTConfig maps to the shared JWT config (the YAML keys are
// unchanged; this is a pure conversion). The keySet is injected by the
// caller: the self-verifying service scenario passes a StaticKeySet holding
// its own public key, while the multi-issuer/rotation scenario passes a
// JWKS-backed implementation.
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

// TenantConfig is the single-database multi-tenant config.
type TenantConfig struct {
	DefaultCode string `mapstructure:"default_code"` // default tenant code when no tenant is specified at login
}

// IDClientConfig is the devops-id issuance-service client config (including the client prefetch buffer)
type IDClientConfig struct {
	Endpoint    string        `mapstructure:"endpoint"`     // devops-id gRPC address
	Batch       int           `mapstructure:"batch"`        // prefetch batch size (1 = prefetch disabled, one RPC per call)
	LowWater    float64       `mapstructure:"low_water"`    // prefetch low-water mark (remaining ratio)
	DialTimeout time.Duration `mapstructure:"dial_timeout"` // dial timeout
	CallTimeout time.Duration `mapstructure:"call_timeout"` // per-call timeout
}

// ObservabilityConfig is the service-level observability config; ServiceName is derived from nacos.registry.service_name.
type ObservabilityConfig struct {
	OTLPEndpoint   string  `mapstructure:"otlp_endpoint"`   // OTLP HTTP exporter; empty means local spans are not reported
	SamplerRatio   float64 `mapstructure:"sampler_ratio"`   // root sampling ratio; <=0 is treated as 1.0
	MetricsEnabled bool    `mapstructure:"metrics_enabled"` // whether to enable Prometheus metrics
}

// APIDocConfig is the OpenAPI documentation gate. BasePath is the documentation mount path, not an OpenAPI servers URL.
type APIDocConfig struct {
	Enabled  bool   `mapstructure:"enabled"`   // enabled in dev/test only; must be disabled in prod
	BasePath string `mapstructure:"base_path"` // defaults to /apidoc
}
