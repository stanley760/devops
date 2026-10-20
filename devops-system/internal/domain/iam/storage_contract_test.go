package iam

import (
	"strconv"
	"testing"
)

// TestEncodeIDArray: the storage form must be a numeric array (existing data and the unique index depend on this form).
//
// Migration note: these three cases correspond to the pre-refactor TestPostIDsJSON /
// TestPostIDsOf / TestPostIDsRoundTrip in dao/user_admin_test.go, and guard against
// "the API's ID-stringification contract leaking into the storage layer".
func TestEncodeIDArray(t *testing.T) {
	cases := []struct {
		name string
		in   []int64
		want string
	}{
		{name: "nil set", in: nil, want: ""},
		{name: "empty slice", in: []int64{}, want: ""},
		{name: "single element", in: []int64{4}, want: "[4]"},
		{name: "multiple elements", in: []int64{4, 5}, want: "[4,5]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EncodeIDArray(c.in); got != c.want {
				t.Fatalf("EncodeIDArray(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDecodeIDArray(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []int64
	}{
		{name: "empty string", in: "", want: []int64{}},
		{name: "numeric array", in: "[4,5]", want: []int64{4, 5}},
		{name: "single element", in: "[4]", want: []int64{4}},
		// String elements are the API contract form (a product of types.ID.MarshalJSON);
		// in the storage layer they are treated as dirty data: a whole-parse failure returns an empty set, with no compatibility read.
		{name: "string elements treated as dirty data", in: `["4","5"]`, want: []int64{}},
		{name: "invalid JSON", in: "not-json", want: []int64{}},
		{name: "null literal", in: "null", want: []int64{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DecodeIDArray(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("DecodeIDArray(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("DecodeIDArray(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

func TestIDArrayRoundTrip(t *testing.T) {
	orig := []int64{4, 5, 100}
	encoded := EncodeIDArray(orig)
	if encoded != "[4,5,100]" {
		t.Fatalf("encoded = %q, want \"[4,5,100]\" (must be a numeric array)", encoded)
	}
	back := DecodeIDArray(encoded)
	if len(back) != len(orig) {
		t.Fatalf("round-trip length = %d, want %d", len(back), len(orig))
	}
	for i := range orig {
		if back[i] != orig[i] {
			t.Fatalf("round-trip value = %v, want %v", back, orig)
		}
	}
}

func TestNormalizeIDArrayJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "already a numeric array", in: "[4,5]", want: "[4,5]"},
		{name: "dirty data normalized to empty", in: `["4"]`, want: ""},
		{name: "empty string", in: "", want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeIDArrayJSON(c.in); got != c.want {
				t.Fatalf("Normalize(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSoftDeletedNaming: the naming format for soft-delete unique-key rewrites is a storage contract
// (existing soft-deleted data coexisting with the unique index depends on this format; do not change).
func TestSoftDeletedNaming(t *testing.T) {
	at := fixedTime()
	// the timestamp suffix is concatenated by Unix seconds; expected values are derived from the same
	// instant rather than hand-written constants, to avoid test and implementation drifting apart.
	want := "alice_del_42_" + strconv.FormatInt(at.Unix(), 10)
	if got := RenameSoftDeleted("alice", 42, at); got != want {
		t.Fatalf("RenameSoftDeleted = %q, want %q", got, want)
	}
	if got, want := SoftDeletedCode("editor", 9), "editor_del_9"; got != want {
		t.Fatalf("SoftDeletedCode = %q, want %q", got, want)
	}
}

func TestIDsFromStrings(t *testing.T) {
	got := IDsFromStrings([]string{"4", " 5 ", "bad", "6"})
	want := []int64{4, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("IDsFromStrings = %v, want %v (invalid entries should be skipped)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("IDsFromStrings = %v, want %v", got, want)
		}
	}
}
