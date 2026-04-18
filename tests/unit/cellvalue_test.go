package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
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

// TestFormatApproximate_AppendsPlusSuffix verifies that FormatApproximate
// returns the decimal with a trailing "+" to signal a lower-bound count.
func TestFormatApproximate_AppendsPlusSuffix(t *testing.T) {
	cases := []struct {
		input int
		want  string
	}{
		{0, "0+"},
		{1000, "1000+"},
	}
	for _, tc := range cases {
		got := resource.FormatApproximate(tc.input)
		if got != tc.want {
			t.Errorf("FormatApproximate(%d) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

// TestFormatUnknown_ReturnsEmDash verifies that FormatUnknown returns the em dash
// sentinel used to represent an indeterminate cell value.
func TestFormatUnknown_ReturnsEmDash(t *testing.T) {
	got := resource.FormatUnknown()
	if got != "—" {
		t.Errorf("FormatUnknown() = %q; want %q", got, "—")
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
		resource.CellKindApproximate,
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

// TestFormatApproximate_NotSameAsExact verifies that for any non-zero n,
// FormatApproximate(n) differs from FormatExact(n). This guards against
// accidentally stripping the "+" suffix.
func TestFormatApproximate_NotSameAsExact(t *testing.T) {
	nonZeroCases := []int{1, 5, 42, 1000, -1}
	for _, n := range nonZeroCases {
		approx := resource.FormatApproximate(n)
		exact := resource.FormatExact(n)
		if approx == exact {
			t.Errorf("FormatApproximate(%d) == FormatExact(%d) == %q; approximate must differ from exact for non-zero n", n, n, exact)
		}
	}
}
