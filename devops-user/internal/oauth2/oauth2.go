// Package oauth2 implements the C-side third-party login (GitHub / Google /
// WeChat) authorization code flow.
//
// ## Flow
//
//  1. GET  /user/api/v1/oauth/providers
//     generates a state (CSRF protection), builds the authorize URL and
//     returns it to the frontend;
//  2. the frontend redirects to the third-party page; after the user
//     authorizes, the third party calls back
//     GET /user/api/v1/oauth/{provider}/callback?code&state
//     verify state → exchange code for a token → fetch third-party user info;
//  3. look up the binding by (provider, provider_user_id):
//     - already bound → issue a local token pair;
//     - not bound → per config, decide between "auto-create an account" or
//     "require login first, then bind".
//
// ## Security notes
//
//   - **state must be verified and one-time**: without verification, login can
//     be hijacked by CSRF (the attacker feeds their own code to the victim to
//     log them into the attacker's account); without one-time use, the same
//     state can be replayed.
//   - **provider_user_id uses the third party's stable identifier**, not
//     username/email/nickname — the first two can change or be swapped, and
//     WeChat nicknames can repeat; using them as the binding key causes
//     misbinding and impersonation. Values per provider: GitHub numeric ID,
//     Google `sub`, WeChat `unionid` (falls back to openid when absent).
//   - **Google / WeChat responses must be verified for authenticity**: Google
//     goes through tokeninfo to verify aud/iss; WeChat has no id_token, so
//     security comes from "one-time code + 10-minute expiry" + HTTPS
//     throughout, hence openid must be verified non-empty and errcode checked
//     explicitly (**WeChat returns HTTP 200 even on error**).
//   - Third-party tokens obtained are highly sensitive credentials and must be
//     encrypted before storage in production (see model/oauth.go).
//
// ## Dependency boundary
//
// This package depends only on golang.org/x/oauth2 and the standard library,
// and **does not depend on this project's dao/model**, so it can be tested and
// reused independently. The parsing differences of third-party user info are
// also contained within provider.go.
//
// ⚠️ WeChat's authorize endpoint is not a standard OAuth2 endpoint (it needs
// the #wechat_redirect fragment, single-layer encoding of redirect_uri, and a
// fixed scope of snsapi_login), so WeChatProvider **does not use**
// oauth2.Config.AuthCodeURL and builds the URL by hand — see provider.go for
// details.
package oauth2

import (
	"context"
	"errors"
	"time"
)

// ErrStateInvalid means the state is invalid, expired, or already used (CSRF
// protection failure).
var ErrStateInvalid = errors.New("state is invalid or has expired")

// ErrProviderUnsupported means the third-party provider is unsupported.
var ErrProviderUnsupported = errors.New("unsupported third-party login provider")

// ErrExchangeFailed means exchanging the authorization code for a token failed
// (usually the code expired or was already used).
var ErrExchangeFailed = errors.New("authorization code verification failed")

// UserInfo is the third-party side's user info (normalized, uniform across
// providers).
//
// ⚠️ Email may not be verified: GitHub's public email is a case in point.
// **It must not be used for account association or password recovery**, only
// for display. Scenarios needing an email must go through each provider's
// "verified email" endpoint or have the user verify it themselves.
type UserInfo struct {
	// ProviderUserID is the stable user identifier within the provider (the
	// binding key).
	ProviderUserID string
	Nickname       string
	Email          string // may be unverified, for display only
	AvatarURL      string
}

// Config is the configuration for a single provider.
type Config struct {
	// ClientID / ClientSecret are third-party app credentials. Secret is
	// injected by secret management and must not be committed.
	ClientID     string
	ClientSecret string
	// RedirectURL is the callback URL; it must match exactly what is
	// registered in the third-party console.
	RedirectURL string
	// Scopes are the requested permission scopes.
	Scopes []string
	// AutoRegister controls whether to auto-create a local user when the
	// third-party account is not bound.
	//
	// ⚠️ Auto-creating accounts turns "anyone who can log into that third
	// party" into a local user. If the product does not want accounts created
	// at will, set it to false: when not bound, return "please log in and bind
	// first", and let an already-logged-in user bind actively (which allows a
	// "logged-in user vs third-party account consistency" check).
	AutoRegister bool
	// NicknamePrefix is the nickname prefix when auto-creating accounts, to
	// identify the source (e.g. "GitHub:").
	NicknamePrefix string
}

// Enabled reports whether the provider is usable (credentials complete).
func (c Config) Enabled() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

// StateStore is the storage of the state (CSRF protection).
//
// It must be consumed "one-time": deleted immediately after a successful
// Verify. Allowing replay is equivalent to no protection.
type StateStore interface {
	// Save stores the state (carrying the state → provider /
	// post-redirect target mapping).
	Save(ctx context.Context, state string, val StateValue, ttl time.Duration) error
	// Verify verifies and **one-time** retrieves it. Invalid or already-used
	// both return ErrStateInvalid.
	Verify(ctx context.Context, state string) (StateValue, error)
}

// StateValue is data associated with the state.
type StateValue struct {
	Provider string
	// RedirectAfter is the frontend URL to return to after a successful login.
	//
	// ⚠️ A whitelist must be enforced: without it, an attacker can use an open
	// redirect to phish (redirecting to their own site to steal the token).
	RedirectAfter string
	// BindUserID > 0 means "a logged-in user actively binding a third-party
	// account"; after the callback it is bound to that user.
	BindUserID int64
}
