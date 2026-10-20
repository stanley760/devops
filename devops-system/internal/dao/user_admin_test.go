// Storage-form contract for ID JSON arrays: only numeric arrays are stored in the DB; all
// string<->number conversion happens in the code layer (write side: PostIDsJSON /
// NormalizeDeptIDs; response side: MarshalJSON of types.ID).
package dao_test

import (
	"reflect"
	"testing"

	"github.com/stanley760/devops/devops-system/internal/dao"
	"github.com/stanley760/devops/devops-system/internal/types"
)

func TestPostIDsJSON(t *testing.T) {
	cases := []struct {
		name string
		in   []types.ID
		want string
	}{
		{"numeric array form persisted", []types.ID{4, 5}, "[4,5]"},
		{"single element", []types.ID{4}, "[4]"},
		{"empty set returns empty string (caller writes NULL)", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dao.PostIDsJSON(c.in); got != c.want {
				t.Fatalf("PostIDsJSON(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestPostIDsOf(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []types.ID
	}{
		{"numeric form (the only stored form)", "[4,5]", []types.ID{4, 5}},
		{"string elements treated as dirty data", `["4","5"]`, []types.ID{}},
		{"mixed form treated as dirty data", `["4",5]`, []types.ID{}},
		{"empty string", "", []types.ID{}},
		{"invalid JSON", "abc", []types.ID{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dao.PostIDsOf(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("PostIDsOf(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("PostIDsOf(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

// Write-read round trip: the output of PostIDsJSON must be read back verbatim by PostIDsOf
// (the numeric array is the only stored form).
func TestPostIDsRoundTrip(t *testing.T) {
	ids := []types.ID{1, 2, 3}
	got := dao.PostIDsOf(dao.PostIDsJSON(ids))
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("round-trip = %v, want %v", got, ids)
	}
}
