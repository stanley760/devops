// operations.go registers huma for all business interfaces of devops-system (split by module).
// gin and huma cannot coexist on the same path: when migrating a module, the corresponding gin
// registration block in router.go must be deleted first before registering here.
package router

import (
	"net/http"
	"net/url"

	"github.com/danielgtaylor/huma/v2"
	humagin "github.com/danielgtaylor/huma/v2/adapters/humagin"

	"github.com/stanley760/devops/devops-infra/http/humaerr"

	apiv1 "github.com/stanley760/devops/devops-system/internal/api/v1"
	"github.com/stanley760/devops/devops-system/internal/middleware"
	"github.com/stanley760/devops/devops-system/internal/service"
)

const systemAPIPrefix = "/system"

// Full paths of the anonymous endpoints. Must be exported by the router package and share the
// same definition with the huma registration below, avoiding each of them writing a separate
// copy of the string against deploy/higress/ingresses/devops-system-public.yaml or the guard
// pass-through list and silently drifting (drifting means login gets locked out or wrongly let through).
//
// These three endpoints must be anonymous: callers have no token at this point. Higress jwt-auth's
// matchRules is per Ingress resource and does not support path matching, so they are carried by a
// separate ingress; the application-layer guard must also let them through in sync, otherwise
// direct connections (bypassing the gateway) would also have login blocked.
const (
	// AuthLoginPath is the login endpoint, issuing access + refresh tokens.
	AuthLoginPath = systemAPIPrefix + "/api/v1/auth/login"
	// AuthRefreshPath is the refresh endpoint, the only way out after the access token expires.
	AuthRefreshPath = systemAPIPrefix + "/api/v1/auth/refresh"
	// AuthCaptchaPath is the captcha issuance endpoint (login/register must present a solved captcha).
	AuthCaptchaPath = systemAPIPrefix + "/api/v1/auth/captcha"
	// JWKSPath is the verification public key, for other services to self-verify and manual troubleshooting.
	JWKSPath = "/.well-known/jwks.json"
)

// AnonymousAuthPaths are the auth endpoints that must stay anonymous (no token):
// login issues the tokens, refresh renews them, and captcha is fetched before any
// credential exists. The application-layer guard (cmd/root.go) and the Higress
// public ingress (deploy/higress/ingresses/devops-system-public.yaml) exempt
// exactly this set — the three must be kept in sync.
//
// GET /api/v1/auth/tenants used to be anonymous too (it backed the login page's
// tenant picker); that endpoint has been removed entirely — the tenant now comes
// from the URL — so it is not part of this set.
func AnonymousAuthPaths() []string {
	return []string{AuthLoginPath, AuthRefreshPath, AuthCaptchaPath}
}

// with merges common Operation fields and adds the devops-system API prefix.
func with(common huma.Operation, op huma.Operation) huma.Operation {
	op.Path = systemAPIPrefix + op.Path
	if op.Tags == nil {
		op.Tags = common.Tags
	}
	if op.Responses == nil {
		op.Responses = common.Responses
	}
	return op
}

// withClientMeta extracts client metadata on the login interface (unwrapped via the gin adapter;
// ClientIP is the real IP when trusted proxies are configured). The four Geo fields come from the
// X-Higress-Geo-* headers injected by the higress geo-ip plugin (the plugin writes URL-escaped
// values, restored here), and are persisted to the t_system_login_log audit table along with login.
func withClientMeta(hctx huma.Context, next func(huma.Context)) {
	c := humagin.Unwrap(hctx)
	geo := func(name string) string {
		v, err := url.QueryUnescape(c.GetHeader(name))
		if err != nil { // invalid escape sequence: fall back to the raw value, so the audit does not fail on a dirty header
			return c.GetHeader(name)
		}
		return v
	}
	next(huma.WithValue(hctx, apiv1.ClientMetaContextKey{}, service.LoginMeta{
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
		GeoCountry:  geo("X-Higress-Geo-Country"),
		GeoProvince: geo("X-Higress-Geo-Province"),
		GeoCity:     geo("X-Higress-Geo-City"),
		GeoIsp:      geo("X-Higress-Geo-Isp"),
	}))
}

// registerPosts posts module 5 interfaces (P2, migration SOP template).
func registerPosts(api huma.API, h *apiv1.PostHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"posts"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "post-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/posts",
		Summary:     "post list",
		Middlewares: kit.Biz("system:post:list"),
	}), h.List)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "post-get",
		Method:      http.MethodGet,
		Path:        "/api/v1/posts/{id}",
		Summary:     "post detail",
		Middlewares: kit.Biz("system:post:query"),
	}), h.Get)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "post-create",
		Method:      http.MethodPost,
		Path:        "/api/v1/posts",
		Summary:     "create post",
		Description: "code cannot be changed after creation.",
		Middlewares: kit.Biz("system:post:create"),
	}), h.Create)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "post-update",
		Method:      http.MethodPut,
		Path:        "/api/v1/posts/{id}",
		Summary:     "update post",
		Middlewares: kit.Biz("system:post:update"),
	}), h.Update)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "post-delete",
		Method:      http.MethodDelete,
		Path:        "/api/v1/posts/{id}",
		Summary:     "delete post",
		Middlewares: kit.Biz("system:post:delete"),
	}), h.Delete)
}

// registerRoles roles module 6 interfaces (P3).
func registerRoles(api huma.API, h *apiv1.RoleHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"roles"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-list", Method: http.MethodGet, Path: "/api/v1/roles",
		Summary: "role list", Middlewares: kit.Biz("system:role:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-get", Method: http.MethodGet, Path: "/api/v1/roles/{id}",
		Summary: "role detail", Middlewares: kit.Biz("system:role:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-create", Method: http.MethodPost, Path: "/api/v1/roles",
		Summary: "create role", Description: "code cannot be changed after creation.",
		Middlewares: kit.Biz("system:role:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-update", Method: http.MethodPut, Path: "/api/v1/roles/{id}",
		Summary: "update role", Middlewares: kit.Biz("system:role:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-assign-menus", Method: http.MethodPut, Path: "/api/v1/roles/{id}/menus",
		Summary: "assign role menus", Description: "rebuilds the role_menu and the role's p row snapshot.",
		Middlewares: kit.Biz("system:role:update"),
	}), h.AssignMenus)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "role-delete", Method: http.MethodDelete, Path: "/api/v1/roles/{id}",
		Summary: "delete role", Middlewares: kit.Biz("system:role:delete"),
	}), h.Delete)
}

// registerMenus menus module 5 interfaces (P3).
func registerMenus(api huma.API, h *apiv1.MenuHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"menus"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "menu-list", Method: http.MethodGet, Path: "/api/v1/menus",
		Summary: "menu tree", Middlewares: kit.Biz("system:menu:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "menu-get", Method: http.MethodGet, Path: "/api/v1/menus/{id}",
		Summary: "menu detail", Middlewares: kit.Biz("system:menu:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "menu-create", Method: http.MethodPost, Path: "/api/v1/menus",
		Summary: "create menu", Middlewares: kit.Biz("system:menu:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "menu-update", Method: http.MethodPut, Path: "/api/v1/menus/{id}",
		Summary: "update menu", Description: "permission changes trigger a rebuild of referencing roles' p rows.",
		Middlewares: kit.Biz("system:menu:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "menu-delete", Method: http.MethodDelete, Path: "/api/v1/menus/{id}",
		Summary: "delete menu", Middlewares: kit.Biz("system:menu:delete"),
	}), h.Delete)
}

// registerDepts depts module 5 interfaces (P3).
func registerDepts(api huma.API, h *apiv1.DeptHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"depts"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "dept-list", Method: http.MethodGet, Path: "/api/v1/depts",
		Summary: "dept tree", Middlewares: kit.Biz("system:dept:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dept-get", Method: http.MethodGet, Path: "/api/v1/depts/{id}",
		Summary: "dept detail", Middlewares: kit.Biz("system:dept:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dept-create", Method: http.MethodPost, Path: "/api/v1/depts",
		Summary: "create dept", Middlewares: kit.Biz("system:dept:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dept-update", Method: http.MethodPut, Path: "/api/v1/depts/{id}",
		Summary: "update dept", Middlewares: kit.Biz("system:dept:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dept-delete", Method: http.MethodDelete, Path: "/api/v1/depts/{id}",
		Summary: "delete dept", Middlewares: kit.Biz("system:dept:delete"),
	}), h.Delete)
}

// registerUsers users module 6 interfaces (P4; row-level data-permission controlled resources, via the scopedBiz chain).
func registerUsers(api huma.API, h *apiv1.UserHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"users"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-list", Method: http.MethodGet, Path: "/api/v1/users",
		Summary: "user list", Middlewares: kit.ScopedBiz("system:user:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-get", Method: http.MethodGet, Path: "/api/v1/users/{id}",
		Summary: "user detail", Middlewares: kit.ScopedBiz("system:user:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-create", Method: http.MethodPost, Path: "/api/v1/users",
		Summary: "create user", Description: "role assignment triggers a Casbin g row sync.",
		Middlewares: kit.ScopedBiz("system:user:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-update", Method: http.MethodPut, Path: "/api/v1/users/{id}",
		Summary: "update user", Middlewares: kit.ScopedBiz("system:user:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-reset-password", Method: http.MethodPut, Path: "/api/v1/users/{id}/password",
		Summary: "reset user password", Description: "kicks all sessions of that user offline after reset.",
		Middlewares: kit.ScopedBiz("system:user:update-password"),
	}), h.ResetPassword)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "user-delete", Method: http.MethodDelete, Path: "/api/v1/users/{id}",
		Summary: "delete user", Middlewares: kit.ScopedBiz("system:user:delete"),
	}), h.Delete)
}

// registerLoginLogs login audit module 3 interfaces (query side; the write side is persisted via login defer).
// Delete/clean are the retention-control entry points with independent permissions; rows are isolated by tenant (TenantPlugin injects tenant_id).
func registerLoginLogs(api huma.API, h *apiv1.LoginLogHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"login-logs"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "loginlog-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/login-logs",
		Summary:     "login log list",
		Description: "paginated login audit query (one row per success and failure attempt), with username/IP/result/time-range filtering.",
		Middlewares: kit.Biz("system:loginlog:list"),
	}), h.List)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "loginlog-delete",
		Method:      http.MethodDelete,
		Path:        "/api/v1/login-logs",
		Summary:     "delete login logs",
		Description: "batch physical delete by ID (ids comma-separated, e.g. ?ids=12,34,56).",
		Middlewares: kit.Biz("system:loginlog:delete"),
	}), h.Delete)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "loginlog-clean",
		Method:      http.MethodDelete,
		Path:        "/api/v1/login-logs/clean",
		Summary:     "clean login logs",
		Description: "deletes all login audit rows of the current tenant (retention control, independent permission gated).",
		Middlewares: kit.Biz("system:loginlog:clean"),
	}), h.Clean)
}

// registerOperationLogs operation audit module 3 interfaces (query side; the write side is
// persisted automatically by the audit-collection middleware via audit.Sink, with no write interface).
// Delete/clean are the retention-control entry points with independent permissions; rows are isolated by tenant (TenantPlugin injects tenant_id).
func registerOperationLogs(api huma.API, h *apiv1.OperationLogHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"operation-logs"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "operationlog-list",
		Method:      http.MethodGet,
		Path:        "/api/v1/operation-logs",
		Summary:     "operation log list",
		Description: "paginated operation audit query (B-side admin write operations), with module/operator/IP/result/time-range filtering.",
		Middlewares: kit.Biz("system:operationlog:list"),
	}), h.List)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "operationlog-delete",
		Method:      http.MethodDelete,
		Path:        "/api/v1/operation-logs",
		Summary:     "delete operation logs",
		Description: "batch physical delete by ID (ids comma-separated, e.g. ?ids=12,34,56).",
		Middlewares: kit.Biz("system:operationlog:delete"),
	}), h.Delete)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "operationlog-clean",
		Method:      http.MethodDelete,
		Path:        "/api/v1/operation-logs/clean",
		Summary:     "clean operation logs",
		Description: "deletes all operation audit rows of the current tenant (retention control, independent permission gated).",
		Middlewares: kit.Biz("system:operationlog:clean"),
	}), h.Clean)
}

// registerTenants tenants module
func registerTenants(api huma.API, h *apiv1.TenantHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"tenants"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-list", Method: http.MethodGet, Path: "/api/v1/tenants",
		Summary: "tenant list", Middlewares: kit.PlatChain("platform:tenant:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-get", Method: http.MethodGet, Path: "/api/v1/tenants/{id}",
		Summary: "tenant detail", Middlewares: kit.PlatChain("platform:tenant:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-create", Method: http.MethodPost, Path: "/api/v1/tenants",
		Summary: "create tenant", Description: "includes the tenant's initial super admin.",
		Middlewares: kit.PlatChain("platform:tenant:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-update", Method: http.MethodPut, Path: "/api/v1/tenants/{id}",
		Summary: "update tenant", Middlewares: kit.PlatChain("platform:tenant:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-update-status", Method: http.MethodPut, Path: "/api/v1/tenants/{id}/status",
		Summary: "tenant enable/disable", Description: "disabling kicks everyone offline.",
		Middlewares: kit.PlatChain("platform:tenant:update"),
	}), h.UpdateStatus)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "tenant-delete", Method: http.MethodDelete, Path: "/api/v1/tenants/{id}",
		Summary: "delete tenant", Description: "the default tenant cannot be deleted; must be disabled first.",
		Middlewares: kit.PlatChain("platform:tenant:delete"),
	}), h.Delete)
}

// registerDicts dictionary module 9 interfaces (dictionary types + dictionary items).
// The two resources are separated into distinct path prefixes (/dict-types, /dict-data),
// matching the yudao-style console layout.
func registerDicts(api huma.API, h *apiv1.DictHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"dicts"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-type-list", Method: http.MethodGet, Path: "/api/v1/dict-types",
		Summary: "dictionary type list", Middlewares: kit.Biz("system:dict:list"),
	}), h.ListTypes)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-type-get", Method: http.MethodGet, Path: "/api/v1/dict-types/{id}",
		Summary: "dictionary type detail", Middlewares: kit.Biz("system:dict:query"),
	}), h.GetType)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-type-create", Method: http.MethodPost, Path: "/api/v1/dict-types",
		Summary: "create dictionary type", Description: "the type code cannot be changed after creation.",
		Middlewares: kit.Biz("system:dict:create"),
	}), h.CreateType)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-type-update", Method: http.MethodPut, Path: "/api/v1/dict-types/{id}",
		Summary: "update dictionary type", Middlewares: kit.Biz("system:dict:update"),
	}), h.UpdateType)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-type-delete", Method: http.MethodDelete, Path: "/api/v1/dict-types/{id}",
		Summary: "delete dictionary type", Description: "a type that still has items cannot be deleted.",
		Middlewares: kit.Biz("system:dict:delete"),
	}), h.DeleteType)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-list", Method: http.MethodGet, Path: "/api/v1/dict-data",
		Summary: "dictionary item list", Middlewares: kit.Biz("system:dict:list"),
	}), h.ListData)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-simple", Method: http.MethodGet, Path: "/api/v1/dict-data/simple",
		Summary: "dictionary items of one type", Description: "returns the enabled items of one dictionary type, for the frontend dropdowns.",
		Middlewares: kit.Biz("system:dict:list"),
	}), h.ListByType)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-get", Method: http.MethodGet, Path: "/api/v1/dict-data/{id}",
		Summary: "dictionary item detail", Middlewares: kit.Biz("system:dict:query"),
	}), h.GetData)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-create", Method: http.MethodPost, Path: "/api/v1/dict-data",
		Summary: "create dictionary item", Middlewares: kit.Biz("system:dict:create"),
	}), h.CreateData)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-update", Method: http.MethodPut, Path: "/api/v1/dict-data/{id}",
		Summary: "update dictionary item", Middlewares: kit.Biz("system:dict:update"),
	}), h.UpdateData)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "dict-data-delete", Method: http.MethodDelete, Path: "/api/v1/dict-data/{id}",
		Summary: "delete dictionary item", Middlewares: kit.Biz("system:dict:delete"),
	}), h.DeleteData)
}

// registerConfigs system configuration module 6 interfaces.
func registerConfigs(api huma.API, h *apiv1.ConfigHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"configs"},
		Responses: humaerr.CommonErrResponses(),
	}

	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-list", Method: http.MethodGet, Path: "/api/v1/configs",
		Summary: "configuration list", Middlewares: kit.Biz("system:config:list"),
	}), h.List)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-get", Method: http.MethodGet, Path: "/api/v1/configs/{id}",
		Summary: "configuration detail", Middlewares: kit.Biz("system:config:query"),
	}), h.Get)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-get-by-key", Method: http.MethodGet, Path: "/api/v1/configs/key/{key}",
		Summary: "configuration detail by key", Middlewares: kit.Biz("system:config:query"),
	}), h.GetByKey)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-create", Method: http.MethodPost, Path: "/api/v1/configs",
		Summary: "create configuration", Description: "the key cannot be changed after creation.",
		Middlewares: kit.Biz("system:config:create"),
	}), h.Create)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-update", Method: http.MethodPut, Path: "/api/v1/configs/{id}",
		Summary: "update configuration", Middlewares: kit.Biz("system:config:update"),
	}), h.Update)
	huma.Register(api, with(common, huma.Operation{
		OperationID: "config-delete", Method: http.MethodDelete, Path: "/api/v1/configs/{id}",
		Summary: "delete configuration", Description: "built-in configurations cannot be deleted.",
		Middlewares: kit.Biz("system:config:delete"),
	}), h.Delete)
}

// registerAuth auth module 7 interfaces (P1).
func registerAuth(api huma.API, h *apiv1.AuthHandler, kit *middleware.Kit) {
	common := huma.Operation{
		Tags:      []string{"auth"},
		Responses: humaerr.CommonErrResponses(),
	}
	logged := kit.Logged()
	biz := kit.TenantBiz()

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-captcha",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/captcha",
		Summary:     "captcha",
		Description: "issues a captcha challenge (click / slide / rotate) of the type selected in the console. Must be called before login.",
		Security:    []map[string][]string{}, // public interface: the caller has no token yet
		Middlewares: huma.Middlewares{withClientMeta},
	}), h.Captcha)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-login",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "login",
		Description: "issues an access token and a refresh token after validating the captcha, the tenant and the user.",
		Security:    []map[string][]string{}, // public interface overrides the global security requirement
		Middlewares: huma.Middlewares{withClientMeta},
	}), h.Login)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-refresh",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "refresh token",
		Description: "rotates the refresh session and issues a new pair of tokens.",
		Security:    []map[string][]string{},
	}), h.Refresh)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-logout",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "logout",
		Description: "revokes the current session (kick offline).",
		Middlewares: logged,
	}), h.Logout)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-info",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/info",
		Summary:     "current user info",
		Description: "user, tenant, role codes, and permission identifiers.",
		Middlewares: biz,
	}), h.Info)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-routers",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/routers",
		Summary:     "frontend route tree",
		Description: "the frontend route node tree generated from the user's menu permissions.",
		Middlewares: biz,
	}), h.Routers)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-update-profile",
		Method:      http.MethodPut,
		Path:        "/api/v1/auth/profile",
		Summary:     "update profile",
		Middlewares: biz,
	}), h.UpdateProfile)

	huma.Register(api, with(common, huma.Operation{
		OperationID: "auth-update-password",
		Method:      http.MethodPut,
		Path:        "/api/v1/auth/profile/password",
		Summary:     "change password",
		Description: "changes after validating the old password, and revokes all sessions of that user (re-login).",
		Middlewares: biz,
	}), h.UpdatePassword)
}
