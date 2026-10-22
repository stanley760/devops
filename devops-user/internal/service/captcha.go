// Captcha use case (C-side): issues captcha challenges and verifies login / registration.
//
// ## Where the captcha type comes from
//
// The C side has no configuration table of its own, but it shares the `devops` database with
// devops-system, so the settings are read from the same two places (managed from the B-side console):
//   - the dictionary t_system_dict_data (dict_type = sys_captcha_type) defines the **value set**:
//     the selectable captcha types (click / slide / rotate);
//   - the configuration item t_system_config (config_key = sys.captcha.type) records the
//     **currently selected type**; sys.captcha.enabled is the master switch.
//
// The configuration value is validated against the dictionary, so the configuration item can only
// select a type the dictionary actually offers; anything else falls back to the default type (a typo
// in the console must not break login).
//
// ## Fail-secure on unavailability
//
// These tables are owned by devops-system's dbinit. If they are missing (fresh database, devops-system
// not yet migrated) or the read fails, the behavior is: captcha **enabled** with the **default type**.
// Enabling by default is the safe direction — the worst case of a misconfiguration is one extra
// (cheap) captcha step, whereas defaulting to disabled would silently remove the protection.
package service

import (
	"context"
	"strings"
	"time"

	"github.com/stanley760/devops/devops-infra/captcha"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/dao"
)

// Captcha configuration keys (t_system_config.config_key, managed from the B-side console).
const (
	// CaptchaEnabledKey is the master switch: false disables captcha verification.
	CaptchaEnabledKey = "sys.captcha.enabled"
	// CaptchaTypeKey holds the currently selected captcha type.
	CaptchaTypeKey = "sys.captcha.type"
)

// CaptchaDictType is the dictionary type defining the selectable captcha types.
const CaptchaDictType = "sys_captcha_type"

// CaptchaTTL is how long a generated challenge stays valid.
const CaptchaTTL = 3 * time.Minute

// defaultCaptchaType is used when the configuration is missing or invalid.
const defaultCaptchaType = captcha.TypeClick

// CaptchaChallenge is the captcha challenge returned to the frontend.
//
// It deliberately carries no answer information — the expected answer stays server-side (Redis).
type CaptchaChallenge struct {
	// ID is the captcha identifier; echo it back in the login / register request.
	ID string `json:"id"`
	// Type is the captcha type: click / slide (horizontal) / dragdrop (free 2D drag) / rotate.
	//
	// The frontend picks its component by this field: slide and dragdrop share the same submission
	// shape (X / Y) but differ in interaction — only dragdrop lets the piece move freely in both axes.
	Type string `json:"type"`
	// Image is the main image (base64 data URI).
	Image string `json:"image"`
	// Thumb is the auxiliary image: click → shape thumbnail; slide / dragdrop → puzzle piece; rotate → thumbnail to rotate.
	Thumb string `json:"thumb"`
	// Width/Height are the main image dimensions; click coordinates use this pixel space.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Text is the click-mode prompt, e.g. "circle,star"; empty for the other modes.
	Text string `json:"text,omitempty"`

	// The auxiliary image's geometry, required by the official go-captcha-vue components to render
	// and position the draggable / rotatable piece. Display parameters only — the piece's initial
	// position is not the answer, so nothing here leaks it. Same contract as the B-side
	// types.CaptchaResp, so both frontends share one captcha widget.
	ThumbX      int `json:"thumb_x,omitempty"`
	ThumbY      int `json:"thumb_y,omitempty"`
	ThumbWidth  int `json:"thumb_width,omitempty"`
	ThumbHeight int `json:"thumb_height,omitempty"`
	ThumbSize   int `json:"thumb_size,omitempty"`
	// Enabled reports whether captcha verification is switched on.
	//
	// Same contract as the B-side types.CaptchaResp, so both frontends can share one captcha
	// widget. When false the frontend skips the captcha step entirely: Verify is a no-op in that
	// state, so solving one would be wasted effort. It also lets the frontend tell "captcha is
	// off" apart from "the request failed", which look identical otherwise.
	Enabled bool `json:"enabled"`
}

// CaptchaPoint is one click coordinate (in main-image pixel space).
type CaptchaPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// CaptchaInput is the captcha portion of a login / register request.
//
// The fields used depend on the captcha type (the frontend knows the type from the challenge it
// received): click → Points, slide / dragdrop → X/Y, rotate → Angle.
type CaptchaInput struct {
	ID     string         `json:"captcha_id"`
	Points []CaptchaPoint `json:"captcha_points,omitempty"`
	X      int            `json:"captcha_x,omitempty"`
	Y      int            `json:"captcha_y,omitempty"`
	Angle  int            `json:"captcha_angle,omitempty"`
}

// Captcha issues and verifies captcha challenges.
type Captcha struct {
	store captcha.Store
}

// NewCaptcha creates the captcha use case.
func NewCaptcha(store captcha.Store) *Captcha { return &Captcha{store: store} }

// Enabled reports whether captcha verification is switched on.
//
// Defaults to true when the setting cannot be read (see the package doc on fail-secure).
func (s *Captcha) Enabled(ctx context.Context) bool {
	v, err := dao.GetSystemConfigValue(ctx, auth.DefaultTenantID, CaptchaEnabledKey)
	if err != nil {
		return true
	}
	return parseBool(v, true)
}

// CurrentType returns the captcha type currently selected.
//
// Falls back to defaultCaptchaType when the configuration item is missing or holds a value outside
// the sys_captcha_type dictionary.
func (s *Captcha) CurrentType(ctx context.Context) captcha.Type {
	raw, err := dao.GetSystemConfigValue(ctx, auth.DefaultTenantID, CaptchaTypeKey)
	if err != nil {
		return defaultCaptchaType
	}
	t, err := captcha.ParseType(raw)
	if err != nil || !s.typeAllowed(ctx, t) {
		return defaultCaptchaType
	}
	return t
}

// typeAllowed reports whether t is present in the sys_captcha_type dictionary, which is what makes
// the dictionary the value-set authority.
func (s *Captcha) typeAllowed(ctx context.Context, t captcha.Type) bool {
	values, err := dao.ListSystemDictValues(ctx, auth.DefaultTenantID, CaptchaDictType)
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

// Generate issues a captcha challenge of the currently selected type.
//
// When captcha is disabled, no challenge is generated (saves an image render plus a Redis write)
// and the response carries Enabled=false so the frontend skips the step.
func (s *Captcha) Generate(ctx context.Context) (*CaptchaChallenge, error) {
	if !s.Enabled(ctx) {
		return &CaptchaChallenge{Enabled: false}, nil
	}
	ch, err := captcha.Generate(ctx, s.CurrentType(ctx), s.store, CaptchaTTL)
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "failed to generate captcha")
	}
	return &CaptchaChallenge{
		ID:          ch.ID,
		Type:        string(ch.Type),
		Image:       ch.Image,
		Thumb:       ch.Thumb,
		Width:       ch.Width,
		Height:      ch.Height,
		Text:        ch.Text,
		ThumbX:      ch.ThumbX,
		ThumbY:      ch.ThumbY,
		ThumbWidth:  ch.ThumbWidth,
		ThumbHeight: ch.ThumbHeight,
		ThumbSize:   ch.ThumbSize,
		Enabled:     true,
	}, nil
}

// Verify validates a login / register submission against the stored captcha answer.
//
// A no-op when captcha is disabled, so callers do not need to branch.
//
// A wrong or expired captcha returns one uniform 400: distinguishing "expired" from "wrong" would
// tell an attacker whether a captcha ID was ever valid, and neither distinction helps a legitimate
// user (they just request a new captcha).
func (s *Captcha) Verify(ctx context.Context, in CaptchaInput) error {
	if !s.Enabled(ctx) {
		return nil
	}
	var points []captcha.Point
	if len(in.Points) > 0 {
		points = make([]captcha.Point, 0, len(in.Points))
		for _, p := range in.Points {
			points = append(points, captcha.Point{X: p.X, Y: p.Y})
		}
	}
	if err := captcha.Verify(ctx, s.store, captcha.Submission{
		ID:     in.ID,
		Points: points,
		X:      in.X,
		Y:      in.Y,
		Angle:  in.Angle,
	}); err != nil {
		return errs.New(errs.CodeParam, "captcha verification failed")
	}
	return nil
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
