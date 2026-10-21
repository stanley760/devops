// Package service carries the C-side business use cases.
//
// Layering: this package only does "business rules + parameter validation",
// with all SQL in internal/dao, and handlers (internal/api/v1) unaware of this
// package. This way, swapping the storage implementation leaves this package's
// interfaces unchanged.
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/stanley760/devops/devops-infra/errs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/dao"
	"github.com/stanley760/devops/devops-user/internal/loginlock"
	"github.com/stanley760/devops/devops-user/internal/model"
)

// authCounter holds business metrics: login/registration attempt counts
// (auth_login / auth_register → auth_*_total). All labels are bounded sets:
// result=success|failure, code=the failure business code (always 200 on
// success). Lazily created: by the first call the global MeterProvider has
// been wired by observability.Init; when uninitialized (unit tests/probes) it
// falls back to a noop meter. Metric failures do not affect business
// responses.
var (
	authLoginCounter = sync.OnceValue(func() metric.Int64Counter {
		c, err := otel.Meter("github.com/stanley760/devops/devops-user/service").Int64Counter(
			"auth_login",
			metric.WithDescription("login attempt count; result=success|failure, code is the failure business code (success=200)"),
		)
		if err != nil {
			otel.Handle(err)
		}
		return c
	})
	authRegisterCounter = sync.OnceValue(func() metric.Int64Counter {
		c, err := otel.Meter("github.com/stanley760/devops/devops-user/service").Int64Counter(
			"auth_register",
			metric.WithDescription("registration attempt count; result=success|failure, code is the failure business code (success=200)"),
		)
		if err != nil {
			otel.Handle(err)
		}
		return c
	})
)

// authResultAttr is the authentication result label value.
func authResultAttr(err error) attribute.KeyValue {
	if err != nil {
		return attribute.String("result", "failure")
	}
	return attribute.String("result", "success")
}

// authCodeAttr is the failure reason code: the bounded business code carried
// by BizError; non-BizError is categorized as 500. Specific failure details
// (wrong password/user not found/locked, etc.) go to the access log and not
// into metric labels, to control cardinality.
func authCodeAttr(err error) attribute.KeyValue {
	return attribute.Int("code", authErrCode(err))
}

// authErrCode is the authentication result business code (200 on success; on
// failure the BizError's 4xx/5xx, non-business errors as 500).
func authErrCode(err error) int {
	if err == nil {
		return errs.CodeOK
	}
	var be *errs.BizError
	if errors.As(err, &be) && be.Code >= 400 && be.Code <= 599 {
		return be.Code
	}
	return errs.CodeInternal
}

// Business errors (defined within the service, invisible to handlers).
//
// ⚠️ All of these errors must ultimately go through errs.ToHumaError to become
// an envelope with a status code; returning them directly makes huma fall back
// to 500 (see the api/v1 package comment).
var (
	// ErrInvalidCredentials means the account or password is incorrect.
	//
	// "User not found" and "wrong password" **must** return the same error:
	// distinguishing them would let attackers enumerate which usernames
	// exist. dao.GetUserByUsername already returns this error per that
	// convention.
	ErrInvalidCredentials = errors.New("incorrect account or password")
	// ErrAccountDisabled means the account is banned.
	ErrAccountDisabled = errors.New("account has been banned")
	// ErrNotSelf means attempting to operate on another user's data (C-side
	// row-level ownership check).
	ErrNotSelf = errors.New("you can only operate on your own data")
)

// usernamePattern is the login name rule: starts with a letter, 3-32
// alphanumeric/underscore characters.
//
// Together with the uk_username unique constraint in the database these are
// two layers: this regex gives the user a friendly message, and the unique
// index guarantees no duplicate names under concurrency (races are covered by
// dao.CreateUser).
var usernamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,31}$`)

// validGender covers gender values: 0=unknown 1=male 2=female (same convention
// as devops-system's t_system_users). Validated centrally in the service: the
// API layer's huma minimum/maximum only blocks direct HTTP calls, and this is
// the backstop for other entry points (e.g. third-party auto-account
// creation), preventing dirty values from entering the database.
func validGender(g int8) bool { return g >= 0 && g <= 2 }

// Auth is the authentication use case (register / login / logout).
type Auth struct {
	authM   *auth.Manager
	limiter *loginlock.Limiter
	captcha *Captcha
}

// NewAuth constructs the authentication use case. When limiter is nil, login is
// not rate limited (for testing only); when captchaSvc is nil, captcha
// verification is skipped (also for testing only).
func NewAuth(authM *auth.Manager, limiter *loginlock.Limiter, captchaSvc *Captcha) *Auth {
	return &Auth{authM: authM, limiter: limiter, captcha: captchaSvc}
}

// RegisterInput is the registration input.
type RegisterInput struct {
	Username string
	Password string
	Nickname string
	Email    string
	Mobile   string
	Gender   int8
	// Captcha carries the captcha verification data (required when captcha verification is enabled).
	//
	// Open registration is the most script-friendly endpoint there is, so registration is captcha-
	// gated just like login (a bot that can register freely can build an arbitrary account corpus).
	Captcha CaptchaInput
}

// TokenPair is the token pair issuance result (aliases auth.TokenPair to
// avoid duplicate definitions).
type TokenPair = auth.TokenPair

// Register creates an account and issues an access token directly (saving an
// immediate second login after registration). Business metric: records one
// auth_register_total{result, code} regardless of outcome; metric failures do
// not affect registration.
func (s *Auth) Register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	pair, err := s.register(ctx, in)
	authRegisterCounter().Add(ctx, 1, metric.WithAttributes(authResultAttr(err), authCodeAttr(err)))
	return pair, err
}

// register is the main registration flow.
func (s *Auth) register(ctx context.Context, in RegisterInput) (TokenPair, error) {
	// Captcha first: it is the cheapest rejection of automated traffic (one Redis round trip, no
	// bcrypt, no unique-index probe), and registration is the endpoint most attractive to automate.
	if s.captcha != nil {
		if err := s.captcha.Verify(ctx, in.Captcha); err != nil {
			return TokenPair{}, err
		}
	}
	if !usernamePattern.MatchString(in.Username) {
		return TokenPair{}, errs.New(errs.CodeParam,
			"username must start with a letter, 3-32 alphanumeric or underscore characters")
	}
	if len(in.Password) < 8 {
		return TokenPair{}, errs.New(errs.CodeParam, "password must be at least 8 characters")
	}
	if !validGender(in.Gender) {
		return TokenPair{}, errs.New(errs.CodeParam, "invalid gender value (0=unknown, 1=male, 2=female)")
	}

	// Intercepted early to give a friendly message; under concurrency the
	// unique index is still the arbiter.
	taken, err := dao.IsUsernameTaken(ctx, in.Username)
	if err != nil {
		return TokenPair{}, err
	}
	if taken {
		return TokenPair{}, errs.New(errs.CodeParam, "username already registered")
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		if errors.Is(err, errPasswordTooLong) {
			return TokenPair{}, errs.New(errs.CodeParam, "password too long (72 bytes maximum)")
		}
		return TokenPair{}, errs.New(errs.CodeInternal, "password hashing failed")
	}
	u := &model.User{
		Username: in.Username,
		Password: hash,
		Nickname: in.Nickname,
		Gender:   in.Gender,
	}
	if in.Email != "" {
		u.Email = &in.Email
	}
	if in.Mobile != "" {
		u.Mobile = &in.Mobile
	}
	if err := dao.CreateUser(ctx, u); err != nil {
		return TokenPair{}, err
	}
	return s.issue(ctx, u.ID, u.Username)
}

// Login verifies the captcha and the account password, then issues a token pair.
//
// ⚠️ "User not found" and "wrong password" **must return exactly the same
// message** (including the status code), otherwise attackers could enumerate
// which usernames exist. dao returns a unified message for nonexistence, and
// the wrong-password branch here uses the same sentence — the two must be
// changed in sync.
//
// Check order (deliberate): captcha → rate limit → password. The captcha is
// checked before the rate limiter so that a flood of unsolved captcha attempts
// never consumes a legitimate user's failure budget (the limiter counts login
// failures per account, and captcha rejections are not the user's fault).
//
// Rate limiting and lockout: check **before** verification, record **after**
// failure.
//   - "User not found" also counts as a failure — otherwise attackers could
//     enumerate usernames at no cost (neither counted nor locked);
//   - A banned account does **not** count as a failure — it is not a user typo
//     but an account state issue, and counting it would wrongly trip the IP
//     count of normal users.
//   - On success the account's count is cleared; the IP count is not,
//     otherwise a normal user's occasional typo would permanently trigger IP
//     rate limiting.
//
// Business metric: records one auth_login_total{result, code} regardless of
// outcome; metric failures do not affect login.
func (s *Auth) Login(ctx context.Context, clientIP, username, password string, captchaIn CaptchaInput) (TokenPair, error) {
	pair, err := s.login(ctx, clientIP, username, password, captchaIn)
	authLoginCounter().Add(ctx, 1, metric.WithAttributes(authResultAttr(err), authCodeAttr(err)))
	return pair, err
}

// login is the main login flow.
func (s *Auth) login(ctx context.Context, clientIP, username, password string, captchaIn CaptchaInput) (TokenPair, error) {
	// Captcha before the rate limit: the cheapest rejection, and it must not
	// burn the account's failure budget on someone else's bot traffic.
	if s.captcha != nil {
		if err := s.captcha.Verify(ctx, captchaIn); err != nil {
			return TokenPair{}, err
		}
	}
	if err := s.checkLimit(ctx, clientIP, username); err != nil {
		return TokenPair{}, err
	}
	u, err := dao.GetUserByUsername(ctx, username)
	if err != nil {
		s.recordFail(ctx, clientIP, username)
		return TokenPair{}, err
	}
	if !checkPassword(password, u.Password) {
		s.recordFail(ctx, clientIP, username)
		return TokenPair{}, errs.New(errs.CodeUnauthorized, ErrInvalidCredentials.Error())
	}
	if u.Disabled {
		return TokenPair{}, errs.New(errs.CodeForbidden, ErrAccountDisabled.Error())
	}
	s.clearFail(ctx, username)
	return s.issue(ctx, u.ID, u.Username)
}

// Refresh exchanges a refresh token for a new token pair.
//
// Flow: first use the refresh token to get an access token (which carries
// identity) → check the database to confirm the account is not banned →
// re-issue with the latest username from the database (on the first Refresh,
// the Manager side's username is empty).
//
// When banning, the refresh token is invalidated as well; otherwise it could
// keep renewing indefinitely after the ban.
func (s *Auth) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	pair, err := s.authM.Refresh(ctx, refreshToken, "")
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeUnauthorized, auth.ErrRefreshInvalid.Error())
	}
	claims, err := s.authM.Parse(pair.AccessToken)
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeUnauthorized, auth.ErrRefreshInvalid.Error())
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return TokenPair{}, err
	}
	if u.Disabled {
		_ = s.authM.RevokeRefresh(ctx, pair.RefreshToken)
		return TokenPair{}, errs.New(errs.CodeForbidden, ErrAccountDisabled.Error())
	}
	return s.issue(ctx, u.ID, u.Username)
}

// Logout revokes the current session and refresh token.
//
// The refresh token is revoked too: otherwise after logout the client could
// still use it to mint a new token, which is not really logging out.
func (s *Auth) Logout(ctx context.Context, claims *auth.AccessClaims, refreshToken string) error {
	if err := s.authM.Logout(ctx, claims); err != nil {
		return err
	}
	return s.authM.RevokeRefresh(ctx, refreshToken)
}

func (s *Auth) issue(ctx context.Context, userID int64, username string) (TokenPair, error) {
	pair, err := s.authM.Issue(ctx, userID, username)
	if err != nil {
		return TokenPair{}, errs.New(errs.CodeInternal, "failed to issue tokens")
	}
	return pair, nil
}

// issueFor is identical to issue; semantically used for the
// "refresh/reissue" path.
func (s *Auth) issueFor(ctx context.Context, userID int64, username string) (TokenPair, error) {
	return s.issue(ctx, userID, username)
}

// checkLimit is the pre-login rate limit check.
func (s *Auth) checkLimit(ctx context.Context, clientIP, username string) error {
	if s.limiter == nil {
		return nil
	}
	switch err := s.limiter.Check(ctx, clientIP, username); {
	case errors.Is(err, loginlock.ErrLocked):
		return errs.New(errs.CodeForbidden, "too many failed login attempts, the account is temporarily locked, please try again in 15 minutes")
	case errors.Is(err, loginlock.ErrTooManyAttempts):
		return errs.New(errs.CodeTooManyRequests, "too many attempts, please try again later")
	case err != nil:
		return errs.New(errs.CodeInternal, "login rate limit check failed")
	}
	return nil
}

// recordFail records a login failure.
func (s *Auth) recordFail(ctx context.Context, clientIP, username string) {
	if s.limiter != nil {
		s.limiter.Fail(ctx, clientIP, username)
	}
}

// clearFail clears the account's failure count after a successful login.
func (s *Auth) clearFail(ctx context.Context, username string) {
	if s.limiter != nil {
		s.limiter.Success(ctx, username)
	}
}

// Profile is the user profile use case (including password change and
// banning).
type Profile struct {
	// authM forces the user offline on all devices after a password
	// change/ban.
	authM *auth.Manager
	// authLimiter clears login failure counts after a successful password
	// change; skipped when nil.
	authLimiter *loginlock.Limiter
}

// NewProfile constructs the profile use case. When limiter is nil, a password
// change does not clear failure counts.
func NewProfile(authM *auth.Manager, limiter *loginlock.Limiter) *Profile {
	return &Profile{authM: authM, authLimiter: limiter}
}

// Get reads the current user's profile.
//
// claims are injected by the JWT middleware; a user ID passed by the client is
// **not accepted** — C-side row-level ownership is enforced by "identity comes
// only from the token", not by parameter filtering.
func (s *Profile) Get(ctx context.Context, claims *auth.AccessClaims) (model.PublicProfile, error) {
	if claims == nil {
		return model.PublicProfile{}, errs.ErrUnauthorized
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return model.PublicProfile{}, err
	}
	return u.ToPublic(), nil
}

// UpdateInput is the profile update input (pointer fields: nil means do not
// modify that field).
type UpdateInput struct {
	Nickname *string
	Email    *string
	Mobile   *string
	Gender   *int8
	Avatar   *string
}

// Update modifies the current user's profile. Only your own (identity comes
// from the token, naturally satisfied).
func (s *Profile) Update(ctx context.Context, claims *auth.AccessClaims, in UpdateInput) (model.PublicProfile, error) {
	if claims == nil {
		return model.PublicProfile{}, errs.ErrUnauthorized
	}

	// Only non-nil fields go into the map: dao updates with a map, avoiding
	// zero-value overwrites of omitted fields.
	fields := make(map[string]any, 4)
	if in.Nickname != nil {
		fields["nickname"] = *in.Nickname
	}
	if in.Email != nil {
		fields["email"] = model.NormalizePtr(in.Email)
	}
	if in.Mobile != nil {
		fields["mobile"] = model.NormalizePtr(in.Mobile)
	}
	if in.Gender != nil {
		if !validGender(*in.Gender) {
			return model.PublicProfile{}, errs.New(errs.CodeParam, "invalid gender value (0=unknown, 1=male, 2=female)")
		}
		fields["gender"] = *in.Gender
	}
	if in.Avatar != nil {
		fields["avatar"] = model.NormalizePtr(in.Avatar)
	}

	if err := dao.UpdateProfile(ctx, claims.UserID, fields); err != nil {
		return model.PublicProfile{}, err
	}
	return s.Get(ctx, claims)
}

// ChangePassword changes one's own password.
//
// Security requirements:
//   - The old password must be verified — otherwise anyone holding the token
//     could change the password and permanently take over the account;
//   - New identical to old is rejected outright;
//   - After verification, the account's login failure count is cleared (it may
//     be temporarily locked from consecutive typos).
//
// **A password change logs out all devices** (default behavior): after a
// successful change, all of the user's devices are forced to log in again.
// Rationale: password leakage is a security incident, and the attacker may
// already hold other devices' refresh tokens; only changing the local session
// would leave a usable intrusion channel.
//
// When keepCurrentRefresh is non-empty the current device is kept (others are
// logged out) — note that the access side still kicks everything (the session
// side has no sid → refresh reverse lookup), so the current device must also
// log in again, except that its refresh token still works and can directly
// mint a new access token. **The current default passes empty**, i.e. kick
// everything.
func (s *Profile) ChangePassword(ctx context.Context, claims *auth.AccessClaims, oldPassword, newPassword, keepCurrentRefresh string) error {
	if claims == nil {
		return errs.ErrUnauthorized
	}
	if len(newPassword) < 8 {
		return errs.New(errs.CodeParam, "new password must be at least 8 characters")
	}
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return err
	}
	if !checkPassword(oldPassword, u.Password) {
		return errs.New(errs.CodeUnauthorized, "the current password is incorrect")
	}
	if oldPassword == newPassword {
		return errs.New(errs.CodeParam, "the new password must not be the same as the current password")
	}
	hashed, err := hashPassword(newPassword)
	if err != nil {
		if errors.Is(err, errPasswordTooLong) {
			return errs.New(errs.CodeParam, "password too long (72 bytes maximum)")
		}
		return errs.New(errs.CodeInternal, "password hashing failed")
	}
	if err := dao.UpdatePassword(ctx, claims.UserID, hashed); err != nil {
		return err
	}
	s.clearAccountFail(ctx, u.Username)

	// A password change logs out all devices: revoke all refresh tokens + all
	// access sessions. Failure does not roll back the password change — the
	// password has already been changed as a matter of fact; here it is only
	// logged and returned upward, avoiding users thinking the change failed
	// and retrying into a confused state.
	if err := s.revokeAllSessions(ctx, claims.UserID, keepCurrentRefresh); err != nil {
		return fmt.Errorf("password has been changed, but forced logout partially failed (the user needs to log in again): %w", err)
	}
	return nil
}

// revokeAllSessions forces the user offline on all devices.
func (s *Profile) revokeAllSessions(ctx context.Context, userID int64, keepRefresh string) error {
	if s.authM == nil {
		return nil
	}
	return s.authM.RevokeUserSessionsExcept(ctx, userID, keepRefresh)
}

// clearAccountFail clears the account's login failure count (called after a
// successful password change).
func (s *Profile) clearAccountFail(ctx context.Context, username string) {
	if s.authLimiter != nil {
		s.authLimiter.Success(ctx, username)
	}
}

// SetDisabled bans / unbans an account and forces it offline on all devices.
//
// **Banning must force logout**: merely marking disabled is not enough — the
// user's issued access tokens would still pass the guard within their validity
// period (this service verifies the Redis session state on every request, so
// revoking the session makes the next request return 401). Therefore,
// immediately after a successful ban, all of the user's refresh tokens and
// sessions are revoked.
//
// Sessions are likewise revoked on unbanning: lifting a ban should not
// automatically restore credentials an attacker may have obtained.
//
// ⚠️ This method **must not be called self-service on the C side** (otherwise
// users could unban themselves). Hence no HTTP endpoint is exposed; it is left
// for the admin side to integrate.
func (s *Profile) SetDisabled(ctx context.Context, userID int64, disabled bool) error {
	if err := dao.SetDisabled(ctx, userID, disabled); err != nil {
		return err
	}
	// Whether banning or unbanning, force a fresh login.
	return s.revokeAllSessions(ctx, userID, "")
}

// SetDisabledByUsername bans by username (common on the admin side, where the
// username is already known).
func (s *Profile) SetDisabledByUsername(ctx context.Context, username string, disabled bool) error {
	u, err := dao.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	return s.SetDisabled(ctx, u.ID, disabled)
}
