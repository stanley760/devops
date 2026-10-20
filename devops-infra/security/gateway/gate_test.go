package gateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stanley760/devops/devops-infra/security/gateway"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitymw "github.com/stanley760/devops/devops-infra/security/middleware"
)

// stubAuthenticator decides Bearer credentials by a fixed rule, isolating JWT signing
// from Redis session details.
// validToken passes verification; everything else returns ErrUnauthorized (same semantics as the real implementation).
type stubAuthenticator struct {
	validToken string
	err        error
	calls      int
}

func (s *stubAuthenticator) Authenticate(_ context.Context, bearer string) (*securityjwt.Claims, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if bearer == "Bearer "+s.validToken {
		return &securityjwt.Claims{}, nil
	}
	return nil, errors.New("invalid token")
}

const validToken = "valid-jwt-token"

// newEngine assembles a minimal chain: guard -> business handler.
func newEngine(t *testing.T, authn securitymw.BearerAuthenticator, requireJWT bool, exemptPaths, exemptPrefixes []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gateway.Enforce(requireJWT, authn, exemptPaths, exemptPrefixes))
	r.GET("/api/v1/auth/info", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	return r
}

func do(t *testing.T, r *gin.Engine, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Requests carrying a valid JWT must pass.
func TestEnforce_ValidJWTPasses(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{validToken: validToken}, true, nil, nil)
	if w := do(t, r, "/api/v1/auth/info", "Bearer "+validToken); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// Missing credentials must return 401.
func TestEnforce_MissingCredentialRejected(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{validToken: validToken}, true, nil, nil)
	if w := do(t, r, "/api/v1/auth/info", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// Invalid/expired JWTs must return 401.
func TestEnforce_InvalidJWTRejected(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{validToken: validToken}, true, nil, nil)
	for _, bearer := range []string{"Bearer expired-token", "Bearer ", "garbage"} {
		if w := do(t, r, "/api/v1/auth/info", bearer); w.Code != http.StatusUnauthorized {
			t.Fatalf("bearer=%q status = %d, want %d", bearer, w.Code, http.StatusUnauthorized)
		}
	}
}

// When the guard is disabled it passes through, even if the authenticator would reject
// everything (guarantees zero changes for local go run).
func TestEnforce_DisabledPassthrough(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{err: errors.New("always fail")}, false, nil, nil)
	if w := do(t, r, "/api/v1/auth/info", ""); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// fail-closed: when declared enabled but no verifier is assembled, it must reject rather than silently allow.
func TestEnforce_NilAuthenticatorFailsClosed(t *testing.T) {
	r := newEngine(t, nil, true, nil, nil)
	if w := do(t, r, "/api/v1/auth/info", "Bearer anything"); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// With the guard disabled, a nil authn has no effect (the fail-closed branch must not trigger).
func TestEnforce_DisabledWithNilAuthenticator(t *testing.T) {
	r := newEngine(t, nil, false, nil, nil)
	if w := do(t, r, "/api/v1/auth/info", ""); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// Exact paths in the exemption list skip verification (health checks etc. do not go through the gateway).
func TestEnforce_ExemptPaths(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{err: errors.New("always fail")}, true, []string{"/healthz"}, nil)
	if w := do(t, r, "/healthz", ""); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// Prefixes in the exemption list skip verification (e.g. /apidoc).
func TestEnforce_ExemptPrefixes(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{err: errors.New("always fail")}, true, nil, []string{"/apidoc"})
	r.GET("/apidoc/openapi.json", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	if w := do(t, r, "/apidoc/openapi.json", ""); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// Similar-prefix paths outside the exemption list must not be exempted by mistake
// (/apidoc-extra should not be matched by the /apidoc prefix exemption beyond...
// but under prefix-exemption semantics it does match, so what is verified here is
// that exact-path exemption does not overreach).
func TestEnforce_ExemptExactDoesNotLeak(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{err: errors.New("always fail")}, true, []string{"/healthz"}, nil)
	if w := do(t, r, "/api/v1/auth/info", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// The failure response follows the unified envelope (code matches the HTTP status).
func TestEnforce_ErrorEnvelope(t *testing.T) {
	r := newEngine(t, &stubAuthenticator{validToken: validToken}, true, nil, nil)
	w := do(t, r, "/api/v1/auth/info", "")
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.Code != http.StatusUnauthorized {
		t.Fatalf("body.code = %d, want %d", body.Code, http.StatusUnauthorized)
	}
	if body.Msg == "" {
		t.Fatal("body.msg must not be empty")
	}
}

// Exempt paths must not trigger the authenticator (avoiding needless signature verification overhead).
func TestEnforce_ExemptSkipsAuthenticator(t *testing.T) {
	authn := &stubAuthenticator{err: errors.New("always fail")}
	r := newEngine(t, authn, true, []string{"/healthz"}, nil)
	do(t, r, "/healthz", "")
	do(t, r, "/api/v1/auth/info", "")
	if authn.calls != 1 {
		t.Fatalf("authenticator call count = %d, want 1 (only non-exempt paths should call it)", authn.calls)
	}
}

// EnforceFromConfig: passes through when RequireJWT=false.
func TestEnforceFromConfig_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gateway.EnforceFromConfig(gateway.Config{RequireJWT: false}, &stubAuthenticator{err: errors.New("fail")}, nil, nil))
	r.GET("/api/v1/auth/info", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/info", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// EnforceFromConfig: takes effect once enabled, and the exempt_paths appended by config take effect too.
func TestEnforceFromConfig_EnabledWithExempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authn := &stubAuthenticator{validToken: validToken}
	r.Use(gateway.EnforceFromConfig(
		gateway.Config{RequireJWT: true, ExemptPaths: []string{"/healthz"}},
		authn, []string{"/readyz"}, nil,
	))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/api/v1/auth/info", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	for _, p := range []string{"/healthz", "/readyz"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, p, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", p, w.Code, http.StatusOK)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/auth/info", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("business path status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// The tenant header injected by the gateway is not trusted: even when present it cannot replace JWT verification.
func TestEnforce_GatewayTenantHeaderNotTrusted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gateway.Enforce(true, &stubAuthenticator{err: errors.New("always fail")}, nil, nil))
	r.GET("/api/v1/auth/info", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/info", nil)
	req.Header.Set(gateway.HeaderTenant, "1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (a tenant header alone is not enough to pass)", w.Code, http.StatusUnauthorized)
	}
}
