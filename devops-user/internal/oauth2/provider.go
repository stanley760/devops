package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Provider is the abstraction of a third-party login provider.
type Provider interface {
	// Name is the provider identifier.
	Name() string
	// AuthCodeURL generates the authorize redirect URL (including state).
	AuthCodeURL(state string) string
	// Exchange exchanges the authorization code for a token, then fetches user
	// info.
	//
	// ctx should carry oauth2.HTTPClient (already set by this package);
	// otherwise the default client is used.
	Exchange(ctx context.Context, code string) (*UserInfo, string, string, error)
	// Enabled reports whether the provider is usable (credentials complete).
	Enabled() bool
}

// httpClient is the HTTP client for third-party interactions.
//
// The timeout is set explicitly: http.DefaultClient has no timeout, so a stuck
// peer would hang requests, and hangs on the login path accumulate into an
// avalanche.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// oauthCtx attaches the HTTP client required by the oauth2 library.
//
// The oauth2.HTTPClient package-level variable cannot be set (it is a context
// key type, not a client); it can only be passed via context when calling
// Exchange.
func oauthCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, httpClient)
}

// ---------------------------------------------------------------- GitHub

// GitHubProvider is GitHub OAuth (no OIDC, no id_token).
type GitHubProvider struct {
	cfg    oauth2.Config
	apiURL string // for injecting a mock in tests
}

// NewGitHubProvider constructs a GitHub provider.
func NewGitHubProvider(cfg Config) *GitHubProvider {
	return &GitHubProvider{
		cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://github.com/login/oauth/authorize",
				TokenURL: "https://github.com/login/oauth/access_token",
			},
		},
		apiURL: "https://api.github.com",
	}
}

// Name implements Provider.
func (p *GitHubProvider) Name() string { return "github" }

// Enabled implements Provider.
func (p *GitHubProvider) Enabled() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != "" && p.cfg.RedirectURL != ""
}

// AuthCodeURL implements Provider.
func (p *GitHubProvider) AuthCodeURL(state string) string { return p.cfg.AuthCodeURL(state) }

// githubUser is the GitHub /user response.
type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Exchange implements Provider: exchanges the token then calls /user to fetch
// info.
func (p *GitHubProvider) Exchange(ctx context.Context, code string) (*UserInfo, string, string, error) {
	tok, err := p.cfg.Exchange(oauthCtx(ctx), code)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: github token exchange failed", ErrExchangeFailed)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/user", nil)
	if err != nil {
		return nil, "", "", err
	}
	// GitHub's API uses Bearer authentication.
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "devops-user")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", "", fmt.Errorf("%w: github /user returned %d", ErrExchangeFailed, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, "", "", err
	}
	var gu githubUser
	if err := json.Unmarshal(body, &gu); err != nil {
		return nil, "", "", fmt.Errorf("failed to parse github user info: %w", err)
	}
	if gu.ID == 0 {
		return nil, "", "", fmt.Errorf("%w: github did not return a user id", ErrExchangeFailed)
	}
	nickname := gu.Name
	if nickname == "" {
		nickname = gu.Login
	}
	return &UserInfo{
		// GitHub's numeric ID is a stable identifier; **do not use login**
		// (it can be renamed).
		ProviderUserID: fmt.Sprintf("%d", gu.ID),
		Nickname:       nickname,
		Email:          gu.Email, // may be a public email, unverified
		AvatarURL:      gu.AvatarURL,
	}, tok.AccessToken, tok.RefreshToken, nil
}

// ---------------------------------------------------------------- Google

// GoogleProvider is Google OAuth 2.0 (OIDC).
type GoogleProvider struct {
	cfg       oauth2.Config
	verifyURL string // for injecting a mock in tests
}

// NewGoogleProvider constructs a Google provider.
func NewGoogleProvider(cfg Config) *GoogleProvider {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		}
	}
	return &GoogleProvider{
		cfg: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		},
		verifyURL: "https://oauth2.googleapis.com/tokeninfo",
	}
}

// Name implements Provider.
func (p *GoogleProvider) Name() string { return "google" }

// Enabled implements Provider.
func (p *GoogleProvider) Enabled() bool {
	return p.cfg.ClientID != "" && p.cfg.ClientSecret != "" && p.cfg.RedirectURL != ""
}

// AuthCodeURL implements Provider.
func (p *GoogleProvider) AuthCodeURL(state string) string { return p.cfg.AuthCodeURL(state) }

// Exchange implements Provider.
//
// ⚠️ **id_token must be verified**: Google's id_token is a JWT, but it cannot
// be trusted just by decoding it. Here the tokeninfo endpoint is called to
// verify it (equivalent to signature verification + iss/aud/exp checks), which
// is simpler than local verification and less likely to miss checks (nonce,
// azp, etc.).
func (p *GoogleProvider) Exchange(ctx context.Context, code string) (*UserInfo, string, string, error) {
	tok, err := p.cfg.Exchange(oauthCtx(ctx), code)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: google token exchange failed", ErrExchangeFailed)
	}
	if tok.AccessToken == "" || !strings.HasPrefix(tok.TokenType, "Bearer") && tok.TokenType == "" {
		return nil, "", "", fmt.Errorf("%w: google did not return an access token", ErrExchangeFailed)
	}
	info, err := p.verifyIDToken(ctx, tok.AccessToken)
	if err != nil {
		return nil, "", "", err
	}
	return info, tok.AccessToken, tok.RefreshToken, nil
}

// googleTokenInfo is the tokeninfo response.
type googleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // may be bool or string
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
	Iss           string `json:"iss"`
}

// verifyIDToken verifies and parses the id_token (via the tokeninfo endpoint).
func (p *GoogleProvider) verifyIDToken(ctx context.Context, accessToken string) (*UserInfo, error) {
	url := p.verifyURL + "?access_token=" + accessToken
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: google tokeninfo returned %d", ErrExchangeFailed, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var ti googleTokenInfo
	if err := json.Unmarshal(body, &ti); err != nil {
		return nil, fmt.Errorf("failed to parse google tokeninfo: %w", err)
	}
	if ti.Sub == "" {
		return nil, fmt.Errorf("%w: google id_token is missing sub", ErrExchangeFailed)
	}
	// aud must be this app's client_id, otherwise it is "using someone else's
	// token to impersonate".
	if p.cfg.ClientID != "" && ti.Aud != "" && ti.Aud != p.cfg.ClientID {
		return nil, fmt.Errorf("%w: google aud mismatch", ErrExchangeFailed)
	}
	// iss must be Google.
	if ti.Iss != "" && !strings.HasPrefix(ti.Iss, "https://accounts.google.com") {
		return nil, fmt.Errorf("%w: google iss is invalid", ErrExchangeFailed)
	}
	return &UserInfo{
		ProviderUserID: ti.Sub, // sub is the OIDC stable identifier
		Nickname:       ti.Name,
		Email:          ti.Email,
		AvatarURL:      ti.Picture,
	}, nil
}

// ---------------------------------------------------------------- WeChat

// WeChatProvider is WeChat Open Platform "Website App" QR code login.
//
// Docs: https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html
//
// ## Four key differences from GitHub / Google (copying the standard library
// blindly will trip on these)
//
//  1. **The authorize endpoint is not a standard OAuth2 endpoint**: it is
//     `open.weixin.qq.com/connect/qrconnect`, and the URL must end with the
//     `#wechat_redirect` fragment — **without it WeChat directly reports "the
//     link cannot be accessed"**. `x/oauth2`'s `Config.AuthCodeURL()` cannot
//     generate that fragment, so this provider writes the URL by hand.
//  2. **`redirect_uri` must be urlEncoded**: WeChat requires the entire
//     callback address to be encoded. `url.Values.Encode()` would encode `%`
//     into `%25`, causing double encoding. It must be assembled manually.
//  3. **scope is fixed to `snsapi_login`**: other values (e.g. `snsapi_base`)
//     are rejected outright, and scope cannot be changed after being requested
//     in the Open Platform console.
//  4. **No id_token, nothing to verify with a signature**: the identity
//     credential is `access_token + openid`. Security comes from "one-time
//     code + 10-minute expiry" + HTTPS throughout, so the returned **openid
//     must be verified non-empty** and must not be allowed through just
//     because a field is missing.
//
// ## Binding key choice: unionid first, openid as fallback
//
// openid is unique per "the current Open Platform account", while unionid is
// unique across **all apps under the same Open Platform account** (website app
// / official account / mobile app). Using unionid as the binding key
// identifies the same user across apps; when the Open Platform account is not
// linked, unionid may be empty, in which case fall back to openid.
type WeChatProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	// apiBase is for injecting a mock in tests.
	apiBase string
}

// NewWeChatProvider constructs a WeChat provider.
func NewWeChatProvider(cfg Config) *WeChatProvider {
	// The WeChat website app's scope has only one legal value and does not
	// accept customization.
	_ = cfg.Scopes
	return &WeChatProvider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		apiBase:      "https://api.weixin.qq.com",
	}
}

// Name implements Provider.
func (p *WeChatProvider) Name() string { return "wechat" }

// Enabled implements Provider.
func (p *WeChatProvider) Enabled() bool {
	return p.clientID != "" && p.clientSecret != "" && p.redirectURL != ""
}

// AuthCodeURL implements Provider.
//
// The URL is assembled by hand instead of using url.Values.Encode(): WeChat
// requires redirect_uri to be encoded only once, while Encode() re-encodes the
// already-encoded % into %25 (double encoding, causing WeChat to parse an
// incorrect callback address).
func (p *WeChatProvider) AuthCodeURL(state string) string {
	// ⚠️ redirect_uri must be urlEncoded (including protocol, port, and path).
	return "https://open.weixin.qq.com/connect/qrconnect" +
		"?appid=" + url.QueryEscape(p.clientID) +
		"&redirect_uri=" + url.QueryEscape(p.redirectURL) +
		"&response_type=code" +
		// Web apps currently only support snsapi_login; other values are
		// rejected by WeChat.
		"&scope=snsapi_login" +
		"&state=" + url.QueryEscape(state) +
		// ⚠️ It must end with #wechat_redirect, otherwise WeChat shows "this
		// link cannot be accessed".
		"#wechat_redirect"
}

// wechatTokenResp is the /sns/oauth2/access_token response.
//
// ⚠️ **All WeChat endpoints return HTTP 200 even on error**; the error is in
// the body's errcode. Checking only the HTTP status code would treat failures
// as successes, so errcode must be checked explicitly.
type wechatTokenResp struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
}

// wechatUserInfo is the /sns/userinfo response.
//
// ⚠️ Since 2021-10-20 WeChat no longer returns sex / province / city /
// country, so these fields are not included here.
type wechatUserInfo struct {
	OpenID     string   `json:"openid"`
	Nickname   string   `json:"nickname"`
	HeadImgURL string   `json:"headimgurl"`
	UnionID    string   `json:"unionid"`
	Privilege  []string `json:"privilege"`
	ErrCode    int      `json:"errcode"`
	ErrMsg     string   `json:"errmsg"`
}

// Exchange implements Provider: exchanges access_token → fetches user info.
func (p *WeChatProvider) Exchange(ctx context.Context, code string) (*UserInfo, string, string, error) {
	tr, err := p.fetchToken(ctx, code)
	if err != nil {
		return nil, "", "", err
	}
	info, err := p.fetchUserInfo(ctx, tr.AccessToken, tr.OpenID)
	if err != nil {
		return nil, "", "", err
	}
	// Binding key: unionid first (stable across apps), falling back to openid
	// when absent.
	//
	// ⚠️ Both being empty means WeChat returned an abnormal response and must
	// be rejected — otherwise an empty binding key would be written, causing
	// all "WeChat returned no identifier" accounts to be misbound to each
	// other.
	providerUserID := info.UnionID
	if providerUserID == "" {
		providerUserID = tr.UnionID
	}
	if providerUserID == "" {
		providerUserID = tr.OpenID
	}
	if providerUserID == "" {
		return nil, "", "", fmt.Errorf("%w: wechat did not return openid/unionid", ErrExchangeFailed)
	}
	if info.OpenID == "" {
		info.OpenID = tr.OpenID
	}
	return &UserInfo{
		ProviderUserID: providerUserID,
		Nickname:       info.Nickname,
		//⚠️ **WeChat APIs do not return an email** (and it cannot be
		// requested), so Email is always empty. This means "password recovery
		// by email" is unavailable for pure WeChat users; a separate
		// mobile/email binding flow is needed.
		AvatarURL: p.normalizeAvatar(info.HeadImgURL),
	}, tr.AccessToken, tr.RefreshToken, nil
}

// fetchToken exchanges the code for an access_token.
//
// Endpoint: GET /sns/oauth2/access_token
//
//	?appid=&secret=&code=&grant_type=authorization_code
func (p *WeChatProvider) fetchToken(ctx context.Context, code string) (*wechatTokenResp, error) {
	q := url.Values{}
	q.Set("appid", p.clientID)
	q.Set("secret", p.clientSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	body, err := p.getJSON(ctx, p.apiBase+"/sns/oauth2/access_token?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("%w: wechat token exchange request failed: %v", ErrExchangeFailed, err)
	}
	var tr wechatTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("%w: failed to parse wechat token response: %v", ErrExchangeFailed, err)
	}
	// WeChat errors also return 200, so errcode must be checked explicitly.
	// Common ones:
	//   40029 invalid code (code expired/already used/does not belong to this
	//   appid)
	//   40013 invalid appid   40125 invalid appsecret
	if tr.ErrCode != 0 {
		return nil, fmt.Errorf("%w: wechat returned errcode=%d errmsg=%s", ErrExchangeFailed, tr.ErrCode, tr.ErrMsg)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("%w: wechat did not return access_token", ErrExchangeFailed)
	}
	return &tr, nil
}

// fetchUserInfo fetches basic user info.
//
// Endpoint: GET /sns/userinfo?access_token=&openid=
func (p *WeChatProvider) fetchUserInfo(ctx context.Context, accessToken, openID string) (*wechatUserInfo, error) {
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("openid", openID)
	q.Set("lang", "zh_CN")
	body, err := p.getJSON(ctx, p.apiBase+"/sns/userinfo?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("%w: wechat user info request failed: %v", ErrExchangeFailed, err)
	}
	var ui wechatUserInfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return nil, fmt.Errorf("%w: failed to parse wechat user info: %v", ErrExchangeFailed, err)
	}
	// 40003 invalid openid / 42001 access_token expired
	if ui.ErrCode != 0 {
		return nil, fmt.Errorf("%w: wechat user info errcode=%d errmsg=%s", ErrExchangeFailed, ui.ErrCode, ui.ErrMsg)
	}
	return &ui, nil
}

// getJSON issues a GET and reads the response body.
func (p *WeChatProvider) getJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// normalizeAvatar normalizes the avatar URL.
//
// A trailing `/0` on a WeChat avatar URL means 640×640, while `/46`~`/132`
// are thumbnails. `/0` is uniformly used to get the original image, avoiding
// the http/https mixing caused by the protocol prefix.
//
// ⚠️ **WeChat avatar URLs stop working once the user changes their avatar**.
// So it can only serve as a "display snapshot"; important scenarios must
// re-host the image in their own storage (not currently done in this project;
// see the AvatarURL comment in model/oauth.go).
func (p *WeChatProvider) normalizeAvatar(raw string) string {
	if raw == "" {
		return ""
	}
	// Strip the trailing size segment (like .../0 or .../96), then append /0
	// for the original image.
	if i := strings.LastIndex(raw, "/"); i > 0 {
		last := raw[i+1:]
		switch last {
		case "0", "46", "64", "96", "132":
			raw = raw[:i]
		}
	}
	return raw + "/0"
}

// errNotImplemented is a placeholder error for unimplemented behavior.
type errNotImplemented string

func (e errNotImplemented) Error() string { return string(e) }
