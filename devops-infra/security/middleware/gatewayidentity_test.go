package middleware

import (
	"context"
	"errors"
	"net/http"
	"testing"

	securitysession "github.com/stanley760/devops/devops-infra/security/session"
)

type stubSessionStore struct {
	active bool
	err    error
	ref    securitysession.SessionRef
}

func (s *stubSessionStore) Active(_ context.Context, ref securitysession.SessionRef) (bool, error) {
	s.ref = ref
	return s.active, s.err
}

func gatewayHeaders(uid, username, sid, tid, tcode string) http.Header {
	h := http.Header{}
	h.Set(HeaderGatewayUID, uid)
	h.Set(HeaderGatewayUsername, username)
	h.Set(HeaderGatewaySessionID, sid)
	h.Set(HeaderGatewayTenantID, tid)
	h.Set(HeaderGatewayTenantCode, tcode)
	return h
}

func TestParseGatewayIdentity(t *testing.T) {
	id, err := ParseGatewayIdentity(gatewayHeaders("42", "admin", "sid-1", "7", "default"))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if id.UID != 42 || id.Username != "admin" || id.SessionID != "sid-1" || id.TenantID != 7 || id.TenantCode != "default" {
		t.Fatalf("身份解析不正确: %+v", id)
	}
}

func TestParseGatewayIdentityRejectsIncomplete(t *testing.T) {
	cases := map[string]http.Header{
		"缺 uid":    gatewayHeaders("", "admin", "sid-1", "7", "default"),
		"uid 非法":   gatewayHeaders("abc", "admin", "sid-1", "7", "default"),
		"缺租户":     gatewayHeaders("42", "admin", "sid-1", "", "default"),
		"缺会话 id":  gatewayHeaders("42", "admin", "", "7", "default"),
		"缺用户名":   gatewayHeaders("42", "", "sid-1", "7", "default"),
		"全部缺失":   http.Header{},
	}
	for name, h := range cases {
		if _, err := ParseGatewayIdentity(h); err == nil {
			t.Errorf("%s：应当报错", name)
		} else if !errors.Is(err, ErrGatewayIdentityMissing) {
			t.Errorf("%s：错误类型不对: %v", name, err)
		}
	}
}

func TestGatewayIdentityAuthenticator(t *testing.T) {
	store := &stubSessionStore{active: true}
	authn := NewGatewayIdentityAuthenticator(store)

	ctx := WithGatewayIdentity(context.Background(), &GatewayIdentity{
		UID: 42, Username: "admin", SessionID: "sid-1", TenantID: 7, TenantCode: "default",
	})
	claims, err := authn.Authenticate(ctx, "")
	if err != nil {
		t.Fatalf("认证失败: %v", err)
	}
	if claims.UID != 42 || claims.TenantID != 7 || claims.SessionID != "sid-1" || claims.Username != "admin" {
		t.Fatalf("claims 不正确: %+v", claims)
	}
	// 会话查询必须带上身份里的三元组，否则吊销判断会查错会话。
	if store.ref.UserID != 42 || store.ref.TenantID != 7 || store.ref.SessionID != "sid-1" {
		t.Fatalf("会话查询参数不正确: %+v", store.ref)
	}
}

func TestGatewayIdentityAuthenticatorRejectsMissingIdentity(t *testing.T) {
	authn := NewGatewayIdentityAuthenticator(&stubSessionStore{active: true})
	// 没有经过网关的请求：没有身份 → 401（必须从网关进来）
	if _, err := authn.Authenticate(context.Background(), "Bearer whatever"); err == nil {
		t.Fatal("没有网关身份时应当拒绝")
	} else if !errors.Is(err, ErrGatewayIdentityMissing) {
		t.Fatalf("错误类型不对: %v", err)
	}
}

func TestGatewayIdentityAuthenticatorRejectsRevokedSession(t *testing.T) {
	authn := NewGatewayIdentityAuthenticator(&stubSessionStore{active: false})
	ctx := WithGatewayIdentity(context.Background(), &GatewayIdentity{
		UID: 42, Username: "admin", SessionID: "sid-1", TenantID: 7,
	})
	if _, err := authn.Authenticate(ctx, ""); !errors.Is(err, ErrSessionInactive) {
		t.Fatalf("会话已失效时应当返回 ErrSessionInactive，实际: %v", err)
	}
}
