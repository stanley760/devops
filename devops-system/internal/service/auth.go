// Package service is the business logic layer: authentication (login/refresh/logout/user info/profile).
package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stanley760/devops/devops-infra/errs"
	infratenant "github.com/stanley760/devops/devops-infra/tenant"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/crypto/bcrypt"
	"sync"
	"time"
)

// loginCounter business metric: login attempt count (auth_login_total).
// All labels are bounded sets: result=success|failure, code=failure business code (always 200 on success).
// Lazily created: by the time the first login happens, the global MeterProvider
// has been assembled by observability.Init; when uninitialized (unit tests/probes)
// it falls back to the noop meter automatically.
var loginCounter = sync.OnceValue(func() metric.Int64Counter {
	c, err := otel.Meter("github.com/stanley760/devops/devops-system/service").Int64Counter(
		"auth_login",
		metric.WithDescription("login attempt count; result=success|failure, code is the failure business code (200 on success)"),
	)
	if err != nil {
		otel.Handle(err)
	}
	return c
})

// loginResultAttr holds the login result label value.
func loginResultAttr(err error) attribute.KeyValue {
	if err != nil {
		return attribute.String("result", "failure")
	}
	return attribute.String("result", "success")
}

// loginCodeAttr failure reason code: the bounded business code carried by a
// BizError; non-BizError failures are classified as 500. The specific failure
// details (wrong password/tenant disabled/expired, etc.) go to the access log
// and slog, never into metric labels, to keep label cardinality in check.
func loginCodeAttr(err error) attribute.KeyValue {
	return attribute.Int("code", loginErrCode(err))
}

// loginErrCode login result business code (200 on success; on failure the
// BizError's 4xx/5xx, or 500 for non-business errors).
// The metric label shares its source with t_system_login_log.code.
func loginErrCode(err error) int {
	if err == nil {
		return errs.CodeOK
	}
	var be *errs.BizError
	if errors.As(err, &be) && be.Code >= 400 && be.Code <= 599 {
		return be.Code
	}
	return errs.CodeInternal
}

// LoginMeta login audit metadata: parsed on the request side (injected into the
// ctx by the withClientMeta middleware), persisted to t_system_login_log with
// the login record. The Geo fields come from the X-Higress-Geo-* headers
// injected by the higress geo-ip plugin (empty when the plugin is disabled;
// an intranet IP yields the literal "内网IP").
type LoginMeta struct {
	IP          string
	UserAgent   string
	GeoCountry  string
	GeoProvince string
	GeoCity     string
	GeoIsp      string
}

// Auth is the authentication service.
type Auth struct {
	cfg   config.Config
	authM *auth.Manager
}

// NewAuth creates the authentication service.
func NewAuth(cfg config.Config, authM *auth.Manager) *Auth {
	return &Auth{cfg: cfg, authM: authM}
}

// Login: tenant validation (exists/disabled/expired) → user validation
// (password/disabled) → issue a token pair.
// Business metric: one auth_login_total{result, code} is recorded regardless of
// the outcome; metric failures never affect the login itself.
func (s *Auth) Login(ctx context.Context, req types.LoginReq, meta LoginMeta) (*types.LoginResp, error) {
	resp, err := s.login(ctx, req, meta)
	loginCounter().Add(ctx, 1, metric.WithAttributes(loginResultAttr(err), loginCodeAttr(err)))
	return resp, err
}

// login is the main login flow. A deferred call writes one login audit row
// (t_system_login_log): the closure captures the tenant-enriched ctx and the
// final err, covering every exit (including failure paths).
func (s *Auth) login(ctx context.Context, req types.LoginReq, meta LoginMeta) (resp *types.LoginResp, err error) {
	audit := &model.LoginLog{
		Username:    req.Username,
		IP:          meta.IP,
		UserAgent:   meta.UserAgent,
		GeoCountry:  meta.GeoCountry,
		GeoProvince: meta.GeoProvince,
		GeoCity:     meta.GeoCity,
		GeoIsp:      meta.GeoIsp,
	}
	defer func() {
		if err != nil {
			audit.Result = 1
		}
		audit.Code = loginErrCode(err)
		dao.CreateLoginLog(ctx, audit)
	}()

	code := req.TenantCode
	if code == "" {
		code = s.cfg.Tenant.DefaultCode
	}

	// Public route: no tenant ctx, query the global table directly
	t, err := dao.GetTenantByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if t.Status != 1 {
		return nil, errs.New(errs.CodeForbidden, "tenant is disabled")
	}
	if t.ExpireTime != nil && t.ExpireTime.Before(time.Now()) {
		return nil, errs.New(errs.CodeForbidden, "tenant has expired")
	}

	// The public route has no Tenant middleware: after resolving the tenant,
	// inject it into the ctx explicitly for TenantPlugin to use.
	ctx = infratenant.With(ctx, t.ID, t.Code)

	u, err := dao.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	audit.UserID = u.ID // user hit: the audit row carries user_id (traceable on later failure paths too)
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
		return nil, errs.New(errs.CodeUnauthorized, "incorrect username or password")
	}
	if u.Status != 1 {
		return nil, errs.New(errs.CodeForbidden, "user is disabled")
	}

	access, refresh, expiresIn, err := s.authM.Issue(ctx, u.ID, u.Username, t.ID, t.Code)
	if err != nil {
		return nil, err
	}
	dao.TouchLogin(ctx, u.ID, meta.IP)

	return &types.LoginResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    expiresIn,
		Tenant:       types.TenantBrief{ID: types.ID(t.ID), Name: t.Name, Code: t.Code},
	}, nil
}

// Refresh rotates the session: sid lookup → session validation → user/tenant
// recheck → new token pair.
func (s *Auth) Refresh(ctx context.Context, req types.RefreshReq) (*types.TokenResp, error) {
	tid, uid, ok := s.authM.LookupSID(ctx, req.RefreshToken)
	if !ok || !s.authM.SIDMatch(ctx, tid, req.RefreshToken, uid) {
		return nil, errs.New(errs.CodeUnauthorized, "refresh token is invalid or has expired")
	}

	// Public route: validate the tenant is available first, then inject the tenant ctx.
	t, err := dao.GetTenantByID(ctx, tid)
	if err != nil || t.Status != 1 {
		return nil, errs.New(errs.CodeForbidden, "tenant is unavailable")
	}
	if t.ExpireTime != nil && t.ExpireTime.Before(time.Now()) {
		return nil, errs.New(errs.CodeForbidden, "tenant has expired")
	}
	ctx = infratenant.With(ctx, tid, t.Code)

	u, err := dao.GetUserByID(ctx, uid)
	if err != nil {
		return nil, errs.New(errs.CodeUnauthorized, "user does not exist or has been deleted")
	}
	if u.Status != 1 {
		return nil, errs.New(errs.CodeForbidden, "user is disabled")
	}

	access, refresh, expiresIn, err := s.authM.Rotate(
		ctx, req.RefreshToken, u.ID, u.Username, t.ID, t.Code)
	if err != nil {
		return nil, err
	}
	return &types.TokenResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    expiresIn,
		Tenant:       types.TenantBrief{ID: types.ID(t.ID), Name: t.Name, Code: t.Code},
	}, nil
}

// Logout revokes the current session.
func (s *Auth) Logout(ctx context.Context, claims *auth.AccessClaims) {
	s.authM.Logout(ctx, claims)
}

// Info returns the user info + tenant + roles + permission identifiers.
func (s *Auth) Info(ctx context.Context, claims *auth.AccessClaims) (*types.UserInfoResp, error) {
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	roles, err := dao.GetUserRoles(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	menus, err := dao.GetUserMenus(ctx, claims.UserID, nil)
	if err != nil {
		return nil, err
	}

	roleCodes := make([]string, 0, len(roles))
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
	}
	perms := make([]string, 0, len(menus))
	permSet := make(map[string]struct{}, len(menus))
	for _, m := range menus {
		if m.Permission != "" {
			if _, dup := permSet[m.Permission]; !dup {
				permSet[m.Permission] = struct{}{}
				perms = append(perms, m.Permission)
			}
		}
	}

	dto := types.UserDTO{
		ID:       types.ID(u.ID),
		Username: u.Username,
		Nickname: u.Nickname,
		Email:    u.Email,
		Mobile:   derefString(u.Mobile),
		Gender:   u.Gender,
		Avatar:   u.Avatar,
		DeptID:   types.ID(u.DeptID),
		PostIDs:  parsePostIDs(u.PostIDs),
		Remark:   u.Remark,
		LoginIP:  u.LoginIP,
	}
	if u.LoginDate != nil {
		dto.LoginDate = *u.LoginDate
	}

	return &types.UserInfoResp{
		User:        dto,
		Tenant:      types.TenantBrief{ID: types.ID(claims.TenantID), Code: claims.TenantCode},
		Roles:       roleCodes,
		Permissions: perms,
	}, nil
}

// Routers returns the frontend route tree (menus of type 1/2 visible to this tenant).
func (s *Auth) Routers(ctx context.Context, claims *auth.AccessClaims) ([]types.RouterNode, error) {
	menus, err := dao.GetUserMenus(ctx, claims.UserID, []int8{1, 2})
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*types.RouterNode, len(menus))
	roots := make([]*types.RouterNode, 0)
	for _, m := range menus {
		node := &types.RouterNode{
			ID:            types.ID(m.ID),
			Name:          m.Name,
			Path:          m.Path,
			Component:     m.Component,
			ComponentName: m.ComponentName,
			Icon:          m.Icon,
			Sort:          m.Sort,
			Visible:       m.Visible,
			KeepAlive:     m.KeepAlive,
			AlwaysShow:    m.AlwaysShow,
		}
		byID[m.ID] = node
	}
	// Tree assembly: a parent outside the visible set (or a root) → root node
	for _, m := range menus {
		node := byID[m.ID]
		if parent, ok := byID[m.ParentID]; ok && m.ParentID != m.ID {
			parent.Children = append(parent.Children, *node)
		} else {
			roots = append(roots, node)
		}
	}
	return derefNodes(roots), nil
}

// UpdateProfile updates the profile (only fields the user themselves may change).
func (s *Auth) UpdateProfile(ctx context.Context, claims *auth.AccessClaims, req types.UpdateProfileReq) error {
	updates := map[string]any{
		"nickname": req.Nickname,
		"updater":  claims.Username,
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Mobile != "" {
		updates["mobile"] = req.Mobile
	} else {
		updates["mobile"] = nil // clearing goes through NULL, so multiple NULLs never conflict on uk_tenant_mobile
	}
	if req.Gender != 0 {
		updates["gender"] = req.Gender
	}
	if req.Avatar != "" {
		updates["avatar"] = req.Avatar
	}

	if err := dao.UpdateUser(ctx, claims.UserID, updates); err != nil {
		return err
	}
	return nil
}

// UpdatePassword changes the password: verify the old password → update →
// revoke all sessions (force re-login).
func (s *Auth) UpdatePassword(ctx context.Context, claims *auth.AccessClaims, req types.UpdatePasswordReq) error {
	u, err := dao.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.OldPassword)) != nil {
		return errs.New(errs.CodeParam, "old password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 10)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}
	if err := dao.UpdateUser(ctx, claims.UserID, map[string]any{
		"password": string(hash),
		"updater":  claims.Username,
	}); err != nil {
		return err
	}

	s.authM.RevokeAllSessions(ctx, claims.TenantID, claims.UserID)
	return nil
}

// derefString safely dereferences the string pointer.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// parsePostIDs parses the post_ids JSON array.
func parsePostIDs(s string) []types.ID {
	if s == "" {
		return []types.ID{}
	}
	var ids []types.ID
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return []types.ID{}
	}
	return ids
}

// derefNodes converts a pointer tree to a value tree (Children is already a value slice).
func derefNodes(nodes []*types.RouterNode) []types.RouterNode {
	out := make([]types.RouterNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, *n)
	}
	return out
}
