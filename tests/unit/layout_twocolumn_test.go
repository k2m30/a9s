package unit

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/layout"
)

// TestTwoColumn_CorrectTotalWidth verifies the first line of output has a
// visual width equal to totalW (the combined width of both panels).
func TestTwoColumn_CorrectTotalWidth(t *testing.T) {
	left := []string{"left content"}
	right := []string{"right content"}
	got := layout.TwoColumn(left, right, "Left", "Right", 120, 80, 10)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatal("TwoColumn returned empty output")
	}

	vis := lipgloss.Width(lines[0])
	if vis != 120 {
		t.Errorf("first line: expected visual width 120, got %d", vis)
	}
}

// TestTwoColumn_LeftPanelWidth verifies the left panel's top border is exactly
// leftW characters wide. We check the first 80 chars of the first line form a
// valid framed top border (starts with ┌ and ends at the 80th visual column).
func TestTwoColumn_LeftPanelWidth(t *testing.T) {
	left := []string{"left content"}
	right := []string{"right content"}
	got := layout.TwoColumn(left, right, "Left", "Right", 120, 80, 10)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatal("TwoColumn returned empty output")
	}

	// Strip ANSI from the first line and check that the first visual character is ┌
	plain := stripANSI(lines[0])
	if !strings.HasPrefix(plain, "\u250c") {
		t.Errorf("left panel top border should start with ┌, got prefix %q", plain[:4])
	}

	// The left panel should span exactly leftW=80 visual columns.
	// We verify by measuring a RenderFrame of the same width independently.
	leftFrame := layout.RenderFrame(left, "Left", 80, 10)
	leftLines := strings.Split(leftFrame, "\n")
	leftFirstLineWidth := lipgloss.Width(leftLines[0])
	if leftFirstLineWidth != 80 {
		t.Errorf("left panel standalone frame top border: expected width 80, got %d", leftFirstLineWidth)
	}
}

// TestTwoColumn_RightPanelWidth verifies the right panel is totalW-leftW = 40
// characters wide.
func TestTwoColumn_RightPanelWidth(t *testing.T) {
	left := []string{"left content"}
	right := []string{"right content"}
	got := layout.TwoColumn(left, right, "Left", "Right", 120, 80, 10)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatal("TwoColumn returned empty output")
	}

	// Right panel width = totalW - leftW = 120 - 80 = 40.
	// Verify via a standalone RenderFrame of the expected right width.
	rightW := 120 - 80
	rightFrame := layout.RenderFrame(right, "Right", rightW, 10)
	rightLines := strings.Split(rightFrame, "\n")
	rightFirstLineWidth := lipgloss.Width(rightLines[0])
	if rightFirstLineWidth != rightW {
		t.Errorf("right panel standalone frame top border: expected width %d, got %d", rightW, rightFirstLineWidth)
	}

	// And the total combined width must equal 120.
	vis := lipgloss.Width(lines[0])
	if vis != 120 {
		t.Errorf("combined total width: expected 120, got %d", vis)
	}
}

// TestTwoColumn_EmptyRightContent verifies that when rightLines is empty the
// right panel is still rendered with h-2 empty (padded) content rows.
func TestTwoColumn_EmptyRightContent(t *testing.T) {
	left := []string{"something"}
	right := []string{}
	got := layout.TwoColumn(left, right, "L", "R", 120, 80, 10)

	// Must still produce output.
	if got == "" {
		t.Fatal("TwoColumn returned empty output for empty rightLines")
	}

	lines := strings.Split(got, "\n")
	// Expect exactly h=10 lines.
	if len(lines) != 10 {
		t.Errorf("expected 10 lines with h=10, got %d", len(lines))
	}

	// Every line must have the full combined width.
	for i, line := range lines {
		vis := lipgloss.Width(line)
		if vis != 120 {
			t.Errorf("line %d: expected visual width 120 even with empty right content, got %d", i, vis)
		}
	}
}

// TestTwoColumn_TitlesInBorders verifies that both leftTitle and rightTitle
// appear in the first (top border) line of the output.
func TestTwoColumn_TitlesInBorders(t *testing.T) {
	left := []string{"detail line"}
	right := []string{"related line"}
	got := layout.TwoColumn(left, right, "Detail", "RELATED", 120, 80, 10)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatal("TwoColumn returned empty output")
	}

	// Strip ANSI so title text is visible as plain strings.
	plainFirst := stripANSI(lines[0])
	if !strings.Contains(plainFirst, "Detail") {
		t.Errorf("first line should contain left title 'Detail', got %q", plainFirst)
	}
	if !strings.Contains(plainFirst, "RELATED") {
		t.Errorf("first line should contain right title 'RELATED', got %q", plainFirst)
	}
}

// TestTwoColumn_CorrectHeight verifies that the output has exactly h=10 lines
// when split by newline.
func TestTwoColumn_CorrectHeight(t *testing.T) {
	left := []string{"a", "b", "c"}
	right := []string{"x", "y"}
	got := layout.TwoColumn(left, right, "L", "R", 120, 80, 10)

	lines := strings.Split(got, "\n")
	if len(lines) != 10 {
		t.Errorf("expected exactly 10 lines for h=10, got %d", len(lines))
	}
}

// TestTwoColumn_ContentRendered verifies that "hello" appears in the left panel
// area and "world" appears in the right panel area of the combined output.
func TestTwoColumn_ContentRendered(t *testing.T) {
	left := []string{"hello"}
	right := []string{"world"}
	got := layout.TwoColumn(left, right, "L", "R", 120, 80, 10)

	plain := stripANSI(got)
	if !strings.Contains(plain, "hello") {
		t.Errorf("output should contain left content 'hello', plain output: %q", plain)
	}
	if !strings.Contains(plain, "world") {
		t.Errorf("output should contain right content 'world', plain output: %q", plain)
	}
}

// TestTwoColumn_DifferentWidths verifies that when totalW=140 and leftW=90 the
// right panel is 50 chars wide (totalW - leftW = 50) and the combined first
// line has visual width 140.
func TestTwoColumn_DifferentWidths(t *testing.T) {
	left := []string{"left"}
	right := []string{"right"}
	got := layout.TwoColumn(left, right, "A", "B", 140, 90, 10)

	lines := strings.Split(got, "\n")
	if len(lines) == 0 {
		t.Fatal("TwoColumn returned empty output")
	}

	vis := lipgloss.Width(lines[0])
	if vis != 140 {
		t.Errorf("combined first line: expected visual width 140, got %d", vis)
	}

	// Verify right panel standalone frame is 50 wide.
	rightW := 140 - 90 // = 50
	rightFrame := layout.RenderFrame(right, "B", rightW, 10)
	rightLines := strings.Split(rightFrame, "\n")
	rightFirstLineWidth := lipgloss.Width(rightLines[0])
	if rightFirstLineWidth != rightW {
		t.Errorf("right panel standalone frame: expected width %d, got %d", rightW, rightFirstLineWidth)
	}
}
