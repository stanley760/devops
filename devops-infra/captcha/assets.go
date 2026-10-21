// Procedural CAPTCHA material generation (no binary resources embedded in the binary).
//
// ## Why not use go-captcha-assets
//
// The official material package go-captcha-assets carries 26MB of bindata, of which the font alone
// is 19MB — a text-mode click CAPTCHA needs a CJK font, but once the binary embeds it, every
// service carries 20MB of dead weight (and the font only serves the "text click" variant).
//
// This file instead generates all materials at runtime with the standard library image package:
//   - click: use shape mode (geometric figures) to avoid fonts entirely;
//   - slide: piece cutouts/masks/shadows are alpha masks;
//   - rotate: a "clock-face" source image with a clear orientation marker.
//
// The cost is a few hundred microseconds of computation per generated captcha (far below the cost
// of JPEG encoding itself), in exchange for not pulling in 26MB of dependencies.
package captcha

import (
	"crypto/rand"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"time"
)

// Sizes of the generated captcha images (kept small to control response size; the frontend scales
// via CSS).
const (
	masterWidth  = 300
	masterHeight = 220
	thumbWidth   = 150
	thumbHeight  = 50
	tileSize     = 64
	rotateSize   = 220
)

// randInt returns a random integer in [0, n).
func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	return int(randUint32() % uint32(n))
}

// randFloat returns a random float in [0, 1).
func randFloat() float64 { return float64(randUint32()%1_000_000) / 1_000_000 }

// randRange returns a random integer in [min, max].
func randRange(min, max int) int {
	if max <= min {
		return min
	}
	return min + randInt(max-min)
}

// randUint32 returns a cryptographically secure random uint32.
//
// Why crypto/rand: everything derived from this value (background color, shape position, cutout
// position) is effectively a challenge parameter. If the source were predictable, an attacker could
// pre-generate solutions offline, which defeats the captcha entirely. Each call reads 4 bytes from
// the OS entropy pool; the volume is tiny (tens of calls per generated captcha), so the overhead is
// irrelevant next to image encoding.
//
// It cannot fail in practice: a failure of the OS entropy source means the process's randomness
// guarantees are already gone. Rather than aborting a login for it, it degrades to a time-derived
// value — still unpredictable to a remote attacker, which is the property that matters here.
func randUint32() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		return binary.BigEndian.Uint32(b[:])
	}
	return uint32(time.Now().UnixNano())
}

// background builds the main image background: a two-axis color gradient + a few translucent
// circles, so each generated captcha looks different (a fixed background would let a script
// pre-locate every cutout).
func background(w, h int) image.Image {
	m := image.NewNRGBA(image.Rect(0, 0, w, h))
	r1, g1, b1 := randInt(200), randInt(200), randInt(200)
	r2, g2, b2 := randInt(200), randInt(200), randInt(200)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetNRGBA(x, y, color.NRGBA{
				R: uint8((r1*(w-x) + r2*x) / w),
				G: uint8((g1*(h-y) + g2*y) / h),
				B: uint8((b1 + b2) / 2),
				A: 255,
			})
		}
	}
	// translucent circles add irregular texture and disrupt simple edge detection
	for i := 0; i < 6; i++ {
		cx, cy := randInt(w), randInt(h)
		rad := randRange(12, 40)
		c := color.NRGBA{R: uint8(randInt(256)), G: uint8(randInt(256)), B: uint8(randInt(256)), A: 26}
		disc(m, cx, cy, rad, c)
	}
	return m
}

// disc fills a translucent circle (alpha blending).
func disc(m *image.NRGBA, cx, cy, rad int, c color.NRGBA) {
	for y := cy - rad; y <= cy+rad; y++ {
		for x := cx - rad; x <= cx+rad; x++ {
			if x < 0 || y < 0 || x >= m.Rect.Dx() || y >= m.Rect.Dy() {
				continue
			}
			if math.Hypot(float64(x-cx), float64(y-cy)) <= float64(rad) {
				blend(m, x, y, c)
			}
		}
	}
}

// blend alpha-composites c onto the pixel at (x, y).
func blend(m *image.NRGBA, x, y int, c color.NRGBA) {
	old := m.NRGBAAt(x, y)
	alpha := float64(c.A) / 255
	m.SetNRGBA(x, y, color.NRGBA{
		R: uint8(float64(old.R)*(1-alpha) + float64(c.R)*alpha),
		G: uint8(float64(old.G)*(1-alpha) + float64(c.G)*alpha),
		B: uint8(float64(old.B)*(1-alpha) + float64(c.B)*alpha),
		A: 255,
	})
}

// ---- Click mode shapes ----

// clickShapes returns the shape set for click mode. The shapes must be visually distinct (the user
// has to tell them apart from the thumbnail) and must fill the entire canvas (transparent margins
// get stretched into smears during scaling, see the sizing constraints in Generate).
func clickShapes() map[string]image.Image {
	palette := []color.NRGBA{
		{R: 235, G: 90, B: 90, A: 255},
		{R: 70, G: 150, B: 235, A: 255},
		{R: 80, G: 200, B: 120, A: 255},
		{R: 240, G: 180, B: 60, A: 255},
		{R: 190, G: 110, B: 225, A: 255},
	}
	return map[string]image.Image{
		"circle":   circleShape(32, palette[0]),
		"square":   squareShape(32, palette[1]),
		"triangle": triangleShape(32, palette[2]),
		"diamond":  diamondShape(32, palette[3]),
		"star":     starShape(32, palette[4]),
	}
}

func newCanvas(size int) *image.NRGBA { return image.NewNRGBA(image.Rect(0, 0, size, size)) }

func circleShape(sz int, c color.NRGBA) image.Image {
	m := newCanvas(sz)
	center, r := float64(sz)/2, float64(sz)/2
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			if math.Hypot(float64(x)-center, float64(y)-center) <= r {
				m.SetNRGBA(x, y, c)
			}
		}
	}
	return m
}

func squareShape(sz int, c color.NRGBA) image.Image {
	m := newCanvas(sz)
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			m.SetNRGBA(x, y, c)
		}
	}
	return m
}

func triangleShape(sz int, c color.NRGBA) image.Image {
	m := newCanvas(sz)
	top := sz / 5 // leave room at the top so scaling does not clip the apex
	for y := top; y < sz; y++ {
		half := int(float64(y-top) * float64(sz) / float64(2*(sz-top)))
		for x := sz/2 - half; x <= sz/2+half; x++ {
			m.SetNRGBA(x, y, c)
		}
	}
	return m
}

func diamondShape(sz int, c color.NRGBA) image.Image {
	m := newCanvas(sz)
	center, r := float64(sz)/2, float64(sz)/2
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			if math.Abs(float64(x)-center)+math.Abs(float64(y)-center) <= r {
				m.SetNRGBA(x, y, c)
			}
		}
	}
	return m
}

// starShape draws a five-pointed star via polar coordinates: the boundary radius alternates between
// outer/inner every half-sector.
func starShape(sz int, c color.NRGBA) image.Image {
	m := newCanvas(sz)
	center := float64(sz) / 2
	outer, inner := center, center/2.6
	sector := math.Pi / 5 // 36 degrees
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx, dy := float64(x)-center, float64(y)-center
			r := math.Hypot(dx, dy)
			if r > outer {
				continue
			}
			// Which half-sector does this pixel fall in → outer or inner vertex
			ang := math.Atan2(dy, dx)
			idx := int(math.Floor((ang + math.Pi*4) / sector))
			bound := outer
			if idx%2 == 1 {
				bound = inner
			}
			if r <= bound {
				m.SetNRGBA(x, y, c)
			}
		}
	}
	return m
}

// ---- Slide mode piece ----

// pieceMask is the puzzle-piece alpha mask: opaque inside, transparent outside. go-captcha uses it
// as the coverage of draw.DrawMask, so the alpha channel is what defines the shape (writing RGB
// black/white with a fully opaque alpha yields a full square instead).
func pieceMask(size int) image.Image {
	m := newCanvas(size)
	cx, cy := float64(size)/2, float64(size)/2
	body := float64(size) / 3
	knob := float64(size) / 6
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x), float64(y)
			inBody := fx >= cx-body && fx <= cx+body && fy >= cy-body && fy <= cy+body
			inKnob := math.Hypot(fx-cx, fy-(cy-body)) <= knob
			if inBody || inKnob {
				m.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	return m
}

// pieceOverlay is the draggable piece: the puzzle shape in translucent white. Translucent (not
// opaque) so the user can still see the image underneath while dragging.
func pieceOverlay(size int) image.Image {
	return maskTinted(pieceMask(size), color.NRGBA{R: 255, G: 255, B: 255, A: 110})
}

// pieceShadow is the drop shadow under the piece (same shape, opaque black).
func pieceShadow(size int) image.Image {
	return maskTinted(pieceMask(size), color.NRGBA{R: 0, G: 0, B: 0, A: 255})
}

// maskTinted recolors the opaque region of mask with c, leaving transparent pixels untouched.
func maskTinted(mask image.Image, c color.NRGBA) image.Image {
	b := mask.Bounds()
	m := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := mask.At(x, y).RGBA(); a > 0 {
				m.SetNRGBA(x, y, c)
			}
		}
	}
	return m
}

// ---- Rotate mode ----

// rotateSource builds the rotate-mode source image: a "clock face" with concentric rings plus a
// wide wedge at the 12 o'clock position.
//
// The orientation marker is required rather than decorative: the library rotates the whole source
// by the target angle and crops the thumbnail out of it, so if the source were a plain gradient
// (rotationally symmetric), the user would have no cue to align against and the puzzle would be
// unsolvable.
func rotateSource(size int) image.Image {
	m := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			r := math.Hypot(dx, dy)
			if r > cx {
				continue // keep the circle; outside stays transparent
			}
			// rings give radial structure
			v := 120 + int(90*math.Sin(r/9))
			// 12 o'clock orientation marker (a ±18° wedge starting at radius size/6)
			norm := math.Mod(math.Atan2(dy, dx)+math.Pi*4+math.Pi/12, math.Pi/6) - math.Pi/12
			if math.Abs(norm) < 0.32 && r > float64(size)/6 {
				v = 250
			}
			m.SetNRGBA(x, y, color.NRGBA{R: uint8(v), G: uint8(v / 2), B: uint8(255 - v), A: 255})
		}
	}
	return m
}
