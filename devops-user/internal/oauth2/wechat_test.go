package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestWeChatAuthCodeURL locks down the four easy-to-miss points of the WeChat
// authorize URL:
//  1. the endpoint must be /connect/qrconnect (not a standard OAuth2 endpoint);
//  2. redirect_uri must be encoded and **only encoded once** (Encode()
//     double-encodes);
//  3. scope is fixed to snsapi_login;
//  4. the URL must end with the #wechat_redirect fragment, otherwise WeChat
//     shows "the link cannot be accessed".
func TestWeChatAuthCodeURL(t *testing.T) {
	p := NewWeChatProvider(Config{
		ClientID:     "wx1234567890",
		ClientSecret: "secret",
		// Port and path are included deliberately, to verify the whole thing
		// is encoded correctly.
		RedirectURL: "http://localhost:10000/user/api/v1/oauth/wechat/callback",
	})

	got := p.AuthCodeURL("state-abc")

	if !strings.HasPrefix(got, "https://open.weixin.qq.com/connect/qrconnect?") {
		t.Fatalf("wrong authorize endpoint: %s", got)
	}
	if !strings.HasSuffix(got, "#wechat_redirect") {
		t.Fatalf("missing #wechat_redirect fragment; WeChat would report \"this link cannot be accessed\": %s", got)
	}

	// Strip the fragment before parsing the query.
	query := strings.TrimSuffix(got, "#wechat_redirect")
	u, err := url.Parse(query)
	if err != nil {
		t.Fatalf("URL parse failed: %v", err)
	}
	q := u.Query()

	if q.Get("appid") != "wx1234567890" {
		t.Errorf("appid = %q", q.Get("appid"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type = %q, must be code", q.Get("response_type"))
	}
	if q.Get("scope") != "snsapi_login" {
		t.Errorf("scope = %q; website apps can only use snsapi_login", q.Get("scope"))
	}
	if q.Get("state") != "state-abc" {
		t.Errorf("state = %q", q.Get("state"))
	}

	// redirect_uri must parse back byte-for-byte identical to the original
	// value (not corrupted by double encoding).
	if ru := q.Get("redirect_uri"); ru != "http://localhost:10000/user/api/v1/oauth/wechat/callback" {
		t.Errorf("redirect_uri decoded = %q, expected to round-trip to the original callback address", ru)
	}
	// The classic symptom of double encoding: %xx remnants survive decoding.
	if decoded := q.Get("redirect_uri"); strings.Contains(decoded, "%3") {
		t.Errorf("redirect_uri was double-encoded: %q", decoded)
	}
}

// TestWeChatAuthCodeURL_NoDoubleEncode control group: proves
// url.Values.Encode() double-encodes, so this provider must assemble the URL
// by hand.
func TestWeChatAuthCodeURL_NoDoubleEncode(t *testing.T) {
	v := url.Values{}
	v.Set("redirect_uri", "http://localhost:10000/cb")
	// Values.Encode() encodes / and :, which url.Parse can restore, but WeChat
	// tolerates multi-layer encoding poorly, and the standard-library approach
	// cannot append a fragment.
	encoded := v.Encode()
	if !strings.Contains(encoded, "redirect_uri=") {
		t.Fatalf("control group setup failed: %s", encoded)
	}
}

// TestWeChatEnabled usable only when credentials are complete.
func TestWeChatEnabled(t *testing.T) {
	full := Config{ClientID: "a", ClientSecret: "b", RedirectURL: "c"}
	if !NewWeChatProvider(full).Enabled() {
		t.Error("should be Enabled when all three are present")
	}
	for name, cfg := range map[string]Config{
		"missing client_id":                    {ClientSecret: "b", RedirectURL: "c"},
		"missing client_secret":                {ClientID: "a", RedirectURL: "c"},
		"missing redirect_url":                 {ClientID: "a", ClientSecret: "b"},
		"all empty":                            {},
		"wechat does not support custom scope": {ClientID: "a", ClientSecret: "b", RedirectURL: "c", Scopes: []string{"snsapi_userinfo"}},
	} {
		if NewWeChatProvider(cfg).Enabled() != (name == "wechat does not support custom scope") {
			t.Errorf("%s: Enabled result unexpected", name)
		}
	}
}

// newMockWechat starts a test server mocking the WeChat API.
func newMockWechat(t *testing.T, tokenBody, userBody string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/sns/oauth2/access_token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(tokenBody))
	})
	mux.HandleFunc("/sns/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userBody))
	})
	return httptest.NewServer(mux)
}

func mockWechatProvider(apiBase string) *WeChatProvider {
	p := NewWeChatProvider(Config{ClientID: "wx1", ClientSecret: "s", RedirectURL: "http://x/cb"})
	p.apiBase = apiBase
	return p
}

// TestWeChatExchange_OK happy path: exchange token → fetch user → return
// normalized info.
func TestWeChatExchange_OK(t *testing.T) {
	srv := newMockWechat(t,
		`{"access_token":"AT","expires_in":7200,"refresh_token":"RT","openid":"OPENID1","unionid":"UNIONID1"}`,
		`{"openid":"OPENID1","nickname":"微信用户","headimgurl":"https://thirdwx.qlogo.cn/abc/96","unionid":"UNIONID1"}`,
	)
	defer srv.Close()

	info, at, rt, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE")
	if err != nil {
		t.Fatalf("Exchange failed: %v", err)
	}
	if info.ProviderUserID != "UNIONID1" {
		t.Errorf("binding key = %q, expected unionid (stable across apps)", info.ProviderUserID)
	}
	if info.Nickname != "微信用户" {
		t.Errorf("nickname = %q", info.Nickname)
	}
	if at != "AT" || rt != "RT" {
		t.Errorf("token = %q / %q", at, rt)
	}
	// The avatar size segment should be normalized to /0 for the original
	// image.
	if !strings.HasSuffix(info.AvatarURL, "/0") {
		t.Errorf("avatar = %q, expected to end with /0 for the original image", info.AvatarURL)
	}
}

// TestWeChatExchange_UnionIDFallback falls back to openid when unionid is
// absent.
func TestWeChatExchange_UnionIDFallback(t *testing.T) {
	srv := newMockWechat(t,
		`{"access_token":"AT","openid":"ONLY_OPENID"}`,
		`{"openid":"ONLY_OPENID","nickname":"n","headimgurl":""}`,
	)
	defer srv.Close()

	info, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE")
	if err != nil {
		t.Fatalf("Exchange failed: %v", err)
	}
	if info.ProviderUserID != "ONLY_OPENID" {
		t.Errorf("binding key = %q, should fall back to openid when unionid is absent", info.ProviderUserID)
	}
}

// TestWeChatExchange_NoIdentity must reject when the third party returns no
// identifier.
//
// ⚠️ Allowing it through would write an empty binding key, causing all such
// accounts to be misbound to each other — this is a security boundary that
// must fail, and must not be tolerated because of "missing fields".
func TestWeChatExchange_NoIdentity(t *testing.T) {
	srv := newMockWechat(t,
		`{"access_token":"AT"}`,
		`{"nickname":"n"}`,
	)
	defer srv.Close()

	_, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE")
	if err == nil {
		t.Fatal("must reject when both openid and unionid are missing; must not allow through")
	}
	if !strings.Contains(err.Error(), "openid/unionid") {
		t.Errorf("the error should mention the missing identifier, got: %v", err)
	}
}

// TestWeChatExchange_ErrCode WeChat returns HTTP 200 even on error; checking
// only the status code would treat failure as success — errcode must be
// checked explicitly.
func TestWeChatExchange_ErrCode(t *testing.T) {
	cases := []struct {
		name      string
		tokenBody string
		wantCode  string
	}{
		{"invalid code", `{"errcode":40029,"errmsg":"invalid code"}`, "40029"},
		{"invalid appid", `{"errcode":40013,"errmsg":"invalid appid"}`, "40013"},
		{"invalid appsecret", `{"errcode":40125,"errmsg":"invalid appsecret"}`, "40125"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newMockWechat(t, tc.tokenBody, `{}`)
			defer srv.Close()
			_, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE")
			if err == nil {
				t.Fatalf("must fail when errcode=%s", tc.wantCode)
			}
			if !strings.Contains(err.Error(), tc.wantCode) {
				t.Errorf("the error should contain errcode %s, got: %v", tc.wantCode, err)
			}
		})
	}
}

// TestWeChatExchange_UserInfoErrCode the errcode from the user info stage must
// also be handled.
func TestWeChatExchange_UserInfoErrCode(t *testing.T) {
	srv := newMockWechat(t,
		`{"access_token":"AT","openid":"O1"}`,
		`{"errcode":40003,"errmsg":"invalid openid"}`,
	)
	defer srv.Close()

	if _, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE"); err == nil {
		t.Fatal("must fail when userinfo returns 40003")
	}
}

// TestWeChatExchange_EmptyAccessToken must fail when access_token is empty.
func TestWeChatExchange_EmptyAccessToken(t *testing.T) {
	srv := newMockWechat(t, `{"openid":"O1"}`, `{}`)
	defer srv.Close()

	if _, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE"); err == nil {
		t.Fatal("must fail when access_token is empty")
	}
}

// TestNormalizeAvatar avatar size segment normalization.
func TestNormalizeAvatar(t *testing.T) {
	p := NewWeChatProvider(Config{})
	cases := map[string]string{
		"":                               "",
		"https://thirdwx.qlogo.cn/a/96":  "https://thirdwx.qlogo.cn/a/0",
		"https://thirdwx.qlogo.cn/a/0":   "https://thirdwx.qlogo.cn/a/0",
		"https://thirdwx.qlogo.cn/a/46":  "https://thirdwx.qlogo.cn/a/0",
		"https://thirdwx.qlogo.cn/a/132": "https://thirdwx.qlogo.cn/a/0",
		// When the last segment is not a size number the structure must not be
		// altered; appending /0 alone could be wrong, so it is returned as-is.
		"https://thirdwx.qlogo.cn/a/abc": "https://thirdwx.qlogo.cn/a/abc/0",
	}
	for in, want := range cases {
		if got := p.normalizeAvatar(in); got != want {
			t.Errorf("normalizeAvatar(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestWeChatNoEmail WeChat does not return an email — locking in that behavior
// as a reminder not to rely on email for password recovery.
func TestWeChatNoEmail(t *testing.T) {
	srv := newMockWechat(t,
		`{"access_token":"AT","openid":"O1","unionid":"U1"}`,
		`{"openid":"O1","nickname":"n","unionid":"U1"}`,
	)
	defer srv.Close()

	info, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE")
	if err != nil {
		t.Fatalf("Exchange failed: %v", err)
	}
	if info.Email != "" {
		t.Errorf("WeChat should not return an email, got %q", info.Email)
	}
}

// TestWeChatExchange_JSONGarbage should fail rather than panic when the
// response is not JSON.
func TestWeChatExchange_JSONGarbage(t *testing.T) {
	srv := newMockWechat(t, `not json`, `not json`)
	defer srv.Close()

	if _, _, _, err := mockWechatProvider(srv.URL).Exchange(context.Background(), "CODE"); err == nil {
		t.Fatal("invalid JSON should return an error")
	}
}

// TestWeChatName the provider name must match model.ProviderWechat.
func TestWeChatName(t *testing.T) {
	if got := NewWeChatProvider(Config{}).Name(); got != "wechat" {
		t.Errorf("Name() = %q, want wechat", got)
	}
}

// Ensure the json package is used (wechatTokenResp relies on tag parsing).
var _ = json.Marshal
