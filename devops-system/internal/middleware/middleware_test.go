package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/glebarez/sqlite"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stanley760/devops/devops-common/auditctx"
	memcache "github.com/stanley760/devops/devops-infra/cache/memory"
	securityauthz "github.com/stanley760/devops/devops-infra/security/authz"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/model"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"testing"
	"time"
)

// testEnv builds the middleware test environment backed by shared security implementations (huma version).
// Returns the auth facade, casbin authorizer, and the underlying sqlite connection (for tenant table seeding).
func testEnv(t *testing.T) (*auth.Manager, securityauthz.Authorizer, *gorm.DB, func()) {
	t.Helper()

	// Redis session store
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	sessions := securitysession.NewRedisClientStore(client, 24*time.Hour)

	// RS256 shared JWT manager
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cfg := config.JWTConfig{
		SigningKey:    string(pemKey),
		KeyID:         "test-key",
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

	// shared Casbin authorizer (held by the authz facade).
	// the DB name carries t.Name() for isolation: a cache=shared in-memory DB with the same name
	// is shared within the same process, and a fixed name would let different cases' seed data
	// pollute each other (e.g. tenant status assertions).
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := securityauthz.New(db)
	if err != nil {
		t.Fatal(err)
	}
	authz.SetAuthorizer(authorizer)

	mgr := auth.New(cfg, tokens, sessions)
	cleanup := func() {
		authz.Close()
		_ = client.Close()
		mr.Close()
	}
	return mgr, authorizer, db, cleanup
}

// login signs a real token (RS256 + session) with the facade.
func login(t *testing.T, m *auth.Manager, uid int64, tid int64) (*auth.AccessClaims, string) {
	t.Helper()
	access, _, _, err := m.Issue(context.Background(), uid, "alice", tid, "tenant-x")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := m.Parse(access)
	if err != nil {
		t.Fatal(err)
	}
	return claims, access
}

// newMWAPI assembles a humatest API with the specified middleware mounted (aligned with production apidoc.New: $schema injection disabled).
func newMWAPI(t *testing.T, m ...func(huma.Context, func(huma.Context))) humatest.TestAPI {
	t.Helper()
	cfg := huma.DefaultConfig("mw-test", "v1")
	cfg.CreateHooks = nil
	_, api := humatest.New(t, cfg)
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/",
		Middlewares: huma.Middlewares(m),
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		return nil, nil
	})
	return api
}

// perform makes a request with an optional Bearer header.
func perform(t *testing.T, api humatest.TestAPI, token string) int {
	t.Helper()
	if token == "" {
		return api.Get("/").Code
	}
	return api.Get("/", "Authorization: Bearer "+token).Code
}

func TestHumaJWTMiddlewareRegressions(t *testing.T) {
	m, authorizer, _, cleanup := testEnv(t)
	defer cleanup()

	kit := NewKit(m, Deps{Authorizer: authorizer})
	api := newMWAPI(t, kit.JWT())

	// missing Authorization header → 401 + original copy envelope
	resp := api.Get("/")
	if resp.Code != http.StatusUnauthorized || !strings.Contains(resp.Body.String(), "not logged in or session expired") {
		t.Fatalf("missing header: %d %s", resp.Code, resp.Body.String())
	}

	// invalid token → 401 + original copy
	resp = api.Get("/", "Authorization: Bearer garbage-token")
	if resp.Code != http.StatusUnauthorized || !strings.Contains(resp.Body.String(), "not logged in or session expired") {
		t.Fatalf("bad token: %d %s", resp.Code, resp.Body.String())
	}

	// valid token → 204 (middleware passes through, handler executes)
	_, access := login(t, m, 42, 7)
	if code := perform(t, api, access); code != http.StatusNoContent {
		t.Fatalf("valid token: %d", code)
	}

	// session revoked (logout/kick offline) → 401 + invalid copy
	claims, access2 := login(t, m, 42, 7)
	m.Logout(context.Background(), claims)
	resp = api.Get("/", "Authorization: Bearer "+access2)
	if resp.Code != http.StatusUnauthorized || !strings.Contains(resp.Body.String(), "login state invalid, please log in again") {
		t.Fatalf("revoked session: %d %s", resp.Code, resp.Body.String())
	}
}

func TestHumaJWTMiddlewareInjectsAuditActor(t *testing.T) {
	m, authorizer, _, cleanup := testEnv(t)
	defer cleanup()

	kit := NewKit(m, Deps{Authorizer: authorizer})
	cfg := huma.DefaultConfig("audit-actor-test", "v1")
	cfg.CreateHooks = nil
	_, api := humatest.New(t, cfg)

	var actor auditctx.Actor
	var actorOK bool
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        "/",
		Middlewares: huma.Middlewares{kit.JWT()},
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		actor, actorOK = auditctx.From(ctx)
		return nil, nil
	})

	_, access := login(t, m, 42, 7)
	if code := api.Get("/", "Authorization: Bearer "+access).Code; code != http.StatusNoContent {
		t.Fatalf("authenticated request: %d", code)
	}
	if !actorOK || actor.Username != "alice" {
		t.Fatalf("audit actor = (%q, %v), want (alice, true)", actor.Username, actorOK)
	}
}

func TestHumaPermMiddlewareRegressions(t *testing.T) {
	m, authorizer, _, cleanup := testEnv(t)
	defer cleanup()

	// user 42 has no permission in tenant 7; user 43 holds the editor role (system:user:query)
	if err := authz.RebuildRolePolicies("editor", 7, []string{"system:user:query"}); err != nil {
		t.Fatal(err)
	}
	if err := authz.RebuildUserGrouping(43, 7, []string{"editor"}); err != nil {
		t.Fatal(err)
	}

	kit := NewKit(m, Deps{Authorizer: authorizer})
	api := newMWAPI(t, kit.JWT(), kit.Perm("system:user:query"))

	// no permission → 403 + original copy
	_, access42 := login(t, m, 42, 7)
	resp := api.Get("/", "Authorization: Bearer "+access42)
	if resp.Code != http.StatusForbidden || !strings.Contains(resp.Body.String(), "no operation permission") {
		t.Fatalf("denied: %d %s", resp.Code, resp.Body.String())
	}

	// with permission → 204
	_, access43 := login(t, m, 43, 7)
	resp = api.Get("/", "Authorization: Bearer "+access43)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("allowed: %d", resp.Code)
	}
}

func TestHumaPlatformPermMiddlewareRegressions(t *testing.T) {
	m, authorizer, _, cleanup := testEnv(t)
	defer cleanup()

	// user 44 is a platform admin; 45 only a tenant role
	if err := authz.SeedPolicies(7, 44); err != nil {
		t.Fatal(err)
	}
	if err := authz.RebuildUserGrouping(45, 7, []string{"editor"}); err != nil {
		t.Fatal(err)
	}

	kit := NewKit(m, Deps{Authorizer: authorizer})
	api := newMWAPI(t, kit.JWT(), kit.Plat("platform:tenant:query"))

	// non-platform admin → 403 + original copy (g2 gate is before Enforce)
	_, access45 := login(t, m, 45, 7)
	resp := api.Get("/", "Authorization: Bearer "+access45)
	if resp.Code != http.StatusForbidden || !strings.Contains(resp.Body.String(), "platform administrator permission required") {
		t.Fatalf("non-admin: %d %s", resp.Code, resp.Body.String())
	}

	// platform admin → 204
	_, access44 := login(t, m, 44, 7)
	resp = api.Get("/", "Authorization: Bearer "+access44)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("platform admin: %d", resp.Code)
	}
}

// seedTenants creates the tenant table in sqlite and seeds three-state tenants: 7 normal, 8 disabled, 9 expired.
func seedTenants(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(&model.Tenant{}); err != nil {
		t.Fatal(err)
	}
	expired := time.Now().Add(-time.Hour)
	tenants := []model.Tenant{
		{BaseModel: model.BaseModel{ID: 7}, Name: "正常租户", Code: "t7", Status: 1},
		{BaseModel: model.BaseModel{ID: 8}, Name: "停用租户", Code: "t8", Status: 0},
		{BaseModel: model.BaseModel{ID: 9}, Name: "过期租户", Code: "t9", Status: 1, ExpireTime: &expired},
	}
	if err := db.Create(&tenants).Error; err != nil {
		t.Fatal(err)
	}
}

// TestHumaTenantMiddlewareStates: tenant usability three-state regression: normal passes, disabled/expired 403 envelope;
// also verifies the Redis cache hit path (a second request for a normal tenant hits the cache).
func TestHumaTenantMiddlewareStates(t *testing.T) {
	m, authorizer, db, cleanup := testEnv(t)
	defer cleanup()
	seedTenants(t, db)

	c := memcache.New()
	kit := NewKit(m, Deps{TenantUsable: newTenantChecker(db, c), Authorizer: authorizer})
	api := newMWAPI(t, kit.JWT(), kit.Tenant())

	// normal tenant → 204 (first request queries the DB and writes the cache)
	_, access7 := login(t, m, 42, 7)
	if code := perform(t, api, access7); code != http.StatusNoContent {
		t.Fatalf("usable tenant: %d", code)
	}
	// cache hit path: a second request for the same tenant
	if code := perform(t, api, access7); code != http.StatusNoContent {
		t.Fatalf("usable tenant (cache hit): %d", code)
	}

	// disabled tenant → 403 + original copy
	_, access8 := login(t, m, 42, 8)
	resp := api.Get("/", "Authorization: Bearer "+access8)
	if resp.Code != http.StatusForbidden || !strings.Contains(resp.Body.String(), "tenant is unavailable (disabled or expired)") {
		t.Fatalf("disabled tenant: %d %s", resp.Code, resp.Body.String())
	}

	// expired tenant → 403 + original copy
	_, access9 := login(t, m, 42, 9)
	resp = api.Get("/", "Authorization: Bearer "+access9)
	if resp.Code != http.StatusForbidden || !strings.Contains(resp.Body.String(), "tenant is unavailable (disabled or expired)") {
		t.Fatalf("expired tenant: %d %s", resp.Code, resp.Body.String())
	}

	// no tenant record (does not exist) → 403
	_, access99 := login(t, m, 42, 99)
	resp = api.Get("/", "Authorization: Bearer "+access99)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("missing tenant: %d %s", resp.Code, resp.Body.String())
	}
}
