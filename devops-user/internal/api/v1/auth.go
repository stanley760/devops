// Package v1 is the C-side HTTP interface layer (huma handlers).
//
// Layering convention: handlers do only three things — "bind params → call
// service → wrap in envelope" — with no business logic. The envelope reuses
// devops-infra/http/response and is aligned verbatim with devops-system, so the
// frontend can share one parsing implementation.
//
// ⚠️ All errors must funnel through errs.ToHumaError (humaerr.Install is wired
// up in main): the *errs.BizError returned by services **does not implement**
// huma.StatusError (no GetStatus), so returning it directly makes huma treat it
// as an unknown error and fall back to 500, discarding the business code and
// message.
package v1

import (
	"context"
	"net/http"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/service"
)

// AuthHandler holds the authentication endpoints.
type AuthHandler struct {
	svc *service.Auth
}

// NewAuthHandler constructs the auth handler.
func NewAuthHandler(svc *service.Auth) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// registerBody is the registration request body.
//
// ⚠️ huma's field rules (from reading schema.go:895-922; two easy-to-miss
// pitfalls):
//  1. By default **every field is required** (FieldsOptionalByDefault is off);
//  2. `required` is an **independent struct tag**, not part of the json tag —
//     writing `json:"email,required=false"` has no effect; huma reads the
//     `required` tag separately.
//
// Optional fields must therefore be written as `json:"email" required:"false"`,
// or use `json:",omitempty"`.
type registerBody struct {
	Username string `json:"username" required:"true" doc:"login name" minLength:"3" maxLength:"32" example:"alice"`
	Password string `json:"password" required:"true" doc:"password, at least 8 characters" minLength:"8" example:"S3cret-pass"`
	Nickname string `json:"nickname" required:"false" doc:"nickname; defaults to the login name when empty" maxLength:"32"`
	Email    string `json:"email" required:"false" doc:"email" example:"alice@example.com"`
	Mobile   string `json:"mobile" required:"false" doc:"mobile number"`
	// Gender 0=unknown 1=male 2=female. The zero value means "unknown", so an
	// optional field can use a non-pointer to express "not provided".
	Gender int8 `json:"gender" required:"false" doc:"gender: 0=unknown, 1=male, 2=female" minimum:"0" maximum:"2" example:"1"`
}

// authOutput is the authentication response body.
type authOutput struct {
	Body commonResp.BaseResponse[service.TokenPair]
}

// Register POST /user/api/v1/auth/register — registers and issues tokens
// directly.
func (h *AuthHandler) Register(ctx context.Context, in *struct {
	Body registerBody
}) (*authOutput, error) {
	pair, err := h.svc.Register(ctx, service.RegisterInput{
		Username: in.Body.Username,
		Password: in.Body.Password,
		Nickname: in.Body.Nickname,
		Email:    in.Body.Email,
		Mobile:   in.Body.Mobile,
		Gender:   in.Body.Gender,
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &authOutput{Body: commonResp.BaseResponse[service.TokenPair]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: pair,
	}}, nil
}

// loginBody is the login request body.
type loginBody struct {
	Username string `json:"username" doc:"login name" required:"true" example:"alice"`
	Password string `json:"password" doc:"password" required:"true" example:"S3cret-pass"`
}

// Login POST /user/api/v1/auth/login — username/password login.
func (h *AuthHandler) Login(ctx context.Context, in *struct {
	Body loginBody
}) (*authOutput, error) {
	pair, err := h.svc.Login(ctx, ClientIPFrom(ctx), in.Body.Username, in.Body.Password)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &authOutput{Body: commonResp.BaseResponse[service.TokenPair]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: pair,
	}}, nil
}

// logoutBody is the logout request body.
type logoutBody struct {
	// RefreshToken is optional: when provided, that refresh token is revoked
	// too (otherwise the old refresh token can still mint new tokens).
	RefreshToken string `json:"refresh_token" required:"false" doc:"refresh token to revoke (optional)"`
}

// Logout POST /user/api/v1/auth/logout — revokes the current session.
func (h *AuthHandler) Logout(ctx context.Context, in *struct {
	Body logoutBody
}) (*authOutput, error) {
	// Identity comes only from the claims injected by the token middleware; a
	// user specified via request parameters is **not accepted**.
	claims := auth.ClaimsFrom(ctx)
	if err := h.svc.Logout(ctx, claims, in.Body.RefreshToken); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &authOutput{Body: commonResp.BaseResponse[service.TokenPair]{
		Code: http.StatusOK,
		Msg:  "ok",
	}}, nil
}

// refreshBody is the refresh request body.
type refreshBody struct {
	RefreshToken string `json:"refresh_token" required:"true" doc:"the refresh token issued at login" maxLength:"128"`
}

// Refresh POST /user/api/v1/auth/refresh — exchanges a refresh token for a new
// token pair (rotating).
//
// ⚠️ The refresh endpoint must be anonymous (the caller's access token has
// expired by now), so kit.Logged() is not applied. Its security comes from the
// refresh token being an opaque random string plus server-side hash
// verification, with the old token dying after one use.
func (h *AuthHandler) Refresh(ctx context.Context, in *struct {
	Body refreshBody
}) (*authOutput, error) {
	pair, err := h.svc.Refresh(ctx, in.Body.RefreshToken)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &authOutput{Body: commonResp.BaseResponse[service.TokenPair]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: pair,
	}}, nil
}
