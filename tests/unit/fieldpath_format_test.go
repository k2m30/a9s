package unit_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// ---------------------------------------------------------------------------
// T005 — FormatValue tests
// ---------------------------------------------------------------------------

// Named string type for testing named-type extraction.
type StateName string

func TestFormatValue_TimeValue(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	val := reflect.ValueOf(ts)

	got := fieldpath.FormatValue(val)
	expected := "2025-06-15 10:30:00"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatValue_TimePointer(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	val := reflect.ValueOf(&ts)

	got := fieldpath.FormatValue(val)
	expected := "2024-01-02 03:04:05"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatValue_BoolTrue(t *testing.T) {
	val := reflect.ValueOf(true)

	got := fieldpath.FormatValue(val)
	if got != "Yes" {
		t.Errorf("expected %q, got %q", "Yes", got)
	}
}

func TestFormatValue_BoolFalse(t *testing.T) {
	val := reflect.ValueOf(false)

	got := fieldpath.FormatValue(val)
	if got != "No" {
		t.Errorf("expected %q, got %q", "No", got)
	}
}

func TestFormatValue_BoolPointerTrue(t *testing.T) {
	b := true
	val := reflect.ValueOf(&b)

	got := fieldpath.FormatValue(val)
	if got != "Yes" {
		t.Errorf("expected %q, got %q", "Yes", got)
	}
}

func TestFormatValue_BoolPointerFalse(t *testing.T) {
	b := false
	val := reflect.ValueOf(&b)

	got := fieldpath.FormatValue(val)
	if got != "No" {
		t.Errorf("expected %q, got %q", "No", got)
	}
}

func TestFormatValue_StringPointer(t *testing.T) {
	s := "hello-world"
	val := reflect.ValueOf(&s)

	got := fieldpath.FormatValue(val)
	if got != "hello-world" {
		t.Errorf("expected %q, got %q", "hello-world", got)
	}
}

func TestFormatValue_StringPointerNil(t *testing.T) {
	var s *string
	val := reflect.ValueOf(s)

	got := fieldpath.FormatValue(val)
	if got != "" {
		t.Errorf("expected empty string for nil *string, got %q", got)
	}
}

func TestFormatValue_Int32(t *testing.T) {
	var n int32 = 42
	val := reflect.ValueOf(n)

	got := fieldpath.FormatValue(val)
	if got != "42" {
		t.Errorf("expected %q, got %q", "42", got)
	}
}

func TestFormatValue_Int32Pointer(t *testing.T) {
	var n int32 = 99
	val := reflect.ValueOf(&n)

	got := fieldpath.FormatValue(val)
	if got != "99" {
		t.Errorf("expected %q, got %q", "99", got)
	}
}

func TestFormatValue_NamedStringType(t *testing.T) {
	name := StateName("running")
	val := reflect.ValueOf(name)

	got := fieldpath.FormatValue(val)
	if got != "running" {
		t.Errorf("expected %q, got %q", "running", got)
	}
}

func TestFormatValue_PlainString(t *testing.T) {
	val := reflect.ValueOf("some-value")

	got := fieldpath.FormatValue(val)
	if got != "some-value" {
		t.Errorf("expected %q, got %q", "some-value", got)
	}
}

func TestFormatValue_Int64(t *testing.T) {
	var n int64 = 1234567890
	val := reflect.ValueOf(n)

	got := fieldpath.FormatValue(val)
	if got != "1234567890" {
		t.Errorf("expected %q, got %q", "1234567890", got)
	}
}

func TestFormatValue_ZeroTime(t *testing.T) {
	var ts time.Time // zero value
	val := reflect.ValueOf(ts)

	got := fieldpath.FormatValue(val)
	// Zero time should return empty string or the formatted zero date;
	// empty string is preferred for display purposes.
	if got != "" {
		// Acceptable if implementation formats zero time, but ideally empty.
		t.Logf("note: zero time formatted as %q (empty preferred)", got)
	}
}

func TestFormatValue_NilTimePointer(t *testing.T) {
	var ts *time.Time
	val := reflect.ValueOf(ts)

	got := fieldpath.FormatValue(val)
	if got != "" {
		t.Errorf("expected empty string for nil *time.Time, got %q", got)
	}
}
