// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// The header's sort indicators. applySortKeyPrefixWidths reserves room for
// whichever is wider, so the width reservation and the glyph that has to fit
// in it can never drift apart.
const (
	sortAscGlyph  = "\u2191"
	sortDescGlyph = "\u2193"
)

// listCol is a resolved column definition for rendering.
type listCol struct {
	title string
	width int
	key   string // resource.Fields key (fallback)
	path  string // config-driven path for ExtractScalar
	// sortKey is app.ColumnDef.SortColKey carried verbatim from the column the
	// controller resolved, so the header arrow lands on the column the
	// controller is actually sorting by. Never re-derived here.
	sortKey string
}

// applySortKeyPrefixWidths auto-grows the first 10 columns' widths to fit the
// "N:Title↑" header colHeaderTitle produces. Without this, columns declared
// narrower would truncate the header (e.g. "5:Instanc…" at width=10) and hide
// the sort hint. The architectural rule: any sortable column (positions 0-9)
// MUST reserve enough room for its prefix and for the arrow — a column wide
// enough for the title alone loses the arrow to the ellipsis and reads as
// unsorted while its rows are in sorted order. The rune is reserved on every
// sortable column, not only the sorted one, so the header does not shift
// sideways under the operator when they press a sort key. Columns beyond
// position 9 have no prefix, no sort binding and keep their declared width.
func applySortKeyPrefixWidths(cols []listCol) []listCol {
	for i := range cols {
		if i >= 10 {
			break
		}
		displayNum := i + 1
		if displayNum == 10 {
			displayNum = 0
		}
		prefix := fmt.Sprintf("%d:", displayNum)
		glyph := max(len([]rune(sortAscGlyph)), len([]rune(sortDescGlyph)))
		minWidth := len([]rune(prefix)) + len([]rune(cols[i].title)) + glyph
		if cols[i].width < minWidth {
			cols[i].width = minWidth
		}
	}
	return cols
}

// fitColumns hides rightmost columns that don't fit in the available width.
// If a column doesn't fit at full width but there's enough remaining space
// (at least 10 chars), it's included with a reduced width instead of dropped.
func (m ResourceListModel) fitColumns(cols []listCol) []listCol {
	if m.width <= 0 {
		return cols
	}
	const minColWidth = 10
	usedWidth := 1 // leading space
	var fit []listCol
	for _, c := range cols {
		needed := c.width + 2 // column width + 2-space gap
		if usedWidth+needed > m.width && len(fit) > 0 {
			// Column doesn't fit at full width. Try shrinking it.
			remaining := m.width - usedWidth - 2 // available minus gap
			if remaining >= minColWidth {
				shrunk := c
				shrunk.width = remaining
				fit = append(fit, shrunk)
			}
			break
		}
		usedWidth += needed
		fit = append(fit, c)
	}
	return fit
}

// renderHeaderRow renders the column header line with sort indicators.
// Uses m.hScrollOffset to compute the absolute column index for position numbering.
func (m ResourceListModel) renderHeaderRow(cols []listCol) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		absIdx := i + m.hScrollOffset
		title := m.colHeaderTitle(c, absIdx)
		parts[i] = text.PadOrTrunc(title, c.width)
	}
	headerText := " " + strings.Join(parts, "  ")
	return styles.TableHeader.Render(headerText)
}

// colHeaderTitle returns the column title with a position number prefix and
// sort indicator. absIdx is the 0-based absolute column index (accounting for
// hScrollOffset). Position numbers 1-9 correspond to keys "1"-"9"; position 10
// shows as "0". The prefix is always shown for columns 0-9 — PadOrTrunc in
// renderHeaders will truncate the rendered text if it exceeds the column width,
// so a truncated "5:Ins" is still more informative than a full "Instances↓".
func (m ResourceListModel) colHeaderTitle(c listCol, absIdx int) string {
	title := c.title
	// Append sort glyph if this is the active sort column.
	if m.sortColKey != "" && c.sortKey == m.sortColKey {
		if m.sortAsc {
			title += sortAscGlyph
		} else {
			title += sortDescGlyph
		}
	}
	// Add position number prefix (1-based, max 10 columns for sort).
	// Always emit the prefix; narrow columns get truncated by PadOrTrunc.
	if absIdx < 10 {
		displayNum := absIdx + 1 // 0-based → 1-based
		if displayNum == 10 {
			displayNum = 0 // key "0" = column 10
		}
		return fmt.Sprintf("%d:%s", displayNum, title)
	}
	return title
}
