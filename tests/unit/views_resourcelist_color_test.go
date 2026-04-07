package unit

// Tests for the per-cell Color classifier: views.ApplyCellColor(classifier, value) string
//
// These tests are written BEFORE the implementation exists (TDD). They will fail
// to compile until the coder adds ApplyCellColor to internal/tui/views/resourcelist.go.
//
// Bug vectors covered:
//   - Wrong color token mapped to a verb (e.g. "W" mapped red but not bold)
//   - Missing bold on write/delete/system/insight verbs
//   - actor classifier: ROOT not detected, service principal not dimmed
//   - outcome: FAILED prefix not matched, START/END not yellow
//   - origin: Console not accented, Service not dimmed
//   - Unknown classifier panicking instead of falling back to passthrough
//   - Empty classifier mutating cell text

import (
	"image/color"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// helpers (reuse colorsEqual from tui_styles_test.go — same package)
// ---------------------------------------------------------------------------

// extractFG renders a lipgloss-styled string and infers foreground from the
// style applied to the returned cell value. Because ApplyCellColor returns an
// ANSI-escaped string we cannot parse it easily; instead we call the function
// and compare against a known-good style rendered on the same input text.
//
// Strategy: build the expected styled string with lipgloss and compare bytes.
// This validates both the color token and the bold flag in one assertion.

func styledEq(a, b string) bool {
	return a == b
}

// expectedRender builds the expected output for a given style applied to text.
func expectedRender(style lipgloss.Style, text string) string {
	return style.Render(text)
}

// ===========================================================================
// Empty classifier — passthrough
// ===========================================================================

func TestApplyCellColor_EmptyClassifier_ReturnsTextUnchanged(t *testing.T) {
	inputs := []string{"R", "W", "D", "running", "OK", "ROOT", "Console", "hello"}
	for _, v := range inputs {
		got := views.ApplyCellColor("", v)
		// Empty classifier must not add any ANSI escapes. The returned string
		// must equal the raw input (no styling wrapper).
		if got != v {
			t.Errorf("ApplyCellColor(%q, %q): expected passthrough %q, got %q", "", v, v, got)
		}
	}
}

// ===========================================================================
// "verb" classifier — all seven values
// ===========================================================================

func TestApplyCellColor_VerbClassifier_R_IsDim(t *testing.T) {
	got := views.ApplyCellColor("verb", "R")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), "R")
	if !styledEq(got, want) {
		t.Errorf("verb 'R': expected dim style %q, got %q", want, got)
	}
}

func TestApplyCellColor_VerbClassifier_W_IsOrangeBold(t *testing.T) {
	got := views.ApplyCellColor("verb", "W")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColYAMLNum).Bold(true), "W")
	if !styledEq(got, want) {
		t.Errorf("verb 'W': expected orange bold %q, got %q", want, got)
	}
}

func TestApplyCellColor_VerbClassifier_D_IsRedBold(t *testing.T) {
	got := views.ApplyCellColor("verb", "D")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), "D")
	if !styledEq(got, want) {
		t.Errorf("verb 'D': expected red bold %q, got %q", want, got)
	}
}

func TestApplyCellColor_VerbClassifier_S_IsAccentBold(t *testing.T) {
	got := views.ApplyCellColor("verb", "S")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true), "S")
	if !styledEq(got, want) {
		t.Errorf("verb 'S': expected accent bold %q, got %q", want, got)
	}
}

func TestApplyCellColor_VerbClassifier_I_IsPurpleBold(t *testing.T) {
	got := views.ApplyCellColor("verb", "I")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColYAMLBool).Bold(true), "I")
	if !styledEq(got, want) {
		t.Errorf("verb 'I': expected purple bold %q, got %q", want, got)
	}
}

func TestApplyCellColor_VerbClassifier_N_IsAccentNoBold(t *testing.T) {
	got := views.ApplyCellColor("verb", "N")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent), "N")
	if !styledEq(got, want) {
		t.Errorf("verb 'N': expected accent (no bold) %q, got %q", want, got)
	}
	// Explicitly verify N is NOT bold (different from S which is also accent but bold).
	boldWant := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true), "N")
	if got == boldWant {
		t.Error("verb 'N': must not be bold (spec: N → accent, S → accent bold)")
	}
}

func TestApplyCellColor_VerbClassifier_Unknown_IsPlain(t *testing.T) {
	got := views.ApplyCellColor("verb", "?")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColHeaderFg), "?")
	if !styledEq(got, want) {
		t.Errorf("verb '?': expected plain (ColHeaderFg) %q, got %q", want, got)
	}
}

// All seven verb values in a table test for regression coverage.
func TestApplyCellColor_VerbClassifier_AllValues(t *testing.T) {
	cases := []struct {
		verb  string
		style lipgloss.Style
	}{
		{"R", lipgloss.NewStyle().Foreground(styles.ColDim)},
		{"W", lipgloss.NewStyle().Foreground(styles.ColYAMLNum).Bold(true)},
		{"D", lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true)},
		{"S", lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true)},
		{"I", lipgloss.NewStyle().Foreground(styles.ColYAMLBool).Bold(true)},
		{"N", lipgloss.NewStyle().Foreground(styles.ColAccent)},
		{"?", lipgloss.NewStyle().Foreground(styles.ColHeaderFg)},
	}
	for _, tc := range cases {
		got := views.ApplyCellColor("verb", tc.verb)
		want := expectedRender(tc.style, tc.verb)
		if got != want {
			t.Errorf("verb %q: got %q, want %q", tc.verb, got, want)
		}
	}
}

// ===========================================================================
// "actor" classifier
// ===========================================================================

func TestApplyCellColor_ActorClassifier_ROOT_IsRedBold(t *testing.T) {
	got := views.ApplyCellColor("actor", "ROOT")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), "ROOT")
	if !styledEq(got, want) {
		t.Errorf("actor 'ROOT': expected red bold %q, got %q", want, got)
	}
}

func TestApplyCellColor_ActorClassifier_ServicePrincipal_IsDim(t *testing.T) {
	// Values ending in ".amazonaws.com" are service principals → dim.
	principals := []string{
		"logs.amazonaws.com",
		"lambda.amazonaws.com",
		"ec2.amazonaws.com",
	}
	for _, p := range principals {
		got := views.ApplyCellColor("actor", p)
		want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), p)
		if !styledEq(got, want) {
			t.Errorf("actor service principal %q: expected dim %q, got %q", p, want, got)
		}
	}
}

func TestApplyCellColor_ActorClassifier_CrossAccount_IsYellow(t *testing.T) {
	// Cross-account actors: the cell value itself carries the indicator. Per spec,
	// the classifier receives the pre-computed _ct.actor string. If the value contains
	// a cross-account marker prefix (e.g. "[cross] arn:..."), it should be yellow.
	// TODO: exact format of cross-account actor string to be confirmed with coder
	// once T013C defines the _ct.actor serialization. For now test a representative
	// value that the coder should handle as cross-account.
	crossAccountValue := "[cross] arn:aws:iam::123456789012:role/SomeRole"
	got := views.ApplyCellColor("actor", crossAccountValue)
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), crossAccountValue)
	if !styledEq(got, want) {
		t.Errorf("actor cross-account: expected yellow %q, got %q", want, got)
	}
}

// Verify ROOT is detected case-sensitively (only all-caps ROOT is special).
func TestApplyCellColor_ActorClassifier_LowercaseRoot_IsNotSpecial(t *testing.T) {
	got := views.ApplyCellColor("actor", "root")
	// "root" (lowercase) is a normal actor — must NOT be red bold.
	boldRed := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), "root")
	if got == boldRed {
		t.Error("actor 'root' (lowercase) must not be styled as ROOT; ROOT detection must be case-sensitive")
	}
}

// ===========================================================================
// "outcome" classifier
// ===========================================================================

func TestApplyCellColor_OutcomeClassifier_OK_IsDimGreen(t *testing.T) {
	got := views.ApplyCellColor("outcome", "OK")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColRunning), "OK")
	// spec says "dim green" — ColRunning is the green token; the dim aspect is
	// conveyed by the colour itself being a muted green vs a bold treatment.
	if !styledEq(got, want) {
		t.Errorf("outcome 'OK': expected dim green %q, got %q", want, got)
	}
	// Also verify it is NOT bold.
	boldGreen := expectedRender(lipgloss.NewStyle().Foreground(styles.ColRunning).Bold(true), "OK")
	if got == boldGreen {
		t.Error("outcome 'OK' must not be bold")
	}
}

func TestApplyCellColor_OutcomeClassifier_FAILED_IsRedBold(t *testing.T) {
	failedValues := []string{
		"FAILED",
		"FAILED: AccessDenied",
		"FAILED: ThrottlingException",
	}
	for _, v := range failedValues {
		got := views.ApplyCellColor("outcome", v)
		want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), v)
		if !styledEq(got, want) {
			t.Errorf("outcome %q: expected red bold %q, got %q", v, want, got)
		}
	}
}

func TestApplyCellColor_OutcomeClassifier_START_IsYellow(t *testing.T) {
	got := views.ApplyCellColor("outcome", "START")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), "START")
	if !styledEq(got, want) {
		t.Errorf("outcome 'START': expected yellow %q, got %q", want, got)
	}
}

func TestApplyCellColor_OutcomeClassifier_END_IsYellow(t *testing.T) {
	got := views.ApplyCellColor("outcome", "END")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), "END")
	if !styledEq(got, want) {
		t.Errorf("outcome 'END': expected yellow %q, got %q", want, got)
	}
}

// Verify that "FAILED" prefix detection does not apply to values that merely
// contain "FAILED" in the middle.
func TestApplyCellColor_OutcomeClassifier_NotFAILEDPrefix_IsNotRedBold(t *testing.T) {
	v := "AccessDenied"
	got := views.ApplyCellColor("outcome", v)
	boldRed := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), v)
	if got == boldRed {
		t.Errorf("outcome %q should not be red bold (does not start with FAILED)", v)
	}
}

// ===========================================================================
// "origin" classifier
// ===========================================================================

func TestApplyCellColor_OriginClassifier_Service_IsDim(t *testing.T) {
	got := views.ApplyCellColor("origin", "Service")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), "Service")
	if !styledEq(got, want) {
		t.Errorf("origin 'Service': expected dim %q, got %q", want, got)
	}
}

func TestApplyCellColor_OriginClassifier_Console_IsAccent(t *testing.T) {
	got := views.ApplyCellColor("origin", "Console")
	want := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent), "Console")
	if !styledEq(got, want) {
		t.Errorf("origin 'Console': expected accent %q, got %q", want, got)
	}
}

func TestApplyCellColor_OriginClassifier_CLI_IsPlain(t *testing.T) {
	got := views.ApplyCellColor("origin", "CLI")
	// Plain means no extra styling. The coder may return the bare text or a
	// zero-style wrapper — both are acceptable; we just check it is not
	// dim (Service) or accent (Console).
	dimStyle := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), "CLI")
	accentStyle := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent), "CLI")
	if got == dimStyle {
		t.Error("origin 'CLI' must not be dim (that is Service)")
	}
	if got == accentStyle {
		t.Error("origin 'CLI' must not be accent (that is Console)")
	}
}

func TestApplyCellColor_OriginClassifier_OtherValues_AreNotDimOrAccent(t *testing.T) {
	others := []string{"SDK", "TF", "Boto", "Browser", "VPCE", "?"}
	for _, v := range others {
		got := views.ApplyCellColor("origin", v)
		dimStyle := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), v)
		accentStyle := expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent), v)
		if got == dimStyle {
			t.Errorf("origin %q: must not be dim (only 'Service' is dim)", v)
		}
		if got == accentStyle {
			t.Errorf("origin %q: must not be accent (only 'Console' is accent)", v)
		}
	}
}

// ===========================================================================
// Unknown classifier — defensive fallback
// ===========================================================================

func TestApplyCellColor_UnknownClassifier_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ApplyCellColor(\"bogus\", \"value\") panicked: %v", r)
		}
	}()
	got := views.ApplyCellColor("bogus", "value")
	// Must return a non-empty string (at minimum the raw value).
	if got == "" {
		t.Error("ApplyCellColor(\"bogus\", \"value\"): must return non-empty string")
	}
}

func TestApplyCellColor_UnknownClassifier_DoesNotApplyColor(t *testing.T) {
	v := "somevalue"
	got := views.ApplyCellColor("bogus", v)
	// The returned string must not include ANSI color codes from any known
	// classifier. Heuristic: raw value must appear as a substring of the result,
	// and the result should not carry a non-default styled wrapper.
	if !strings.Contains(got, v) {
		t.Errorf("ApplyCellColor(\"bogus\", %q): raw value %q not present in result %q", v, v, got)
	}
	// Compare against a few colored variants — none should match.
	colored := []string{
		expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped), v),
		expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), v),
		expectedRender(lipgloss.NewStyle().Foreground(styles.ColRunning), v),
		expectedRender(lipgloss.NewStyle().Foreground(styles.ColAccent), v),
		expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), v),
	}
	for _, c := range colored {
		if got == c {
			t.Errorf("ApplyCellColor(\"bogus\", %q): unexpected color applied, got %q", v, got)
		}
	}
}

// ===========================================================================
// Edge cases
// ===========================================================================

func TestApplyCellColor_EmptyValue_DoesNotPanic(t *testing.T) {
	classifiers := []string{"", "verb", "actor", "outcome", "origin", "bogus"}
	for _, cls := range classifiers {
		func(c string) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ApplyCellColor(%q, \"\") panicked: %v", c, r)
				}
			}()
			_ = views.ApplyCellColor(c, "")
		}(cls)
	}
}

// Verify that bold verbs (W, D, S, I) actually have bold set — catches a bug
// where the coder uses foreground-only style and forgets Bold(true).
func TestApplyCellColor_VerbBoldVerbs_AreBold(t *testing.T) {
	boldVerbs := []string{"W", "D", "S", "I"}
	for _, v := range boldVerbs {
		got := views.ApplyCellColor("verb", v)
		// A non-bold version of the same colour should NOT equal got.
		var noBoldStyle lipgloss.Style
		switch v {
		case "W":
			noBoldStyle = lipgloss.NewStyle().Foreground(styles.ColYAMLNum)
		case "D":
			noBoldStyle = lipgloss.NewStyle().Foreground(styles.ColStopped)
		case "S":
			noBoldStyle = lipgloss.NewStyle().Foreground(styles.ColAccent)
		case "I":
			noBoldStyle = lipgloss.NewStyle().Foreground(styles.ColYAMLBool)
		}
		noBold := expectedRender(noBoldStyle, v)
		if got == noBold {
			t.Errorf("verb %q: got non-bold output %q, but spec requires bold", v, got)
		}
	}
}

// Verify non-bold verbs (R, N, ?) are NOT bold — catches the opposite bug.
func TestApplyCellColor_VerbNonBoldVerbs_AreNotBold(t *testing.T) {
	type tc struct {
		verb  string
		style lipgloss.Style
	}
	cases := []tc{
		{"R", lipgloss.NewStyle().Foreground(styles.ColDim)},
		{"N", lipgloss.NewStyle().Foreground(styles.ColAccent)},
		{"?", lipgloss.NewStyle().Foreground(styles.ColHeaderFg)},
	}
	for _, c := range cases {
		got := views.ApplyCellColor("verb", c.verb)
		boldVersion := expectedRender(c.style.Bold(true), c.verb)
		if got == boldVersion {
			t.Errorf("verb %q: must not be bold, but got bold-styled output %q", c.verb, got)
		}
	}
}

// ===========================================================================
// T040Q — ApplyCellColor independent of row tint
// ===========================================================================

// TestApplyCellColor_IndependentOfRowTint verifies that the per-cell color
// classifier (verb) is not clobbered by the row-level Resource.Status tint.
//
// Bug vector: if the cell formatter applies row tint after the cell classifier,
// the verb glyph cell would lose its classifier color and render in the row
// color instead (e.g. a "D" verb glyph turning yellow on a ct-read row instead
// of staying red-bold).
//
// This test exercises ApplyCellColor in isolation — the unit-level contract is:
// ApplyCellColor("verb", "D") must return a red-bold-styled string regardless
// of any surrounding row tint. The integration concern (row tint not clobbering
// cell color in the actual table renderer) is addressed by the demo smoke test
// (./a9s --demo → CloudTrail Events list).
func TestApplyCellColor_IndependentOfRowTint(t *testing.T) {
	// WRITE event: verb="D", row tint would be ct-write (red fg).
	// The verb glyph cell must still be red-bold (ColStopped.Bold), not just red fg.
	gotD := views.ApplyCellColor("verb", "D")
	wantD := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true), "D")
	if gotD != wantD {
		t.Errorf("verb 'D' cell color = %q, want red-bold %q (must be independent of ct-write row tint)", gotD, wantD)
	}
	// Specifically: it must NOT equal a non-bold red (which is what the row tint alone would produce).
	rowTintD := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped), "D")
	if gotD == rowTintD {
		t.Error("verb 'D': cell color equals bare row tint (no bold); per-cell classifier must add bold on top")
	}

	// READ event: verb="R", row tint would be ct-read (yellow fg).
	// The verb glyph cell must be dim (ColDim), not yellow.
	gotR := views.ApplyCellColor("verb", "R")
	wantR := expectedRender(lipgloss.NewStyle().Foreground(styles.ColDim), "R")
	if gotR != wantR {
		t.Errorf("verb 'R' cell color = %q, want dim %q (must be independent of ct-read row tint)", gotR, wantR)
	}
	rowTintR := expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), "R")
	if gotR == rowTintR {
		t.Error("verb 'R': cell color equals ct-read row tint (yellow); per-cell classifier must apply dim instead")
	}

	// WRITE event: verb="W", row tint would be ct-write (red fg).
	// The verb glyph cell must be orange-bold, not red.
	gotW := views.ApplyCellColor("verb", "W")
	wantW := expectedRender(lipgloss.NewStyle().Foreground(styles.ColYAMLNum).Bold(true), "W")
	if gotW != wantW {
		t.Errorf("verb 'W' cell color = %q, want orange-bold %q (must differ from ct-write row tint red)", gotW, wantW)
	}
	rowTintW := expectedRender(lipgloss.NewStyle().Foreground(styles.ColStopped), "W")
	if gotW == rowTintW {
		t.Error("verb 'W': cell color equals ct-write row tint red; per-cell classifier must produce orange-bold instead")
	}

	// READ event: verb="I" (Insight), row tint would be ct-read (yellow fg).
	// The verb glyph cell must be purple-bold, not yellow.
	gotI := views.ApplyCellColor("verb", "I")
	wantI := expectedRender(lipgloss.NewStyle().Foreground(styles.ColYAMLBool).Bold(true), "I")
	if gotI != wantI {
		t.Errorf("verb 'I' cell color = %q, want purple-bold %q (must differ from ct-read row tint yellow)", gotI, wantI)
	}
	rowTintI := expectedRender(lipgloss.NewStyle().Foreground(styles.ColPending), "I")
	if gotI == rowTintI {
		t.Error("verb 'I': cell color equals ct-read row tint yellow; per-cell classifier must produce purple-bold instead")
	}
}

// Ensure ApplyCellColor does not mutate the input string.
func TestApplyCellColor_DoesNotMutateInput(t *testing.T) {
	original := "W"
	_ = views.ApplyCellColor("verb", original)
	if original != "W" {
		t.Error("ApplyCellColor mutated the input string")
	}
}

// Sentinel: verify the helper function uses the correct palette color values
// (cross-checks this test file against tui_styles_test.go palette verification).
func TestApplyCellColor_ColorTokenSanityCheck(t *testing.T) {
	checks := []struct {
		name string
		got  color.Color
		want string
	}{
		{"ColDim", styles.ColDim, "#565f89"},
		{"ColYAMLNum", styles.ColYAMLNum, "#ff9e64"},
		{"ColStopped", styles.ColStopped, "#f7768e"},
		{"ColAccent", styles.ColAccent, "#7aa2f7"},
		{"ColYAMLBool", styles.ColYAMLBool, "#bb9af7"},
		{"ColRunning", styles.ColRunning, "#9ece6a"},
		{"ColPending", styles.ColPending, "#e0af68"},
		{"ColHeaderFg", styles.ColHeaderFg, "#c0caf5"},
	}
	for _, c := range checks {
		want := lipgloss.Color(c.want)
		if !colorsEqual(c.got, want) {
			t.Errorf("palette sentinel %s: hex mismatch, expected %s", c.name, c.want)
		}
	}
}
