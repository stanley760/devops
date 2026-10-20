// Package types provides basic types for cross-layer transport.
//
// This package keeps zero external dependencies (standard library only): stringifying
// IDs is a cross-business contract and must not bind to any infrastructure
// (error codes, logging, storage, etc.).
package types

import (
	"fmt"
	"strconv"
)

// ID is the DTO form of the int64 primary key: serialized to JSON as a string.
//
// This prepares for the future snowflake 19-digit ID convention: JSON numbers lose
// precision in JS beyond 2^53, so the online contract always emits them as strings.
type ID int64

// ParamID keeps path and query IDs as strings until explicitly parsed,
// preventing clients from treating snowflake IDs as imprecise JSON numbers.
type ParamID string

// InvalidIDError indicates a failed ID parameter parse.
//
// Deliberately not bound to infrastructure error codes (errs): this package must keep
// zero external dependencies, and each business's error-mapping layer converts it to
// the corresponding business code.
type InvalidIDError struct {
	Value string
	Cause error
}

func (e *InvalidIDError) Error() string {
	return fmt.Sprintf("invalid ID parameter: %q", e.Value)
}

// Unwrap supports errors.Is / errors.As reaching through to strconv's underlying error.
func (e *InvalidIDError) Unwrap() error { return e.Cause }

// Parse converts a path or query ID into the internal numeric ID type.
func (i ParamID) Parse() (ID, error) { return ParseID(string(i)) }

// String implements Stringer.
func (i ID) String() string { return strconv.FormatInt(int64(i), 10) }

// Int64 converts back to int64.
func (i ID) Int64() int64 { return int64(i) }

// MarshalJSON serializes to a JSON string (quoted) to avoid JS precision loss on the frontend.
func (i ID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + i.String() + `"`), nil
}

// UnmarshalJSON accepts a quoted string (standard) or a bare number (lenient).
func (i *ID) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		*i = 0
		return nil
	}
	// strip quotes
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid ID %q: %w", s, err)
	}
	*i = ID(v)
	return nil
}

// UnmarshalText supports path/query parameter parsing (the huma parameter pipeline falls back to encoding.TextUnmarshaler).
// The documented form is automatically derived as string (the real JSON form).
func (i *ID) UnmarshalText(b []byte) error {
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid ID %q", string(b))
	}
	*i = ID(v)
	return nil
}

// ParseID parses an ID from a path/query parameter (in its string form).
// On failure it returns *InvalidIDError; the caller's error-mapping layer decides the response code.
func ParseID(s string) (ID, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, &InvalidIDError{Value: s, Cause: err}
	}
	return ID(v), nil
}
