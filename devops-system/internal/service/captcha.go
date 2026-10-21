// Captcha service (B-side): issues captcha challenges and verifies login submissions.
//
// ## Where the captcha type comes from
//
// Two tables, both managed from the console rather than from YAML:
//   - the dictionary t_system_dict_data (dict_type = sys_captcha_type) defines the **value set**:
//     the selectable captcha types (click / slide / rotate). The frontend renders the type selector
//     from it, so adding a type to the dictionary makes it selectable;
//   - the configuration item t_system_config (config_key = sys.captcha.type) records the
//     **currently selected type**; sys.captcha.enabled is the master switch.
//
// The configuration value is validated against the dictionary: a hand-edited configuration value
// outside the dictionary falls back to the default type (login must not break because someone typed
// a typo into the console).
//
// ## Why the type is read per request rather than cached at startup
//
// The captcha type is exactly the kind of setting an operator flips and expects to take effect
// immediately. Caching it would mean a restart after every change. The cost is one primary-key
// lookup plus one indexed dictionary query per captcha request — negligible next to the bcrypt
// comparison login already performs.
//
// ## Tenant context
//
// Both t_system_config and t_system_dict_data are tenant tables, and TenantPlugin fails closed
// without a tenant in ctx. Login is a public route, so every method here resolves the tenant first
// (mirroring the login flow: an empty tenant_code falls back to tenant.default_code) and injects it
// before touching those tables.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/captcha"
	"github.com/stanley760/devops/devops-infra/errs"
	infratenant "github.com/stanley760/devops/devops-infra/tenant"

	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/types"
)

// Captcha configuration keys (t_system_config.config_key).
const (
	// CaptchaEnabledKey is the master switch: false disables captcha verification entirely.
	CaptchaEnabledKey = "sys.captcha.enabled"
	// CaptchaTypeKey holds the currently selected captcha type; its value must be one of the
	// sys_captcha_type dictionary values.
	CaptchaTypeKey = "sys.captcha.type"
)

// CaptchaDictType is the dictionary type defining the selectable captcha types.
const CaptchaDictType = "sys_captcha_type"

// CaptchaTTL is how long a generated challenge stays valid.
//
// Long enough for a user to read the image and interact (humans are slow, especially on mobile),
// short enough that a stale captcha cannot be harvested and replayed much later.
const CaptchaTTL = 3 * time.Minute

// defaultCaptchaType is used when the configuration is missing or invalid.
const defaultCaptchaType = captcha.TypeClick

// Captcha issues and verifies captcha challenges.
type Captcha struct {
	store captcha.Store
	// defaultTenantCode is tenant.default_code, used when a request carries no tenant_code.
	defaultTenantCode string
}

// NewCaptcha creates the captcha service. store persists the expected answer between the generate
// and verify requests (Redis-backed).
func NewCaptcha(store captcha.Store, defaultTenantCode string) *Captcha {
	return &Captcha{store: store, defaultTenantCode: defaultTenantCode}
}

// tenantCtx resolves the tenant and injects it into ctx so the tenant-scoped configuration and
// dictionary tables become readable.
//
// A missing / disabled tenant yields a 403 here rather than later: without a usable tenant there
// is nothing to log into, and the captcha settings of an unusable tenant are irrelevant.
//
// Only the entry points that start from an anonymous request (the captcha endpoint) call this. The
// login flow already resolved the tenant and injected it before calling Verify, so Verify reads the
// settings from the ctx it is given without resolving anything again.
func (s *Captcha) tenantCtx(ctx context.Context, tenantCode string) (context.Context, error) {
	code := strings.TrimSpace(tenantCode)
	if code == "" {
		code = s.defaultTenantCode
	}
	t, err := dao.GetTenantByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if t.Status != 1 {
		return nil, errs.New(errs.CodeForbidden, "tenant is disabled")
	}
	return infratenant.With(ctx, t.ID, t.Code), nil
}

// Enabled reports whether captcha verification is switched on.
//
// ctx must already carry the tenant (the login flow injects it before calling here).
//
// A missing configuration item is treated as "on": the secure default. Turning captcha off must be
// an explicit act (set sys.captcha.enabled = false), never a side effect of deleting a row.
func (s *Captcha) Enabled(ctx context.Context) bool {
	v, err := dao.GetConfigValue(ctx, CaptchaEnabledKey)
	if err != nil {
		return true
	}
	return parseBool(v, true)
}

// CurrentType returns the captcha type currently selected for the tenant.
//
// ctx must already carry the tenant.
//
// Falls back to defaultCaptchaType when the configuration item is missing or holds a value outside
// the sys_captcha_type dictionary: a typo in the console must not break login.
func (s *Captcha) CurrentType(ctx context.Context) captcha.Type {
	raw, err := dao.GetConfigValue(ctx, CaptchaTypeKey)
	if err != nil {
		return defaultCaptchaType
	}
	t, err := captcha.ParseType(raw)
	if err != nil || !s.typeAllowed(ctx, t) {
		return defaultCaptchaType
	}
	return t
}

// typeAllowed reports whether t is present in the sys_captcha_type dictionary.
//
// This is what makes the dictionary the value-set authority: the configuration item can only select
// a type the dictionary actually offers.
func (s *Captcha) typeAllowed(ctx context.Context, t captcha.Type) bool {
	enabled := int8(1)
	values, err := dao.ListDictValues(ctx, CaptchaDictType, &enabled)
	if err != nil || len(values) == 0 {
		// Dictionary unavailable (e.g. not migrated yet): fall back to the built-in type set rather
		// than rejecting every type.
		return t.Valid()
	}
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), t.String()) {
			return true
		}
	}
	return false
}

// Generate issues a captcha challenge of the type currently selected for the tenant.
//
// This is the only entry point that resolves the tenant itself: the captcha endpoint is anonymous,
// so ctx carries no tenant yet, while the settings tables are tenant-scoped.
func (s *Captcha) Generate(ctx context.Context, tenantCode string) (*types.CaptchaResp, error) {
	tctx, err := s.tenantCtx(ctx, tenantCode)
	if err != nil {
		return nil, err
	}
	ch, err := captcha.Generate(tctx, s.CurrentType(tctx), s.store, CaptchaTTL)
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to generate captcha")
	}
	return &types.CaptchaResp{
		ID:     ch.ID,
		Type:   string(ch.Type),
		Image:  ch.Image,
		Thumb:  ch.Thumb,
		Width:  ch.Width,
		Height: ch.Height,
		Text:   ch.Text,
	}, nil
}

// Verify validates a login submission against the stored captcha answer.
//
// The caller passes a ctx that already carries the tenant (the login flow resolves the tenant
// before this point), so the settings are read without re-resolving.
//
// When captcha is disabled this is a no-op, so the login flow does not need to branch.
//
// A wrong or expired captcha returns one uniform 400: distinguishing "expired" from "wrong" would
// tell an attacker whether a given captcha ID was ever valid, and neither distinction helps a
// legitimate user (they just request a new captcha).
func (s *Captcha) Verify(ctx context.Context, sub types.CaptchaSubmit) error {
	if !s.Enabled(ctx) {
		return nil
	}
	if err := captcha.Verify(ctx, s.store, captcha.Submission{
		ID:     sub.ID,
		Points: toCaptchaPoints(sub.Points),
		X:      sub.X,
		Y:      sub.Y,
		Angle:  sub.Angle,
	}); err != nil {
		return errs.New(errs.CodeParam, "captcha verification failed")
	}
	return nil
}

// toCaptchaPoints converts the DTO points to the captcha package's type.
func toCaptchaPoints(pts []types.CaptchaPoint) []captcha.Point {
	if len(pts) == 0 {
		return nil
	}
	out := make([]captcha.Point, 0, len(pts))
	for _, p := range pts {
		out = append(out, captcha.Point{X: p.X, Y: p.Y})
	}
	return out
}

// parseBool parses a configuration boolean leniently; unparseable values fall back to def.
func parseBool(v string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
