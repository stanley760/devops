package dao

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// TestDuplicateField_RealMySQLMessage verifies index-name parsing against real
// MySQL error text.
//
// Based on real logs captured when devops-user integrated GORM:
//
//	Error 1062 (23000): Duplicate entry 'shared@example.com' for key 't_user.uk_email'
func TestDuplicateField_RealMySQLMessage(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{"email", "Duplicate entry 'shared@example.com' for key 't_user.uk_email'", "uk_email"},
		{"mobile", "Duplicate entry '13900000001' for key 't_user.uk_mobile'", "uk_mobile"},
		{"username", "Duplicate entry 'alice' for key 't_user.uk_username'", "uk_username"},
		{"no table prefix", "Duplicate entry 'x' for key 'uk_email'", "uk_email"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dup, index := duplicateField(&mysql.MySQLError{Number: errDupEntry, Message: c.message})
			if !dup {
				t.Fatal("dup = false, want true")
			}
			if index != c.want {
				t.Fatalf("index = %q, want %q", index, c.want)
			}
		})
	}
}

// TestDuplicateField_ThroughWrapping the index name is still parsed after the
// error is wrapped.
//
// GORM errors may carry context, so errors.As must be able to see through them.
func TestDuplicateField_ThroughWrapping(t *testing.T) {
	base := &mysql.MySQLError{Number: errDupEntry, Message: "Duplicate entry 'a@b.com' for key 't_user.uk_email'"}

	if _, index := duplicateField(fmt.Errorf("insert failed: %w", base)); index != "uk_email" {
		t.Fatalf("index after fmt.Errorf wrapping = %q", index)
	}
	if _, index := duplicateField(errors.Join(errors.New("outer"), base)); index != "uk_email" {
		t.Fatalf("index after errors.Join = %q", index)
	}
}

// TestDuplicateField_NotDuplicate non-unique-constraint errors must not be
// misjudged.
func TestDuplicateField_NotDuplicate(t *testing.T) {
	cases := []error{
		errors.New("ordinary error"),
		&mysql.MySQLError{Number: 1146, Message: "Table 'devops.t_user' doesn't exist"},
	}
	for _, err := range cases {
		if dup, _ := duplicateField(err); dup {
			t.Fatalf("err=%v should not be treated as a duplicate key", err)
		}
	}
}

// TestDuplicateField_UnknownIndex when the conflict is detected but the index
// name cannot be parsed, dup is true and the index is empty.
//
// In that case the caller falls back to a generic message (without leaking
// details), which is the expected behavior.
func TestDuplicateField_UnknownIndex(t *testing.T) {
	dup, index := duplicateField(&mysql.MySQLError{Number: errDupEntry, Message: "Duplicate entry 'x'"})
	if !dup {
		t.Fatal("dup = false, want true")
	}
	if index != "" {
		t.Fatalf("index = %q, want empty string", index)
	}
}
