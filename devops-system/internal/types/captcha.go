// Captcha module DTOs (B-side).
package types

// CaptchaResp is the GET /system/api/v1/auth/captcha response: one captcha challenge.
//
// It deliberately carries no answer information — the expected answer stays server-side (Redis),
// so reading this response gives an attacker nothing but pixels.
type CaptchaResp struct {
	// ID is the captcha identifier; echo it back in the login request.
	ID string `json:"id" doc:"captcha id, echoed back on login"`
	// Type is the captcha type: click (click-to-select) / slide (slide-to-fit, horizontal) /
	// dragdrop (free 2D drag) / rotate (rotate-to-correct).
	//
	// The frontend picks its component by this field: slide and dragdrop share the same submission
	// shape (captcha_x / captcha_y) but differ in interaction — only dragdrop lets the piece move
	// freely in both axes.
	Type string `json:"type" doc:"captcha type: click / slide / dragdrop / rotate"`
	// Image is the main image (base64 data URI).
	Image string `json:"image" doc:"main image, base64 data URI"`
	// Thumb is the auxiliary image: click → shape thumbnail; slide / dragdrop → puzzle piece; rotate → thumbnail to rotate.
	Thumb string `json:"thumb" doc:"auxiliary image, base64 data URI"`
	// Width/Height are the main image dimensions; click coordinates are expressed in this pixel space.
	Width  int `json:"width" doc:"main image width (px)"`
	Height int `json:"height" doc:"main image height (px)"`
	// Text is the click-mode prompt, e.g. "circle,star" (the click order); empty for other modes.
	Text string `json:"text,omitempty" doc:"click mode: shapes to click in order, comma-separated"`

	// The auxiliary image's geometry, required by the official go-captcha-vue components to render
	// and position the draggable / rotatable piece. Display parameters only — the piece's initial
	// position is not the answer, so nothing here leaks it.
	ThumbX      int `json:"thumb_x,omitempty" doc:"slide/dragdrop: piece initial x position in the main image (px)"`
	ThumbY      int `json:"thumb_y,omitempty" doc:"slide/dragdrop: piece initial y position in the main image (px)"`
	ThumbWidth  int `json:"thumb_width,omitempty" doc:"slide/dragdrop: piece image width (px)"`
	ThumbHeight int `json:"thumb_height,omitempty" doc:"slide/dragdrop: piece image height (px)"`
	ThumbSize   int `json:"thumb_size,omitempty" doc:"rotate: thumbnail edge length (px)"`
	// Enabled reports whether captcha verification is switched on for this tenant.
	//
	// The frontend needs this to decide whether to prompt at all. When captcha is off, Verify is a
	// no-op, so making the user solve one would be wasted effort — the login page reads this flag
	// and skips the captcha step entirely when it is false. It also lets the frontend tell
	// "captcha is off" apart from "the request failed", which look identical otherwise.
	Enabled bool `json:"enabled" doc:"whether captcha verification is required for this tenant"`
}

// CaptchaPoint is one click coordinate (in main-image pixel space).
type CaptchaPoint struct {
	X int `json:"x" doc:"click x coordinate (px)"`
	Y int `json:"y" doc:"click y coordinate (px)"`
}

// CaptchaSubmit is the captcha portion of the login request.
//
// The fields used depend on the captcha type (the frontend knows the type from the challenge it
// received): click → Points, slide / dragdrop → X/Y, rotate → Angle. Only the relevant group needs
// to be sent.
type CaptchaSubmit struct {
	// ID is the captcha identifier (required).
	ID string `json:"captcha_id,omitempty" binding:"omitempty,max=64" maxLength:"64" doc:"captcha id returned by /auth/captcha"`
	// Points are the click coordinates in order (click mode).
	Points []CaptchaPoint `json:"captcha_points,omitempty" doc:"click mode: clicked coordinates, in the order shown in the thumbnail"`
	// X/Y are the puzzle piece drop coordinates (slide and drag-drop modes).
	//
	// They are the piece's final top-left position in the main image's pixel space — the same
	// coordinate space as the challenge's thumb_x / thumb_y, so no rescaling is needed.
	X int `json:"captcha_x,omitempty" doc:"slide/dragdrop mode: x coordinate the puzzle piece was dropped at"`
	Y int `json:"captcha_y,omitempty" doc:"slide/dragdrop mode: y coordinate the puzzle piece was dropped at"`
	// Angle is the rotation angle in degrees (rotate mode): the angle the user applied, which the
	// server adds to the server-side offset and expects to total ~360.
	Angle int `json:"captcha_angle,omitempty" doc:"rotate mode: rotation angle the user rotated to (degrees)"`
}
