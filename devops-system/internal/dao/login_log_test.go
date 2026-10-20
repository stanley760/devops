// Pure-function test for the login audit dao: clipRunes truncates by (multi-byte) rune, preventing invalid UTF-8 from reaching the DB.
package dao

import (
	"strings"
	"testing"
)

func TestClipRunes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"short string unchanged", "admin", 30, "admin"},
		{"byte length within limit returns as-is (fast path)", "中国", 64, "中国"},
		{"overlong Chinese truncated by rune", "黑龙江省哈尔滨市道里区", 3, "黑龙江"},
		{"ASCII truncation", strings.Repeat("a", 100), 64, strings.Repeat("a", 64)},
		{"mixed Chinese/English counted by rune", "中国abc中国abc", 5, "中国abc"},
		{"empty string", "", 30, ""},
		{"zero limit", "abc", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clipRunes(c.in, c.n); got != c.want {
				t.Fatalf("clipRunes(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}
