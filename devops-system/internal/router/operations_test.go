// withClientMeta regression: URL unescaping of geo-ip plugin injected headers, dirty-header fallback, and XFF real IP resolution.
package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	humagin "github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

	apiv1 "github.com/stanley760/devops/devops-system/internal/api/v1"
	"github.com/stanley760/devops/devops-system/internal/service"
)

// TestWithClientMeta exercises withClientMeta through the real gin+humagin
// adapter layer: the IP comes from the XFF chain (gin trusts all proxies by
// default, which applies directly in test mode), the UA is read as-is, the
// four geo headers are URL-unescaped (Chinese locale strings being the main
// case), and invalid escape sequences fall back to the raw value.
func TestWithClientMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := humagin.New(r, huma.DefaultConfig("meta-echo-test", "v1"))
	huma.Register(api, huma.Operation{
		OperationID: "meta-echo",
		Method:      http.MethodPost,
		Path:        "/meta",
		Middlewares: huma.Middlewares{withClientMeta},
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body service.LoginMeta
	}, error) {
		return &struct{ Body service.LoginMeta }{Body: apiv1.ClientMetaFrom(ctx)}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/meta", nil)
	req.Header.Set("User-Agent", "meta-echo-test/1.0")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	// The geo-ip plugin writes URL-escaped values (Chinese locale strings being the main case)
	req.Header.Set("X-Higress-Geo-Country", "%E4%B8%AD%E5%9B%BD")                    // China
	req.Header.Set("X-Higress-Geo-Province", "%E9%BB%91%E9%BE%99%E6%B1%9F%E7%9C%81") // Heilongjiang Province
	req.Header.Set("X-Higress-Geo-City", "Hangzhou")                                 // pure ASCII, valid even without escaping
	req.Header.Set("X-Higress-Geo-Isp", "%zz-bad-escape")                            // invalid escape sequence: fall back to the raw value

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got service.LoginMeta
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, rec.Body.String())
	}
	if got.IP != "203.0.113.7" {
		t.Errorf("IP = %q, want 203.0.113.7 (XFF chain resolution)", got.IP)
	}
	if got.UserAgent != "meta-echo-test/1.0" {
		t.Errorf("UserAgent = %q", got.UserAgent)
	}
	if got.GeoCountry != "中国" {
		t.Errorf("GeoCountry = %q, want %q (URL unescaping)", got.GeoCountry, "中国")
	}
	if got.GeoProvince != "黑龙江省" {
		t.Errorf("GeoProvince = %q, want %q", got.GeoProvince, "黑龙江省")
	}
	if got.GeoCity != "Hangzhou" {
		t.Errorf("GeoCity = %q, want Hangzhou", got.GeoCity)
	}
	if got.GeoIsp != "%zz-bad-escape" {
		t.Errorf("GeoIsp = %q, want the raw value (invalid escape sequence)", got.GeoIsp)
	}
}
