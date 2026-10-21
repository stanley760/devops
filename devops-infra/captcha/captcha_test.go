// Captcha behavioral contract (in-memory Store, no external dependencies):
//   - all three types generate valid images + a stored answer;
//   - the correct answer passes verification, a wrong one does not;
//   - one-shot semantics: an answer cannot be verified twice;
//   - the returned Challenge must not leak the answer.
package captcha

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// memStore is an in-memory Store with one-shot Read and TTL, for testing.
//
// Read deletes on access (mirroring RedisStore's GetDel), which is what makes "verify once"
// testable without Redis.
type memStore struct {
	mu   sync.Mutex
	data map[string]Answer
}

func newMemStore() *memStore { return &memStore{data: make(map[string]Answer)} }

func (s *memStore) Save(_ context.Context, id string, a Answer, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = a
	return nil
}

func (s *memStore) Read(_ context.Context, id string) (Answer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.data[id]
	if !ok {
		return Answer{}, ErrNotFound
	}
	delete(s.data, id)
	return a, nil
}

// correctSubmission builds a submission that must pass, given the stored answer.
//
// This is the "oracle" a real user approximates by looking at the image; the test uses the answer
// directly so that a failure means the comparison logic is wrong, not that the test guessed wrong.
func correctSubmission(a Answer) Submission {
	sub := Submission{ID: "id"}
	switch a.Type {
	case TypeClick:
		for _, d := range a.Dots {
			sub.Points = append(sub.Points, Point{X: d.X, Y: d.Y})
		}
	case TypeSlide:
		sub.X, sub.Y = a.X, a.Y
	case TypeDragDrop:
		sub.X, sub.Y = a.X, a.Y
	case TypeRotate:
		sub.Angle = 360 - a.Angle
	}
	return sub
}

func withID(sub Submission, id string) Submission { sub.ID = id; return sub }

// TestGenerateAllTypes: every type generates a challenge with images and stores an answer.
func TestGenerateAllTypes(t *testing.T) {
	ctx := context.Background()
	for _, typ := range AllTypes() {
		t.Run(typ.String(), func(t *testing.T) {
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate(%s): %v", typ, err)
			}
			if ch.Type != typ {
				t.Fatalf("challenge type = %s, want %s", ch.Type, typ)
			}
			if ch.ID == "" {
				t.Fatal("challenge ID must not be empty")
			}
			if !strings.HasPrefix(ch.Image, "data:image/") {
				t.Errorf("master image is not a data URI: %.40s", ch.Image)
			}
			if !strings.HasPrefix(ch.Thumb, "data:image/") {
				t.Errorf("thumb image is not a data URI: %.40s", ch.Thumb)
			}
			if ch.Width <= 0 || ch.Height <= 0 {
				t.Errorf("dimensions = %dx%d, want positive", ch.Width, ch.Height)
			}
			// the answer must be stored under the returned ID
			stored, ok := store.data[ch.ID]
			if !ok {
				t.Fatal("answer was not stored under the returned ID")
			}
			if stored.Type != typ {
				t.Fatalf("stored answer type = %s, want %s", stored.Type, typ)
			}
			// click mode carries the shape prompt; the others must not
			if typ == TypeClick && ch.Text == "" {
				t.Error("click challenge should carry the shape prompt in Text")
			}
			if typ != TypeClick && ch.Text != "" {
				t.Errorf("non-click challenge should not carry Text, got %q", ch.Text)
			}
		})
	}
}

// TestVerifyCorrectAnswer: the correct answer passes for every type.
func TestVerifyCorrectAnswer(t *testing.T) {
	ctx := context.Background()
	for _, typ := range AllTypes() {
		t.Run(typ.String(), func(t *testing.T) {
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			answer := store.data[ch.ID]
			if err := Verify(ctx, store, withID(correctSubmission(answer), ch.ID)); err != nil {
				t.Fatalf("the correct answer should pass: %v", err)
			}
		})
	}
}

// TestVerifyWrongAnswerFails: a wrong answer is rejected for every type.
func TestVerifyWrongAnswerFails(t *testing.T) {
	ctx := context.Background()
	for _, typ := range AllTypes() {
		t.Run(typ.String(), func(t *testing.T) {
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			answer := store.data[ch.ID]

			var wrong Submission
			switch typ {
			case TypeClick:
				// shift every click far off target
				for _, d := range answer.Dots {
					wrong.Points = append(wrong.Points, Point{X: d.X + 100, Y: d.Y + 100})
				}
			case TypeSlide, TypeDragDrop:
				wrong.X, wrong.Y = answer.X+100, answer.Y+100
			case TypeRotate:
				wrong.Angle = 1 // far from 360-angle
			}
			if err := Verify(ctx, store, withID(wrong, ch.ID)); err == nil {
				t.Fatal("a wrong answer must not pass verification")
			}
		})
	}
}

// TestVerifyOneShot: an answer can be used only once (anti-brute-force).
func TestVerifyOneShot(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	ch, err := Generate(ctx, TypeSlide, store, 0)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	answer := store.data[ch.ID]
	sub := withID(correctSubmission(answer), ch.ID)

	if err := Verify(ctx, store, sub); err != nil {
		t.Fatalf("first verification should pass: %v", err)
	}
	// replay of the same correct answer must now fail: the challenge was consumed
	if err := Verify(ctx, store, sub); err == nil {
		t.Fatal("a consumed captcha must not verify a second time")
	}
}

// TestVerifyUnknownID: an unknown ID is rejected uniformly.
func TestVerifyUnknownID(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	if err := Verify(ctx, store, Submission{ID: "nope", X: 1, Y: 1}); err == nil {
		t.Fatal("an unknown captcha ID must not verify")
	}
	if err := Verify(ctx, store, Submission{ID: ""}); err == nil {
		t.Fatal("an empty captcha ID must not verify")
	}
}

// TestChallengeDoesNotLeakAnswer: the JSON-serialized challenge must not contain the answer.
//
// This is the property that makes the design safe — if the answer leaked into the response, the
// frontend (and any attacker reading the network traffic) could read it off directly.
func TestChallengeDoesNotLeakAnswer(t *testing.T) {
	ctx := context.Background()
	for _, typ := range AllTypes() {
		t.Run(typ.String(), func(t *testing.T) {
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			b, err := json.Marshal(ch)
			if err != nil {
				t.Fatalf("marshal challenge: %v", err)
			}
			s := string(b)
			answer := store.data[ch.ID]
			switch {
			case typ.UsesDrag():
				// the cutout coordinates must not be in the response
				if strings.Contains(s, `"x":`+itoa(answer.X)) {
					t.Errorf("challenge leaks the cutout X coordinate: %s", s)
				}
			case typ.UsesAngle():
				if strings.Contains(s, `"angle":`+itoa(answer.Angle)) {
					t.Errorf("challenge leaks the rotate angle: %s", s)
				}
			case typ.UsesPoint():
				for _, d := range answer.Dots {
					if strings.Contains(s, `"width":`+itoa(d.Width)) {
						t.Errorf("challenge leaks click dot geometry: %s", s)
					}
				}
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// TestParseType: configuration values are parsed leniently but reject unknown types.
func TestParseType(t *testing.T) {
	for in, want := range map[string]Type{
		"click":  TypeClick,
		"CLICK":  TypeClick,
		" slide": TypeSlide,
		"rotate": TypeRotate,
	} {
		got, err := ParseType(in)
		if err != nil {
			t.Errorf("ParseType(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseType(%q) = %s, want %s", in, got, want)
		}
	}
	for _, bad := range []string{"", "drag", "point", "slide1"} {
		if _, err := ParseType(bad); err == nil {
			t.Errorf("ParseType(%q) should reject an unknown type", bad)
		}
	}
}

// TestAllTypesMatchesDictionary: the captcha types are exactly the set the sys_captcha_type
// dictionary seeds (click / slide / dragdrop / rotate).
//
// A type present in the code but missing from the dictionary would be unreachable from the console
// (the configuration value is validated against the dictionary), and vice versa — a dictionary item
// with no matching type would be selectable but always fall back to the default. The seed-side half
// of this contract is asserted by devops-system's TestCaptchaSeedContract.
func TestAllTypesMatchesDictionary(t *testing.T) {
	got := AllTypes()
	want := []Type{TypeClick, TypeSlide, TypeDragDrop, TypeRotate}
	if len(got) != len(want) {
		t.Fatalf("AllTypes() = %v, want %v", got, want)
	}
	for i, typ := range want {
		if got[i] != typ {
			t.Errorf("AllTypes()[%d] = %s, want %s", i, got[i], typ)
		}
		if !typ.Valid() {
			t.Errorf("%s should be a valid type", typ)
		}
	}
	// every declared type must appear exactly once
	seen := map[Type]int{}
	for _, typ := range got {
		seen[typ]++
	}
	for _, typ := range got {
		if seen[typ] != 1 {
			t.Errorf("type %s appears %d times in AllTypes()", typ, seen[typ])
		}
	}
}

// TestVerifyRejectsEmptySubmission: a submission carrying no interaction at all must be rejected.
//
// Without this guard, slide.Validate's padding widens the accepted window far enough that an
// all-zero submission passes against a cutout near the top-left corner (block coordinates start at
// 5, so the window [X-pad, X+pad] can include 0).
func TestVerifyRejectsEmptySubmission(t *testing.T) {
	ctx := context.Background()
	for _, typ := range AllTypes() {
		t.Run(typ.String(), func(t *testing.T) {
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			// valid ID, but no points / no angle / no coordinates at all
			if err := Verify(ctx, store, Submission{ID: ch.ID}); err == nil {
				t.Fatal("a submission with no interaction must not verify")
			}
		})
	}
}

// TestDragTypesVerifyIdentically: slide and drag-drop share the verification contract (both
// compare a drop position); only the permitted interaction differs. This pins that a drag-drop
// challenge is not accidentally validated with different rules than a slide challenge.
func TestDragTypesVerifyIdentically(t *testing.T) {
	ctx := context.Background()
	for _, typ := range []Type{TypeSlide, TypeDragDrop} {
		t.Run(typ.String(), func(t *testing.T) {
			if !typ.UsesDrag() {
				t.Fatalf("%s should use drag verification", typ)
			}
			store := newMemStore()
			ch, err := Generate(ctx, typ, store, 0)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			answer := store.data[ch.ID]
			if answer.Type != typ {
				t.Fatalf("stored answer type = %s, want %s", answer.Type, typ)
			}
			if !typ.UsesDrag() || typ.UsesPoint() || typ.UsesAngle() {
				t.Fatalf("%s must verify by drag only", typ)
			}
			if err := Verify(ctx, store, withID(correctSubmission(answer), ch.ID)); err != nil {
				t.Fatalf("the correct answer should pass: %v", err)
			}
		})
	}
}
