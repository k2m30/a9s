package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
)

// TestFormatExact_ProducesPlainDecimal verifies that FormatExact returns the
// plain decimal string of the integer, with no suffix or decoration.
func TestFormatExact_ProducesPlainDecimal(t *testing.T) {
	cases := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{150, "150"},
		{-1, "-1"},
	}
	for _, tc := range cases {
		got := resource.FormatExact(tc.input)
		if got != tc.want {
			t.Errorf("FormatExact(%d) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestFormatTruncated_AppendsPlusSuffix verifies that FormatTruncated
// returns the decimal with a trailing "+" to signal a lower-bound count.
func TestFormatTruncated_AppendsPlusSuffix(t *testing.T) {
	cases := []struct {
		input int
		want  string
	}{
		{0, "0+"},
		{1000, "1000+"},
	}
	for _, tc := range cases {
		got := resource.FormatTruncated(tc.input)
		if got != tc.want {
			t.Errorf("FormatTruncated(%d) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestCellUnknownText_IsEmDash verifies that the exported constant equals the
// em dash character used throughout the UI.
func TestCellUnknownText_IsEmDash(t *testing.T) {
	if resource.CellUnknownText != "—" {
		t.Errorf("CellUnknownText = %q; want %q", resource.CellUnknownText, "—")
	}
}

// TestCellKind_Enum_DistinctValues verifies that the three CellKind constants
// are pairwise distinct so downstream switches cannot accidentally conflate them.
func TestCellKind_Enum_DistinctValues(t *testing.T) {
	values := []resource.CellKind{
		resource.CellKindExact,
		resource.CellKindTruncated,
		resource.CellKindUnknown,
	}
	seen := make(map[resource.CellKind]bool)
	for _, v := range values {
		if seen[v] {
			t.Errorf("CellKind constant %d appears more than once — enum values must be pairwise distinct", int(v))
		}
		seen[v] = true
	}
}

// TestFormatTruncated_NotSameAsExact verifies that for any non-zero n,
// FormatTruncated(n) differs from FormatExact(n). This guards against
// accidentally stripping the "+" suffix.
func TestFormatTruncated_NotSameAsExact(t *testing.T) {
	nonZeroCases := []int{1, 5, 42, 1000, -1}
	for _, n := range nonZeroCases {
		truncated := resource.FormatTruncated(n)
		exact := resource.FormatExact(n)
		if truncated == exact {
			t.Errorf("FormatTruncated(%d) == FormatExact(%d) == %q; truncated must differ from exact for non-zero n", n, n, exact)
		}
	}
}
