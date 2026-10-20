package apidoc_test

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
	apidoc "github.com/stanley760/devops/devops-infra/http/apidoc"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// pingOutput is the ping output.
type pingOutput struct {
	Body struct {
		Pong bool `json:"pong" doc:"whether the service is alive"`
	}
}

// registerPing registers a minimal operation on the docs to verify business registration is visible.
func registerPing(t *testing.T, api huma.API) {
	t.Helper()
	huma.Register(api, huma.Operation{
		OperationID: "ping",
		Method:      http.MethodGet,
		Path:        "/ping",
		Summary:     "health probe",
		Tags:        []string{"test"},
	}, func(ctx context.Context, _ *pingInput) (*pingOutput, error) {
		out := &pingOutput{}
		out.Body.Pong = true
		return out, nil
	})
}

// pingInput is the ping input (no parameters).
type pingInput struct{}

func TestNew_DefaultPaths(t *testing.T) {
	r := newEngine()
	api := apidoc.New(r, apidoc.Options{Title: "test API", Version: "v1", Enabled: true})
	registerPing(t, api)

	// docs UI is reachable
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("docs UI: got %d, want 200", w.Code)
	}

	// openapi.json is reachable
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}

	// the registered business operation appears in the OpenAPI document
	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	if _, ok := paths["/ping"]; !ok {
		t.Fatalf("openapi.json is missing the registered operation /ping, paths=%v", paths)
	}
	// title and version
	info, _ := doc["info"].(map[string]any)
	if info["title"] != "test API" || info["version"] != "v1" {
		t.Fatalf("info mismatch: %v", info)
	}
}

func TestNew_DocJSONAlias(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{Enabled: true})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/doc.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("doc.json alias: got %d, want 200", w.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("doc.json is not valid JSON: %v", err)
	}
}

func TestNew_CustomDocsPath(t *testing.T) {
	r := newEngine()
	api := apidoc.New(r, apidoc.Options{DocsPath: "/api-docs", Enabled: true})
	registerPing(t, api)

	for _, path := range []string{"/api-docs", "/api-docs/openapi.json", "/api-docs/doc.json"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want 200", path, w.Code)
		}
	}
}

func TestNew_BasePathServer(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{ServerURL: "https://ops.example.com", Enabled: true})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "https://ops.example.com") {
		t.Fatalf("openapi.json servers does not contain BasePath: %s", w.Body.String())
	}
}

func TestNew_BasePathLegacyServerAlias(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{BasePath: "https://legacy.example.com", Enabled: true})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "https://legacy.example.com") {
		t.Fatalf("openapi.json servers does not contain the compatibility BasePath: %s", w.Body.String())
	}
}

func TestNew_BearerAuth(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{
		BearerAuth: true,
		Security:   []map[string][]string{{"bearerAuth": {}}},
		Enabled:    true,
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "bearerAuth") {
		t.Fatalf("openapi.json does not declare the bearerAuth security scheme")
	}
	if !strings.Contains(body, `"bearerFormat":"JWT"`) && !strings.Contains(body, `"bearerFormat": "JWT"`) {
		t.Fatalf("openapi.json does not declare the JWT bearerFormat")
	}
}

func TestMount_Deprecated(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{Title: "mounted", Enabled: true})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}
}

// TestNew_Disabled docs gating (D5): when Enabled=false the docs endpoints and /schemas
// all return 404, while huma route registration still works and the OpenAPI can still be
// exported programmatically (route registration is no longer affected by Enabled).
func TestNew_Disabled(t *testing.T) {
	r := newEngine()
	api := apidoc.New(r, apidoc.Options{}) // Enabled zero value is false
	registerPing(t, api)

	for _, path := range []string{"/apidoc", "/apidoc/openapi.json", "/apidoc/doc.json", "/schemas/Ping"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: got %d, want 404", path, w.Code)
		}
	}

	// business routes can still be registered via huma and respond normally
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/ping: got %d, want 200 (Enabled=false must not affect route registration)", w.Code)
	}

	// the programmatic export remains complete
	paths := api.OpenAPI().Paths
	if _, ok := paths["/ping"]; !ok {
		t.Fatalf("the programmatic export is missing /ping, paths=%v", paths)
	}
}

func TestDefaults_Fill(t *testing.T) {
	got := apidoc.Options{}.Defaults()
	if got.Title != "API" || got.Version != "v1" || got.DocsPath != "/apidoc" {
		t.Fatalf("Defaults fill mismatch: %+v", got)
	}
}

func TestNewTenantHeader(t *testing.T) {
	r := newEngine()
	_ = apidoc.New(r, apidoc.Options{
		TenantHeader: "X-Tenant-ID",
		Enabled:      true,
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("openapi.json: got %d, want 200", w.Code)
	}

	var doc struct {
		Components struct {
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	scheme, ok := doc.Components.SecuritySchemes["tenantId"]
	if !ok {
		t.Fatalf("openapi.json is missing the tenantId security scheme: %+v", doc.Components.SecuritySchemes)
	}
	if scheme.Type != "apiKey" || scheme.Name != "X-Tenant-ID" || scheme.In != "header" {
		t.Fatalf("tenantId security scheme mismatch: %+v", scheme)
	}
}
