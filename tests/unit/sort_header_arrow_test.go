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
	m := views.NewResourceList(td, nil, keys.Default())
	// Wide enough that fitColumns never shrinks a column: the subject is the
	// declared width, not the terminal's.
	m.SetSize(4000, 40)
	body := app.ListBody{
		Columns: cols,
		Rows:    []app.ListRow{{Cells: make([]string, len(cols)), ResourceID: "r-1"}},
		Sort:    app.SortSpec{Col: sortKey, Dir: "asc"},
	}
	out := stripAnsi(m.RenderList(body))
	return strings.SplitN(out, "\n", 2)[0]
}
