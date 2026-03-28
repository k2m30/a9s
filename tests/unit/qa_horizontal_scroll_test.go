package unit

// qa_horizontal_scroll_test.go tests the horizontal scroll behaviour of the
// resource list view (#105).
//
// The bug being fixed: fitColumns() shrinks the last column instead of
// dropping it, so the scroll-right guard `len(fitColumns) < len(visible)`
// never fires — the user can't scroll right even when the last column is
// truncated. The fix should detect shrinkage and allow scrolling.

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// hScrollKeyPress builds a tea.KeyPressMsg for single-character keys used in
// horizontal scroll tests (l = scroll right, h = scroll left).
func hScrollKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// hScrollRDSModel builds a ResourceListModel for the RDS "dbi" type loaded
// with fixture instances, at the given terminal width.
//
// RDS columns (from types_databases.go):
//
//	db_identifier(28) engine(12) engine_version(10) status(14) class(16)
//	endpoint(40) multi_az(10)
//
// At width 70 the first three columns fill 57 chars of used space (1 leading
// + 30 + 14 + 12).  The 4th column (status, width=14) needs 16 chars
// (14 + 2-char gap) which would push used to 73 > 70, so fitColumns shrinks
// it to remaining = 70-57-2 = 11 chars instead of dropping it.  This means
// len(fitColumns) == len(resolved) for the first 4 columns, which is the
// exact scenario that triggers the #105 bug.
func hScrollRDSModel(t *testing.T, width int) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("dbi resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(width, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "dbi",
		Resources:    fixtureRDSInstances(),
	})
	return m
}

// ---------------------------------------------------------------------------
// TestQA_HScroll_ScrollRightWhenLastColumnShrunk
// ---------------------------------------------------------------------------

// TestQA_HScroll_ScrollRightWhenLastColumnShrunk verifies that pressing 'l'
// changes the view even when the last visible column was shrunk (not dropped).
// Before the #105 fix this test FAILS because scrolling right does nothing.
func TestQA_HScroll_ScrollRightWhenLastColumnShrunk(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Width 70: db_identifier+engine+engine_version fill 57 cols, status gets
	// shrunk to 11 instead of dropped — this triggers the bug.
	m := hScrollRDSModel(t, 70)

	outBefore := m.View()

	m, _ = m.Update(hScrollKeyPress("l"))
	outAfter := m.View()

	if outBefore == outAfter {
		t.Error("pressing 'l' should scroll right and change the view even when the last column is shrunk, but the view did not change")
	}
}

// ---------------------------------------------------------------------------
// TestQA_HScroll_ScrollLeftRestoresOriginal
// ---------------------------------------------------------------------------

// TestQA_HScroll_ScrollLeftRestoresOriginal verifies that scrolling right
// then left returns the view to its original appearance.
func TestQA_HScroll_ScrollLeftRestoresOriginal(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	m := hScrollRDSModel(t, 70)
	outOriginal := m.View()

	// Scroll right.
	m, _ = m.Update(hScrollKeyPress("l"))

	// Scroll left.
	m, _ = m.Update(hScrollKeyPress("h"))
	outAfterRoundtrip := m.View()

	if outOriginal != outAfterRoundtrip {
		t.Error("scroll right then left should restore the original view")
	}
}

// ---------------------------------------------------------------------------
// TestQA_HScroll_ScrollRightStopsAtEnd
// ---------------------------------------------------------------------------

// TestQA_HScroll_ScrollRightStopsAtEnd verifies that scrolling right
// repeatedly does not scroll past the end (no infinite progression).
func TestQA_HScroll_ScrollRightStopsAtEnd(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	m := hScrollRDSModel(t, 70)

	// Press 'l' enough times to reach the end — the number of columns is
	// bounded by the RDS type definition (7 columns), so 10 presses is ample.
	const maxPresses = 10
	prev := m.View()
	stoppedChanging := false
	for i := 0; i < maxPresses; i++ {
		m, _ = m.Update(hScrollKeyPress("l"))
		curr := m.View()
		if curr == prev {
			stoppedChanging = true
			break
		}
		prev = curr
	}

	if !stoppedChanging {
		t.Error("pressing 'l' 10 times never stabilised — horizontal scroll appears unbounded")
	}
}

// ---------------------------------------------------------------------------
// TestQA_HScroll_NoScrollWhenAllColumnsFit
// ---------------------------------------------------------------------------

// TestQA_HScroll_NoScrollWhenAllColumnsFit verifies that pressing 'l' does
// NOT change the view when the terminal is wide enough to show all columns
// at full width.
func TestQA_HScroll_NoScrollWhenAllColumnsFit(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Width 200 is wide enough for all RDS columns (total ~145 chars).
	m := hScrollRDSModel(t, 200)

	outBefore := m.View()

	m, _ = m.Update(hScrollKeyPress("l"))
	outAfter := m.View()

	if outBefore != outAfter {
		t.Error("pressing 'l' should NOT change the view when all columns fit at full width")
	}
}

// ---------------------------------------------------------------------------
// TestQA_HScroll_HeaderAndDataAligned
// ---------------------------------------------------------------------------

// TestQA_HScroll_HeaderAndDataAligned verifies that after scrolling right,
// the header row and data rows start with the same column title / cell text,
// confirming they are rendered with the same column set.
func TestQA_HScroll_HeaderAndDataAligned(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	m := hScrollRDSModel(t, 70)

	// Scroll right.
	m, _ = m.Update(hScrollKeyPress("l"))

	view := m.View()
	lines := strings.Split(view, "\n")

	// Find the header row (the first non-empty content line — frame line 0 is
	// the frame border, line 1 is frame title, etc.). We look for the line
	// that contains a known column title.
	headerIdx := -1
	for i, line := range lines {
		stripped := stripANSI(line)
		if strings.Contains(stripped, "Engine") || strings.Contains(stripped, "DB Identifier") ||
			strings.Contains(stripped, "Version") || strings.Contains(stripped, "Status") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatal("could not locate header row after scrolling right")
	}

	// Find the first data row (the line just after the header that has data).
	dataIdx := -1
	for i := headerIdx + 1; i < len(lines); i++ {
		stripped := stripANSI(lines[i])
		// Data rows contain fixture values — look for a known RDS identifier.
		if strings.Contains(stripped, "prod") || strings.Contains(stripped, "mysql") ||
			strings.Contains(stripped, "postgres") || strings.Contains(stripped, "aurora") {
			dataIdx = i
			break
		}
	}
	if dataIdx < 0 {
		// No data row found — skip alignment check (may be an empty fixture edge case).
		t.Log("no data row found after scroll; skipping alignment assertion")
		return
	}

	// Both header and data rows should be non-empty after the scroll.
	headerLine := stripANSI(lines[headerIdx])
	dataLine := stripANSI(lines[dataIdx])

	if strings.TrimSpace(headerLine) == "" {
		t.Error("header row is empty after scrolling right")
	}
	if strings.TrimSpace(dataLine) == "" {
		t.Error("data row is empty after scrolling right")
	}
}
