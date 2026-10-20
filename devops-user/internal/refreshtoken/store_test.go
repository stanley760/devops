package refreshtoken

import (
	"strings"
	"testing"
)

// TestGenerate covers the shape contract between plaintext and hash.
//
// Contract: the plaintext is 43 chars of base64url (32 random bytes), and the
// hash is 43 chars and **not equal** to the plaintext. If they were equal, the
// stored value would be the plaintext, and a dumped database could be replayed
// directly.
func TestGenerate(t *testing.T) {
	plain, hash, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != 43 {
		t.Fatalf("plain length = %d, want 43", len(plain))
	}
	if len(hash) != 43 {
		t.Fatalf("hash length = %d, want 43", len(hash))
	}
	if plain == hash {
		t.Fatal("hash equals plain — the server would store the plaintext")
	}
	if !IsHashLike(Hash(plain)) {
		t.Fatal("Hash(plain) does not match the hash shape")
	}
}

// TestGenerateUnique two generations must differ (otherwise it is a fixed
// credential).
func TestGenerateUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		plain, hash, err := Generate()
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := seen[plain]; dup {
			t.Fatalf("duplicate plaintext generated on iteration %d", i)
		}
		if _, dup := seen[hash]; dup {
			t.Fatalf("duplicate hash generated on iteration %d", i)
		}
		seen[plain] = struct{}{}
		seen[hash] = struct{}{}
	}
}

// TestHashDeterministic the same plaintext always yields the same hash
// (otherwise lookups are guaranteed to miss).
func TestHashDeterministic(t *testing.T) {
	plain := "some-refresh-token"
	if Hash(plain) != Hash(plain) {
		t.Fatal("Hash is non-deterministic — unusable for lookups")
	}
	if Hash(plain) == Hash(plain+"x") {
		t.Fatal("different plaintexts produced the same hash")
	}
}

// TestIsHashLike rejects obviously invalid input, avoiding oversized Redis
// lookups.
func TestIsHashLike(t *testing.T) {
	_, hash, _ := Generate()
	if !IsHashLike(hash) {
		t.Fatal("a valid hash should pass")
	}
	for _, bad := range []string{
		"",
		"short",
		strings.Repeat("a", 100),
		"has space here 123456789012345678901234567890",
	} {
		if IsHashLike(bad) {
			t.Fatalf("invalid input %q should not pass", bad)
		}
	}
}

// TestConstantTimeEqual equality is only possible when lengths match.
func TestConstantTimeEqual(t *testing.T) {
	if ConstantTimeEqual("abc", "abc") != true {
		t.Fatal("equal inputs should compare equal")
	}
	if ConstantTimeEqual("abc", "abd") != false {
		t.Fatal("different inputs should compare unequal")
	}
	if ConstantTimeEqual("abc", "abcd") != false {
		t.Fatal("different lengths should compare unequal")
	}
}
