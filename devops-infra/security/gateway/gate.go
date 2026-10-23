// Package gateway provides the application-layer guard that forces traffic to enter through the gateway.
package gateway

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	commonResp "github.com/stanley760/devops/devops-common/response"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// HeaderTenant is the tenant header injected by the gateway's jwt-auth claims_to_headers
// (override semantics). This guard only enforces "a valid JWT must be present" and does
// not trust the header — tenant identity is always taken from the token claims.
const HeaderTenant = "X-Tenant-ID"

// Options configures the guard.
type Options struct {
	// RequireJWT enables the guard when true; passes through when false.
	RequireJWT bool
	// Authenticator provides independent verification of Bearer JWTs (signature + session state).
	// When nil and RequireJWT is true, it fails closed.
	Authenticator securitymw.BearerAuthenticator
	// ExemptPaths are paths exempt from verification (exact match), such as health check and metrics endpoints.
	ExemptPaths []string
	// ExemptPrefixes are path prefixes exempt from verification (exact match), such as /apidoc.
	ExemptPrefixes []string
}

// Config is mapped from a business config section, making it easy to switch per environment.
//
// Note: there is deliberately no whitelist entry here. The exempt list is not configurable —
// it is maintained as a single policy in infra
// (devops-infra/security/gateway/exposed-endpoints.yaml), which the application-layer guard
// reads and the Higress public ingress is generated from.
//
// The reason it is not configurable: an exemption that exists only in the application layer
// does not expose any new endpoint (the gateway still requires a JWT there) — it only means
// "the gateway already verified the token, skip the second verification in the application
// layer". That is a performance/defense-in-depth trade knob, not an access-control switch,
// and the exemption would be naked whenever a container is reached directly bypassing the
// gateway (the compose form has no NetworkPolicy).
type Config struct {
	// RequireJWT enables the application-layer coarse guard. Only meaningful in JWT mode:
	// in gateway mode the gateway is the only verifier and there is no coarse guard here.
	RequireJWT bool `mapstructure:"require_jwt"`
	// IdentitySource decides where the caller's identity comes from. See the
	// IdentitySource* constants. Empty means gateway mode.
	IdentitySource string `mapstructure:"identity_source"`
	// Expose is this service's gateway exposure contract: the endpoints the gateway
	// must let through without JWT verification.
	//
	// It is declared in the service's own bootstrap.yaml (gateway.expose) — each service
	// owns its own list, and the Higress public ingress is rendered from it, so the
	// gateway side and the application layer cannot drift apart.
	Expose ExposeConfig `mapstructure:"expose"`
}

// The identity source modes.
const (
	// IdentitySourceGateway: the gateway is the only JWT verifier. It verifies the signature
	// and passes the verified claims down in headers; this service takes the identity from
	// those headers (no signature verification here) and only checks that the session is
	// still active — so kicking a user offline still takes effect immediately.
	//
	// The premise is network isolation: the backend must be reachable only from the gateway
	// (K8s NetworkPolicy, or the dedicated compose network). Requests that did not come
	// through the gateway carry no identity and are rejected with 401.
	IdentitySourceGateway = "gateway"
	// IdentitySourceJWT: the caller presents a Bearer JWT and this service verifies it
	// itself. Legacy path, for environments where the gateway is not in front.
	IdentitySourceJWT = "jwt"
)

// ResolveIdentitySource returns the configured identity source, defaulting to gateway mode.
//
// Gateway mode is the default: a service that is not explicitly told otherwise must not
// verify signatures itself, because the whole point is that the JWT is verified exactly once,
// at the gateway.
func ResolveIdentitySource(cfg Config) string {
	src := strings.TrimSpace(cfg.IdentitySource)
	if src == "" {
		return IdentitySourceGateway
	}
	return src
}

// ExemptPathsFor returns the exempt list of a service: the probe endpoints plus the
// endpoints the service declared anonymous in its own bootstrap.yaml.
//
// Used by the legacy JWT mode's coarse guard; in gateway mode the gateway's own ingress
// (rendered from the same list) is what exempts those paths.
func ExemptPathsFor(cfg Config) []string {
	paths := cfg.Expose.ExemptPaths()
	out := make([]string, 0, len(DefaultProbePaths)+len(paths))
	out = append(out, DefaultProbePaths...)
	out = append(out, paths...)
	return out
}

// Enforce returns the traffic entrypoint guard middleware: requests must carry a Bearer
// JWT that passes independent verification.
//
// When requireJWT is false it returns a pass-through middleware, so environments where the
// guard is not enabled (local go run, unit tests) need no extra configuration and will not
// end up returning 401 site-wide because of a missing setting.
func Enforce(requireJWT bool, authn securitymw.BearerAuthenticator, exemptPaths, exemptPrefixes []string) gin.HandlerFunc {
	if !requireJWT {
		slog.Info("gateway guard not enabled (gateway.require_jwt=false), no mandatory verification at the traffic entrypoint")
		return func(c *gin.Context) { c.Next() }
	}
	if authn == nil {
		// fail-closed: when the guard is declared enabled but no verifier is assembled,
		// it must reject; otherwise it amounts to letting everything through site-wide,
		// making the guard useless with no alert at all.
		slog.Error("gateway guard is enabled but no JWT verifier is assembled; all non-exempt requests will be rejected")
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, commonResp.BaseResponse[any]{
				Code: http.StatusUnauthorized,
				Msg:  "requests must be made through the gateway",
			})
		}
	}

	exact := make(map[string]struct{}, len(exemptPaths))
	for _, p := range exemptPaths {
		if p != "" {
			exact[p] = struct{}{}
		}
	}
	prefixes := make([]string, 0, len(exemptPrefixes))
	for _, p := range exemptPrefixes {
		if p != "" {
			prefixes = append(prefixes, p)
		}
	}

	// Precompile the exemption check to avoid re-scanning prefixes on every request.
	isExempt := func(path string) bool {
		if _, ok := exact[path]; ok {
			return true
		}
		for _, p := range prefixes {
			if len(path) >= len(p) && path[:len(p)] == p {
				return true
			}
		}
		return false
	}

	return func(c *gin.Context) {
		if isExempt(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Do not trust the tenant header injected by the gateway: tenant identity is always
		// taken from the token claims, to avoid privilege escalation when the gateway is
		// misconfigured or bypassed. Only record it here, to help diagnose whether injection works.
		_, err := authn.Authenticate(c.Request.Context(), c.GetHeader("Authorization"))
		if err != nil {
			// WARN rather than ERROR: it is usually direct traffic that bypassed the gateway
			// (internal liveness probe without a gateway, an ops mistake), suspicious but not
			// necessarily an attack. Log enough fields to locate it, but not the credential itself.
			slog.Warn("rejected a request without valid credentials",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP(),
				"gateway_tenant", c.GetHeader(HeaderTenant),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, commonResp.BaseResponse[any]{
				Code: http.StatusUnauthorized,
				Msg:  "requests must be made through the gateway",
			})
			return
		}
		c.Next()
	}
}

// EnforceFromConfig assembles the guard from business config. Passes through when RequireJWT is false.
//
// The exempt list is not taken from the config: it comes from the caller (the exposure
// endpoint policy). See Config for why it is deliberately not configurable.
func EnforceFromConfig(cfg Config, authn securitymw.BearerAuthenticator, exemptPaths, exemptPrefixes []string) gin.HandlerFunc {
	return Enforce(cfg.RequireJWT, authn, exemptPaths, exemptPrefixes)
}

// DefaultProbePaths 是探针与监控端点：容器编排的存活/就绪探针、Prometheus 抓取。
//
// 它们不经网关，但直连也要放行——否则容器健康检查一直失败、监控抓不到数据。
// 放在 infra 是因为每个服务都是同一组。
var DefaultProbePaths = []string{"/healthz", "/readyz", "/metrics"}
