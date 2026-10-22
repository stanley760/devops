// Package types provides basic types for cross-layer transport.
//
// This package keeps zero external dependencies (standard library only): stringifying
// IDs and formatting timestamps are cross-business contracts and must not bind to any
// infrastructure (error codes, logging, storage, etc.).
package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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

/* ---------------- 时间 ---------------- */

// DateTimeLayout is the unified wire format for timestamps: "2006-01-02 15:04:05".
//
// Why a custom layout instead of RFC3339 (Go's default):
//   - RFC3339 carries a timezone offset and fractional seconds, which the console UI
//     never displays — it only ever renders "年-月-日 时:分:秒".
//   - The offset makes the string longer than the value it conveys, and the browser
//     has to parse it back (new Date) just to drop the parts nobody asked for.
//   - A single fixed format on the wire also means the value can be sorted and compared
//     as plain text, and it renders identically regardless of the reader's locale.
//
// The format is intentionally zone-less: it is produced in the server's local time
// (which for this project is the configured business timezone, Asia/Shanghai).
// A timestamp is a wall-clock reading, not an instant to be re-interpreted elsewhere.
const DateTimeLayout = "2006-01-02 15:04:05"

// DateTime is a timestamp serialized as "YYYY-MM-DD HH:mm:ss".
//
// The zero value serializes to an empty string rather than "0001-01-01 00:00:00":
// a zero time means "not set", and showing a year-1 date in the UI is worse than
// showing nothing.
type DateTime struct {
	time.Time
}

// NewDateTime wraps a time.Time for the unified wire format.
func NewDateTime(t time.Time) DateTime { return DateTime{Time: t} }

// DateTimePtr wraps a *time.Time (nil stays nil).
func DateTimePtr(t *time.Time) *DateTime {
	if t == nil {
		return nil
	}
	d := DateTime{Time: *t}
	return &d
}

// MarshalJSON writes the timestamp in the unified format.
func (d DateTime) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + d.Time.Format(DateTimeLayout) + `"`), nil
}

// UnmarshalJSON accepts the unified format, and also RFC3339 for compatibility
// (older clients may still send it, and the query-parameter time filters are
// documented as RFC3339).
func (d *DateTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	if s == "null" {
		d.Time = time.Time{}
		return nil
	}
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" {
		d.Time = time.Time{}
		return nil
	}
	parsed, err := ParseDateTime(s)
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

// UnmarshalText supports path/query parameter parsing.
func (d *DateTime) UnmarshalText(b []byte) error {
	parsed, err := ParseDateTime(string(b))
	if err != nil {
		return err
	}
	d.Time = parsed
	return nil
}

// ParseDateTime parses a timestamp in the unified format, falling back to RFC3339.
//
// The unified format is tried first because it is the documented wire format;
// RFC3339 is accepted so that clients sending the previous format keep working.
func ParseDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation(DateTimeLayout, s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid datetime %q: expected %q or RFC3339", s, DateTimeLayout)
}
