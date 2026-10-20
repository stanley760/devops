// Package router registers devops-user's (C-side) business routes.
//
// Division of labor: Huma drives all endpoints (operations.go calls
// huma.Register per module, combining routing/validation/business
// middleware/docs in one); Gin is only the substrate (HTTP engine, global
// middleware, and the /healthz /readyz /metrics /jwks base endpoints, all
// wired up by devops-infra/http and this file).
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpinfra "github.com/stanley760/devops/devops-infra/http"
	"github.com/stanley760/devops/devops-infra/http/apidoc"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/config"
)

// NewRegister composes the business module routes provided by the application
// layer and returns the registration callback handed to httpinfra.Run.
// Modules and their dependencies are wired by cmd/root.go; this package depends
// on no business implementation types.
func NewRegister(docCfg config.APIDocConfig, authM *auth.Manager, modules Modules) httpinfra.Register {
	return func(r *gin.Engine) {
		// Verification public key (RFC 7517 JWKS): for other services'
		// self-verification and manual debugging. Note that Higress jwt-auth
		// has no jwksFromUrl field; the public key is inlined in its plugin
		// yaml, so this endpoint is useless for gateway verification (the
		// wasm-go implementation can point remote_jwks at it to follow
		// rotation automatically).
		r.GET(JWKSPath, jwksHandler(authM))

		// The huma API is always wired up: Enabled only gates the doc
		// endpoint; business route registration is unaffected.
		humaAPI := apidoc.New(r, apidoc.Options{
			Title:      "devops-user API",
			Version:    "v1",
			DocsPath:   docCfg.BasePath,
			BearerAuth: true,
			Security:   []map[string][]string{{"bearerAuth": {}}},
			Enabled:    docCfg.Enabled,
		})

		// Each business module has its own Handler, Service, and middleware
		// dependencies.
		modules.Auth(humaAPI)
		modules.Profile(humaAPI)
		modules.OAuth(humaAPI)
	}
}

// jwksHandler outputs the RS256 verification public key as a JWKS document
// (public key material only, n/e).
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
