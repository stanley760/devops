package router

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	httpinfra "github.com/stanley760/devops/devops-infra/http"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/middleware"
	"github.com/stanley760/devops/devops-system/internal/service"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// newTestKit builds the test middleware kit (no DB/cache environment; the test path only reaches the JWT layer).
func newTestKit(m *auth.Manager) *middleware.Kit {
	return middleware.NewKit(m, middleware.Deps{})
}

func newTestRegister(docCfg config.APIDocConfig, kit *middleware.Kit, authM *auth.Manager, svcCfg config.Config) httpinfra.Register {
	modules := Modules{
		Auth:          NewAuthRoutes(service.NewAuth(svcCfg, authM), kit),
		Users:         NewUserRoutes(service.NewUser(authM, func() {}), kit),
		Roles:         NewRoleRoutes(service.NewRole(func() {}), kit),
		Menus:         NewMenuRoutes(service.NewMenu(), kit),
		Depts:         NewDeptRoutes(service.NewDept(), kit),
		Posts:         NewPostRoutes(service.NewPost(), kit),
		Tenants:       NewTenantRoutes(service.NewTenant(authM), kit),
		LoginLogs:     NewLoginLogRoutes(service.NewLoginLog(), kit),
		OperationLogs: NewOperationLogRoutes(service.NewOperationLog(), kit),
		Dicts:         NewDictRoutes(service.NewDict(), kit),
		Configs:       NewConfigRoutes(service.NewConfig(), kit),
	}
	return NewRegister(docCfg, authM, modules)
}

// jwksEnv builds a real shared security assembly (RS256 + miniredis session) together with the business routes.
func jwksEnv(t *testing.T) (*auth.Manager, *rsa.PublicKey, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cfg := config.JWTConfig{
		SigningKey:    string(pemKey),
		KeyID:         "router-test-key",
		Issuer:        "devops",
		AccessExpire:  time.Hour,
		RefreshExpire: 24 * time.Hour,
	}
	pub, err := securityjwt.PublicKeyFromPrivatePEM(pemKey)
	if err != nil {
		t.Fatal(err)
	}
	keySet, err := securityjwt.NewStaticKeySet(map[string]*rsa.PublicKey{cfg.EffectiveKeyID(): pub})
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := securityjwt.NewManager(cfg.SharedJWTConfig(keySet))
	if err != nil {
		t.Fatal(err)
	}
	m := auth.New(cfg, tokens, securitysession.NewRedisClientStore(client, cfg.RefreshExpire))

	svcCfg := config.Config{}
	reg := newTestRegister(config.APIDocConfig{Enabled: true, BasePath: "/apidoc"}, newTestKit(m), m, svcCfg)
	r := gin.New()
	reg(r)
	return m, pub, r
}

// operationIDRE matches the OperationID literals in operations.go.
var operationIDRE = regexp.MustCompile(`OperationID:\s*"([a-z0-9-]+)"`)

// declaredOperationIDs parses every declared OperationID from the operations.go source file.
//
// The source file is the single contract of truth, avoiding drift between a
// manually maintained list and the actual registration —
// which is exactly how the hardcoded count 43 used to fail.
func declaredOperationIDs(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatalf("failed to read operations.go: %v", err)
	}
	matches := operationIDRE.FindAllStringSubmatch(string(src), -1)
	if len(matches) == 0 {
		t.Fatal("no OperationID parsed from operations.go")
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, m := range matches {
		if seen[m[1]] {
			t.Errorf("OperationID %q declared more than once", m[1])
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

func TestRegisterAPIDoc(t *testing.T) {
	_, _, r := jwksEnv(t)

	docResponse := httptest.NewRecorder()
	r.ServeHTTP(docResponse, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if docResponse.Code != http.StatusOK {
		t.Fatalf("openapi.json status = %d, body = %s", docResponse.Code, docResponse.Body.String())
	}

	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string             `json:"operationId"`
			Tags        []string           `json:"tags"`
			Security    []map[string][]any `json:"security"`
		} `json:"paths"`
		Components struct {
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				Name string `json:"name"`
				In   string `json:"in"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(docResponse.Body.Bytes(), &doc); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if _, ok := doc.Paths["/system/api/v1/auth/login"]; !ok {
		t.Fatalf("login operation missing: %+v", doc.Paths)
	}
	if _, ok := doc.Paths["/system/api/v1/auth/refresh"]; !ok {
		t.Fatalf("refresh operation missing: %+v", doc.Paths)
	}
	// Business operation count: when apidoc is enabled it self-registers
	// export-openapi (tag "apidoc"), which must be excluded.
	//
	// The expected value is no longer hardcoded but derived by parsing the
	// OperationID list from the operations.go source file —
	// a declaration is the contract, so the test cannot drift from the
	// implementation. Before the refactor this hardcoded 43, while the
	// feat(audit) commit added three operationlog endpoints (list/delete/clean)
	// without updating the constant, leaving the guardrail silently broken
	// for a long time (git log 046b6b9).
	declared := declaredOperationIDs(t)

	operationCount := 0
	registered := make(map[string]struct{})
	for _, path := range doc.Paths {
		for _, op := range path {
			if slices.Contains(op.Tags, "apidoc") {
				continue
			}
			operationCount++
			if op.OperationID != "" {
				registered[op.OperationID] = struct{}{}
			}
		}
	}
	if operationCount != len(declared) {
		t.Fatalf("business operation count = %d, operations.go declares %d", operationCount, len(declared))
	}
	// Name every ID: a missing registration reports the exact missing ID, which is more actionable than a bare count.
	for _, id := range declared {
		if _, ok := registered[id]; !ok {
			t.Errorf("OperationID %q is declared but not registered in OpenAPI", id)
		}
	}
	bearer, ok := doc.Components.SecuritySchemes["bearerAuth"]
	if !ok || bearer.Type != "http" {
		t.Fatalf("bearerAuth scheme missing or invalid: %+v", doc.Components.SecuritySchemes)
	}
	tenant, ok := doc.Components.SecuritySchemes["tenantId"]
	if !ok || tenant.Type != "apiKey" || tenant.Name != "X-Tenant-ID" || tenant.In != "header" {
		t.Fatalf("tenantId scheme missing or invalid: %+v", doc.Components.SecuritySchemes)
	}
}

// TestRegisterAPIDocEnabledKeepsAuthRoutes regression: when apidoc is enabled,
// login/refresh move to huma registration, and the remaining auth routes
// (gin-registered) must stay available — in early versions authH was nil when
// enabled, so calling routes like logout/info panicked (nil handler dereference).
func TestRegisterAPIDocEnabledKeepsAuthRoutes(t *testing.T) {
	_, _, r := jwksEnv(t)

	// Without a token: the JWT middleware should return a 401 envelope, not a nil handler panic
	logout := httptest.NewRecorder()
	r.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/system/api/v1/auth/logout", nil))
	if logout.Code != http.StatusUnauthorized {
		t.Fatalf("logout without token status = %d, want 401, body = %s", logout.Code, logout.Body.String())
	}

	info := httptest.NewRecorder()
	r.ServeHTTP(info, httptest.NewRequest(http.MethodGet, "/system/api/v1/auth/info", nil))
	if info.Code != http.StatusUnauthorized {
		t.Fatalf("info without token status = %d, want 401, body = %s", info.Code, info.Body.String())
	}

	// The huma-registered login route is reachable: the request should hit (any parameter validation/content negotiation error is fine), not 404
	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/system/api/v1/auth/login", strings.NewReader("{}")))
	if login.Code == http.StatusNotFound {
		t.Fatalf("huma login route missing: %s", login.Body.String())
	}

	legacyLogin := httptest.NewRecorder()
	r.ServeHTTP(legacyLogin, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{}")))
	if legacyLogin.Code != http.StatusNotFound {
		t.Fatalf("unprefixed login route unexpectedly available: status = %d", legacyLogin.Code)
	}
}

func TestRegisterAPIDocDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	defer func() { _ = client.Close() }()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	jwtCfg := config.JWTConfig{
		SigningKey:    string(pemKey),
		KeyID:         "router-test-key",
		Issuer:        "devops",
		AccessExpire:  time.Hour,
		RefreshExpire: 24 * time.Hour,
	}
	pub, err := securityjwt.PublicKeyFromPrivatePEM(pemKey)
	if err != nil {
		t.Fatal(err)
	}
	keySet, err := securityjwt.NewStaticKeySet(map[string]*rsa.PublicKey{jwtCfg.EffectiveKeyID(): pub})
	if err != nil {
		t.Fatal(err)
	}
	tokenMgr, err := securityjwt.NewManager(jwtCfg.SharedJWTConfig(keySet))
	if err != nil {
		t.Fatal(err)
	}
	authMgr := auth.New(jwtCfg, tokenMgr, securitysession.NewRedisClientStore(client, jwtCfg.RefreshExpire))
	svcCfg := config.Config{}

	r := gin.New()
	newTestRegister(config.APIDocConfig{}, newTestKit(authMgr), authMgr, svcCfg)(r)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apidoc/openapi.json", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled apidoc status = %d, want 404", rec.Code)
	}

	// D5: Enabled gates only the documentation endpoints; business routes are always registered
	// (auth has moved to huma, so it remains reachable when docs are disabled)
	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/system/api/v1/auth/login", nil))
	if login.Code == http.StatusNotFound {
		t.Fatalf("auth route missing when apidoc disabled: %s", login.Body.String())
	}

	logout := httptest.NewRecorder()
	r.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/system/api/v1/auth/logout", nil))
	if logout.Code != http.StatusUnauthorized {
		t.Fatalf("logout without token status = %d, want 401, body = %s", logout.Code, logout.Body.String())
	}
}

// TestJWKSEndpointVerifiesIssuedToken simulates the full Higress jwt-auth
// verification path: fetch /.well-known/jwks.json → select the public key by
// kid → verify with RS256 PKCS#1 v1.5.
func TestJWKSEndpointVerifiesIssuedToken(t *testing.T) {
	m, signingPub, r := jwksEnv(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("jwks status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			Alg string `json:"alg"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("jwks body: %v", err)
	}
	if len(doc.Keys) != 1 {
		t.Fatalf("keys = %d, want 1", len(doc.Keys))
	}
	k := doc.Keys[0]
	if k.Kty != "RSA" || k.Use != "sig" || k.Alg != "RS256" || k.Kid != "router-test-key" {
		t.Fatalf("jwks key fields: %+v", k)
	}

	// The JWKS public key pairs with the signing private key
	n, _ := base64.RawURLEncoding.DecodeString(k.N)
	e, _ := base64.RawURLEncoding.DecodeString(k.E)
	jwksPub := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	if jwksPub.N.Cmp(signingPub.N) != 0 || jwksPub.E != signingPub.E {
		t.Fatal("jwks public key does not match signing key")
	}

	// Verify the issued access token's RS256 signature with the JWKS public key (equivalent to Higress jwt-auth behavior)
	access, _, _, err := m.Issue(context.Background(), 42, "alice", 7, "tenant-7")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(access, ".")
	if len(parts) != 3 {
		t.Fatalf("token segments = %d", len(parts))
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(jwksPub, crypto.SHA256, digest[:], sig); err != nil {
		t.Fatalf("higress-style signature verification failed: %v", err)
	}
}
