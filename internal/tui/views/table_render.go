// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"fmt"
	"strings"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
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
// "N:Title" sort-key prefix produced by colHeaderTitle. Without this, columns
// declared with Width < len("N:Title") would truncate the header (e.g.
// "5:Instanc…" at width=10) and hide the sort hint. The architectural rule:
// any sortable column (positions 0-9) MUST reserve enough room for its prefix.
// Columns beyond position 9 have no prefix and keep their declared width.
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
		minWidth := len([]rune(prefix)) + len([]rune(cols[i].title))
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
			title += "\u2191"
		} else {
			title += "\u2193"
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
