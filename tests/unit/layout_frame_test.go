package unit

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/internal/tui/layout"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// ── PadOrTrunc tests ─────────────────────────────────────────────────────────

func TestPadOrTrunc_PadsShortString(t *testing.T) {
	got := layout.PadOrTrunc("hello", 10)
	if lipgloss.Width(got) != 10 {
		t.Errorf("expected visible width 10, got %d", lipgloss.Width(got))
	}
	if got != "hello     " {
		t.Errorf("expected %q, got %q", "hello     ", got)
	}
}

func TestPadOrTrunc_ExactWidth(t *testing.T) {
	got := layout.PadOrTrunc("hello", 5)
	if got != "hello" {
		t.Errorf("expected %q, got %q", "hello", got)
	}
}

func TestPadOrTrunc_TruncatesLongString(t *testing.T) {
	got := layout.PadOrTrunc("hello world", 6)
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
	got := layout.PadOrTrunc("hello", 0)
	if got != "" {
		t.Errorf("expected empty string for w=0, got %q", got)
	}
}

func TestPadOrTrunc_WidthNegative(t *testing.T) {
	got := layout.PadOrTrunc("hello", -1)
	if got != "" {
		t.Errorf("expected empty string for w=-1, got %q", got)
	}
}

func TestPadOrTrunc_WidthOne(t *testing.T) {
	got := layout.PadOrTrunc("hello", 1)
	vis := lipgloss.Width(got)
	if vis != 1 {
		t.Errorf("expected visible width 1, got %d for %q", vis, got)
	}
}

func TestPadOrTrunc_EmptyString(t *testing.T) {
	got := layout.PadOrTrunc("", 5)
	if lipgloss.Width(got) != 5 {
		t.Errorf("expected visible width 5, got %d", lipgloss.Width(got))
	}
	if got != "     " {
		t.Errorf("expected 5 spaces, got %q", got)
	}
}

func TestPadOrTrunc_ANSIStyled(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(styles.ColAccent).Render("hello")
	got := layout.PadOrTrunc(styled, 10)
	vis := lipgloss.Width(got)
	if vis != 10 {
		t.Errorf("expected visible width 10, got %d", vis)
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
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help")
	plain := stripANSI(got)
	if !strings.Contains(plain, "a9s") {
		t.Error("header should contain 'a9s'")
	}
}

func TestLayoutRenderHeader_ContainsVersion(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help")
	plain := stripANSI(got)
	if !strings.Contains(plain, "v0.5.0") {
		t.Errorf("header should contain 'v0.5.0', got %q", plain)
	}
}

func TestLayoutRenderHeader_ContainsProfileRegion(t *testing.T) {
	got := layout.RenderHeader("prod", "us-west-2", "0.5.0", 80, "? for help")
	plain := stripANSI(got)
	if !strings.Contains(plain, "prod:us-west-2") {
		t.Errorf("header should contain 'prod:us-west-2', got %q", plain)
	}
}

func TestLayoutRenderHeader_ContainsRightContent(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help")
	plain := stripANSI(got)
	if !strings.Contains(plain, "? for help") {
		t.Errorf("header should contain '? for help', got %q", plain)
	}
}

func TestLayoutRenderHeader_RightContentAligned(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 80, "? for help")
	vis := lipgloss.Width(got)
	if vis != 80 {
		t.Errorf("header should be exactly 80 columns wide, got %d", vis)
	}
}

func TestLayoutRenderHeader_CustomRightContent(t *testing.T) {
	got := layout.RenderHeader("prod", "us-east-1", "0.5.0", 80, "Copied!")
	plain := stripANSI(got)
	if !strings.Contains(plain, "Copied!") {
		t.Errorf("header should contain custom right content 'Copied!', got %q", plain)
	}
}

func TestLayoutRenderHeader_NarrowWidth(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 40, "? for help")
	vis := lipgloss.Width(got)
	if vis != 40 {
		t.Errorf("narrow header should be exactly 40 columns wide, got %d", vis)
	}
}

func TestLayoutRenderHeader_LeftRightSeparation(t *testing.T) {
	got := layout.RenderHeader("default", "us-east-1", "0.5.0", 120, "? for help")
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
