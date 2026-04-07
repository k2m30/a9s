package unit

// Tests for Bug OFC: applyOutcomeColor does not handle AWS error codes.
//
// Design spec (docs/design/ct-event-list.md) requires that ANY non-OK, non-START,
// non-END outcome value render as RED BOLD. This includes CloudTrail errorCode
// values such as "AccessDenied", "Throttling", "ValidationException", and
// "NoSuchBucket" — all of which currently render with NO styling because
// applyOutcomeColor (resourcelist.go:923) only matches prefix "FAILED".
//
// Tests OFC1–OFC3 MUST FAIL against HEAD (red bold not applied to error codes).
// Tests OFC4–OFC5 are regression guards for FAILED* and OK/START/END (must PASS).
//
// Repair location: internal/tui/views/resourcelist.go:applyOutcomeColor — add an
// else-clause that renders any non-empty, unrecognised outcome as ColStopped.Bold.

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// OFC1: AWS error codes — raw (unpadded) values must render red bold
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_AccessDenied_Raw(t *testing.T) {
	val := "AccessDenied"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_AccessDenied_Raw FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q)\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor only matches prefix \"FAILED\"; AWS error codes like\n"+
				"  \"AccessDenied\" fall through with no styling.\n"+
				"  Fix: resourcelist.go:applyOutcomeColor — add red-bold fallback for unrecognised values.",
			val, got, want,
		)
	}
}

func TestOutcomeFailureColor_Throttling_Raw(t *testing.T) {
	val := "Throttling"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_Throttling_Raw FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q)\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor does not handle AWS error code %q.",
			val, got, want, val,
		)
	}
}

func TestOutcomeFailureColor_ValidationException_Raw(t *testing.T) {
	val := "ValidationException"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_ValidationException_Raw FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q)\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor does not handle AWS error code %q.",
			val, got, want, val,
		)
	}
}

// ---------------------------------------------------------------------------
// OFC2: AWS error codes — padded to column width 14 (as renderDataRow passes them)
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_AccessDenied_PaddedToWidth14(t *testing.T) {
	const colWidth = 14
	raw := "AccessDenied"
	val := padTo(raw, colWidth)

	if val == raw {
		t.Fatalf("test setup error: padTo(%q, %d) did not pad", raw, colWidth)
	}

	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	stripped := stripANSI(got)
	if !strings.Contains(stripped, raw) {
		t.Errorf("ApplyCellColor(outcome, padded AccessDenied): stripped output %q does not contain %q", stripped, raw)
	}

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_AccessDenied_PaddedToWidth14 FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q) [padded to width %d]\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor does not handle AWS error code %q (padded).",
			val, colWidth, got, want, raw,
		)
	}
}

// ---------------------------------------------------------------------------
// OFC3: Additional AWS error codes — raw values must render red bold
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_NoSuchBucket_Raw(t *testing.T) {
	val := "NoSuchBucket"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_NoSuchBucket_Raw FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q)\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor does not handle AWS error code %q.",
			val, got, want, val,
		)
	}
}

func TestOutcomeFailureColor_UnauthorizedOperation_Raw(t *testing.T) {
	val := "UnauthorizedOperation"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf(
			"TestOutcomeFailureColor_UnauthorizedOperation_Raw FAIL (bug present):\n"+
				"  ApplyCellColor(\"outcome\", %q)\n"+
				"  got:  %q\n"+
				"  want: %q (red bold)\n"+
				"  Bug: applyOutcomeColor does not handle AWS error code %q.",
			val, got, want, val,
		)
	}
}

// ---------------------------------------------------------------------------
// OFC4: FAILED* regression guards — must still render red bold (must PASS)
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_FAILED_Exact_RegressionGuard(t *testing.T) {
	val := "FAILED"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): got %q, want red-bold %q", val, got, want)
	}
}

func TestOutcomeFailureColor_FAILEDColon_RegressionGuard(t *testing.T) {
	val := "FAILED:Conflict"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(val)

	if got != want {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): got %q, want red-bold %q", val, got, want)
	}
}

// ---------------------------------------------------------------------------
// OFC5: OK / START / END regression guards (must PASS)
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_OK_RegressionGuard(t *testing.T) {
	val := "OK"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColRunning).Render(val)

	if got != want {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): got %q, want green (no bold) %q", val, got, want)
	}
	// Must NOT be bold.
	bold := lipgloss.NewStyle().Foreground(styles.ColRunning).Bold(true).Render(val)
	if got == bold {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): result is bold-styled, expected no bold", val)
	}
}

func TestOutcomeFailureColor_START_RegressionGuard(t *testing.T) {
	val := "START"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColPending).Render(val)

	if got != want {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): got %q, want yellow %q", val, got, want)
	}
}

func TestOutcomeFailureColor_END_RegressionGuard(t *testing.T) {
	val := "END"
	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColPending).Render(val)

	if got != want {
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q): got %q, want yellow %q", val, got, want)
	}
}

// ---------------------------------------------------------------------------
// OFC6: OK padded — regression guard for padded OK (must PASS after CP3 fix)
// Note: this test validates the padded-OK case mirrors the padded-AccessDenied
// behavior — both must work after the respective fixes.
// ---------------------------------------------------------------------------

func TestOutcomeFailureColor_OK_PaddedToWidth16_RegressionGuard(t *testing.T) {
	const colWidth = 16
	raw := "OK"
	val := padTo(raw, colWidth)

	if val == raw {
		t.Fatalf("test setup error: padTo(%q, %d) did not pad", raw, colWidth)
	}

	got := views.ApplyCellColor("outcome", val)
	want := lipgloss.NewStyle().Foreground(styles.ColRunning).Render(val)

	if got != want {
		// This may also fail until the CP3 fix (pad before classify) is applied.
		t.Errorf("regression guard: ApplyCellColor(\"outcome\", %q) [padded OK]: got %q, want green %q", val, got, want)
	}
}
