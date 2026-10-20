// Package gateway provides the application-layer guard that forces traffic to enter through the gateway.
package gateway

import (
	"log/slog"
	"net/http"

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
type Config struct {
	// RequireJWT enables the guard when true; passes through when false.
	RequireJWT bool `mapstructure:"require_jwt"`
	// ExemptPaths appends exact paths exempt from verification.
	ExemptPaths []string `mapstructure:"exempt_paths"`
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
func EnforceFromConfig(cfg Config, authn securitymw.BearerAuthenticator, exemptPaths, exemptPrefixes []string) gin.HandlerFunc {
	return Enforce(cfg.RequireJWT, authn, append(append([]string{}, exemptPaths...), cfg.ExemptPaths...), exemptPrefixes)
}
