// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestSort_HeaderArrowSurvivesTheColumnWidth renders every sortable column of
// every registered type as the active sort column and reads the arrow back off
// the header. A column whose declared width leaves no room for the arrow loses
// it to the ellipsis, and the header then says the list is unsorted while the
// rows underneath are in sorted order.
func TestSort_HeaderArrowSurvivesTheColumnWidth(t *testing.T) {
	ctrl := newTestController(t)
	checked := 0
	for _, name := range colsAllShortNames() {
		td := resource.FindResourceType(name)
		if td == nil {
			continue
		}
		cols := ctrl.ResolveColumnsForType(name)
		for i, c := range cols {
			// Only positions 0-9 carry the "N:" prefix and a sort binding.
			if i >= 10 {
				break
			}
			checked++
			header := sortHeaderLine(t, *td, cols, c.SortColKey())
			if !strings.Contains(header, c.Title+"↑") {
				t.Errorf("%s: sorting on column %d (%q) leaves no arrow in the header; the column is %d wide and the prefix, the title and the arrow need %d",
					name, i, c.Title, c.Width, len("N:")+len([]rune(c.Title))+1)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no sortable columns walked — the gate would pass vacuously")
	}
}

// sortHeaderLine renders a one-row list of td sorted ascending on sortKey and
// returns its header line.
func sortHeaderLine(t *testing.T, td resource.ResourceTypeDef, cols []app.ColumnDef, sortKey string) string {
	t.Helper()
	// Wide enough that fitColumns never shrinks a column: the subject there is
	// the declared width, not the terminal's.
	return sortHeaderLineAt(t, td, cols, sortKey, "asc", 4000)
}

// sortHeaderLineAtWidth is sortHeaderLine on an arbitrary terminal width, for
// the cases whose subject is what fitColumns does to a column.
func sortHeaderLineAtWidth(t *testing.T, cols []app.ColumnDef, sortKey, dir string, width int) string {
	t.Helper()
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 not registered")
	}
	return sortHeaderLineAt(t, *td, cols, sortKey, dir, width)
}

func sortHeaderLineAt(t *testing.T, td resource.ResourceTypeDef, cols []app.ColumnDef, sortKey, dir string, width int) string {
	t.Helper()
	m := views.NewResourceList(td, nil, keys.Default())
	m.SetSize(width, 40)
	body := app.ListBody{
		Columns: cols,
		Rows:    []app.ListRow{{Cells: make([]string, len(cols)), ResourceID: "r-1"}},
		Sort:    app.SortSpec{Col: sortKey, Dir: dir},
	}
	out := stripAnsi(m.RenderList(body))
	return strings.SplitN(out, "\n", 2)[0]
}

// TestSort_HeaderArrowSurvivesATerminalTooNarrowForTheTitle pins the second
// truncation path. fitColumns shrinks the last visible column to whatever the
// terminal has left over, without knowing or caring which column is sorted, so
// a sorted column landing there is cut below its own title. The arrow has to
// outlive that cut: the operator resized the window, they did not unsort the
// list.
func TestSort_HeaderArrowSurvivesATerminalTooNarrowForTheTitle(t *testing.T) {
	cols := []app.ColumnDef{
		{Key: "alpha", Title: "Alpha", Width: 8},
		{Key: "beta", Title: "Beta Gamma Delta", Width: 40},
	}
	// 30 columns fits "Alpha" whole and leaves the second column far short of
	// its 16-rune title, so fitColumns shrinks it and PadOrTrunc cuts it.
	header := sortHeaderLineAtWidth(t, cols, "beta", "asc", 30)
	if !strings.HasSuffix(strings.TrimRight(header, " "), "↑") {
		t.Errorf("a sorted column shrunk to fit a 30-column terminal renders %q — the arrow went with the truncated title, so the header says the list is unsorted while its rows are sorted",
			strings.TrimRight(header, " "))
	}
}

// TestSort_UnsortedHeaderIsUnchangedByTheArrowFit pins the other half: a list
// with no sort renders exactly the header it always did, arrow logic or not.
func TestSort_UnsortedHeaderIsUnchangedByTheArrowFit(t *testing.T) {
	cols := []app.ColumnDef{
		{Key: "alpha", Title: "Alpha", Width: 8},
		{Key: "beta", Title: "Beta Gamma Delta", Width: 40},
	}
	if got, want := sortHeaderLineAtWidth(t, cols, "", "asc", 30), " 1:Alpha   2:Beta Gamma Del…"; got != want {
		t.Errorf("unsorted header = %q, want %q", got, want)
	}
}

// TestSort_SortingAColumnDoesNotMoveTheHeader pins the invariant the
// arrow-after-fit design rests on: a header cell is exactly its column's width
// whether or not the arrow is in it. If sorting made a cell one wider, every
// column to its right would slide out from over its own rows.
func TestSort_SortingAColumnDoesNotMoveTheHeader(t *testing.T) {
	for _, width := range []int{1, 2, 3, 10, 20, 40} {
		cols := []app.ColumnDef{
			{Key: "beta", Title: "Beta", Width: width},
			{Key: "ceta", Title: "Ceta", Width: 6},
		}
		sorted := sortHeaderLineAtWidth(t, cols, "beta", "asc", 4000)
		unsorted := sortHeaderLineAtWidth(t, cols, "", "asc", 4000)
		if len([]rune(sorted)) != len([]rune(unsorted)) {
			t.Errorf("width %d: sorted header is %d columns and unsorted is %d — sorting shifts every column to the right of it\n sorted   %q\n unsorted %q",
				width, len([]rune(sorted)), len([]rune(unsorted)), sorted, unsorted)
		}
	}
}
