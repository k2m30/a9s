package unit

// Tests for the cursor-selection composition bug in internal/tui/views/resourcelist.go.
//
// Bug: styles.RowSelected.Width(m.width).Render(rowText) wraps a row string
// that already contains per-cell ANSI escape sequences from ApplyCellColor.
// Inner \x1b[0m reset sequences cancel the cursor background mid-row.
// The cursor background therefore only appears in the leading prefix and in the
// padding/whitespace after the last cell — it does NOT cover the cell text.
//
// Tests CS1-CS2 MUST FAIL against HEAD.
// Test CS3 (unselected rows have no cursor bg) MUST PASS (regression guard).
//
// Repair location: internal/tui/views/resourcelist.go:429-430.

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// helpers shared within this file
// ---------------------------------------------------------------------------

// cursorBgOpenSeq extracts the ANSI escape sequence that RowSelected opens
// with. Used to assert where the cursor background is applied.
func cursorBgOpenSeq() string {
	sentinel := "X"
	styled := styles.RowSelected.Width(40).Render(sentinel)
	idx := strings.Index(styled, sentinel)
	if idx <= 0 {
		return ""
	}
	return styled[:idx]
}

// buildCTCursorModel builds a loaded model with the given resources and
// cursor positioned at the given index. Uses a non-ct-events ShortName to
// avoid the default SortAge that would reorder rows.
func buildCTCursorModel(t *testing.T, resources []resource.Resource, cursorIdx int) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	// Use a custom ShortName that is NOT "ct-events" to avoid the automatic
	// SortAge default in NewResourceList, which would reverse row order.
	vc := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ct-events-cursor": {
				List: []config.ListColumn{
					{Title: "V", Key: "verb", Width: 2, Color: "verb"},
					{Title: "Actor", Key: "actor", Width: 16, Color: "actor"},
					{Title: "Event", Key: "event", Width: 24, Color: ""},
					{Title: "Outcome", Key: "outcome", Width: 16, Color: "outcome"},
				},
			},
		},
	}

	td := resource.ResourceTypeDef{
		Name:      "CloudTrail Events Cursor Test",
		ShortName: "ct-events-cursor",
	}

	k := keys.Default()
	m := views.NewResourceList(td, vc, k)
	m.SetSize(120, 10)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ct-events-cursor",
		Resources:    resources,
	})

	// Move cursor to the desired position using Down key presses.
	for i := 0; i < cursorIdx; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}

	return m
}

// findRowContaining returns the first row from rows that contains substr (ANSI-stripped).
// Returns "" if not found.
func findRowContaining(rows []string, substr string) string {
	for _, r := range rows {
		if strings.Contains(stripANSI(r), substr) {
			return r
		}
	}
	return ""
}

// splitCursorDataRows splits View() output into data rows (skips header).
func splitCursorDataRows(view string) []string {
	lines := strings.Split(view, "\n")
	if len(lines) <= 1 {
		return nil
	}
	return lines[1:]
}

// ---------------------------------------------------------------------------
// CS1: TestCursor_SelectedRow_BackgroundCoversAllCells
//
// The cursor background must cover the entire row width, including cells that
// contain per-cell ANSI colour sequences.
//
// Setup: cursor at row 0. The selected row has cells: verb=W (orange bold),
// actor=alice (plain), event=PutObject, outcome=OK (green).
//
// Assertion: the cursor bg ANSI sequence must appear MORE THAN ONCE in the
// selected row string (re-applied after each cell's ANSI reset, once the fix lands).
//
// With the current bug:
//   - RowSelected.Width(m.width).Render(rowText) puts the bg sequence once at the
//     start. Inner cell resets cancel it. The bg only covers the outer opening.
//   - The sequence appears only once (at the leading wrap), not across each cell.
// ---------------------------------------------------------------------------

func TestCursor_SelectedRow_BackgroundCoversAllCells(t *testing.T) {
	r0 := ctEventResource("ev0", "ct-write", "W", "alice", "PutObject", "OK")
	r1 := ctEventResource("ev1", "ct-read", "R", "bob", "GetObject", "OK")

	// Cursor at row 0.
	m := buildCTCursorModel(t, []resource.Resource{r0, r1}, 0)
	out := m.View()
	rows := splitCursorDataRows(out)
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 data rows, got %d; view=%q", len(rows), out)
	}

	// Find the selected row (row 0, cursor=0) by looking for "PutObject".
	selectedRow := findRowContaining(rows, "PutObject")
	if selectedRow == "" {
		t.Fatalf("could not find selected row containing 'PutObject'; rows=%q", rows)
	}
	unselectedRow := findRowContaining(rows, "GetObject")
	if unselectedRow == "" {
		t.Fatalf("could not find unselected row containing 'GetObject'; rows=%q", rows)
	}

	bgSeq := cursorBgOpenSeq()
	if bgSeq == "" {
		t.Fatal("cursorBgOpenSeq returned empty; check NO_COLOR env")
	}

	// Count how many times the cursor bg sequence appears in selected vs unselected.
	selectedBgCount := strings.Count(selectedRow, bgSeq)
	unselectedBgCount := strings.Count(unselectedRow, bgSeq)

	if unselectedBgCount > 0 {
		t.Logf("WARNING: unselected row contains cursor bg sequence (%d times): %q", unselectedBgCount, unselectedRow)
	}

	// After the fix: cursor bg appears multiple times in the selected row
	// (re-applied after each cell reset to cover all cells).
	// With the bug: appears only once (at the opening wrap).
	if selectedBgCount <= 1 {
		t.Errorf(
			"TestCursor_SelectedRow_BackgroundCoversAllCells FAIL (bug present):\n"+
				"  cursor bg sequence appears only %d time(s) in selected row.\n"+
				"  Expected > 1 (re-applied after each per-cell ANSI reset) to cover all cells.\n"+
				"  With the bug, RowSelected.Width.Render wraps a string with inner resets;\n"+
				"  inner resets cancel the background after the first cell, leaving later\n"+
				"  cells without cursor bg coverage.\n"+
				"  Bug location: resourcelist.go:429-430.\n"+
				"  bgSeq=%q\n"+
				"  selectedRow: %q",
			selectedBgCount, bgSeq, selectedRow,
		)
	}
}

// ---------------------------------------------------------------------------
// CS2: TestCursor_SelectedRow_PreservesCellColors
//
// On the cursor row, per-cell colour overrides (verb=D red bold, actor=ROOT red
// bold) must be SUPPRESSED so the cursor highlight foreground/bold wins.
// This mirrors ec2/list behavior — uniform readable text on cursor bg.
//
// Design intent: cursor row shows uniform styles.RowSelected styling across all
// cells; per-cell classifiers only apply to non-cursor rows.
// ---------------------------------------------------------------------------

func TestCursor_SelectedRow_PreservesCellColors(t *testing.T) {
	// verb=D (would be red bold off-cursor), actor=ROOT (would be red bold off-cursor).
	res := ctEventResource("ev0", "ct-write", "D", "ROOT", "DeleteBucket", "AccessDenied")
	extra := ctEventResource("ev1", "ct-read", "R", "alice", "GetObject", "OK")

	// Cursor at row 0 = res (D, ROOT, DeleteBucket).
	m := buildCTCursorModel(t, []resource.Resource{res, extra}, 0)
	out := m.View()
	rows := splitCursorDataRows(out)
	if len(rows) < 1 {
		t.Fatalf("expected at least 1 data row; view=%q", out)
	}

	// Find the selected row by looking for "DeleteBucket".
	selectedRow := findRowContaining(rows, "DeleteBucket")
	if selectedRow == "" {
		t.Fatalf("could not find selected row containing 'DeleteBucket'; rows=%q", rows)
	}

	// On the cursor row, per-cell overrides must be suppressed.
	// ColStopped is red: \x1b[1;38;2;247;118;142m (bold + ColStopped fg).
	// Build the standalone per-cell red-bold sequence that renderDataRow would
	// emit WITHOUT the isSelected guard — its absence proves suppression.
	standaloneRedBold := styles.RowSelected.Foreground(styles.ColStopped).Bold(true)
	// The per-cell override that would appear if isSelected were ignored is the
	// difference: a sequence opening with ColStopped fg applied on top of base.
	// We detect it by checking for the ColStopped ANSI color component.
	// ColStopped = #F7769E = rgb(247,118,158) → \x1b[38;2;247;118;158m
	// Build via rendering a single char and extracting the escape prefix.
	perCellSeq := strings.SplitN(standaloneRedBold.Render("X"), "X", 2)[0]

	// The standalone per-cell red-bold prefix must NOT appear in the selected row
	// (suppression is the fix; its presence means the guard is broken).
	// Note: the base cursor style alone may share some sub-sequences with
	// perCellSeq; use the full combined style prefix for specificity.
	if strings.Contains(selectedRow, perCellSeq) {
		t.Errorf(
			"TestCursor_SelectedRow_PreservesCellColors [per-cell override suppressed on cursor row]:\n"+
				"  per-cell red-bold override sequence found in cursor row — expected suppression.\n"+
				"  The cursor row should use only styles.RowSelected (uniform fg/bold),\n"+
				"  not per-cell ColStopped overrides.\n"+
				"  perCellSeq=%q\n"+
				"  selectedRow: %q",
			perCellSeq, selectedRow,
		)
	}

	// The cell text must still be present (content is not dropped).
	plainRow := stripANSI(selectedRow)
	if !strings.Contains(plainRow, "D ") {
		t.Errorf("TestCursor_SelectedRow_PreservesCellColors: verb 'D' text missing from cursor row; plainRow=%q", plainRow)
	}
	if !strings.Contains(plainRow, "ROOT") {
		t.Errorf("TestCursor_SelectedRow_PreservesCellColors: actor 'ROOT' text missing from cursor row; plainRow=%q", plainRow)
	}

	// The cursor bg must also be present (the selection is visible).
	bgSeq := cursorBgOpenSeq()
	if bgSeq != "" && !strings.Contains(selectedRow, bgSeq) {
		t.Errorf(
			"TestCursor_SelectedRow_PreservesCellColors [cursor bg]:\n"+
				"  cursor bg sequence not present in selected row at all.\n"+
				"  bgSeq=%q\n"+
				"  selectedRow: %q",
			bgSeq, selectedRow,
		)
	}
}

// ---------------------------------------------------------------------------
// CS3: TestCursor_UnselectedRows_NoBackground — REGRESSION GUARD (must PASS)
//
// Non-cursor rows must not contain the cursor background sequence.
// ---------------------------------------------------------------------------

func TestCursor_UnselectedRows_NoBackground(t *testing.T) {
	r0 := ctEventResource("ev0", "ct-write", "D", "bob.smith", "DeleteBucket", "OK")
	r1 := ctEventResource("ev1", "ct-read", "R", "alice", "GetObject", "OK")
	r2 := ctEventResource("ev2", "ct-read", "W", "svc", "PutBucketPolicy", "OK")

	// Cursor at row 0.
	m := buildCTCursorModel(t, []resource.Resource{r0, r1, r2}, 0)
	out := m.View()
	rows := splitCursorDataRows(out)
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 data rows, got %d; view=%q", len(rows), out)
	}

	bgSeq := cursorBgOpenSeq()
	if bgSeq == "" {
		t.Fatal("cursorBgOpenSeq returned empty; check NO_COLOR env")
	}

	// Rows 1 and 2 are not selected — must not have cursor bg.
	for i, row := range rows[1:] {
		if strings.Contains(row, bgSeq) {
			t.Errorf(
				"TestCursor_UnselectedRows_NoBackground: row[%d] contains cursor bg sequence.\n"+
					"  bgSeq=%q\n"+
					"  row: %q",
				i+1, bgSeq, row,
			)
		}
	}
}
