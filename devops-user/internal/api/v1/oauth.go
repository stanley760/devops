package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/service"
)

// OAuthHandler holds the third-party login endpoints.
type OAuthHandler struct {
	svc *service.OAuth
}

// NewOAuthHandler constructs the OAuth handler.
func NewOAuthHandler(svc *service.OAuth) *OAuthHandler {
	return &OAuthHandler{svc: svc}
}

// providerOutput holds the third-party provider statuses.
type providerOutput struct {
	Body commonResp.BaseResponse[[]service.ProviderStatus]
}

// ListProviders GET /user/api/v1/oauth/providers — lists available providers
// and their authorize URLs.
//
// Anonymous: the user is not logged in yet, which is exactly when they pick a
// login method.
func (h *OAuthHandler) ListProviders(ctx context.Context, _ *struct{}) (*providerOutput, error) {
	list, err := h.svc.ListProviders(ctx)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &providerOutput{Body: commonResp.BaseResponse[[]service.ProviderStatus]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: list,
	}}, nil
}

// bindBody is the explicit bind request body.
type bindBody struct {
	Provider string `json:"provider" required:"true" doc:"provider (github / google / wechat)" maxLength:"16"`
	// RedirectAfter is the frontend URL to return to after login. **The server
	// enforces a whitelist**; without that check it would be an open redirect
	// (phishing).
	RedirectAfter string `json:"redirect_after,required=false" doc:"post-login redirect URL (must be in the whitelist)" maxLength:"512"`
}

// startBindData holds the authorize URL.
type startBindData struct {
	AuthorizeURL string `json:"authorize_url" doc:"third-party authorize redirect URL"`
}

// startBindOutput holds the authorize URL.
type startBindOutput struct {
	Body commonResp.BaseResponse[startBindData]
}

// StartBind POST /user/api/v1/oauth/bind/start — a logged-in user starts a
// bind.
//
// Requires login: binding is the act of "attaching a third-party account to
// the current user".
func (h *OAuthHandler) StartBind(ctx context.Context, in *struct {
	Body bindBody
}) (*startBindOutput, error) {
	claims := auth.ClaimsFrom(ctx)
	if claims == nil {
		return nil, errs.ToHumaError(errs.ErrUnauthorized)
	}
	url, err := h.svc.StartBind(ctx, in.Body.Provider, in.Body.RedirectAfter, claims.UserID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &startBindOutput{Body: commonResp.BaseResponse[startBindData]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: startBindData{AuthorizeURL: url},
	}}, nil
}

// callbackOutput holds the callback result.
type callbackOutput struct {
	Body commonResp.BaseResponse[*service.OAuthResult]
}

// ⚠️ **Do not put parameters in an embedded struct**: huma's findParams returns
// nil directly for f.Anonymous (no recursion), so the `path` / `query` /
// `form` tags on embedded types **have no effect at all** — path params stay
// empty and query params are never received. They must be declared directly on
// the handler's input struct. See huma@v2.39.1/huma.go:263-266.

// Callback GET /user/api/v1/oauth/{provider}/callback — third-party authorize
// callback (query mode).
//
// **Must be anonymous**: when the third party redirects back, the user does
// not yet have a local token. Security relies on one-time state verification
// (CSRF protection), not on login state.
func (h *OAuthHandler) Callback(ctx context.Context, in *struct {
	Provider string `path:"provider" doc:"provider identifier"`
	Code     string `query:"code" doc:"authorization code returned by the third party"`
	State    string `query:"state" doc:"state for CSRF protection"`
	OAuthErr string `query:"error" doc:"error code returned by the third party (present when the user denies authorization)"`
}) (*callbackOutput, error) {
	return h.handleCallback(ctx, in.Provider, in.Code, in.State, in.OAuthErr)
}

// CallbackPost POST /user/api/v1/oauth/{provider}/callback — form_post mode
// callback.
//
// Kept separate from the GET version rather than merged: huma requires "a
// struct with body parameters must have a request body", and merging would
// make query-mode requests fail outright with "request body is required".
// GitHub / Google / WeChat all use query (GET) responses; this endpoint is
// reserved for future form_post-style providers (when one is added, the router
// comments and the anonymous path list must be updated in sync).
func (h *OAuthHandler) CallbackPost(ctx context.Context, in *struct {
	Body struct {
		Provider string `path:"provider" doc:"provider identifier"`
		Code     string `form:"code" doc:"authorization code returned by the third party"`
		State    string `form:"state" doc:"state for CSRF protection"`
		OAuthErr string `form:"error" doc:"error code returned by the third party"`
	}
}) (*callbackOutput, error) {
	return h.handleCallback(ctx, in.Body.Provider, in.Body.Code, in.Body.State, in.Body.OAuthErr)
}

// handleCallback is the shared handling logic for both modes.
func (h *OAuthHandler) handleCallback(ctx context.Context, provider, code, state, oerr string) (*callbackOutput, error) {
	if provider == "" {
		return nil, errs.ToHumaError(errs.New(errs.CodeParam, "missing provider"))
	}
	// The third party also calls back when the user actively denies
	// authorization, carrying an error parameter.
	if oerr != "" {
		msg := "third-party authorization failed"
		if oerr == "access_denied" {
			msg = "You have denied authorization"
		}
		return nil, errs.ToHumaError(errs.New(errs.CodeParam, msg))
	}
	res, err := h.svc.Callback(ctx, provider, code, state)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &callbackOutput{Body: commonResp.BaseResponse[*service.OAuthResult]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: res,
	}}, nil
}

// unbindOutput holds the unbind result.
type unbindOutput struct {
	Body commonResp.BaseResponse[any]
}

// Unbind DELETE /user/api/v1/oauth/bind — unbinds a third-party account.
//
// Requires login: you can only unbind your own bindings.
//
// ⚠️ If after unbinding the user has **only third-party login left** (no
// usable local password), they cannot log in. The current implementation
// therefore requires that a local username/password always exists
// (auto-created users also get a random strong password), so unbinding does
// not affect their ability to log in again.
func (h *OAuthHandler) Unbind(ctx context.Context, in *struct {
	Provider string `query:"provider" required:"true" doc:"provider to unbind" maxLength:"16"`
}) (*unbindOutput, error) {
	claims := auth.ClaimsFrom(ctx)
	if claims == nil {
		return nil, errs.ToHumaError(errs.ErrUnauthorized)
	}
	if err := h.svc.Unbind(ctx, claims.UserID, in.Provider); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &unbindOutput{Body: commonResp.BaseResponse[any]{
		Code: http.StatusOK,
		Msg:  "Unbound",
	}}, nil
}

// listBoundOutput holds the list of bindings.
type listBoundOutput struct {
	Body commonResp.BaseResponse[[]oauthBoundView]
}

// oauthBoundView is the binding view (contains no tokens).
type oauthBoundView struct {
	Provider  string `json:"provider" doc:"provider"`
	Nickname  string `json:"nickname" doc:"third-party nickname snapshot"`
	AvatarURL string `json:"avatar_url" doc:"third-party avatar snapshot"`
	CreatedAt int64  `json:"created_at" doc:"bind time (Unix milliseconds)"`
}

// ListBound GET /user/api/v1/oauth/bind — lists bound third-party accounts.
func (h *OAuthHandler) ListBound(ctx context.Context, _ *struct{}) (*listBoundOutput, error) {
	claims := auth.ClaimsFrom(ctx)
	if claims == nil {
		return nil, errs.ToHumaError(errs.ErrUnauthorized)
	}
	list, err := h.svc.ListBound(ctx, claims.UserID)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	out := make([]oauthBoundView, 0, len(list))
	for _, m := range list {
		out = append(out, oauthBoundView{
			Provider:  string(m.Provider),
			Nickname:  m.Nickname,
			AvatarURL: m.AvatarURL,
			CreatedAt: m.CreateTime.UnixMilli(),
		})
	}
	return &listBoundOutput{Body: commonResp.BaseResponse[[]oauthBoundView]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: out,
	}}, nil
}

// Ensure huma is referenced (this file uses its schema tags).
var _ = huma.Operation{}
