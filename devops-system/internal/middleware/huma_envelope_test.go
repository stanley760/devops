package middleware

import (
	"errors"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
	"net/http"
	"strings"
	"testing"
)

// TestHumaEnvelopeWriters: the envelope callback factory table-driven golden standard: asserts each scenario's complete JSON body and status code.
func TestHumaEnvelopeWriters(t *testing.T) {
	cases := []struct {
		name   string
		writer func(huma.Context, error)
		err    error
		status int
		body   string
	}{
		{"401 no token", writeHuma401, securitymw.ErrUnauthorized, http.StatusUnauthorized, `{"code":401,"msg":"not logged in or session expired","data":null}`},
		{"401 session invalid (wrapped)", writeHuma401, fmt.Errorf("session: %w", securitymw.ErrSessionInactive), http.StatusUnauthorized, `{"code":401,"msg":"login state invalid, please log in again","data":null}`},
		{"401 authentication infrastructure error", writeHumaAuthInfraErr, errors.New("redis down"), http.StatusUnauthorized, `{"code":401,"msg":"login state invalid, please log in again","data":null}`},
		{"403 tenant unavailable", writeHuma403Tenant, nil, http.StatusForbidden, `{"code":403,"msg":"tenant is unavailable (disabled or expired)","data":null}`},
		{"403 in-domain authorization denied", writeHuma403Perm, nil, http.StatusForbidden, `{"code":403,"msg":"no operation permission","data":null}`},
		{"403 not platform admin (wrapped)", writeHuma403Platform, fmt.Errorf("gate: %w", securitymw.ErrNotPlatformAdmin), http.StatusForbidden, `{"code":403,"msg":"platform administrator permission required","data":null}`},
		{"403 platform Enforce denied", writeHuma403Platform, securitymw.ErrForbidden, http.StatusForbidden, `{"code":403,"msg":"no operation permission","data":null}`},
		{"403 authorization infrastructure error", writeHumaAuthzInfraErr, errors.New("casbin down"), http.StatusForbidden, `{"code":403,"msg":"no operation permission","data":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newMWAPI(t, func(hctx huma.Context, _ func(huma.Context)) { tc.writer(hctx, tc.err) })
			resp := api.Get("/")
			if resp.Code != tc.status {
				t.Fatalf("status: want %d got %d %s", tc.status, resp.Code, resp.Body.String())
			}
			if got := strings.TrimSpace(resp.Body.String()); got != tc.body {
				t.Fatalf("body: want %s got %s", tc.body, got)
			}
		})
	}
}
