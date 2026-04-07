package unit

// Tests for Bug P2: ApplyCellColor is called on PADDED text in renderDataRow.
//
// The bug is in internal/tui/views/table_render.go:133-136:
//
//   padded := text.PadOrTrunc(val, c.width)   // pads "ROOT" → "ROOT            "
//   if c.color != "" {
//     padded = ApplyCellColor(c.color, padded)  // receives "ROOT            " not "ROOT"
//   }
//
// Then in applyActorColor (resourcelist.go:921):
//   if value == "ROOT" {  // FAILS: "ROOT            " != "ROOT"
//
// And in applyOutcomeColor (resourcelist.go:939):
//   if value == "OK" {   // FAILS: "OK            " != "OK"
//
// Tests CP1, CP2, CP3 document the failing cases by calling ApplyCellColor with padded
// input — they currently FAIL because the classifiers do exact equality/suffix checks
// against the unpadded value.
//
// Test CP4 ([cross] prefix, HasPrefix survives trailing spaces) is a regression guard
// — it currently PASSES.
//
// Test CP5 (row tint after cell coloring) checks the outer RowColorStyle wrapping.
// The assertion is left as t.Skip until the coder defines a testable API for row
// rendering (see comment in the test).

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// padTo pads s with trailing spaces to totalWidth, matching text.PadOrTrunc behavior
// for values shorter than the column width.
func padTo(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

// ===========================================================================
// CP1: "ROOT" padded to width 16 must still get red-bold styling
// Currently FAILS: applyActorColor checks `value == "ROOT"` against "ROOT            "
// Bug source: table_render.go:133-135 (pad then classify)
// ===========================================================================

func TestTableRenderCellColor_ActorROOT_PaddedToWidth16(t *testing.T) {
	const colWidth = 16
	rawVal := "ROOT"
	paddedVal := padTo(rawVal, colWidth)

	// Verify we're actually testing with a padded string.
	if paddedVal == rawVal {
		t.Fatalf("test setup error: padTo(%q, %d) did not pad the string", rawVal, colWidth)
	}

	got := views.ApplyCellColor("actor", paddedVal)

	// The result must contain "ROOT" (ANSI-stripped) and carry the red-bold escape.
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "ROOT") {
		t.Errorf("ApplyCellColor(actor, padded ROOT): ANSI-stripped output %q does not contain ROOT", stripped)
	}

	// Build the expected styled output: red bold applied to the padded string.
	want := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render(paddedVal)

	if got != want {
		// Bug: current code returns `paddedVal` unchanged (no styling) because
		// applyActorColor's `value == "ROOT"` check fails on the padded string.
		// Fix location: table_render.go — trim value before calling ApplyCellColor,
		// OR ApplyCellColor must trim internally.
		t.Errorf("ApplyCellColor(actor, %q): got %q, want red-bold styled %q\n"+
			"bug: table_render.go:133-135 pads before classifying, causing applyActorColor equality check to miss",
			paddedVal, got, want)
	}
}

// ===========================================================================
// CP2: "lambda.amazonaws.com" padded to width 32 must still get dim styling
// Currently FAILS: applyActorColor checks HasSuffix(value, ".amazonaws.com")
// but "lambda.amazonaws.com                " does NOT have suffix ".amazonaws.com"
// Bug source: table_render.go:133-135
// ===========================================================================

func TestTableRenderCellColor_ActorServicePrincipal_PaddedToWidth32(t *testing.T) {
	const colWidth = 32
	rawVal := "lambda.amazonaws.com"
	paddedVal := padTo(rawVal, colWidth)

	if paddedVal == rawVal {
		t.Fatalf("test setup error: padTo(%q, %d) did not pad the string", rawVal, colWidth)
	}

	got := views.ApplyCellColor("actor", paddedVal)

	stripped := stripANSI(got)
	if !strings.Contains(stripped, "lambda.amazonaws.com") {
		t.Errorf("ApplyCellColor(actor, padded lambda.amazonaws.com): stripped output %q missing service principal name", stripped)
	}

	want := lipgloss.NewStyle().Foreground(styles.ColDim).Render(paddedVal)

	if got != want {
		// Bug: HasSuffix(paddedVal, ".amazonaws.com") is false due to trailing spaces
		t.Errorf("ApplyCellColor(actor, %q): got %q, want dim styled %q\n"+
			"bug: table_render.go:133-135 pads before classifying; HasSuffix fails on trailing spaces",
			paddedVal, got, want)
	}
}

// ===========================================================================
// CP3: "OK" padded to width 12 must still get green styling
// Currently FAILS: applyOutcomeColor checks `value == "OK"` against "OK          "
// Bug source: table_render.go:133-135
// ===========================================================================

func TestTableRenderCellColor_OutcomeOK_PaddedToWidth12(t *testing.T) {
	const colWidth = 12
	rawVal := "OK"
	paddedVal := padTo(rawVal, colWidth)

	if paddedVal == rawVal {
		t.Fatalf("test setup error: padTo(%q, %d) did not pad the string", rawVal, colWidth)
	}

	got := views.ApplyCellColor("outcome", paddedVal)

	stripped := stripANSI(got)
	if !strings.Contains(stripped, "OK") {
		t.Errorf("ApplyCellColor(outcome, padded OK): stripped output %q does not contain OK", stripped)
	}

	want := lipgloss.NewStyle().Foreground(styles.ColRunning).Render(paddedVal)

	if got != want {
		// Bug: applyOutcomeColor's `value == "OK"` check fails on "OK          "
		t.Errorf("ApplyCellColor(outcome, %q): got %q, want green styled %q\n"+
			"bug: table_render.go:133-135 pads before classifying; equality check misses padded value",
			paddedVal, got, want)
	}
}

// ===========================================================================
// CP4: "[cross] alice" padded to width 32 — regression guard (currently PASSES)
// HasPrefix survives trailing padding, so this works even with the bug.
// ===========================================================================

func TestTableRenderCellColor_CrossAccountActor_PaddedToWidth32_RegressionGuard(t *testing.T) {
	// regression guard — HasPrefix is not affected by trailing spaces
	const colWidth = 32
	rawVal := "[cross] alice"
	paddedVal := padTo(rawVal, colWidth)

	got := views.ApplyCellColor("actor", paddedVal)
	want := lipgloss.NewStyle().Foreground(styles.ColPending).Render(paddedVal)

	if got != want {
		t.Errorf("ApplyCellColor(actor, %q) regression guard: got %q, want yellow styled %q",
			paddedVal, got, want)
	}
}

// ===========================================================================
// CP5: Row tint assertion (ct-write row with pre-colored verb cell)
// The outer RowColorStyle("ct-write").Render(rowText) wraps a string that already
// contains per-cell ANSI codes. Lipgloss v2's Render() does NOT re-apply foreground
// inside already-escaped spans, so cells after the verb cell lose the red row tint.
// This is hard to assert precisely without a testing API on renderDataRow.
// Skipped until the coder exposes a testable entry point.
// ===========================================================================

func TestTableRenderCellColor_RowTint_SurvivesCellColoring(t *testing.T) {
	t.Skip("TODO(coder): expose renderDataRow or a row-level test helper so we can assert " +
		"that RowColorStyle('ct-write') foreground is applied to uncolored cells even when " +
		"the verb cell contains its own ANSI reset. Currently blocked on API design — " +
		"see Bug P2 in docs/design/ct-event-list.md and table_render.go:119-140")
}
