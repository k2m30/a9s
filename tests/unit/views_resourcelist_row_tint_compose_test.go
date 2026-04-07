package unit

// Tests for the row-tint composition bug in internal/tui/views/resourcelist.go.
//
// Bug: renderDataRow embeds per-cell ANSI escape sequences (via ApplyCellColor).
// When the outer RowColorStyle("ct-write").Render(rowText) wraps the joined cell
// string, the inner \x1b[0m (reset) sequences cancel the outer foreground
// mid-row. Only the text before the first inner reset carries the row tint colour.
//
// Symptom: on a ct-write row, the verb glyph is red (from the verb cell classifier),
// but all subsequent cells revert to default colour instead of staying ct-write red.
// On a ct-read row, cells after the verb glyph lose the yellow ct-read tint.
//
// Tests RT1-RT3 MUST FAIL against HEAD (the bug is not yet fixed).
// Test RT4 (non-ct status) MUST PASS (regression guard).
//
// Repair location: internal/tui/views/resourcelist.go:429-433 (styled row composition).

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// ctViewConfig returns a ViewsConfig with ct-events-rt columns that carry
// per-cell colour classifiers, matching the production YAML.
// Uses ShortName "ct-events-rt" (not "ct-events") to avoid the automatic
// SortAge default in NewResourceList, which would reverse row order and
// break positional assertions.
func ctViewConfig() *config.ViewsConfig {
	return &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ct-events-rt": {
				List: []config.ListColumn{
					{Title: "V", Key: "verb", Width: 2, Color: "verb"},
					{Title: "Actor", Key: "actor", Width: 16, Color: "actor"},
					{Title: "Event", Key: "event", Width: 24, Color: ""},
					{Title: "Outcome", Key: "outcome", Width: 16, Color: "outcome"},
				},
			},
		},
	}
}

// ctEventResource builds a synthetic ct-events resource with given status and field values.
func ctEventResource(id, status, verb, actor, event, outcome string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   event,
		Status: status,
		Fields: map[string]string{
			"verb":    verb,
			"actor":   actor,
			"event":   event,
			"outcome": outcome,
		},
	}
}

// ctTypeDef returns a minimal ResourceTypeDef for ct-events-rt.
// Uses ShortName "ct-events-rt" (not "ct-events") to avoid the automatic
// SortAge default in NewResourceList, which would reverse row order and
// break positional assertions.
func ctTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "CloudTrail Events",
		ShortName: "ct-events-rt",
	}
}

// buildCTModelAtCursor creates a loaded ResourceListModel with cursor moved to cursorIdx.
// The cursor is moved using Down key presses so the row at cursorIdx gets RowSelected
// and all other rows get RowColorStyle treatment.
func buildCTModelAtCursor(t *testing.T, resources []resource.Resource, cursorIdx int) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := ctTypeDef()
	vc := ctViewConfig()
	k := keys.Default()
	m := views.NewResourceList(td, vc, k)
	m.SetSize(120, 10)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ct-events-rt",
		Resources:    resources,
	})
	// Move cursor to cursorIdx using Down key presses.
	for i := 0; i < cursorIdx; i++ {
		m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	return m
}

// ctDataRows splits View() output into individual data rows (skips the header line).
// View() format: "<header>\n<row0>\n<row1>..."
func ctDataRows(view string) []string {
	lines := strings.Split(view, "\n")
	if len(lines) <= 1 {
		return nil
	}
	return lines[1:]
}

// rowTintOpenSeq returns the ANSI opening escape sequence that RowColorStyle
// applies for the given status. We derive it by rendering a sentinel character
// and extracting everything up to the sentinel itself.
func rowTintOpenSeq(status string) string {
	sentinel := "X"
	styled := styles.RowColorStyle(status).Render(sentinel)
	idx := strings.Index(styled, sentinel)
	if idx <= 0 {
		return ""
	}
	return styled[:idx]
}

// countANSISegmentsWithSeq counts how many times the given ANSI opening sequence
// appears in s.
func countANSISegmentsWithSeq(s, seq string) int {
	if seq == "" {
		return 0
	}
	return strings.Count(s, seq)
}

// ---------------------------------------------------------------------------
// RT1: TestRowTint_CtWrite_AppliesToAllCells
//
// For a ct-write row the row-tint red fg sequence must appear across ALL cells,
// not just the first one. The current bug: inner ANSI resets from per-cell
// classifiers break the outer Render(), so only the prefix before the first
// reset stays red.
//
// Setup: 3 resources — [0]=ct-write(D), [1]=neutral(cursor), [2]=ct-read(R)
// Cursor is placed at row 1 so rows 0 and 2 are rendered with RowColorStyle.
// We compare the ct-write row tint coverage in row 0 vs row 2 (ct-read).
// ---------------------------------------------------------------------------

func TestRowTint_CtWrite_AppliesToAllCells(t *testing.T) {
	writeRes := ctEventResource("ev-write", "ct-write", "D", "bob.smith", "DeleteBucket", "AccessDenied")
	neutralRes := ctEventResource("ev-neutral", "ct-read", "R", "svc", "GetObject", "OK")
	readRes := ctEventResource("ev-read", "ct-read", "R", "alice", "ListObjects", "OK")

	// Cursor at row 1 (neutral) so row 0 (ct-write) and row 2 (ct-read) are tinted.
	m := buildCTModelAtCursor(t, []resource.Resource{writeRes, neutralRes, readRes}, 1)
	out := m.View()
	rows := ctDataRows(out)
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 data rows, got %d; view=%q", len(rows), out)
	}

	writeRow := rows[0] // ct-write — rendered with RowColorStyle("ct-write")
	readRow := rows[2]  // ct-read  — rendered with RowColorStyle("ct-read")

	writeSeq := rowTintOpenSeq("ct-write")
	if writeSeq == "" {
		t.Fatal("rowTintOpenSeq returned empty for ct-write; check NO_COLOR env")
	}

	// Count how many times the ct-write red fg sequence appears in each row.
	writeTintCount := countANSISegmentsWithSeq(writeRow, writeSeq)
	readTintCount := countANSISegmentsWithSeq(readRow, writeSeq)

	// After the fix: ct-write row should carry the red sequence on all cells,
	// meaning the sequence appears MORE than once (re-applied after each cell reset).
	// With the current bug: the outer Render puts the sequence exactly once at the
	// start, and inner cell resets cancel it. Additionally, the verb cell classifier
	// itself is red — so the ct-write sequence may appear 0 additional times past
	// the opening, while readRow picks up the ct-write sequence 0 times.
	//
	// Concrete assertion: the non-verb, non-classified cells (Event="DeleteBucket",
	// Outcome="AccessDenied") must carry the ct-write fg tint. We verify by checking
	// that the ct-write sequence appears at least once AFTER the text "D " (the verb
	// cell padded to width 2) within the writeRow.
	verbPadded := "D " // verb column width=2: PadOrTrunc("D", 2) = "D "
	verbIdx := strings.Index(writeRow, verbPadded)
	if verbIdx < 0 {
		t.Fatalf("verb text %q not found in write row; row=%q", verbPadded, writeRow)
	}
	postVerbSection := writeRow[verbIdx+len(verbPadded):]
	tintAfterVerb := countANSISegmentsWithSeq(postVerbSection, writeSeq)
	_ = readTintCount // used for context only

	if tintAfterVerb == 0 {
		t.Errorf(
			"TestRowTint_CtWrite_AppliesToAllCells FAIL (bug present):\n"+
				"  ct-write row tint sequence (%q) does not appear AFTER the verb cell.\n"+
				"  Expected the row tint to be re-applied after each per-cell reset,\n"+
				"  so non-verb cells (Event, Outcome) carry the red fg tint.\n"+
				"  With the bug: inner cell ANSI resets cancel the outer RowColorStyle\n"+
				"  foreground; cells after the verb revert to default colour.\n"+
				"  Bug location: resourcelist.go:432.\n"+
				"  writeTintCount (total)=%d, tintAfterVerb=%d\n"+
				"  writeRow: %q\n"+
				"  post-verb section: %q",
			writeSeq, writeTintCount, tintAfterVerb, writeRow, postVerbSection,
		)
	}
}

// ---------------------------------------------------------------------------
// RT2: TestRowTint_CtRead_AppliesToAllCells
//
// Same assertion for ct-read rows: the yellow fg tint must appear on ALL cells,
// not just the leading portion before the first cell reset.
// ---------------------------------------------------------------------------

func TestRowTint_CtRead_AppliesToAllCells(t *testing.T) {
	writeRes := ctEventResource("ev-write", "ct-write", "D", "bob", "DeleteBucket", "OK")
	neutralRes := ctEventResource("ev-neutral", "ct-write", "W", "svc", "PutObject", "OK")
	readRes := ctEventResource("ev-read", "ct-read", "R", "alice", "GetObject", "OK")

	// Cursor at row 1 (neutral) so row 0 (ct-write) and row 2 (ct-read) are tinted.
	m := buildCTModelAtCursor(t, []resource.Resource{writeRes, neutralRes, readRes}, 1)
	out := m.View()
	rows := ctDataRows(out)
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 data rows, got %d; view=%q", len(rows), out)
	}

	readRow := rows[2] // ct-read — rendered with RowColorStyle("ct-read")

	readSeq := rowTintOpenSeq("ct-read")
	if readSeq == "" {
		t.Fatal("rowTintOpenSeq returned empty for ct-read; check NO_COLOR env")
	}

	// The verb "R" classifier uses ColDim, not the ct-read yellow.
	// So the ct-read yellow tint should appear on the non-verb cells.
	// Check that the ct-read sequence appears AFTER the verb cell text.
	verbPadded := "R " // verb column width=2
	verbIdx := strings.Index(readRow, verbPadded)
	if verbIdx < 0 {
		t.Fatalf("verb text %q not found in read row; row=%q", verbPadded, readRow)
	}
	postVerbSection := readRow[verbIdx+len(verbPadded):]
	tintAfterVerb := countANSISegmentsWithSeq(postVerbSection, readSeq)

	if tintAfterVerb == 0 {
		t.Errorf(
			"TestRowTint_CtRead_AppliesToAllCells FAIL (bug present):\n"+
				"  ct-read row tint sequence (%q) does not appear AFTER the verb cell.\n"+
				"  Expected the yellow tint to cover non-verb cells (Actor, Event, Outcome).\n"+
				"  With the bug: inner resets from the verb cell classifier cancel the\n"+
				"  outer RowColorStyle foreground; subsequent cells revert to default.\n"+
				"  Bug location: resourcelist.go:432.\n"+
				"  tintAfterVerb=%d\n"+
				"  readRow: %q\n"+
				"  post-verb section: %q",
			readSeq, tintAfterVerb, readRow, postVerbSection,
		)
	}
}

// ---------------------------------------------------------------------------
// RT3: TestRowTint_CellColorAndRowTintCoexist
//
// Per design docs/design/ct-event-list.md:298-300:
//   - verb "D"   → red bold  (cell classifier, ColStopped.Bold)
//   - actor ROOT → red bold  (cell classifier, ColStopped.Bold)
//   - Row status="ct-write" → red fg tint on the whole row
//
// This test verifies all three coexist on the same row:
//   1. verb "D" cell carries red bold escape (per-cell colour present)
//   2. actor ROOT cell carries red bold escape (per-cell colour present)
//   3. The non-classified "Event" cell text "DeleteBucket" is preceded by the
//      ct-write row-tint sequence (row tint active on that cell)
//
// Cursor is placed on a different row so this row gets RowColorStyle treatment.
// ---------------------------------------------------------------------------

func TestRowTint_CellColorAndRowTintCoexist(t *testing.T) {
	writeRes := ctEventResource("ev-write", "ct-write", "D", "ROOT", "DeleteBucket", "AccessDenied")
	otherRes := ctEventResource("ev-other", "ct-read", "R", "alice", "GetObject", "OK")

	// Cursor on row 1 so row 0 (ct-write) is rendered with RowColorStyle, not RowSelected.
	m := buildCTModelAtCursor(t, []resource.Resource{writeRes, otherRes}, 1)
	out := m.View()
	rows := ctDataRows(out)
	if len(rows) < 1 {
		t.Fatalf("expected at least 1 data row; view=%q", out)
	}

	row := rows[0]

	// 1. Verb "D" must appear with red bold styling.
	// The verb column is width=2, so the padded value is "D ". The cell is styled as
	// ApplyCellColor("verb", "D "). After the CP1 fix (trim before classify), the
	// classifier sees "D" and applies ColStopped.Bold to "D " (the padded value).
	// We verify the red-bold ANSI escape prefix appears in the row.
	redBoldPrefix := lipgloss.NewStyle().Foreground(styles.ColStopped).Bold(true).Render("X")
	redBoldOpen := strings.SplitN(redBoldPrefix, "X", 2)[0]
	if !strings.Contains(row, redBoldOpen) {
		t.Errorf(
			"TestRowTint_CellColorAndRowTintCoexist [verb D cell]:\n"+
				"  row does not contain red-bold escape prefix (%q).\n"+
				"  Expected verb 'D' and/or actor 'ROOT' to be red-bold styled.\n"+
				"  Bug: per-cell classifier colour may not be applied (CP1 pad bug also needed).\n"+
				"  row: %q",
			redBoldOpen, row,
		)
	}

	// 2. Actor ROOT must appear with red bold styling.
	// Same red-bold prefix check (ColStopped.Bold) — already covered above.
	// Additionally check that "ROOT" text is present.
	if !strings.Contains(stripANSI(row), "ROOT") {
		t.Errorf(
			"TestRowTint_CellColorAndRowTintCoexist [actor ROOT text]:\n"+
				"  'ROOT' not found (plain text) in row. row=%q",
			row,
		)
	}

	// 3. "DeleteBucket" (the unclassified Event cell) must be preceded by the
	//    row-tint red fg sequence — meaning the tint is still active at that cell.
	rowTintSeq := rowTintOpenSeq("ct-write")
	deleteBucketPos := strings.Index(row, "DeleteBucket")
	if deleteBucketPos < 0 {
		t.Errorf("TestRowTint_CellColorAndRowTintCoexist: 'DeleteBucket' not found in row; row=%q", row)
	} else {
		prefix := row[:deleteBucketPos]
		tintBeforeEvent := strings.Count(prefix, rowTintSeq)
		if tintBeforeEvent == 0 {
			t.Errorf(
				"TestRowTint_CellColorAndRowTintCoexist [Event cell row tint]:\n"+
					"  Row tint sequence (%q) does not appear before 'DeleteBucket' text.\n"+
					"  Expected: after the fix, the row tint is re-applied after each per-cell\n"+
					"  reset, so 'DeleteBucket' (an unclassified cell) carries the ct-write\n"+
					"  red fg tint.\n"+
					"  Bug: inner ANSI resets from verb/actor cells cancel the outer\n"+
					"  RowColorStyle foreground before the Event cell is reached.\n"+
					"  row prefix up to DeleteBucket: %q",
				rowTintSeq, prefix,
			)
		}
	}
}

// ---------------------------------------------------------------------------
// RT4: TestRowTint_NonCTStatusUnchanged — REGRESSION GUARD (must PASS)
//
// A resource with Status="running" renders with ColRunning green fg, not with
// the ct-write or ct-read tints. This guard ensures we don't break normal rows.
// ---------------------------------------------------------------------------

func TestRowTint_NonCTStatusUnchanged(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Columns: []resource.Column{
			{Key: "state", Title: "State", Width: 12},
			{Key: "name", Title: "Name", Width: 20},
		},
	}

	res := resource.Resource{
		ID:     "i-123",
		Name:   "web-server",
		Status: "running",
		Fields: map[string]string{
			"state": "running",
			"name":  "web-server",
		},
	}
	extra := resource.Resource{
		ID:     "i-456",
		Name:   "db-server",
		Status: "stopped",
		Fields: map[string]string{
			"state": "stopped",
			"name":  "db-server",
		},
	}

	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 10)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{res, extra},
	})
	// Move cursor to row 1 so row 0 gets RowColorStyle("running") not RowSelected.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	out := m.View()
	rows := ctDataRows(out)
	if len(rows) < 1 {
		t.Fatalf("expected at least 1 data row; view=%q", out)
	}

	row := rows[0] // running resource

	// Must contain the "running" text.
	if !strings.Contains(stripANSI(row), "running") {
		t.Errorf("regression guard: 'running' not found in row; row=%q", row)
	}

	// Must NOT contain ct-write red fg sequence.
	writeSeq := rowTintOpenSeq("ct-write")
	if strings.Contains(row, writeSeq) {
		t.Errorf("regression guard: ct-write row tint unexpectedly present in running row; row=%q", row)
	}

	// Must NOT contain ct-read yellow fg sequence.
	readSeq := rowTintOpenSeq("ct-read")
	if strings.Contains(row, readSeq) {
		t.Errorf("regression guard: ct-read row tint unexpectedly present in running row; row=%q", row)
	}
}
