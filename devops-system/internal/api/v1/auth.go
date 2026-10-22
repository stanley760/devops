// Authentication endpoints (huma). The Input/Output structs double as the
// documentation schema; errors are funneled through errs.ToHumaError into
// the unified envelope (humaerr.Install is wired up in main).
package api

import (
	"context"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/response"

	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/service"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// AuthHandler serves the authentication endpoints.
type AuthHandler struct {
	svc     *service.Auth
	captcha *service.Captcha
	tenants *service.Tenant
}

// NewAuthHandler creates the handler.
func NewAuthHandler(svc *service.Auth, captchaSvc *service.Captcha, tenantSvc *service.Tenant) *AuthHandler {
	return &AuthHandler{svc: svc, captcha: captchaSvc, tenants: tenantSvc}
}

// ---- Input / Output (the structs are the OpenAPI schema) ----

// LoginInput is POST /system/api/v1/auth/login.
type LoginInput struct {
	Body types.LoginReq
}

// LoginOutput is the login response envelope.
type LoginOutput struct {
	Body commonResp.BaseResponse[types.LoginResp]
}

// CaptchaInput is GET /system/api/v1/auth/captcha.
//
// tenant_code mirrors the login input: omitting it falls back to tenant.default_code, so the
// frontend can fetch a captcha before it knows (or cares about) the tenant.
type CaptchaInput struct {
	TenantCode string `query:"tenant_code" maxLength:"30" doc:"tenant code; omitting it falls back to tenant.default_code"`
}

// CaptchaOutput is the GET /system/api/v1/auth/captcha response envelope.
type CaptchaOutput struct {
	Body commonResp.BaseResponse[types.CaptchaResp]
}

// Captcha handles GET /system/api/v1/auth/captcha: issues a captcha challenge of the type currently
// selected in the console (t_system_config + sys_captcha_type dictionary).
func (h *AuthHandler) Captcha(ctx context.Context, in *CaptchaInput) (*CaptchaOutput, error) {
	if h.captcha == nil {
		return nil, errs.New(errs.CodeInternal, "captcha service is not configured")
	}
	resp, err := h.captcha.Generate(ctx, in.TenantCode)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &CaptchaOutput{Body: response.OKBody(*resp)}, nil
}

// ListTenantsInput is GET /system/api/v1/auth/tenants (no parameters).
type ListTenantsInput struct{}

// ListTenantsOutput is the GET /system/api/v1/auth/tenants response envelope.
type ListTenantsOutput struct {
	Body commonResp.BaseResponse[[]types.TenantBrief]
}

// ListTenants handles GET /system/api/v1/auth/tenants: the loginable tenants.
//
// This is a public interface (no token): the login page needs to offer a tenant
// picker before anyone has signed in. Only the code and the name are exposed.
func (h *AuthHandler) ListTenants(ctx context.Context, _ *ListTenantsInput) (*ListTenantsOutput, error) {
	if h.tenants == nil {
		return nil, errs.New(errs.CodeInternal, "tenant service is not configured")
	}
	list, err := h.tenants.ListLoginable(ctx)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &ListTenantsOutput{Body: response.OKBody(list)}, nil
}

// RefreshInput is POST /system/api/v1/auth/refresh.
type RefreshInput struct {
	Body types.RefreshReq
}

// RefreshOutput is the refresh response envelope.
type RefreshOutput struct {
	Body commonResp.BaseResponse[types.TokenResp]
}

// LogoutOutput is the logout response envelope (no data).
type LogoutOutput struct {
	Body commonResp.BaseResponse[any]
}

// InfoOutput is the GET /auth/info response envelope.
type InfoOutput struct {
	Body commonResp.BaseResponse[types.UserInfoResp]
}

// RoutersOutput is the GET /auth/routers response envelope.
type RoutersOutput struct {
	Body commonResp.BaseResponse[[]types.RouterNode]
}

// UpdateProfileInput is PUT /auth/profile.
type UpdateProfileInput struct {
	Body types.UpdateProfileReq
}

// UpdateProfileOutput is the update-profile response envelope (no data).
type UpdateProfileOutput struct {
	Body commonResp.BaseResponse[any]
}

// UpdatePasswordInput is PUT /auth/profile/password.
type UpdatePasswordInput struct {
	Body types.UpdatePasswordReq
}

// UpdatePasswordOutput is the change-password response envelope (no data; changing the password kicks the user offline).
type UpdatePasswordOutput struct {
	Body commonResp.BaseResponse[any]
}

// ---- Handlers ----

// Login handles POST /system/api/v1/auth/login (client metadata is injected by the withClientMeta middleware).
func (h *AuthHandler) Login(ctx context.Context, in *LoginInput) (*LoginOutput, error) {
	resp, err := h.svc.Login(ctx, in.Body, ClientMetaFrom(ctx))
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &LoginOutput{Body: response.OKBody(*resp)}, nil
}

// Refresh handles POST /system/api/v1/auth/refresh (rotates the refresh session).
func (h *AuthHandler) Refresh(ctx context.Context, in *RefreshInput) (*RefreshOutput, error) {
	resp, err := h.svc.Refresh(ctx, in.Body)
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RefreshOutput{Body: response.OKBody(*resp)}, nil
}

// Logout handles POST /system/api/v1/auth/logout (requires JWT).
func (h *AuthHandler) Logout(ctx context.Context, _ *struct{}) (*LogoutOutput, error) {
	h.svc.Logout(ctx, auth.ClaimsFrom(ctx))
	return &LogoutOutput{Body: response.EmptyBody()}, nil
}

// Info handles GET /system/api/v1/auth/info (requires JWT + Tenant).
func (h *AuthHandler) Info(ctx context.Context, _ *struct{}) (*InfoOutput, error) {
	resp, err := h.svc.Info(ctx, auth.ClaimsFrom(ctx))
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &InfoOutput{Body: response.OKBody(*resp)}, nil
}

// Routers handles GET /system/api/v1/auth/routers (requires JWT + Tenant).
func (h *AuthHandler) Routers(ctx context.Context, _ *struct{}) (*RoutersOutput, error) {
	routers, err := h.svc.Routers(ctx, auth.ClaimsFrom(ctx))
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &RoutersOutput{Body: response.OKBody(routers)}, nil
}

// UpdateProfile handles PUT /system/api/v1/auth/profile (requires JWT + Tenant).
func (h *AuthHandler) UpdateProfile(ctx context.Context, in *UpdateProfileInput) (*UpdateProfileOutput, error) {
	if err := h.svc.UpdateProfile(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UpdateProfileOutput{Body: response.EmptyBody()}, nil
}

// UpdatePassword handles PUT /system/api/v1/auth/profile/password (requires JWT + Tenant; changing the password kicks the user offline).
func (h *AuthHandler) UpdatePassword(ctx context.Context, in *UpdatePasswordInput) (*UpdatePasswordOutput, error) {
	if err := h.svc.UpdatePassword(ctx, auth.ClaimsFrom(ctx), in.Body); err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &UpdatePasswordOutput{Body: response.EmptyBody()}, nil
}

// ClientMetaContextKey is the context key for login client metadata (injected by the withClientMeta middleware).
type ClientMetaContextKey struct{}

// ClientMetaFrom reads the client metadata of the login request
// (IP/UA/Geo; available after withClientMeta) for the t_system_login_log
// audit table.
func ClientMetaFrom(ctx context.Context) service.LoginMeta {
	if v, ok := ctx.Value(ClientMetaContextKey{}).(service.LoginMeta); ok {
		return v
	}
	return service.LoginMeta{}
}
