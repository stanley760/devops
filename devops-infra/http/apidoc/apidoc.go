// Package apidoc provides unified OpenAPI documentation assembly on top of huma/v2.
//
// Design goals:
//   - Docs as code: the OpenAPI 3.1 schema is derived directly from Go struct tags,
//     with no swag annotations, no `swag init` generation step, and no docs directory to keep in sync
//   - Reuse the existing Gin router: the humagin adapter runs directly on the passed
//     gin.IRouter, so business routes, response envelopes, and the tenant/JWT chain stay untouched
//   - Progressive migration: new endpoints always use huma.Register; legacy handlers can
//     be wrapped via humagin.Wrap to add documentation and migrated at each iteration's pace
//
// One New call provides three things:
//
//	-huma.API: business-side huma.Register for endpoints (routes + validation + docs in one)
//	- OpenAPI JSON: GET {DocsPath}/openapi.json (default /apidoc/openapi.json)
//	- Docs UI: GET {DocsPath} (huma's built-in Stoplight Elements interactive docs,
//
// It must be mounted before the auth group (public routes); whether it is enabled is
// gated by the caller per environment (on in dev/test, off in prod; see each
// service's apidoc.enabled config).
package apidoc

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Options configures docs assembly.
type Options struct {
	// Title is the docs title, e.g. "devops-system API"; defaults to "API".
	Title string
	// Version is the docs version; defaults to "v1".
	Version string
	// BasePath is the public prefix of the business API, used to derive the OpenAPI
	// servers address (e.g. the gateway domain). It only affects the request URL shown
	// in the docs; defaults to empty (relative path).
	// ServerURL is the OpenAPI servers[0] address (e.g. https://ops.example.com).
	ServerURL string
	// BasePath is a deprecated compatibility alias for ServerURL; new code should use ServerURL.
	BasePath string
	// DocsPath is the docs mount prefix; defaults to /apidoc.
	// OpenAPI JSON lives at {DocsPath}/openapi.json, the UI at {DocsPath}.
	DocsPath string
	// BearerAuth declares a JWT Bearer auth scheme in the OpenAPI Components.
	// Once declared, the docs page can run requests with a token directly; defaults to false.
	BearerAuth bool
	// TenantHeader is the tenant header name (e.g. X-Tenant-ID). When non-empty, a
	// reusable apiKey security scheme is declared in the OpenAPI Components so the
	// docs page can debug with tenant context attached.
	TenantHeader string
	// Security is the global security requirement (OpenAPI security field), e.g.
	// []map[string][]string{{"bearerAuth": {}}}. Once set, all operations require
	// authentication by default; public endpoints (login/refresh etc.) override it
	// with an empty list in their own Operation.
	Security []map[string][]string
	// Enabled mounts the docs endpoints (/apidoc UI, openapi.json, /schemas).
	// When false, a programmatically exportable huma.API is still returned (routes
	// can always be registered); only the docs HTTP endpoints are not registered —
	// D5: gating affects docs only, no longer route registration.
	Enabled bool
}

// Defaults fills in missing values and returns a normalized copy.
func (o Options) Defaults() Options {
	if o.Title == "" {
		o.Title = "API"
	}
	if o.Version == "" {
		o.Version = "v1"
	}
	if o.DocsPath == "" {
		o.DocsPath = "/apidoc"
	}
	return o
}

// New creates a huma API on the existing Gin router, mounts the docs endpoints,
// and returns the huma.API for business-side huma.Register.
//
// Usage (inside the router.Register callback, before the auth group):
//
//	api := apidoc.New(r, apidoc.Options{Title: "devops-system API", BearerAuth: true})
//	v1.RegisterUsers(api, userH) // new endpoints use huma.Register
func New(r gin.IRouter, opts Options) huma.API {
	opts = opts.Defaults()

	cfg := huma.DefaultConfig(opts.Title, opts.Version)
	// Disable the SchemaLinkTransformer mounted by DefaultConfig (assembled inline in
	// CreateHooks, executed at api.go:513): it injects a $schema field and a Link header
	// into struct responses, breaking the envelope contract that must stay byte-identical
	// with the Gin version. Docs are carried by openapi.json alone; schema links are unnecessary.
	cfg.CreateHooks = nil
	// huma defaults to DocsPath=/docs and OpenAPIPath=/openapi (no extension; huma appends
	// .json/.yaml via content negotiation). Move both under the DocsPath the business agrees on.
	// When Enabled=false, all three paths are blanked: huma only skips endpoint registration
	// (api.go gates on this field independently), the OpenAPI can still be exported
	// programmatically via api.OpenAPI(); SchemasPath must be blanked too, otherwise the
	// /schemas/{schema} routes would still be registered.
	if !opts.Enabled {
		cfg.DocsPath = ""
		cfg.OpenAPIPath = ""
		cfg.SchemasPath = ""
	} else {
		cfg.DocsPath = opts.DocsPath
		cfg.OpenAPIPath = opts.DocsPath + "/openapi"
	}
	serverURL := opts.ServerURL
	if serverURL == "" {
		serverURL = opts.BasePath
	}
	if serverURL != "" {
		cfg.Servers = append(cfg.Servers, &huma.Server{
			URL:         serverURL,
			Description: "service endpoint",
		})
	}
	if opts.BearerAuth || opts.TenantHeader != "" {
		if cfg.Components == nil {
			cfg.Components = &huma.Components{}
		}
		if cfg.Components.SecuritySchemes == nil {
			cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
		}
	}
	if opts.BearerAuth {
		cfg.Components.SecuritySchemes["bearerAuth"] = &huma.SecurityScheme{
			Type:         "http",
			Scheme:       "bearer",
			BearerFormat: "JWT",
		}
	}
	if opts.TenantHeader != "" {
		cfg.Components.SecuritySchemes["tenantId"] = &huma.SecurityScheme{
			Type:        "apiKey",
			Name:        opts.TenantHeader,
			In:          "header",
			Description: "multi-tenant context identifier",
		}
	}
	if len(opts.Security) > 0 {
		cfg.Security = opts.Security
	}

	api := humagin.New(mustEngine(r), cfg)

	// OpenAPI JSON export endpoint: consumed by frontend code generation, gateway route
	// discovery, and CI contract tests.
	// huma already mounts openapi.json; an explicit semantic alias is registered here to
	// keep the external convention close to the legacy /apidoc/doc.json (frontend changes
	// only the suffix during migration).
	// Gated by the same flag as the docs endpoint (not registered when Enabled=false).
	if opts.Enabled {
		huma.Register(api, huma.Operation{
			OperationID: "export-openapi",
			Method:      http.MethodGet,
			Path:        opts.DocsPath + "/doc.json",
			Summary:     "Export the OpenAPI 3.1 document",
			Tags:        []string{"apidoc"},
		}, func(ctx context.Context, _ *docInput) (*openapiDocOutput, error) {
			return &openapiDocOutput{Body: api.OpenAPI()}, nil
		})
	}

	return api
}

// mustEngine narrows gin.IRouter down to the *gin.Engine humagin needs.
// A *gin.Engine or a gin.RouterGroup (taking its underlying Engine) may be passed.
func mustEngine(r gin.IRouter) *gin.Engine {
	if e, ok := r.(*gin.Engine); ok {
		return e
	}
	if g, ok := r.(*gin.RouterGroup); ok {
		// gin's RouterGroup holds a read-only base path that cannot be used to get the
		// Engine directly; however the business side mounts docs on the Engine root
		// (public route group) by convention, so this branch is purely defensive.
		// The RouterGroup's group Engines field is not exported, so an Engine is required.
		_ = g
	}
	panic("apidoc: the humagin adapter requires a *gin.Engine (mount docs on the gin.Engine root router)")
}

// docInput is the docs export endpoint input (no parameters).
type docInput struct{}

// openapiDocOutput is the OpenAPI docs export response body.
type openapiDocOutput struct {
	Body any
}
