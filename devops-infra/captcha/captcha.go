// Package captcha provides behavioral CAPTCHA (behavioral verification code) capabilities shared by
// the B-side (devops-system) and C-side (devops-user) login/register endpoints.
//
// ## Supported types
//
//   - Click: the user clicks the designated shapes on the main image in the order shown in the
//     thumbnail (shape mode, no font resources needed);
//   - Slide: the user drags the puzzle piece into the cutout;
//   - Rotate: the user rotates the thumbnail back to the orientation of the main image.
//
// ## Division of labor (this package vs. each business service)
//
// This package only owns "generate + verify the answer to a captcha challenge" and is unaware of
// which type the system currently uses. Reading the type from the system configuration table and
// taking the value range from the dictionary is business semantics, so it is implemented separately
// in each service (the two sides are single-database but two independent services, and the C-side
// has no configuration/dictionary table of its own).
//
// ## Why the answer is kept server-side
//
// Generate and Verify are two independent HTTP requests, so "where the correct answer is" must be
// decided here: Generate only returns the images to the frontend and writes the expected answer
// into the Store; Verify reads the answer back from the Store for comparison. If the answer were
// returned to the frontend along with the images (or embedded in a signed token), the frontend
// could read the answer directly and defeat the CAPTCHA. Therefore:
//
//   - the answer never leaves the server (the response contains only images + a captcha ID);
//   - the Store must have a TTL (an unanswered challenge expires on its own, with no cleanup job);
//   - verification is one-shot (the answer is deleted as soon as it is read), preventing one
//     challenge from being brute-forced repeatedly.
package captcha

import (
	"errors"
	"fmt"
	"strings"
)

// Type is the CAPTCHA type.
//
// The string values are the identifiers stored in the system configuration table and the dictionary;
// they are part of the external contract (frontend switch + database values) and must not be
// changed at will.
type Type string

const (
	// TypeClick is click-to-select (the user clicks the shapes in order).
	TypeClick Type = "click"
	// TypeSlide is slide-to-fit (the user slides the puzzle piece along a fixed Y axis).
	TypeSlide Type = "slide"
	// TypeDragDrop is drag-and-drop (the user drags the piece in 2D from its start position into
	// the cutout). Verification is identical to TypeSlide — what differs is the interaction, which is
	// exactly why it is a separately selectable type: the frontend must allow free 2D dragging
	// instead of constraining the piece to a single row.
	TypeDragDrop Type = "dragdrop"
	// TypeRotate is rotate-to-correct (the user rotates the thumbnail).
	TypeRotate Type = "rotate"
)

// AllTypes returns all supported CAPTCHA types (the value range for the configuration item, also
// the item set of the sys_captcha_type dictionary).
func AllTypes() []Type { return []Type{TypeClick, TypeSlide, TypeDragDrop, TypeRotate} }

// Valid reports whether t is a supported type.
func (t Type) Valid() bool {
	switch t {
	case TypeClick, TypeSlide, TypeDragDrop, TypeRotate:
		return true
	default:
		return false
	}
}

// UsesPoint reports whether the type is verified by click coordinates.
func (t Type) UsesPoint() bool { return t == TypeClick }

// UsesDrag reports whether the type is verified by a drag end position.
//
// Slide and drag-drop share this: both compare the drop point against the cutout, only the permitted
// interaction differs.
func (t Type) UsesDrag() bool { return t == TypeSlide || t == TypeDragDrop }

// UsesAngle reports whether the type is verified by a rotation angle.
func (t Type) UsesAngle() bool { return t == TypeRotate }

// String implements Stringer.
func (t Type) String() string { return string(t) }

// ErrInvalidType is returned when the type cannot be recognized.
var ErrInvalidType = errors.New("unsupported captcha type")

// ParseType parses a CAPTCHA type.
//
// Unrecognized values uniformly return ErrInvalidType (callers map it to a business error), which
// matters when the type comes from a configuration table someone can edit: one bad configuration
// value must not panic the login flow, it can only fall back to the caller's default.
func ParseType(s string) (Type, error) {
	t := Type(strings.TrimSpace(strings.ToLower(s)))
	if !t.Valid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidType, s)
	}
	return t, nil
}
