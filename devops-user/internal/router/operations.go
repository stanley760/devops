// operations.go holds the huma registration for all devops-user business
// endpoints (split by module).
//
// Differences from devops-system (deliberate):
//   - the prefix is /user instead of /system;
//   - no TenantHeader (X-Tenant-ID in OpenAPI) — the C side has no tenant
//     context;
//   - authorization uses kit.JWT() (identity verification only), **not**
//     kit.Perm(...): C-side permissions are "you can only operate on your own
//     data", a row-level ownership check done in the service layer by
//     filtering on claims.UID, not RBAC permission points.
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/stanley760/devops/devops-infra/http/humaerr"

	apiv1 "github.com/stanley760/devops/devops-user/internal/api/v1"
	"github.com/stanley760/devops/devops-user/internal/middleware"
)

// with merges the common Operation fields and adds this service's endpoint
// prefix.
func with(common huma.Operation, op huma.Operation) huma.Operation {
	op.Path = userAPIPrefix + op.Path
	if op.Tags == nil {
		op.Tags = common.Tags
	}
	if op.Responses == nil {
		op.Responses = common.Responses
	}
	return op
}

// registerAuth registers the auth module endpoints.
func registerAuth(api huma.API, h *apiv1.AuthHandler, kit *middleware.Kit) {
	// Client IP injected into ctx: needed by login rate limiting, and it must
	// go through trusted-proxy resolution (against forged XFF).
	ipMw := huma.Middlewares{apiv1.ClientIPMiddleware}
	common := huma.Operation{
		Tags:      []string{"auth"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-captcha",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/captcha",
		Summary:     "Captcha",
		Description: "Issue a captcha challenge (click / slide / rotate) of the type selected in the console. Must be called before login / register.",
		Security:    []map[string][]string{}, // public endpoint: the caller has no token yet
		Middlewares: ipMw,
	}), h.Captcha)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-register",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/register",
		Summary:     "Register",
		Description: "Create a C-side account and issue an access + refresh token. Requires a solved captcha.",
		Security:    []map[string][]string{}, // public endpoint overrides the global security requirement
		Middlewares: ipMw,
	}), h.Register)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Log in",
		Description: "Issue an access + refresh token after verifying the captcha and the account password. Consecutive failures trigger rate limiting and account lockout.",
		Security:    []map[string][]string{},
		Middlewares: ipMw,
	}), h.Login)

	// ⚠️ refresh must be anonymous: the caller's access token has expired by
	// now, and forcing a JWT would make refreshing impossible.
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-refresh",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "Refresh token",
		Description: "Exchange a refresh token for a new access + refresh pair (rotating: the old refresh token dies immediately).",
		Security:    []map[string][]string{},
		Middlewares: ipMw,
	}), h.Refresh)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "Log out",
		Description: "Revoke the current session (force logout).",
		Middlewares: kit.Logged(),
	}), h.Logout)
}

// registerProfile registers the user profile module endpoints (all require
// login).
func registerProfile(api huma.API, h *apiv1.ProfileHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"profile"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-profile-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/profile",
		Summary:     "Get the current user's profile",
		Middlewares: kit.Logged(),
	}), h.Get)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-password-change",
		Method:      http.MethodPut,
		Path:        "/api/v1/profile/password",
		Summary:     "Change password",
		Description: "The current password must be verified; log in again after success.",
		Middlewares: kit.Logged(),
	}), h.ChangePassword)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-profile-update",
		Method:      http.MethodPut,
		Path:        "/api/v1/profile",
		Summary:     "Update the current user's profile",
		Description: "You can only update your own profile; unauthorized access is intercepted by the service layer on claims.UID.",
		Middlewares: kit.Logged(),
	}), h.Update)
}

// registerOAuth registers the third-party login module endpoints.
//
// Anonymous vs authenticated grouping (**this is where this module most easily
// goes wrong**):
//
//	Anonymous: providers / callback — the user is picking a login method or
//	being redirected back, and has no local token yet; security relies on the
//	callback's one-time state verification, not on login state.
//	Authenticated: bind / bind/start / bind/list — these "attach a
//	third-party account to the current user", so the operator must be
//	confirmed to be the logged-in user, otherwise someone else's third-party
//	account could be bound to their name.
func registerOAuth(api huma.API, h *apiv1.OAuthHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"oauth"},
		Responses: humaerr.CommonErrResponses(),
	}
	ipMw := huma.Middlewares{apiv1.ClientIPMiddleware}

	// List available providers (each with its authorize URL and a one-time
	// state).
	huma.Register(api, with(common, huma.Operation{
		OperationID: "oauth-providers",
		Method:      http.MethodGet,
		Path:        "/api/v1/oauth/providers",
		Summary:     "List available third-party login methods",
		Middlewares: ipMw,
	}), h.ListProviders)

	// Third-party callback. GET and POST are registered on the same path:
	// GitHub/Google/WeChat all use query (GET) responses; the POST branch is
	// reserved for form_post-style providers. Method must be given explicitly
	// — huma.Register panics without a method (it does not return an error).
	callbackOp := huma.Operation{
		OperationID: "oauth-callback",
		Method:      http.MethodGet,
		Path:        "/api/v1/oauth/{provider}/callback",
		Summary:     "Third-party authorization callback",
		Description: "Called by a redirect from the third-party platform; after one-time state verification, logs in or creates an account depending on the binding.",
		Security:    []map[string][]string{},
		Middlewares: ipMw,
	}
	huma.Register(api, with(common, callbackOp), h.Callback)

	callbackOpPost := callbackOp
	callbackOpPost.Method = http.MethodPost
	callbackOpPost.OperationID = "oauth-callback-post"
	callbackOpPost.Description = "form_post-mode providers post back through this endpoint (all three current providers use GET; reserved for future use)."
	huma.Register(api, with(common, callbackOpPost), h.CallbackPost)

	// Start a bind (requires login).
	huma.Register(api, with(common, huma.Operation{
		OperationID: "oauth-bind-start",
		Method:      http.MethodPost,
		Path:        "/api/v1/oauth/bind/start",
		Summary:     "Start binding a third-party account",
		Description: "Returns the third-party authorize URL; on callback the third-party account is bound to the current logged-in user.",
		Middlewares: append(kit.Logged(), ipMw...),
	}), h.StartBind)

	// Binding list (requires login).
	huma.Register(api, with(common, huma.Operation{
		OperationID: "oauth-bind-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/oauth/bind",
		Summary:     "List bound third-party accounts",
		Middlewares: kit.Logged(),
	}), h.ListBound)

	// Unbind (requires login).
	huma.Register(api, with(common, huma.Operation{
		OperationID: "oauth-unbind",
		Method:      http.MethodDelete,
		Path:        "/api/v1/oauth/bind",
		Summary:     "Unbind a third-party account",
		Middlewares: kit.Logged(),
	}), h.Unbind)
}
