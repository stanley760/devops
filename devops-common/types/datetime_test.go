package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateTimeMarshalUsesUnifiedLayout(t *testing.T) {
	ts := time.Date(2026, 10, 22, 16, 46, 14, 0, time.Local)
	b, err := json.Marshal(NewDateTime(ts))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(b), `"2026-10-22 16:46:14"`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

// The zero time means "not set" — showing a year-1 date is worse than showing nothing.
func TestDateTimeMarshalZeroValueIsEmptyString(t *testing.T) {
	b, err := json.Marshal(DateTime{})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(b), `""`; got != want {
		t.Fatalf("Marshal() = %s, want %s", got, want)
	}
}

func TestDateTimeUnmarshalAcceptsUnifiedLayout(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`"2026-10-22 16:46:14"`), &d); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := time.Date(2026, 10, 22, 16, 46, 14, 0, time.Local)
	if !d.Time.Equal(want) {
		t.Fatalf("Unmarshal() = %v, want %v", d.Time, want)
	}
}

// Older clients may still send RFC3339; it must keep working.
func TestDateTimeUnmarshalAcceptsRFC3339(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`"2026-10-22T16:46:14+08:00"`), &d); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	want := time.Date(2026, 10, 22, 16, 46, 14, 0, time.FixedZone("", 8*3600))
	if !d.Time.Equal(want) {
		t.Fatalf("Unmarshal() = %v, want %v", d.Time, want)
	}
}

func TestDateTimeUnmarshalNullAndEmpty(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`null`), &d); err != nil {
		t.Fatalf("Unmarshal(null) error = %v", err)
	}
	if !d.IsZero() {
		t.Fatalf("Unmarshal(null) should produce the zero time")
	}
	if err := json.Unmarshal([]byte(`""`), &d); err != nil {
		t.Fatalf("Unmarshal(\"\") error = %v", err)
	}
	if !d.IsZero() {
		t.Fatalf("Unmarshal(\"\") should produce the zero time")
	}
}

func TestDateTimeUnmarshalRejectsGarbage(t *testing.T) {
	var d DateTime
	if err := json.Unmarshal([]byte(`"not-a-time"`), &d); err == nil {
		t.Fatal("Unmarshal() should reject an unparsable timestamp")
	}
}

func TestDateTimePtrNilStaysNil(t *testing.T) {
	if got := DateTimePtr(nil); got != nil {
		t.Fatalf("DateTimePtr(nil) = %v, want nil", *got)
	}
	ts := time.Date(2026, 10, 22, 16, 46, 14, 0, time.Local)
	got := DateTimePtr(&ts)
	if got == nil {
		t.Fatal("DateTimePtr(&ts) = nil, want non-nil")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != `"2026-10-22 16:46:14"` {
		t.Fatalf("Marshal() = %s", b)
	}
}

// A pointer to DateTime must serialize as null when absent, not as a zero date.
func TestDateTimePtrNilMarshalsAsNull(t *testing.T) {
	b, err := json.Marshal(DateTimePtr(nil))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != "null" {
		t.Fatalf("Marshal(nil) = %s, want null", b)
	}
}
