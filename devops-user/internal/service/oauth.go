package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/dao"
	"github.com/stanley760/devops/devops-user/internal/model"
	cendoauth2 "github.com/stanley760/devops/devops-user/internal/oauth2"
)

// OAuth is the third-party login use case.
type OAuth struct {
	authM      *auth.Manager
	providers  map[string]cendoauth2.Provider
	states     cendoauth2.StateStore
	stateTTL   time.Duration
	authzNames map[string]string // provider → nickname prefix
	autoReg    map[string]bool   // provider → whether to auto-create accounts
	// redirectAllow holds whitelist prefixes for post-login redirect URLs
	// (against open redirect).
	redirectAllow []string
}

// OAuthDeps holds the dependencies of the OAuth use case.
type OAuthDeps struct {
	AuthM     *auth.Manager
	Providers map[string]cendoauth2.Provider
	States    cendoauth2.StateStore
	StateTTL  time.Duration
	// AutoRegister controls whether to auto-create accounts when not bound
	// (per provider). nil means no provider auto-creates.
	AutoRegister map[string]bool
	// NicknamePrefix is the nickname prefix when auto-creating accounts (per
	// provider).
	NicknamePrefix map[string]string
	// RedirectWhitelist holds allowed redirect-back prefixes (against open
	// redirect / phishing).
	RedirectWhitelist []string
}

// NewOAuth constructs the OAuth use case.
func NewOAuth(d OAuthDeps) *OAuth {
	if d.StateTTL <= 0 {
		d.StateTTL = 10 * time.Minute
	}
	return &OAuth{
		authM:         d.AuthM,
		providers:     d.Providers,
		states:        d.States,
		stateTTL:      d.StateTTL,
		autoReg:       d.AutoRegister,
		authzNames:    d.NicknamePrefix,
		redirectAllow: d.RedirectWhitelist,
	}
}

// SafeRedirect verifies that a post-login redirect URL is within the
// whitelist.
//
// ⚠️ **Skipping this check enables an open redirect (phishing)**: an attacker
// crafts
//
//	/oauth/callback?...&redirect_after=https://evil.example
//
// and after the victim logs in they are redirected to the attacker's site,
// handing over the token.
//
// An empty whitelist rejects everything — a conservative choice: better to
// produce no redirect-back URL than to let an arbitrary URL become a
// legitimate redirect target.
func (s *OAuth) SafeRedirect(after string) (string, error) {
	if after == "" {
		return "", nil
	}
	if len(s.redirectAllow) == 0 {
		return "", errs.New(errs.CodeParam, "redirect whitelist is not configured; the redirect URL has been rejected")
	}
	for _, prefix := range s.redirectAllow {
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(after, prefix) {
			return after, nil
		}
	}
	return "", errs.New(errs.CodeParam, "redirect URL is not in the whitelist")
}

// ProviderStatus is the availability status of a provider (for the frontend to
// decide whether to show its entry).
type ProviderStatus struct {
	Name         string `json:"name" doc:"provider identifier"`
	Enabled      bool   `json:"enabled" doc:"whether usable (credentials complete)"`
	AuthorizeURL string `json:"authorize_url,omitempty" doc:"authorize redirect URL (state already generated, one-time)"`
}

// ListProviders returns each provider's status and generates an authorize URL
// (with a one-time state) for each.
func (s *OAuth) ListProviders(ctx context.Context) ([]ProviderStatus, error) {
	if s == nil || len(s.providers) == 0 {
		return nil, errs.New(errs.CodeInternal, "third-party login is not configured")
	}
	out := make([]ProviderStatus, 0, len(s.providers))
	for name, p := range s.providers {
		st := ProviderStatus{Name: name, Enabled: p.Enabled()}
		if !p.Enabled() {
			// Providers with incomplete credentials are still returned, so the
			// frontend hides their entries instead of erroring.
			out = append(out, st)
			continue
		}
		state, err := generateState()
		if err != nil {
			return nil, errs.New(errs.CodeInternal, "failed to generate state")
		}
		if err := s.states.Save(ctx, state, cendoauth2.StateValue{Provider: name}, s.stateTTL); err != nil {
			return nil, errs.New(errs.CodeInternal, "failed to save state")
		}
		st.AuthorizeURL = p.AuthCodeURL(state)
		out = append(out, st)
	}
	return out, nil
}

// StartBind lets a logged-in user actively bind a third-party account:
// generates a state and marks BindUserID.
func (s *OAuth) StartBind(ctx context.Context, providerName, redirectAfter string, userID int64) (string, error) {
	p, err := s.provider(providerName)
	if err != nil {
		return "", err
	}
	if !p.Enabled() {
		return "", errs.New(errs.CodeParam, "this third-party login is not configured")
	}
	// The redirect-back URL must pass the whitelist first (against open
	// redirect).
	safeAfter, err := s.SafeRedirect(redirectAfter)
	if err != nil {
		return "", err
	}
	state, err := generateState()
	if err != nil {
		return "", errs.New(errs.CodeInternal, "failed to generate state")
	}
	if err := s.states.Save(ctx, state, cendoauth2.StateValue{
		Provider:      providerName,
		RedirectAfter: safeAfter,
		BindUserID:    userID,
	}, s.stateTTL); err != nil {
		return "", errs.New(errs.CodeInternal, "failed to save state")
	}
	return p.AuthCodeURL(state), nil
}

// OAuthResult is the result of third-party login/binding.
type OAuthResult struct {
	// TokenPair is the issued token pair (empty in the binding scenario).
	TokenPair *TokenPair `json:"token_pair,omitempty" doc:"issued on successful login; empty in the binding scenario"`
	// Bound true means this operation is a "bind" rather than a "login".
	Bound bool `json:"bound" doc:"whether this operation is a bind (true) or a login (false)"`
	// UserID is the local user ID.
	UserID int64 `json:"user_id" doc:"local user ID"`
	// Nickname is the local nickname.
	Nickname string `json:"nickname" doc:"local nickname"`
}

// Callback handles the third-party callback: verify state → exchange token →
// fetch info → log in or bind.
//
// Two paths (decided by state.BindUserID):
//   - BindUserID == 0: login. If already bound, issue a token pair; if not,
//     per autoReg decide between auto-creating an account or erroring to
//     require login first;
//   - BindUserID > 0: bind. Bind to that local user; reject if the
//     third-party account is already bound to someone else.
func (s *OAuth) Callback(ctx context.Context, providerName, code, state string) (*OAuthResult, error) {
	p, err := s.provider(providerName)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, errs.New(errs.CodeParam, "missing authorization code")
	}
	// One-time state verification: failure means a CSRF attack or an expired
	// replay.
	sv, err := s.states.Verify(ctx, state)
	if err != nil {
		return nil, errs.New(errs.CodeUnauthorized, "state verification failed, please start the login again")
	}
	// The provider recorded in the state must match the callback path,
	// preventing "using A's state on B's callback".
	if sv.Provider != providerName {
		return nil, errs.New(errs.CodeUnauthorized, "state does not match the provider")
	}

	info, accessTok, refreshTok, err := p.Exchange(ctx, code)
	if err != nil {
		return nil, errs.New(errs.CodeUnauthorized, "third-party authorization verification failed")
	}
	if info == nil || info.ProviderUserID == "" {
		return nil, errs.New(errs.CodeUnauthorized, "the third party did not return a user identifier")
	}
	prov := model.OAuthProvider(providerName)

	identity, err := dao.GetOAuthIdentity(ctx, prov, info.ProviderUserID)
	if err != nil {
		return nil, err
	}

	// Path one: a logged-in user actively binds.
	if sv.BindUserID > 0 {
		if identity != nil {
			if identity.UserID == sv.BindUserID {
				// Binding the same account again: idempotent, just refresh the
				// token.
				_ = dao.UpdateOAuthToken(ctx, identity.ID, accessTok, refreshTok, 0, info.Nickname, info.AvatarURL)
				return &OAuthResult{Bound: true, UserID: sv.BindUserID}, nil
			}
			return nil, errs.New(errs.CodeParam, "this third-party account is already bound to another user")
		}
		// Before binding, verify the user exists and is not banned (otherwise
		// it could be bound to a deleted account).
		u, err := dao.GetUserByID(ctx, sv.BindUserID)
		if err != nil {
			return nil, err
		}
		if u.Disabled {
			return nil, errs.New(errs.CodeForbidden, "account has been banned")
		}
		if err := dao.CreateOAuthIdentity(ctx, &model.OAuthIdentity{
			UserID:         sv.BindUserID,
			Provider:       prov,
			ProviderUserID: info.ProviderUserID,
			Nickname:       info.Nickname,
			AvatarURL:      info.AvatarURL,
			AccessToken:    accessTok,
			RefreshToken:   refreshTok,
		}); err != nil {
			return nil, err
		}
		return &OAuthResult{Bound: true, UserID: sv.BindUserID, Nickname: u.Nickname}, nil
	}

	// Path two: login. Issue directly if already bound.
	if identity != nil {
		u, err := dao.GetUserByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		if u.Disabled {
			return nil, errs.New(errs.CodeForbidden, "account has been banned")
		}
		_ = dao.UpdateOAuthToken(ctx, identity.ID, accessTok, refreshTok, 0, info.Nickname, info.AvatarURL)
		pair, err := s.issue(ctx, u.ID, u.Username)
		if err != nil {
			return nil, err
		}
		return &OAuthResult{TokenPair: &pair, UserID: u.ID, Nickname: u.Nickname}, nil
	}

	// Not bound: decide whether to auto-create an account per config.
	if !s.autoReg[providerName] {
		return nil, errs.New(errs.CodeParam, "this third-party account is not bound yet, please log in first and then bind")
	}
	u, err := s.autoCreateUser(ctx, providerName, info)
	if err != nil {
		return nil, err
	}
	// Account creation implies binding (done after creation: if binding fails
	// an orphan account is left, but doing binding first and then failing
	// account creation would leave a binding pointing at a nonexistent user,
	// which is worse).
	if err := dao.CreateOAuthIdentity(ctx, &model.OAuthIdentity{
		UserID:         u.ID,
		Provider:       prov,
		ProviderUserID: info.ProviderUserID,
		Nickname:       info.Nickname,
		AvatarURL:      info.AvatarURL,
		AccessToken:    accessTok,
		RefreshToken:   refreshTok,
	}); err != nil {
		return nil, err
	}
	pair, err := s.issue(ctx, u.ID, u.Username)
	if err != nil {
		return nil, err
	}
	return &OAuthResult{TokenPair: &pair, UserID: u.ID, Nickname: u.Nickname}, nil
}

// autoCreateUser auto-creates a local account on a third party's first login.
func (s *OAuth) autoCreateUser(ctx context.Context, providerName string, info *cendoauth2.UserInfo) (*model.User, error) {
	// Username: prefer the third-party nickname (sanitized into legal
	// characters), with a numeric suffix on conflict.
	base := sanitizeUsername(info.Nickname)
	if base == "" {
		base = providerName + "_user"
	}
	username := base
	for i := 1; i <= 20; i++ {
		taken, err := dao.IsUsernameTaken(ctx, username)
		if err != nil {
			return nil, err
		}
		if !taken {
			break
		}
		username = fmt.Sprintf("%s%d", base, i)
	}
	// Auto-created accounts have **no usable password**: a random,
	// unmemorable 32-byte strong random string, ruling out login via
	// "guessing the default password" or an "empty password".
	random, err := generateRandomSecret()
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "failed to generate the initial password")
	}
	hash, err := hashPassword(random)
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "password hashing failed")
	}
	nickname := info.Nickname
	if prefix := s.authzNames[providerName]; prefix != "" {
		nickname = prefix + nickname
	}
	if nickname == "" {
		nickname = username
	}
	u := &model.User{
		Username: username,
		Password: hash,
		Nickname: nickname,
	}
	if info.Email != "" {
		// ⚠️ Third-party email may be unverified; it is for display only here,
		// **not for password recovery or account association**.
		u.Email = &info.Email
	}
	if err := dao.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Unbind unbinds a third-party account.
func (s *OAuth) Unbind(ctx context.Context, userID int64, providerName string) error {
	if !model.OAuthProvider(providerName).Valid() {
		return errs.New(errs.CodeParam, "unsupported third-party provider")
	}
	return dao.DeleteOAuthIdentity(ctx, userID, model.OAuthProvider(providerName))
}

// ListBound lists bound third-party accounts.
func (s *OAuth) ListBound(ctx context.Context, userID int64) ([]model.OAuthIdentity, error) {
	list, err := dao.ListOAuthIdentities(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Token fields are not returned (though already marked json:"-", this is
	// an extra guard against misuse).
	for i := range list {
		list[i].AccessToken = ""
		list[i].RefreshToken = ""
	}
	return list, nil
}

func (s *OAuth) issue(ctx context.Context, userID int64, username string) (TokenPair, error) {
	pair, err := s.authM.Issue(ctx, userID, username)
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeInternal, "failed to issue tokens")
	}
	return pair, nil
}

func (s *OAuth) provider(name string) (cendoauth2.Provider, error) {
	p, ok := s.providers[name]
	if !ok {
		return nil, errs.New(errs.CodeParam, "unsupported third-party login provider")
	}
	return p, nil
}

// generateState generates a 256-bit random state (key to CSRF protection; must
// be unpredictable).
func generateState() (string, error) {
	return generateRandomSecret()
}

// generateRandomSecret generates the base64url form of a 32-byte random
// string.
func generateRandomSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// sanitizeUsername sanitizes a third-party nickname into a legal local
// username. Illegal characters are replaced with underscores; if the result is
// empty, an empty string is returned for the caller to decide.
func sanitizeUsername(s string) string {
	if s == "" {
		return ""
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
		if len(out) >= 24 { // cap the length, leaving room for a suffix
			break
		}
	}
	res := string(out)
	for len(res) > 0 && res[0] == '_' {
		// must start with a letter (required by usernamePattern)
		res = res[1:]
	}
	return res
}
