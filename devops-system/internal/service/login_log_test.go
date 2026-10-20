// Login audit service pure-function tests: lenient parsing in parseIDsCSV plus all-or-nothing rejection of invalid input.
package service

import (
	"reflect"
	"testing"
)

func TestParseIDsCSV(t *testing.T) {
	got, err := parseIDsCSV("12, 34,56")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if want := []int64{12, 34, 56}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got = %v, want %v", got, want)
	}

	if got, err := parseIDsCSV(""); err != nil || got != nil {
		t.Fatalf("empty string should return an empty set without error: got=%v err=%v", got, err)
	}

	for _, bad := range []string{"abc", "12,,34", "-1", "1.5", "12,x"} {
		if _, err := parseIDsCSV(bad); err == nil {
			t.Fatalf("ids=%q should be rejected as a whole", bad)
		}
	}
}
