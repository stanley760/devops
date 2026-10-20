package v1

import (
	"context"
	"net/http"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/errs"

	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/model"
	"github.com/stanley760/devops/devops-user/internal/service"
)

// ProfileHandler holds the user profile endpoints.
type ProfileHandler struct {
	svc *service.Profile
}

// NewProfileHandler constructs the profile handler.
func NewProfileHandler(svc *service.Profile) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// profileOutput is the profile response body.
type profileOutput struct {
	Body commonResp.BaseResponse[model.PublicProfile]
}

// Get GET /user/api/v1/profile — reads the current user's profile.
func (h *ProfileHandler) Get(ctx context.Context, _ *struct{}) (*profileOutput, error) {
	profile, err := h.svc.Get(ctx, auth.ClaimsFrom(ctx))
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &profileOutput{Body: commonResp.BaseResponse[model.PublicProfile]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: profile,
	}}, nil
}

// updateBody is the profile update request body.
//
// ⚠️ No id / user_id fields: C-side row-level ownership is enforced by
// "identity comes only from the token"; allowing the client to pass an ID and
// comparing it would hand the authorization decision to client parameters.
//
// Pointers + `required:"false"`: nil means "do not modify this field". huma
// requires all fields by default, and pointer fields must also be explicitly
// marked optional, otherwise a missing field fails validation (making partial
// updates impossible). `required` must be an independent struct tag; putting it
// inside the json tag has no effect (see the comments in auth.go).
type updateBody struct {
	Nickname *string `json:"nickname" required:"false" doc:"nickname; unchanged when omitted" maxLength:"32"`
	Email    *string `json:"email" required:"false" doc:"email; unchanged when omitted"`
	Mobile   *string `json:"mobile" required:"false" doc:"mobile number; unchanged when omitted"`
	// Gender uses a pointer: nil = no change, 0 = change back to "unknown" —
	// the two have different semantics; a non-pointer would make it impossible
	// for clients to change gender back to unknown.
	Gender *int8   `json:"gender" required:"false" doc:"gender: 0=unknown, 1=male, 2=female; unchanged when omitted" minimum:"0" maximum:"2"`
	Avatar *string `json:"avatar" required:"false" doc:"avatar URL; unchanged when omitted"`
}

// Update PUT /user/api/v1/profile — updates the current user's profile.
func (h *ProfileHandler) Update(ctx context.Context, in *struct {
	Body updateBody
}) (*profileOutput, error) {
	profile, err := h.svc.Update(ctx, auth.ClaimsFrom(ctx), service.UpdateInput{
		Nickname: in.Body.Nickname,
		Email:    in.Body.Email,
		Mobile:   in.Body.Mobile,
		Gender:   in.Body.Gender,
		Avatar:   in.Body.Avatar,
	})
	if err != nil {
		return nil, errs.ToHumaError(err)
	}
	return &profileOutput{Body: commonResp.BaseResponse[model.PublicProfile]{
		Code: http.StatusOK,
		Msg:  "ok",
		Data: profile,
	}}, nil
}

// changePasswordBody is the password change request body.
type changePasswordBody struct {
	OldPassword string `json:"old_password" required:"true" doc:"current password" minLength:"1" maxLength:"72"`
	NewPassword string `json:"new_password" required:"true" doc:"new password, at least 8 characters" minLength:"8" maxLength:"72"`
	// CurrentRefreshToken is optional: when provided, the current device is
	// kept (other devices are logged out).
	CurrentRefreshToken string `json:"current_refresh_token" required:"false" doc:"refresh token of the current device; when provided, this device is kept" maxLength:"128"`
	// KeepCurrentDevice is an explicit form equivalent to "passing
	// current_refresh_token", making it easier for clients to express intent:
	// kick other devices after a password change but keep this one.
	KeepCurrentDevice bool `json:"keep_current_device" required:"false" doc:"keep this device after the password change; all other devices must log in again"`
}

// okOutput is a success response with no business data.
type okOutput struct {
	Body commonResp.BaseResponse[any]
}

// ChangePassword PUT /user/api/v1/profile/password — changes one's own
// password.
//
// The old password must be correct: otherwise anyone holding the token could
// change the password and permanently take over the account. The response
// prompts "please log in again": after a password change the old access token
// is still valid for a while, so clients should proactively discard local
// credentials and log in again.
func (h *ProfileHandler) ChangePassword(ctx context.Context, in *struct {
	Body changePasswordBody
}) (*okOutput, error) {
	claims := auth.ClaimsFrom(ctx)
	// Keeping the current device requires its refresh token; either field
	// being set counts as keeping it.
	keep := in.Body.CurrentRefreshToken
	if in.Body.KeepCurrentDevice && keep == "" {
		// Saying "keep this device" without providing a refresh token → it
		// cannot be located, so it is treated as not keeping it (kick
		// everything). Conservative choice: better to kick one extra time
		// than to leave an uncontrolled exception.
		keep = ""
	}
	if err := h.svc.ChangePassword(ctx, claims, in.Body.OldPassword, in.Body.NewPassword, keep); err != nil {
		return nil, errs.ToHumaError(err)
	}
	msg := "Password changed. All devices have been logged out. Please log in again."
	if keep != "" {
		msg = "Password changed. Other devices have been logged out."
	}
	return &okOutput{Body: commonResp.BaseResponse[any]{
		Code: http.StatusOK,
		Msg:  msg,
	}}, nil
}
