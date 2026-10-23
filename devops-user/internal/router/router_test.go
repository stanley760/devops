package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stanley760/devops/devops-infra/http/apidoc"
	"github.com/stanley760/devops/devops-infra/security/gateway"

	apiv1 "github.com/stanley760/devops/devops-user/internal/api/v1"
	"github.com/stanley760/devops/devops-user/internal/middleware"
)

// TestExposePolicyMatchesRoutes regression: the gateway exposure policy in infra
// and the actually registered API surface must not drift.
//
//   - The allowlist has an entry, but no matching anonymous endpoint is registered:
//     the endpoint was renamed or switched to requiring a login, and the
//     allowlist is now a dead entry (or, worse, a hole);
//   - The endpoint is anonymous, but the allowlist has no entry: the gateway and
//     the application-layer guard block it, the user gets a 401, and the list
//     still looks "configured" — the most hard-to-diagnose direction.
//
// This is exactly the drift that was hand-maintained before: the allowlist in
// cmd/root.go and the paths in deploy/higress/ingresses/devops-user-public.yaml
// were two separate copies, and the OAuth callback had already fallen out of
// sync (the gateway let it through while the application layer returned 401).
func TestExposePolicyMatchesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	kit := middleware.NewKit(nil, middleware.Deps{})
	api := apidoc.New(r, apidoc.Options{
		Title:      "devops-user API",
		Version:    "v1",
		DocsPath:   "/apidoc",
		BearerAuth: true,
		Security:   []map[string][]string{{"bearerAuth": {}}},
		Enabled:    true,
	})
	// The handlers are never invoked during registration, so empty services are enough:
	// what matters here is the routing table (path / method / whether it's anonymous).
	registerAuth(api, apiv1.NewAuthHandler(nil, nil), kit)
	registerProfile(api, apiv1.NewProfileHandler(nil), kit)
	registerOAuth(api, apiv1.NewOAuthHandler(nil), kit)

	docResponse := httptest.NewRecorder()
	r.ServeHTTP(docResponse, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if docResponse.Code != http.StatusOK {
		t.Fatalf("openapi.json status = %d", docResponse.Code)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Security *[]map[string][]any `json:"security"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(docResponse.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	var ops []gateway.ExposedOp
	for path, methods := range doc.Paths {
		for method, op := range methods {
			// An empty "security": [] means explicitly anonymous; a missing field
			// means the operation inherits the global bearerAuth requirement.
			ops = append(ops, gateway.ExposedOp{
				Path:      path,
				Method:    method,
				Anonymous: op.Security != nil && len(*op.Security) == 0,
			})
		}
	}
	if len(ops) == 0 {
		t.Fatal("the OpenAPI document did not yield a single endpoint — the check below would pass vacuously")
	}
	expose := loadServiceExpose(t)
	// 主机级端点（绝对路径）不是 huma 注册的接口，不在 OpenAPI 文档里，
	// 所以单独断言它与服务实际挂载的 JWKS 路径一致。
	var absolute []string
	for _, e := range expose.Endpoints {
		if e.Absolute {
			absolute = append(absolute, e.Path)
		}
	}
	if len(absolute) != 1 || absolute[0] != JWKSPath {
		t.Fatalf("主机级白名单 = %v，期望恰好是 JWKS 路径 %s", absolute, JWKSPath)
	}

	values := map[string][]string {
		"provider": {"github", "google"},
	}
	if err := gateway.CheckExpose(expose, ops, values); err != nil {
		t.Fatalf("网关暴露白名单与实际注册的接口不一致: %v", err)
	}
}

// loadServiceExpose 读取本服务 config/bootstrap.yaml 里的 gateway.expose 段。
//
// 白名单与 API 面是同一份代码事实，所以测试比对的正是「配置里声明为匿名的端点」与
// 「实际注册为匿名的端点」——任何一边漏改，测试立刻失败。
func loadServiceExpose(t *testing.T) gateway.ExposeConfig {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("取工作目录失败: %v", err)
	}
	for i := 0; i < 6; i++ {
		p := filepath.Join(dir, "config", "bootstrap.yaml")
		if _, err := os.Stat(p); err == nil {
			f, err := os.Open(p)
			if err != nil {
				t.Fatalf("打开 %s 失败: %v", p, err)
			}
			defer f.Close()
			cfg, err := gateway.LoadExposeConfig(f)
			if err != nil {
				t.Fatalf("读取 %s 的白名单失败: %v", p, err)
			}
			if err := cfg.Validate(); err != nil {
				t.Fatalf("%s 的白名单不合法: %v", p, err)
			}
			return cfg
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("找不到 config/bootstrap.yaml")
	return gateway.ExposeConfig{}
}
