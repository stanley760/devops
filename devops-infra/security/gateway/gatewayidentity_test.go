package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

func TestIsTrustedPeer(t *testing.T) {
	cases := []struct {
		remote string
		list   []string
		want   bool
	}{
		{"172.18.0.3:54321", []string{"172.16.0.0/12"}, true},
		{"172.18.0.3:54321", []string{"172.18.0.3"}, true},
		{"172.18.0.3", []string{"172.16.0.0/12"}, true},
		{"10.1.2.3:80", []string{"172.16.0.0/12"}, false},
		{"10.1.2.3:80", nil, false},
		{"10.1.2.3:80", []string{""}, false},
		{"not-an-ip", []string{"172.16.0.0/12"}, false},
		{"[::1]:8080", []string{"::1"}, true},
	}
	for _, c := range cases {
		if got := IsTrustedPeer(c.remote, c.list); got != c.want {
			t.Errorf("IsTrustedPeer(%q, %v) = %v，期望 %v", c.remote, c.list, got, c.want)
		}
	}
}

func TestIdentityMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(trusted []string) *gin.Engine {
		r := gin.New()
		r.Use(IdentityMiddleware(trusted))
		r.GET("/probe", func(c *gin.Context) {
			id, ok := securitymw.GatewayIdentityFrom(c.Request.Context())
			if !ok {
				c.JSON(http.StatusOK, gin.H{"identity": ""})
				return
			}
			c.JSON(http.StatusOK, gin.H{"identity": id.Username})
		})
		return r
	}

	headers := http.Header{}
	headers.Set(securitymw.HeaderGatewayUID, "42")
	headers.Set(securitymw.HeaderGatewayUsername, "admin")
	headers.Set(securitymw.HeaderGatewaySessionID, "sid-1")
	headers.Set(securitymw.HeaderGatewayTenantID, "7")

	// 可信网关的请求 → 身份被提取
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "172.18.0.3:1234"
	req.Header = headers
	w := httptest.NewRecorder()
	newRouter([]string{"172.16.0.0/12"}).ServeHTTP(w, req)
	if w.Body.String() != `{"identity":"admin"}` {
		t.Fatalf("可信网关的身份没有被提取: %s", w.Body.String())
	}

	// 不可信对端（伪造身份头）→ 身份不被提取
	req = httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	req.Header = headers
	w = httptest.NewRecorder()
	newRouter([]string{"172.16.0.0/12"}).ServeHTTP(w, req)
	if w.Body.String() != `{"identity":""}` {
		t.Fatalf("不可信对端的身份被提取了: %s", w.Body.String())
	}

	// 可信网关但头不完整 → 不提取（宁可没有身份，也不能半截身份）
	req = httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "172.18.0.3:1234"
	w = httptest.NewRecorder()
	newRouter([]string{"172.16.0.0/12"}).ServeHTTP(w, req)
	if w.Body.String() != `{"identity":""}` {
		t.Fatalf("头不完整时提取了身份: %s", w.Body.String())
	}
}
