package captcha

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"

	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/click"
	"github.com/wenlng/go-captcha/v2/rotate"
	"github.com/wenlng/go-captcha/v2/slide"
)

// Tolerance (padding) for answer comparison, in pixels / degrees.
//
// Too small makes the CAPTCHA frustrating (a 2px miss fails); too large lets a script brute-force
// by sweeping a coarse grid. These are the values the official frontend components assume (those
// components were built against the same library defaults).
const (
	clickPadding  = 10
	slidePadding  = 5
	rotatePadding = 10
)

// DefaultTTL is the fallback lifetime of an unanswered captcha challenge.
const DefaultTTL = 2 * time.Minute

// ErrNotFound is returned by Store implementations when the captcha ID does not exist (expired,
// already verified, or never existed).
//
// All three cases must return this same error: distinguishing them would tell an attacker whether a
// captcha ID was ever valid.
var ErrNotFound = errors.New("captcha does not exist or has expired")

// ErrInvalid is returned by Verify when verification fails (wrong answer, or unknown ID).
//
// An unknown ID and a wrong answer deliberately produce the same error: distinguishing them would
// leak whether an ID was ever valid.
var ErrInvalid = errors.New("captcha verification failed")

// Store persists the expected answer of a generated captcha between the generate and verify
// requests.
//
// Implementations must satisfy two hard requirements (otherwise the captcha is defeatable):
//   - Read must be one-shot (the answer is deleted on read, so a single challenge cannot be
//     brute-forced repeatedly);
//   - entries must expire on their own (a TTL), so unanswered challenges need no cleanup job.
type Store interface {
	// Save writes the answer under id with the given TTL.
	Save(ctx context.Context, id string, answer Answer, ttl time.Duration) error
	// Read reads and deletes the answer under id (one-shot).
	Read(ctx context.Context, id string) (Answer, error)
}

// Challenge is the data returned to the frontend after Generate.
//
// It deliberately carries no answer information: Image/Thumb are display-only, and everything
// needed for verification lives server-side in the Store (see the package doc).
type Challenge struct {
	// ID is the captcha identifier, echoed back on verification.
	ID string `json:"id"`
	// Type is the captcha type (the frontend picks a component by this field).
	Type Type `json:"type"`
	// Image is the main image, a base64 data URI (image/jpeg or image/png).
	Image string `json:"image"`
	// Thumb is the auxiliary image: click → shape thumbnail; slide → puzzle piece; rotate → thumbnail to rotate.
	Thumb string `json:"thumb"`
	// Width/Height are the main image dimensions (the frontend needs them to map click coordinates).
	Width  int `json:"width"`
	Height int `json:"height"`
	// Text is the click-mode prompt, e.g. "circle,star" (the click order); empty for other modes.
	Text string `json:"text,omitempty"`
}

// Point is a click coordinate submitted by the frontend.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Dot is the target region of one click point.
//
// Only the fields needed for verification are persisted (a subset of click.Dot), so the stored JSON
// does not depend on the upstream library's struct layout.
type Dot struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Answer is the server-side expected answer.
type Answer struct {
	Type  Type  `json:"type"`
	Dots  []Dot `json:"dots,omitempty"`
	X     int   `json:"x,omitempty"`
	Y     int   `json:"y,omitempty"`
	Angle int   `json:"angle,omitempty"`
}

// Submission is what the frontend sends back for verification.
//
// The three modes use different fields:
//   - click:  Points, in the same order as the thumbnail;
//   - slide:  X/Y, the drop position of the puzzle piece;
//   - rotate: Angle, the angle (degrees) the user rotated to.
type Submission struct {
	// ID is the captcha identifier returned by Generate (required).
	ID string `json:"id"`
	// Points are the click coordinates, in order (click mode).
	Points []Point `json:"points,omitempty"`
	// X/Y are the puzzle piece drop coordinates (slide mode).
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
	// Angle is the rotation angle (rotate mode).
	Angle int `json:"angle,omitempty"`
}

// Generate produces a captcha challenge of the given type and stores the expected answer.
//
// ttl is the answer lifetime; callers should pick a value long enough for the user to finish
// interacting (30–120s is typical). ttl <= 0 falls back to DefaultTTL.
//
// The returned Challenge contains no answer information.
func Generate(ctx context.Context, t Type, store Store, ttl time.Duration) (*Challenge, error) {
	if !t.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidType, t)
	}
	if store == nil {
		return nil, errors.New("captcha: store is nil")
	}

	ch, answer, err := generate(t)
	if err != nil {
		return nil, err
	}

	id, err := newID()
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if err := store.Save(ctx, id, answer, ttl); err != nil {
		return nil, err
	}

	ch.ID = id
	return ch, nil
}

// newID generates a captcha ID: 128 bits of cryptographic randomness, base64url without padding.
//
// The ID is the only handle an attacker gets on the stored answer, so it must be unguessable
// (128 bits makes enumeration impossible) and URL-safe.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("captcha: failed to generate captcha id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// generate creates the challenge images and the expected answer for one type.
//
// Slide and drag-drop share one implementation: the library expresses the difference as a builder
// mode (Make vs MakeDragDrop), and everything else — assets, answer, comparison — is identical.
func generate(t Type) (*Challenge, Answer, error) {
	switch {
	case t == TypeClick:
		return generateClick()
	case t.UsesDrag():
		return generateDrag(t)
	case t == TypeRotate:
		return generateRotate()
	default:
		return nil, Answer{}, fmt.Errorf("%w: %q", ErrInvalidType, t)
	}
}

// generateClick builds a shape-mode click captcha.
//
// Shape mode (MakeShape) rather than text mode (Make): text mode draws Chinese characters, which
// requires embedding a multi-megabyte font. Shapes are visually distinguishable and need no font.
//
// Size constraints (violating them produces garbled images rather than errors, so they are
// hardcoded here with comments):
//   - rangeSize (main-image dot size) must stay <= thumbnail height, otherwise dots overflow it;
//   - rangeThumbSize (thumbnail dot size) must satisfy thumbWidth/len(dots) >= dot width, otherwise
//     adjacent dots overlap;
//   - with 3–5 shapes on a 150x50 thumbnail, a thumbnail dot size of 14–18 fits exactly.
func generateClick() (*Challenge, Answer, error) {
	builder := click.NewBuilder(
		click.WithRangeLen(option.RangeVal{Min: 3, Max: 5}),
		click.WithRangeVerifyLen(option.RangeVal{Min: 2, Max: 3}),
		click.WithRangeSize(option.RangeVal{Min: 26, Max: 32}),
		click.WithRangeThumbSize(option.RangeVal{Min: 14, Max: 18}),
		click.WithRangeThumbImageSize(option.Size{Width: thumbWidth, Height: thumbHeight}),
		// Non-deformable thumbnail: the distortion algorithm smears the small shape icons into
		// unrecognizable streaks (it is tuned for full-size text).
		click.WithIsThumbNonDeformAbility(false),
		click.WithRangeThumbBgDistort(0),
	)
	builder.SetResources(
		click.WithShapes(clickShapes()),
		click.WithBackgrounds([]image.Image{background(masterWidth, masterHeight)}),
	)

	data, err := builder.MakeShape().Generate()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to generate click captcha: %w", err)
	}
	dots := data.GetData()

	// The map key is the click order (the library reassigns Index after shuffling), so sorting by
	// key is what makes "click in order" well-defined.
	order := make([]int, 0, len(dots))
	for i := range dots {
		order = append(order, i)
	}
	sort.Ints(order)

	answer := Answer{Type: TypeClick, Dots: make([]Dot, 0, len(order))}
	names := make([]string, 0, len(order))
	for _, i := range order {
		d := dots[i]
		answer.Dots = append(answer.Dots, Dot{X: d.X, Y: d.Y, Width: d.Width, Height: d.Height})
		names = append(names, d.Shape)
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode click master image: %w", err)
	}
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode click thumb image: %w", err)
	}

	return &Challenge{
		Type:   TypeClick,
		Image:  master,
		Thumb:  thumb,
		Width:  masterWidth,
		Height: masterHeight,
		Text:   strings.Join(names, ","),
	}, answer, nil
}

// generateDrag builds a slide (fixed Y axis) or drag-drop (free 2D) captcha.
//
// The two modes differ only in how the piece is allowed to move, which is a frontend concern:
//   - slide:  the piece starts on the cutout's row and only moves horizontally;
//   - dragdrop: the piece starts in a corner and must be dragged in 2D into the cutout.
//
// The answer and the comparison are the same (block.X/block.Y is the cutout either way), so both
// modes are verified by slide.Validate against the drop position.
//
// The three piece images share one shape: pieceMask defines the silhouette (its alpha channel is
// the coverage used by draw.DrawMask), pieceOverlay is the translucent draggable piece, and
// pieceShadow is the drop shadow. An opaque mask would make the "piece" a full square instead.
func generateDrag(t Type) (*Challenge, Answer, error) {
	builder := slide.NewBuilder()
	builder.SetResources(
		slide.WithBackgrounds([]image.Image{background(masterWidth, masterHeight)}),
		slide.WithGraphImages([]*slide.GraphImage{{
			OverlayImage: pieceOverlay(tileSize),
			ShadowImage:  pieceShadow(tileSize),
			MaskImage:    pieceMask(tileSize),
		}}),
	)

	capt := builder.Make()
	if t == TypeDragDrop {
		capt = builder.MakeDragDrop()
	}

	data, err := capt.Generate()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to generate %s captcha: %w", t, err)
	}
	block := data.GetData()
	if block == nil {
		return nil, Answer{}, fmt.Errorf("captcha: %s captcha generated no block data", t)
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode %s master image: %w", t, err)
	}
	tile, err := data.GetTileImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode %s tile image: %w", t, err)
	}

	// X/Y is the cutout position; the frontend reports where the user dropped the piece, and the
	// two are compared within slidePadding.
	answer := Answer{Type: t, X: block.X, Y: block.Y}
	return &Challenge{
		Type:   t,
		Image:  master,
		Thumb:  tile,
		Width:  masterWidth,
		Height: masterHeight,
	}, answer, nil
}

// generateRotate builds a rotate captcha.
func generateRotate() (*Challenge, Answer, error) {
	builder := rotate.NewBuilder()
	builder.SetResources(rotate.WithImages([]image.Image{rotateSource(rotateSize * 2)}))

	data, err := builder.Make().Generate()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to generate rotate captcha: %w", err)
	}
	block := data.GetData()
	if block == nil {
		return nil, Answer{}, errors.New("captcha: rotate captcha generated no block data")
	}

	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode rotate master image: %w", err)
	}
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, Answer{}, fmt.Errorf("captcha: failed to encode rotate thumb image: %w", err)
	}

	// The library rotates the source by block.Angle; to align, the user must rotate the thumbnail
	// by (360 - block.Angle), and rotate.Validate compares the sum against 360.
	answer := Answer{Type: TypeRotate, Angle: block.Angle}
	return &Challenge{
		Type:   TypeRotate,
		Image:  master,
		Thumb:  thumb,
		Width:  rotateSize,
		Height: rotateSize,
	}, answer, nil
}

// Verify checks the submission against the stored answer.
//
// A false result means either the captcha ID does not exist (expired / already used), or the answer
// did not match; both surface as ErrInvalid (see its doc).
//
// The answer is consumed on every attempt, including failed ones: otherwise an attacker could keep
// submitting guesses against one ID until one happened to be correct.
func Verify(ctx context.Context, store Store, sub Submission) error {
	if store == nil {
		return errors.New("captcha: store is nil")
	}
	if strings.TrimSpace(sub.ID) == "" {
		return ErrInvalid
	}
	if sub.isEmpty() {
		return ErrInvalid
	}

	answer, err := store.Read(ctx, sub.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrInvalid
		}
		return err
	}
	if !answer.Type.Valid() {
		return ErrInvalid
	}

	switch {
	case answer.Type.UsesPoint():
		if len(answer.Dots) == 0 || len(sub.Points) != len(answer.Dots) {
			return ErrInvalid
		}
		// Each click must land inside its target; comparing in order makes "click in order" meaningful.
		for i, want := range answer.Dots {
			got := sub.Points[i]
			if !click.Validate(got.X, got.Y, want.X, want.Y, want.Width, want.Height, clickPadding) {
				return ErrInvalid
			}
		}
		return nil

	case answer.Type.UsesDrag():
		// slide.Validate(sx, sy, dx, dy, padding) checks that sx is within [dx-padding, dx+padding],
		// i.e. the answer position (dx) against the user's drop position (sx).
		if !slide.Validate(sub.X, sub.Y, answer.X, answer.Y, slidePadding) {
			return ErrInvalid
		}
		return nil

	case answer.Type.UsesAngle():
		// rotate.Validate(angle, dAngle, padding) checks that angle+dAngle ≈ 360: the user rotates
		// by sub.Angle and the master was rotated by answer.Angle, so they cancel out at 360.
		if !rotate.Validate(sub.Angle, answer.Angle, rotatePadding) {
			return ErrInvalid
		}
		return nil

	default:
		return ErrInvalid
	}
}

// isEmpty reports whether the submission carries no interaction at all.
//
// Why this guard exists: slide.Validate widens the accepted window by the padding on each side, so
// an all-zero submission would pass against any cutout sitting near the top-left corner (block.X/Y
// start at 5, and a window of [X-pad, X+pad] can then include 0). The piece can never legitimately
// be dropped at exactly (0,0) — the library always places it at ≥5 — so rejecting the empty
// submission removes that blind spot without rejecting any real interaction.
func (s Submission) isEmpty() bool {
	return len(s.Points) == 0 && s.Angle == 0 && s.X == 0 && s.Y == 0
}

// MarshalAnswer serializes the answer (for Store implementations that persist JSON).
func MarshalAnswer(a Answer) ([]byte, error) { return json.Marshal(a) }

// UnmarshalAnswer deserializes the answer (for Store implementations that persist JSON).
func UnmarshalAnswer(b []byte) (Answer, error) {
	var a Answer
	if err := json.Unmarshal(b, &a); err != nil {
		return Answer{}, err
	}
	return a, nil
}
