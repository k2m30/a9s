// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// A header column reserves and paints its title in the same measure, display
// columns. A rune-counted reservation fails two ways for a wide-rune title: a
// short one overflows it and shifts every column to its right off the rows
// below, and a longer one is truncated inside it, taking the sort arrow with
// it.
package unit

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// secondColumnTitle and secondCellValue name the column to the right of the
// wide-rune one, whose left edge is where the reservation and the render can
// be seen to disagree.
const (
	secondColumnTitle = "2:Second"
	secondCellValue   = "second-cell"
)

// wideTitleHeaderAndRow renders a two-column, one-row list whose first column
// carries title and is the sort column, and returns the header line and the
// data line. The terminal is far wider than the columns, so nothing but the
// column's own reservation decides how much room the title gets.
func wideTitleHeaderAndRow(t *testing.T, title string) (string, string) {
	t.Helper()
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 is not registered")
	}
	m := views.NewResourceList(*td, nil, keys.Default())
	m.SetSize(4000, 40)
	out := m.RenderList(app.ListBody{
		Columns: []app.ColumnDef{
			{Key: "name", Title: title, Width: 1},
			{Key: "second", Title: "Second", Width: len(secondCellValue)},
		},
		Rows: []app.ListRow{{Cells: []string{"row-1", secondCellValue}, ResourceID: "r-1"}},
		Sort: app.SortSpec{Col: "name", Dir: "asc"},
		// No status column and no marker column: the widen pass would
		// otherwise grow the first column past its declared width and hide
		// what the reservation alone does with the title.
		StatusCol:   -1,
		IdentityCol: -1,
	})
	lines := strings.SplitN(stripANSI(out), "\n", 3)
	if len(lines) < 2 {
		t.Fatalf("expected a header line and a data line, got %q", out)
	}
	return lines[0], lines[1]
}

// columnOffset returns the display column at which needle starts in line.
func columnOffset(t *testing.T, line, needle string) int {
	t.Helper()
	i := strings.Index(line, needle)
	if i < 0 {
		t.Fatalf("%q is not in %q", needle, line)
	}
	return lipgloss.Width(line[:i])
}

// TestListHeader_WideRuneTitleReservesTheColumnsItIsPaintedInto sweeps the
// two lengths a rune-counted reservation fails at: one short enough to
// overflow the reservation, one long enough to be truncated inside it.
func TestListHeader_WideRuneTitleReservesTheColumnsItIsPaintedInto(t *testing.T) {
	for _, title := range []string{"実行", "実行状態名前"} {
		t.Run(title, func(t *testing.T) {
			if lipgloss.Width(title) == len([]rune(title)) {
				t.Fatalf("fixture no longer carries a wide rune: %q measures %d columns over %d runes",
					title, lipgloss.Width(title), len([]rune(title)))
			}
			header, row := wideTitleHeaderAndRow(t, title)

			if !strings.Contains(header, title+"↑") {
				t.Errorf(
					"the header truncated a title it had just reserved room for: %q\n"+
						"want the whole title and its sort arrow — the reservation must measure "+
						"display columns, the same measure the cell is padded and truncated to.",
					header,
				)
			}
			if got, want := columnOffset(t, header, secondColumnTitle), columnOffset(t, row, secondCellValue); got != want {
				t.Errorf(
					"the second column's header starts at display column %d and its cells at %d, so the table is skewed:\n"+
						"  header: %q\n"+
						"  row:    %q\n"+
						"a header cell wider than the column it was reserved pushes every column right of it off its rows.",
					got, want, header, row,
				)
			}
		})
	}
}
