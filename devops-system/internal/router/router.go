// Package router registers the business routes of devops-system.
//
// Division of labor: Huma leads all business interfaces (operations.go registers by module via
// huma.Register, combining routing/validation/business middleware/docs in one); Gin is only the
// foundation (HTTP engine, global middleware, /healthz /readyz /metrics /jwks basic endpoints,
// all assembled by devops-infra/http and this file).
package router

import (
	"github.com/gin-gonic/gin"
	httpinfra "github.com/stanley760/devops/devops-infra/http"
	"github.com/stanley760/devops/devops-infra/http/apidoc"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/config"
	"net/http"
)

// NewRegister combines the business module routes provided by the application layer and returns
// the registration callback handed to httpinfra.Run. Modules and their dependencies are
// assembled by internal/app; infra does not depend on any business type.
func NewRegister(docCfg config.APIDocConfig, authM *auth.Manager, modules Modules) httpinfra.Register {
	return func(r *gin.Engine) {
		// verification public key (RFC 7517 JWKS): for other services to self-verify and manual troubleshooting.
		// Note: the Higress jwt-auth plugin has no jwksFromUrl field; the public key is inlined in
		// wasmplugins/jwt-auth-2.0.0.yaml, so this endpoint is useless for gateway verification.
		// (the wasm-go implementation supports remote_jwks, which can point to this path to auto-follow
		//  rotation, but that service must first be reachable in McpBridge, and requests within the
		//  fetch-failure window will be rejected fail-close.)
		r.GET(JWKSPath, jwksHandler(authM))

		// the huma API is always assembled (D5): Enabled only gates the docs endpoint; route registration is unaffected.
		humaAPI := apidoc.New(r, apidoc.Options{
			Title:        "devops-system API",
			Version:      "v1",
			DocsPath:     docCfg.BasePath,
			BearerAuth:   true,
			TenantHeader: "X-Tenant-ID",
			Security:     []map[string][]string{{"bearerAuth": {}, "tenantId": {}}},
			Enabled:      docCfg.Enabled,
		})

		// each business module has its own Handler, Service, and middleware dependencies.
		modules.Auth(humaAPI)
		modules.Users(humaAPI)
		modules.Roles(humaAPI)
		modules.Menus(humaAPI)
		modules.Depts(humaAPI)
		modules.Posts(humaAPI)
		modules.Tenants(humaAPI)
		modules.LoginLogs(humaAPI)
		modules.OperationLogs(humaAPI)
		modules.Dicts(humaAPI)
		modules.Configs(humaAPI)
	}
}

// jwksHandler outputs the JWKS document of the RS256 verification public key (public key material only, n/e).
func jwksHandler(m *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		doc, err := m.JWKS()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "jwks unavailable"})
			return
		}
		c.JSON(http.StatusOK, doc)
	}
}
