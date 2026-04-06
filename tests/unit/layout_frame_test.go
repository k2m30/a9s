package unit

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ── PadOrTrunc tests ─────────────────────────────────────────────────────────

func TestPadOrTrunc_PadsShortString(t *testing.T) {
	got := text.PadOrTrunc("hello", 10)
	if lipgloss.Width(got) != 10 {
		t.Errorf("expected visible width 10, got %d", lipgloss.Width(got))
	}
	if got != "hello     " {
		t.Errorf("expected %q, got %q", "hello     ", got)
	}
}

func TestPadOrTrunc_ExactWidth(t *testing.T) {
	got := text.PadOrTrunc("hello", 5)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestPadOrTrunc_TruncatesLongString(t *testing.T) {
	got := text.PadOrTrunc("hello world", 6)
	vis := lipgloss.Width(got)
	if vis != 6 {
		t.Errorf("expected visible width 6, got %d for %q", vis, got)
	}
	// Should end with ellipsis character
	if !strings.HasSuffix(got, "\u2026") {
		t.Errorf("expected truncated string to end with ellipsis, got %q", got)
	}
}

func TestPadOrTrunc_WidthZero(t *testing.T) {
	got := text.PadOrTrunc("hello", 0)
	if got != "" {
		t.Errorf("expected empty string for w=0, got %q", got)
	}
}

func TestPadOrTrunc_WidthNegative(t *testing.T) {
	got := text.PadOrTrunc("hello", -1)
	if got != "" {
		t.Errorf("expected empty string for w=-1, got %q", got)
	}
}

func TestPadOrTrunc_WidthOne(t *testing.T) {
	got := text.PadOrTrunc("hello", 1)
	vis := lipgloss.Width(got)
	if vis != 1 {
		t.Errorf("expected visible width 1, got %d for %q", vis, got)
	}
}

func TestPadOrTrunc_EmptyString(t *testing.T) {
	got := text.PadOrTrunc("", 5)
	if lipgloss.Width(got) != 5 {
		t.Errorf("expected visible width 5, got %d", lipgloss.Width(got))
	}
	if got != "     " {
		t.Errorf("expected 5 spaces, got %q", got)
	}
}

func TestPadOrTrunc_ANSIStyled(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(styles.ColAccent).Render("hello")
	got := text.PadOrTrunc(styled, 10)
	vis := lipgloss.Width(got)
	if vis != 10 {
		t.Errorf("expected visible width 10, got %d", vis)
	}
}

func TestPadOrTrunc_UnicodeArrow(t *testing.T) {
	// Sort indicator arrows are multi-byte UTF-8 but single display column.
	// PadOrTrunc must produce correct visible width, not byte length.
	got := text.PadOrTrunc("Name\u2191", 10) // "Name↑" = 5 display cols
	vis := lipgloss.Width(got)
	if vis != 10 {
		t.Errorf("expected visible width 10 for %q, got %d (len=%d)", got, vis, len(got))
	}
}

func TestPadOrTrunc_UnicodeExactWidth(t *testing.T) {
	got := text.PadOrTrunc("Name\u2193", 5) // "Name↓" = 5 display cols, exact fit
	vis := lipgloss.Width(got)
	if vis != 5 {
		t.Errorf("expected visible width 5 for %q, got %d", got, vis)
	}
}

func TestPadOrTrunc_UnicodeTruncate(t *testing.T) {
	got := text.PadOrTrunc("LongName\u2191", 6) // "LongName↑" = 9 display cols, truncate to 6
	vis := lipgloss.Width(got)
	if vis != 6 {
		t.Errorf("expected visible width 6 for %q, got %d", got, vis)
	}
}

// TestPadOrTrunc_NewlinesStripped verifies that embedded newlines in the input
// are replaced with spaces. Log messages often contain \n (stack traces,
// multi-line output). If newlines leak through, a single "row" renders as
// multiple terminal lines, breaking scroll behavior.
func TestPadOrTrunc_NewlinesStripped(t *testing.T) {
	got := text.PadOrTrunc("line1\nline2\nline3", 20)
	if strings.Contains(got, "\n") {
		t.Errorf("PadOrTrunc output must not contain newlines, got: %q", got)
	}
	vis := lipgloss.Width(got)
	if vis != 20 {
		t.Errorf("expected visible width 20, got %d", vis)
	}
}

func TestPadOrTrunc_NewlinesInLongString(t *testing.T) {
	// A message with embedded newlines that exceeds the column width
	got := text.PadOrTrunc("ERROR: something failed\nTraceback:\n  File app.py, line 42", 30)
	if strings.Contains(got, "\n") {
		t.Errorf("PadOrTrunc output must not contain newlines, got: %q", got)
	}
}

func TestPadOrTrunc_CarriageReturnStripped(t *testing.T) {
	got := text.PadOrTrunc("line1\r\nline2", 20)
	if strings.Contains(got, "\r") || strings.Contains(got, "\n") {
		t.Errorf("PadOrTrunc output must not contain CR/LF, got: %q", got)
	}
}

// ── CenterTitle tests ────────────────────────────────────────────────────────

func TestLayoutCenterTitle_EmptyTitle(t *testing.T) {
	got := layout.CenterTitle("", 20)
	vis := lipgloss.Width(got)
	if vis != 20 {
		t.Errorf("expected visible width 20, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.HasPrefix(plain, "\u250c") {
		t.Errorf("expected to start with top-left corner, got prefix %q", plain[:4])
	}
	if !strings.HasSuffix(plain, "\u2510") {
		t.Errorf("expected to end with top-right corner, got suffix %q", plain[len(plain)-3:])
	}
	// Should be all dashes between corners
	inner := plain[len("\u250c") : len(plain)-len("\u2510")]
	for _, r := range inner {
		if r != '\u2500' {
			t.Errorf("expected all dashes for empty title, found %q", string(r))
			break
		}
	}
}

func TestLayoutCenterTitle_WithTitle(t *testing.T) {
	got := layout.CenterTitle("test", 20)
	vis := lipgloss.Width(got)
	if vis != 20 {
		t.Errorf("expected visible width 20, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, " test ") {
		t.Errorf("expected title surrounded by spaces in plain text, got %q", plain)
	}
	if !strings.HasPrefix(plain, "\u250c") {
		t.Errorf("expected to start with top-left corner")
	}
	if !strings.HasSuffix(plain, "\u2510") {
		t.Errorf("expected to end with top-right corner")
	}
}

func TestLayoutCenterTitle_TitleTooLong(t *testing.T) {
	got := layout.CenterTitle("very long title text", 20)
	vis := lipgloss.Width(got)
	if vis != 20 {
		t.Errorf("expected visible width 20, got %d", vis)
	}
}

func TestLayoutCenterTitle_MinimalWidth(t *testing.T) {
	got := layout.CenterTitle("x", 6)
	vis := lipgloss.Width(got)
	if vis != 6 {
		t.Errorf("expected visible width 6, got %d", vis)
	}
}

// ── RenderFrame tests ────────────────────────────────────────────────────────

func TestLayoutRenderFrame_BasicBox(t *testing.T) {
	lines := []string{"hello", "world"}
	got := layout.RenderFrame(lines, "test", 20, 6)
	outLines := strings.Split(got, "\n")

	// Should have h lines total: top border + content + padding + bottom border
	if len(outLines) != 6 {
		t.Errorf("expected 6 lines, got %d", len(outLines))
	}

	// Each line should be 20 visible columns wide
	for i, line := range outLines {
		vis := lipgloss.Width(line)
		if vis != 20 {
			t.Errorf("line %d: expected visible width 20, got %d: %q", i, vis, line)
		}
	}
}

func TestLayoutRenderFrame_EmptyTitle(t *testing.T) {
	lines := []string{"content"}
	got := layout.RenderFrame(lines, "", 20, 5)
	outLines := strings.Split(got, "\n")

	topPlain := stripANSI(outLines[0])
	if !strings.HasPrefix(topPlain, "\u250c") {
		t.Errorf("expected top border to start with corner, got %q", topPlain)
	}
}

func TestLayoutRenderFrame_ContentPaddedToInnerWidth(t *testing.T) {
	lines := []string{"hi"}
	got := layout.RenderFrame(lines, "", 20, 4)
	outLines := strings.Split(got, "\n")

	if len(outLines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(outLines))
	}
	contentLine := outLines[1]
	vis := lipgloss.Width(contentLine)
	if vis != 20 {
		t.Errorf("content line: expected visible width 20, got %d", vis)
	}
}

func TestLayoutRenderFrame_PadsShortContent(t *testing.T) {
	lines := []string{"line1"}
	got := layout.RenderFrame(lines, "", 20, 6)
	outLines := strings.Split(got, "\n")

	// h=6 means: top(1) + content(4) + bottom(1) = 6 lines total
	if len(outLines) != 6 {
		t.Errorf("expected 6 lines (1 top + 4 content + 1 bottom), got %d", len(outLines))
	}
}

func TestLayoutRenderFrame_BorderChars(t *testing.T) {
	lines := []string{"x"}
	got := layout.RenderFrame(lines, "", 10, 4)
	outLines := strings.Split(got, "\n")

	topPlain := stripANSI(outLines[0])
	if !strings.HasPrefix(topPlain, "\u250c") {
		t.Errorf("top border should start with corner char")
	}
	if !strings.HasSuffix(topPlain, "\u2510") {
		t.Errorf("top border should end with corner char")
	}

	bottomPlain := stripANSI(outLines[len(outLines)-1])
	if !strings.HasPrefix(bottomPlain, "\u2514") {
		t.Errorf("bottom border should start with corner char")
	}
	if !strings.HasSuffix(bottomPlain, "\u2518") {
		t.Errorf("bottom border should end with corner char")
	}

	for i := 1; i < len(outLines)-1; i++ {
		plain := stripANSI(outLines[i])
		if !strings.HasPrefix(plain, "\u2502") {
			t.Errorf("line %d should start with side border, got %q", i, plain[:4])
		}
		if !strings.HasSuffix(plain, "\u2502") {
			t.Errorf("line %d should end with side border", i)
		}
	}
}

func TestLayoutRenderFrame_WidthConsistency(t *testing.T) {
	lines := []string{"short", "a longer content line here"}
	got := layout.RenderFrame(lines, "My Title", 40, 8)
	outLines := strings.Split(got, "\n")

	for i, line := range outLines {
		vis := lipgloss.Width(line)
		if vis != 40 {
			t.Errorf("line %d: expected width 40, got %d", i, vis)
		}
	}
}

// ── RenderHeader tests ───────────────────────────────────────────────────────

func TestLayoutRenderHeader_ContainsAppName(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help", "", "")
	plain := stripANSI(got)
	if !strings.Contains(plain, "a9s") {
		t.Error("header should contain 'a9s'")
	}
}

func TestLayoutRenderHeader_ContainsVersion(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help", "", "")
	plain := stripANSI(got)
	if !strings.Contains(plain, "v0.5.0") {
		t.Errorf("header should contain 'v0.5.0', got %q", plain)
	}
}

func TestLayoutRenderHeader_ContainsProfileRegion(t *testing.T) {
	got := layout.RenderHeader("prod", "us-west-2", "0.5.0", 80, "? for help", "", "")
	plain := stripANSI(got)
	if !strings.Contains(plain, "prod:us-west-2") {
		t.Errorf("header should contain 'prod:us-west-2', got %q", plain)
	}
}

func TestLayoutRenderHeader_ContainsRightContent(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help", "", "")
	plain := stripANSI(got)
	if !strings.Contains(plain, "? for help") {
		t.Errorf("header should contain '? for help', got %q", plain)
	}
}

func TestLayoutRenderHeader_RightContentAligned(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help", "", "")
	vis := lipgloss.Width(got)
	if vis != 80 {
		t.Errorf("header should be exactly 80 columns wide, got %d", vis)
	}
}

func TestLayoutRenderHeader_CustomRightContent(t *testing.T) {
	got := layout.RenderHeader("prod", "us-east-1", "0.5.0", 80, "Copied!", "", "")
	plain := stripANSI(got)
	if !strings.Contains(plain, "Copied!") {
		t.Errorf("header should contain custom right content 'Copied!', got %q", plain)
	}
}

func TestLayoutRenderHeader_NarrowWidth(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 40, "? for help", "", "")
	vis := lipgloss.Width(got)
	if vis != 40 {
		t.Errorf("narrow header should be exactly 40 columns wide, got %d", vis)
	}
}

func TestLayoutRenderHeader_LeftRightSeparation(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 120, "? for help", "", "")
	plain := stripANSI(got)

	leftIdx := strings.Index(plain, "default:us-east-1")
	rightIdx := strings.Index(plain, "? for help")
	if leftIdx < 0 || rightIdx < 0 {
		t.Fatalf("expected both left and right content in header, got %q", plain)
	}
	if rightIdx <= leftIdx {
		t.Errorf("right content should appear after left content")
	}
}

// ── RenderFramePrepadded tests ────────────────────────────────────────────

func TestLayoutRenderFramePrepadded_BasicBox(t *testing.T) {
	// Pre-pad lines to innerW = 20-2 = 18
	innerW := 18
	line1 := "hello" + strings.Repeat(" ", innerW-5)
	line2 := "world" + strings.Repeat(" ", innerW-5)
	lines := []string{line1, line2}
	got := layout.RenderFramePrepadded(lines, "test", 20, 6)
	outLines := strings.Split(got, "\n")

	if len(outLines) != 6 {
		t.Errorf("expected 6 lines, got %d", len(outLines))
	}

	for i, line := range outLines {
		vis := lipgloss.Width(line)
		if vis != 20 {
			t.Errorf("line %d: expected visible width 20, got %d: %q", i, vis, line)
		}
	}
}

func TestLayoutRenderFramePrepadded_MatchesRenderFrame(t *testing.T) {
	// Pre-pad content to innerW = 40-2 = 38
	innerW := 38
	rawLines := []string{"short", "a longer content line here"}
	paddedLines := make([]string, len(rawLines))
	for i, line := range rawLines {
		visW := lipgloss.Width(line)
		if visW < innerW {
			paddedLines[i] = line + strings.Repeat(" ", innerW-visW)
		} else {
			paddedLines[i] = line
		}
	}

	got1 := layout.RenderFrame(rawLines, "My Title", 40, 8)
	got2 := layout.RenderFramePrepadded(paddedLines, "My Title", 40, 8)

	if got1 != got2 {
		t.Errorf("RenderFramePrepadded should produce identical output to RenderFrame when content is pre-padded")
	}
}

func TestLayoutRenderFramePrepadded_EmptyLines(t *testing.T) {
	got := layout.RenderFramePrepadded(nil, "test", 20, 5)
	outLines := strings.Split(got, "\n")

	if len(outLines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(outLines))
	}

	for i, line := range outLines {
		vis := lipgloss.Width(line)
		if vis != 20 {
			t.Errorf("line %d: expected visible width 20, got %d", i, vis)
		}
	}
}

func TestLayoutRenderFramePrepadded_BorderChars(t *testing.T) {
	innerW := 8
	line := "x" + strings.Repeat(" ", innerW-1)
	got := layout.RenderFramePrepadded([]string{line}, "", 10, 4)
	outLines := strings.Split(got, "\n")

	topPlain := stripANSI(outLines[0])
	if !strings.HasPrefix(topPlain, "\u250c") {
		t.Errorf("top border should start with corner char")
	}
	if !strings.HasSuffix(topPlain, "\u2510") {
		t.Errorf("top border should end with corner char")
	}

	bottomPlain := stripANSI(outLines[len(outLines)-1])
	if !strings.HasPrefix(bottomPlain, "\u2514") {
		t.Errorf("bottom border should start with corner char")
	}
	if !strings.HasSuffix(bottomPlain, "\u2518") {
		t.Errorf("bottom border should end with corner char")
	}

	for i := 1; i < len(outLines)-1; i++ {
		plain := stripANSI(outLines[i])
		if !strings.HasPrefix(plain, "\u2502") {
			t.Errorf("line %d should start with side border", i)
		}
		if !strings.HasSuffix(plain, "\u2502") {
			t.Errorf("line %d should end with side border", i)
		}
	}
}

// ── BottomBorderWithHints tests ──────────────────────────────────────────────

// TestBottomBorderWithHints_EmptyHints verifies that nil hints produces a plain
// bottom border with correct corners and width.
func TestBottomBorderWithHints_EmptyHints(t *testing.T) {
	got := layout.BottomBorderWithHints(nil, 40)
	vis := lipgloss.Width(got)
	if vis != 40 {
		t.Errorf("expected visual width 40, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.HasPrefix(plain, "\u2514") {
		t.Errorf("expected to start with bottom-left corner '\u2514', got %q", plain[:4])
	}
	if !strings.HasSuffix(plain, "\u2518") {
		t.Errorf("expected to end with bottom-right corner '\u2518', got %q", plain[len(plain)-3:])
	}
	// No hint text — plain border only
	if strings.Contains(plain, "YAML") || strings.Contains(plain, "Detail") {
		t.Errorf("expected no hint text for nil hints, got %q", plain)
	}
}

// TestBottomBorderWithHints_SingleHint verifies that a single hint renders key
// and description text inside the border at the correct width.
func TestBottomBorderWithHints_SingleHint(t *testing.T) {
	hints := []layout.KeyHint{{Key: "y", Desc: "YAML"}}
	got := layout.BottomBorderWithHints(hints, 40)
	vis := lipgloss.Width(got)
	if vis != 40 {
		t.Errorf("expected visual width 40, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.HasPrefix(plain, "\u2514") {
		t.Errorf("expected to start with '\u2514', got %q", plain[:4])
	}
	if !strings.HasSuffix(plain, "\u2518") {
		t.Errorf("expected to end with '\u2518', got %q", plain[len(plain)-3:])
	}
	if !strings.Contains(plain, "y") {
		t.Errorf("expected hint key 'y' in plain text, got %q", plain)
	}
	if !strings.Contains(plain, "YAML") {
		t.Errorf("expected hint description 'YAML' in plain text, got %q", plain)
	}
}

// TestBottomBorderWithHints_MultipleHints verifies that multiple hints render
// in order (d before y) with both keys and descriptions present.
func TestBottomBorderWithHints_MultipleHints(t *testing.T) {
	hints := []layout.KeyHint{
		{Key: "d", Desc: "Detail"},
		{Key: "y", Desc: "YAML"},
	}
	got := layout.BottomBorderWithHints(hints, 60)
	vis := lipgloss.Width(got)
	if vis != 60 {
		t.Errorf("expected visual width 60, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.Contains(plain, "d") {
		t.Errorf("expected hint key 'd' in plain text, got %q", plain)
	}
	if !strings.Contains(plain, "Detail") {
		t.Errorf("expected hint description 'Detail' in plain text, got %q", plain)
	}
	if !strings.Contains(plain, "y") {
		t.Errorf("expected hint key 'y' in plain text, got %q", plain)
	}
	if !strings.Contains(plain, "YAML") {
		t.Errorf("expected hint description 'YAML' in plain text, got %q", plain)
	}
	// Verify order: "d" (Detail) must appear before "y" (YAML)
	idxD := strings.Index(plain, "Detail")
	idxY := strings.Index(plain, "YAML")
	if idxD < 0 || idxY < 0 {
		t.Fatalf("expected both 'Detail' and 'YAML' in output, got %q", plain)
	}
	if idxD >= idxY {
		t.Errorf("expected 'Detail' (idx %d) to appear before 'YAML' (idx %d)", idxD, idxY)
	}
}

// TestBottomBorderWithHints_Truncation verifies that when 5 hints exceed the
// available width, earlier hints are present but later ones are dropped.
func TestBottomBorderWithHints_Truncation(t *testing.T) {
	hints := []layout.KeyHint{
		{Key: "a", Desc: "Alpha"},
		{Key: "b", Desc: "Beta"},
		{Key: "c", Desc: "Gamma"},
		{Key: "d", Desc: "Delta"},
		{Key: "e", Desc: "Epsilon"},
	}
	got := layout.BottomBorderWithHints(hints, 30)
	vis := lipgloss.Width(got)
	if vis != 30 {
		t.Errorf("expected visual width 30, got %d", vis)
	}
	plain := stripANSI(got)
	// Width is 30 — not all 5 hints can fit. At least the first should be present.
	if !strings.Contains(plain, "Alpha") {
		t.Errorf("expected first hint 'Alpha' to be present at w=30, got %q", plain)
	}
	// Not all 5 hints should fit — at least one must be dropped.
	allPresent := strings.Contains(plain, "Alpha") &&
		strings.Contains(plain, "Beta") &&
		strings.Contains(plain, "Gamma") &&
		strings.Contains(plain, "Delta") &&
		strings.Contains(plain, "Epsilon")
	if allPresent {
		t.Errorf("expected some hints to be dropped at w=30 with 5 hints, but all are present: %q", plain)
	}
}

// TestBottomBorderWithHints_VeryNarrow verifies that when the hint doesn't fit
// at a very narrow width, a plain border is returned.
//
// With right-aligned layout the overhead is 4 chars (1 for └, 3 for ──┘).
// "y YAML" renders as 6 visible chars, so minimum fitting width is 10.
// At w=9 the hint is dropped and only a plain border is produced.
func TestBottomBorderWithHints_VeryNarrow(t *testing.T) {
	hints := []layout.KeyHint{{Key: "y", Desc: "YAML"}}
	got := layout.BottomBorderWithHints(hints, 9)
	vis := lipgloss.Width(got)
	if vis != 9 {
		t.Errorf("expected visual width 9, got %d", vis)
	}
	plain := stripANSI(got)
	if !strings.HasPrefix(plain, "\u2514") {
		t.Errorf("expected to start with '\u2514', got %q", plain[:4])
	}
	if !strings.HasSuffix(plain, "\u2518") {
		t.Errorf("expected to end with '\u2518', got %q", plain[len(plain)-3:])
	}
	// At w=9, "y YAML" (6 chars) + overhead 4 (└ + ──┘) = 10 > 9 — hint must be dropped.
	if strings.Contains(plain, "YAML") {
		t.Errorf("expected hint 'YAML' to be dropped at w=9, but found it in %q", plain)
	}
}

// TestBottomBorderWithHints_CornerInvariant verifies that all inputs produce
// output starting with '└', ending with '┘', and matching the requested width.
func TestBottomBorderWithHints_CornerInvariant(t *testing.T) {
	cases := []struct {
		name  string
		hints []layout.KeyHint
		w     int
	}{
		{"nil hints w=40", nil, 40},
		{"empty hints w=40", []layout.KeyHint{}, 40},
		{"single hint w=40", []layout.KeyHint{{Key: "y", Desc: "YAML"}}, 40},
		{"single hint w=60", []layout.KeyHint{{Key: "y", Desc: "YAML"}}, 60},
		{"single hint narrow w=10", []layout.KeyHint{{Key: "y", Desc: "YAML"}}, 10},
		{"5 hints w=30", []layout.KeyHint{
			{Key: "a", Desc: "Alpha"},
			{Key: "b", Desc: "Beta"},
			{Key: "c", Desc: "Gamma"},
			{Key: "d", Desc: "Delta"},
			{Key: "e", Desc: "Epsilon"},
		}, 30},
		{"5 hints w=120", []layout.KeyHint{
			{Key: "esc", Desc: "Back"},
			{Key: "enter", Desc: "Objects"},
			{Key: "d", Desc: "Detail"},
			{Key: "y", Desc: "YAML"},
			{Key: "ctrl+r", Desc: "Refresh"},
		}, 120},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := layout.BottomBorderWithHints(tc.hints, tc.w)
			vis := lipgloss.Width(got)
			if vis != tc.w {
				t.Errorf("expected visual width %d, got %d", tc.w, vis)
			}
			plain := stripANSI(got)
			if !strings.HasPrefix(plain, "\u2514") {
				t.Errorf("expected to start with '\u2514', got prefix %q", plain[:4])
			}
			if !strings.HasSuffix(plain, "\u2518") {
				t.Errorf("expected to end with '\u2518', got suffix %q", plain[len(plain)-3:])
			}
		})
	}
}

// TestBottomBorderWithHints_WidthExact verifies that BottomBorderWithHints
// always returns exactly w visual columns regardless of hint content.
func TestBottomBorderWithHints_WidthExact(t *testing.T) {
	widths := []int{20, 30, 40, 60, 80, 120, 200}
	hintSets := [][]layout.KeyHint{
		nil,
		{{Key: "y", Desc: "YAML"}},
		{{Key: "d", Desc: "Detail"}, {Key: "y", Desc: "YAML"}},
		{
			{Key: "esc", Desc: "Back"},
			{Key: "enter", Desc: "Objects"},
			{Key: "d", Desc: "Detail"},
			{Key: "y", Desc: "YAML"},
			{Key: "ctrl+r", Desc: "Refresh"},
		},
	}
	for _, w := range widths {
		for _, hints := range hintSets {
			got := layout.BottomBorderWithHints(hints, w)
			vis := lipgloss.Width(got)
			if vis != w {
				t.Errorf("w=%d hints=%d: expected visual width %d, got %d", w, len(hints), w, vis)
			}
		}
	}
}

// TestBottomBorderWithHints_HintOrder verifies that hints appear left-to-right
// in the exact order they are supplied.
func TestBottomBorderWithHints_HintOrder(t *testing.T) {
	hints := []layout.KeyHint{
		{Key: "esc", Desc: "Back"},
		{Key: "enter", Desc: "Objects"},
		{Key: "d", Desc: "Detail"},
	}
	got := layout.BottomBorderWithHints(hints, 80)
	plain := stripANSI(got)

	idxBack := strings.Index(plain, "Back")
	idxObjects := strings.Index(plain, "Objects")
	idxDetail := strings.Index(plain, "Detail")

	if idxBack < 0 || idxObjects < 0 || idxDetail < 0 {
		t.Fatalf("expected all hints present at w=80, got %q", plain)
	}
	if idxBack >= idxObjects {
		t.Errorf("expected 'Back' (idx %d) before 'Objects' (idx %d)", idxBack, idxObjects)
	}
	if idxObjects >= idxDetail {
		t.Errorf("expected 'Objects' (idx %d) before 'Detail' (idx %d)", idxObjects, idxDetail)
	}
}

// ── RenderFrameWithHints tests ───────────────────────────────────────────────

// TestRenderFrameWithHints_NilHints verifies that nil hints produces output
// identical to layout.RenderFrame.
func TestRenderFrameWithHints_NilHints(t *testing.T) {
	lines := []string{"hello", "world"}
	want := layout.RenderFrame(lines, "title", 40, 8)
	got := layout.RenderFrameWithHints(lines, "title", nil, 40, 8)
	if got != want {
		t.Errorf("RenderFrameWithHints(nil hints) must equal RenderFrame output.\nwant: %q\ngot:  %q", want, got)
	}
}

// TestRenderFrameWithHints_EmptyHints verifies that an empty hint slice produces
// output identical to layout.RenderFrame.
func TestRenderFrameWithHints_EmptyHints(t *testing.T) {
	lines := []string{"hello", "world"}
	want := layout.RenderFrame(lines, "title", 40, 8)
	got := layout.RenderFrameWithHints(lines, "title", []layout.KeyHint{}, 40, 8)
	if got != want {
		t.Errorf("RenderFrameWithHints(empty hints) must equal RenderFrame output.\nwant: %q\ngot:  %q", want, got)
	}
}

// TestRenderFrameWithHints_WithHints verifies that hints appear in the bottom
// border while the top border and content rows are unchanged.
func TestRenderFrameWithHints_WithHints(t *testing.T) {
	lines := []string{"content row 1", "content row 2"}
	hints := []layout.KeyHint{{Key: "y", Desc: "YAML"}}
	got := layout.RenderFrameWithHints(lines, "My Title", hints, 40, 6)
	outLines := strings.Split(got, "\n")

	if len(outLines) != 6 {
		t.Errorf("expected 6 lines (h=6), got %d", len(outLines))
	}

	// Top border must still carry the title
	topPlain := stripANSI(outLines[0])
	if !strings.Contains(topPlain, "My Title") {
		t.Errorf("expected title 'My Title' in top border, got %q", topPlain)
	}
	if !strings.HasPrefix(topPlain, "\u250c") {
		t.Errorf("top border must start with '\u250c'")
	}

	// Bottom border must contain hint text
	bottomPlain := stripANSI(outLines[len(outLines)-1])
	if !strings.Contains(bottomPlain, "YAML") {
		t.Errorf("expected hint 'YAML' in bottom border, got %q", bottomPlain)
	}
	if !strings.HasPrefix(bottomPlain, "\u2514") {
		t.Errorf("bottom border must start with '\u2514'")
	}
	if !strings.HasSuffix(bottomPlain, "\u2518") {
		t.Errorf("bottom border must end with '\u2518'")
	}

	// Content rows (lines 1..h-2) must be framed with side borders
	for i := 1; i < len(outLines)-1; i++ {
		plain := stripANSI(outLines[i])
		if !strings.HasPrefix(plain, "\u2502") {
			t.Errorf("content line %d must start with '\u2502', got %q", i, plain)
		}
		if !strings.HasSuffix(plain, "\u2502") {
			t.Errorf("content line %d must end with '\u2502', got %q", i, plain)
		}
	}
}

// TestRenderFrameWithHints_WidthConsistency verifies that every line in the
// output has visual width exactly equal to w.
func TestRenderFrameWithHints_WidthConsistency(t *testing.T) {
	cases := []struct {
		name  string
		hints []layout.KeyHint
		w     int
		h     int
	}{
		{
			name:  "no hints w=40",
			hints: nil,
			w:     40,
			h:     8,
		},
		{
			name:  "single hint w=40",
			hints: []layout.KeyHint{{Key: "y", Desc: "YAML"}},
			w:     40,
			h:     8,
		},
		{
			name: "multiple hints w=80",
			hints: []layout.KeyHint{
				{Key: "esc", Desc: "Back"},
				{Key: "enter", Desc: "Objects"},
				{Key: "d", Desc: "Detail"},
				{Key: "y", Desc: "YAML"},
				{Key: "ctrl+r", Desc: "Refresh"},
			},
			w: 80,
			h: 10,
		},
		{
			name: "many hints narrow w=30",
			hints: []layout.KeyHint{
				{Key: "a", Desc: "Alpha"},
				{Key: "b", Desc: "Beta"},
				{Key: "c", Desc: "Gamma"},
			},
			w: 30,
			h: 6,
		},
	}
	lines := []string{"short line", "another line", "third"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := layout.RenderFrameWithHints(lines, "Title", tc.hints, tc.w, tc.h)
			outLines := strings.Split(got, "\n")
			for i, line := range outLines {
				vis := lipgloss.Width(line)
				if vis != tc.w {
					t.Errorf("line %d: expected visual width %d, got %d (line=%q)", i, tc.w, vis, line)
				}
			}
		})
	}
}
