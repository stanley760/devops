// Huma-flavored authentication/authorization middleware (D2/D7).
//
// Decisions live in infra; presentation is injected via HumaOptions callbacks.
// The default callbacks render 401/403/503 using the unified envelope
// (code/msg/data), and business code may inject custom text.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/http/response"
	securityauthz "github.com/stanley760/devops/devops-infra/security/authz"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
)

// ErrNotPlatformAdmin is returned when the caller is not a platform admin
// (lets callbacks tailor the message; the behavior is still a rejection).
var ErrNotPlatformAdmin = errors.New("middleware: not a platform admin")

// claimsContextKey is the context.Context key under which claims are stored.
type claimsContextKey struct{}

// BearerAuthenticator is the framework-agnostic authentication decision interface.
// *Authenticator and business facades (e.g. devops-system's auth.Manager) both implement it.
type BearerAuthenticator interface {
	Authenticate(ctx context.Context, bearerHeader string) (*securityjwt.Claims, error)
}

// HumaOptions holds error-presentation callbacks (isomorphic to GinOptions).
type HumaOptions struct {
	OnUnauthorized        func(hctx huma.Context, err error)
	OnForbidden           func(hctx huma.Context, err error)
	OnInfrastructureError func(hctx huma.Context, err error)
}

// HumaCurrentClaims reads the authenticated claims from the context
// (available after HumaAuthenticate has run).
func HumaCurrentClaims(ctx context.Context) *securityjwt.Claims {
	claims, _ := ctx.Value(claimsContextKey{}).(*securityjwt.Claims)
	return claims
}

// HumaAuthenticate is the huma authentication middleware:
// Bearer parsing -> token parsing -> session state. On success the claims
// are injected into the context via huma.WithValue.
func HumaAuthenticate(authn BearerAuthenticator, opts HumaOptions) func(huma.Context, func(huma.Context)) {
	opts = withHumaDefaults(opts)
	return func(hctx huma.Context, next func(huma.Context)) {
		if authn == nil {
			opts.OnInfrastructureError(hctx, ErrUnauthorized)
			return
		}
		claims, err := authn.Authenticate(hctx.Context(), hctx.Header("Authorization"))
		if err != nil {
			if errors.Is(err, ErrInfrastructure) {
				opts.OnInfrastructureError(hctx, err)
			} else {
				opts.OnUnauthorized(hctx, err)
			}
			return // not calling next short-circuits the chain
		}
		next(huma.WithValue(hctx, claimsContextKey{}, claims))
	}
}

// claimsOrReject returns false when claims are missing (a 401 has already been
// written); it is the shared entry point for fail-closed guards in business middleware.
func claimsOrReject(hctx huma.Context, opts HumaOptions) (*securityjwt.Claims, bool) {
	claims := HumaCurrentClaims(hctx.Context())
	if claims == nil {
		opts.OnUnauthorized(hctx, ErrUnauthorized)
		return nil, false
	}
	return claims, true
}

// HumaRequirePermission performs in-domain authorization (uid x tid x perm).
// When the authorizer is not wired (nil) it fails closed, matching the gin version.
func HumaRequirePermission(authorizer securityauthz.Authorizer, permission string, opts HumaOptions) func(huma.Context, func(huma.Context)) {
	opts = withHumaDefaults(opts)
	return func(hctx huma.Context, next func(huma.Context)) {
		claims, ok := claimsOrReject(hctx, opts)
		if !ok {
			return
		}
		if authorizer == nil {
			opts.OnForbidden(hctx, ErrForbidden)
			return
		}
		ok, err := authorizer.Enforce(
			hctx.Context(),
			strconv.FormatInt(claims.UID, 10),
			strconv.FormatInt(claims.TenantID, 10),
			permission,
		)
		if err != nil {
			opts.OnInfrastructureError(hctx, err)
			return
		}
		if !ok {
			opts.OnForbidden(hctx, ErrForbidden)
			return
		}
		next(hctx)
	}
}

// HumaRequirePlatformPermission performs platform-admin authorization
// (g2 check + Enforce with dom="*"). Both "not a platform admin" and an
// Enforce denial go through OnForbidden, distinguished by different error values.
func HumaRequirePlatformPermission(authorizer securityauthz.Authorizer, permission string, opts HumaOptions) func(huma.Context, func(huma.Context)) {
	opts = withHumaDefaults(opts)
	return func(hctx huma.Context, next func(huma.Context)) {
		claims, ok := claimsOrReject(hctx, opts)
		if !ok {
			return
		}
		if authorizer == nil {
			opts.OnForbidden(hctx, ErrForbidden)
			return
		}
		subject := strconv.FormatInt(claims.UID, 10)
		isAdmin, err := authorizer.IsPlatformAdmin(hctx.Context(), subject)
		if err != nil {
			opts.OnInfrastructureError(hctx, err)
			return
		}
		if !isAdmin {
			opts.OnForbidden(hctx, ErrNotPlatformAdmin)
			return
		}
		ok, err = authorizer.EnforcePlatform(hctx.Context(), subject, permission)
		if err != nil {
			opts.OnInfrastructureError(hctx, err)
			return
		}
		if !ok {
			opts.OnForbidden(hctx, ErrForbidden)
			return
		}
		next(hctx)
	}
}

// withHumaDefaults renders the unified envelope by default (message is the HTTP
// status phrase), so a forgotten callback injection does not yield a bare status
// code with no body.
func withHumaDefaults(opts HumaOptions) HumaOptions {
	if opts.OnUnauthorized == nil {
		opts.OnUnauthorized = func(hctx huma.Context, _ error) {
			response.WriteHumaEnvelope(hctx, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		}
	}
	if opts.OnForbidden == nil {
		opts.OnForbidden = func(hctx huma.Context, _ error) {
			response.WriteHumaEnvelope(hctx, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		}
	}
	if opts.OnInfrastructureError == nil {
		opts.OnInfrastructureError = func(hctx huma.Context, _ error) {
			response.WriteHumaEnvelope(hctx, http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
		}
	}
	return opts
}
