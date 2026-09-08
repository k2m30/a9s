// SPDX-License-Identifier: GPL-3.0-or-later

// costs.go — Cost Explorer grid renderer (RenderCosts). Thin per
// CostsBody's doc comment: every amount, delta bucket, and open-period
// marker is pre-resolved by core/app.buildCostsBody; this file only
// lays out and colors what it is given — no grid math, no aggregation.
package views

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

const (
	// costsLabelColWidth fits the longest common vendor-prefix-stripped
	// SERVICE label ("Elastic Compute Cloud - Compute", 32 chars) with a
	// little headroom.
	costsLabelColWidth = 34
	costsMinColWidth   = 9
)

// costsLabelWidth returns the label column's width at terminal width —
// the single budget both RenderCosts and CostsViewportCols size against, so
// the two can never compute the grid's columns-that-fit differently.
func costsLabelWidth(width int) int {
	if costsLabelColWidth > width-costsMinColWidth {
		return max(width/3, 1)
	}
	return costsLabelColWidth
}

// RenderCosts renders the Cost Explorer grid body: label column, time
// columns with the open-period "*" marker, per-cell delta/anomaly/
// estimated/negative coloring, a pinned TOTAL row, and the footer line
// (anomaly/delta slot, API counter, data-through date). Never panics at
// tiny or zero sizes.
func RenderCosts(body app.CostsBody, width, height int) string {
	if body.ErrorMsg != "" {
		return renderCostsError(body, width, height)
	}
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	// A one-row viewport fits only a single pinned line; render the footer
	// alone so total output never exceeds height (blank+footer would be 2).
	if height == 1 {
		return renderCostsFooter(body)
	}

	labelW := costsLabelWidth(width)
	colW := costsColWidth(width, labelW, len(body.Columns))

	// Only columns that fully fit the label+column budget render — a
	// partial trailing column would push the row past width and break the
	// frame's right border. body.Columns is normally already sized to fit
	// (via CostsViewportCols -> SetCostsViewportCols), this is a second,
	// authoritative guard against any transient width/column-count
	// mismatch (e.g. a resize landing between snapshot and render).
	nCols := costsFittingCols(width, labelW, colW, len(body.Columns))
	if nCols < len(body.Columns) {
		body.Columns = body.Columns[:nCols]
		trimmedRows := make([]app.CostRow, len(body.Rows))
		for i, row := range body.Rows {
			trimmedRows[i] = app.CostRow{Label: row.Label, Cells: row.Cells[:nCols]}
		}
		body.Rows = trimmedRows
		body.Totals = body.Totals[:nCols]
	}

	lines := make([]string, 0, len(body.Rows)+2)
	lines = append(lines, padCostsRowToWidth(renderCostsHeaderRow(body, labelW, colW), width))
	for ri, row := range body.Rows {
		lines = append(lines, padCostsRowToWidth(renderCostsDataRow(row.Label, row.Cells, body, ri, labelW, colW), width))
	}
	lines = append(lines, padCostsRowToWidth(renderCostsDataRow("TOTAL", body.Totals, body, -1, labelW, colW), width))

	// RenderCosts unconditionally appends 2 more lines (blank + footer)
	// after clipping — the clip budget must reserve room for them so the
	// total output never exceeds the requested height.
	lines = clipCostsRows(lines, body.CursorRow, max(height-2, 0))

	lines = append(lines, "")
	lines = append(lines, renderCostsFooter(body))

	return strings.Join(lines, "\n")
}

// costsColWidth returns the per-column width that, together with its
// leading separator space, tiles the label+columns budget as evenly as
// possible without exceeding it (each column consumes colW+1 cells).
func costsColWidth(width, labelW, nCols int) int {
	if nCols == 0 {
		return costsMinColWidth
	}
	avail := width - labelW
	return max(avail/nCols-1, costsMinColWidth)
}

// costsFittingCols caps nCols to however many colW-wide columns (plus their
// leading separator space) actually fit in the label+columns budget,
// guarding against colW having been floored to costsMinColWidth.
func costsFittingCols(width, labelW, colW, nCols int) int {
	avail := width - labelW
	fit := max(avail/(colW+1), 0)
	return min(nCols, fit)
}

// CostsViewportCols returns how many time columns fit at the given terminal
// width, using the same label/column-width budget RenderCosts itself
// applies. Callers (the TUI renderer) call this BEFORE snapshotting to set
// Controller.SetCostsViewportCols, so buildCostsBody slices exactly what
// will render.
func CostsViewportCols(width int) int {
	if width <= 0 {
		return 1
	}
	avail := width - costsLabelWidth(width)
	return max(avail/(costsMinColWidth+1), 1)
}

// padCostsRowToWidth pads line with trailing spaces up to width so every
// rendered row (header, data, TOTAL) is exactly the frame's inner width —
// a shorter row would leave the right border unclosed.
func padCostsRowToWidth(line string, width int) string {
	w := text.Width(line)
	if w >= width {
		return line
	}
	return line + strings.Repeat(" ", width-w)
}

func renderCostsHeaderRow(body app.CostsBody, labelW, colW int) string {
	var b strings.Builder
	b.WriteString(styles.TableHeader.Render(text.PadOrTrunc(strings.ToUpper(body.Pivot), labelW)))
	for _, col := range body.Columns {
		b.WriteByte(' ')
		label := col.Label
		if col.Open {
			label += "*"
		}
		b.WriteString(styles.TableHeader.Render(padLeft(label, colW)))
	}
	return b.String()
}

// renderCostsDataRow renders one row (data or the pinned TOTAL, ri=-1) with
// per-cell cursor/delta/anomaly/estimated/negative coloring.
func renderCostsDataRow(label string, cells []app.CostCell, body app.CostsBody, ri, labelW, colW int) string {
	var b strings.Builder
	labelStyle := styles.RowNormal
	if ri == -1 {
		labelStyle = styles.TableHeader
	}
	b.WriteString(labelStyle.Render(text.PadOrTrunc(label, labelW)))
	for ci, cell := range cells {
		b.WriteByte(' ')
		selected := ri >= 0 && ri == body.CursorRow && ci == body.CursorCol
		content := cell.Amount
		if cell.Anomaly {
			content += " ▲"
		}
		b.WriteString(styleForCostCell(cell, selected).Render(padLeft(content, colW)))
	}
	return b.String()
}

// styleForCostCell picks the pre-resolved cell's display style. Selected
// (cursor) always wins — the wireframe's cursor highlight must be visible
// regardless of the cell's delta/anomaly coloring underneath it.
func styleForCostCell(cell app.CostCell, selected bool) lipgloss.Style {
	if selected {
		return styles.RowSelected
	}
	switch {
	case cell.Estimated:
		return styles.DimText
	case cell.Negative:
		return styles.StatusCheckOk
	case cell.DeltaTag == "growth-soft":
		return styles.CostGrowthSoft
	case cell.DeltaTag == "growth-strong":
		return styles.CostGrowthStrong
	case cell.DeltaTag == "drop-soft":
		return styles.CostDropSoft
	case cell.DeltaTag == "drop-strong":
		return styles.CostDropStrong
	default:
		return styles.RowNormal
	}
}

// clipCostsRows keeps the header (index 0) and pinned TOTAL (last index)
// always visible, vertically scrolling the data rows in between around
// cursorRow when they exceed height (wireframe.md: "TOTAL stays pinned").
func clipCostsRows(lines []string, cursorRow, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	header := lines[0]
	total := lines[len(lines)-1]
	data := lines[1 : len(lines)-1]

	if height == 1 {
		return []string{total}
	}

	budget := height - 2
	if budget <= 0 {
		return []string{header, total}
	}
	if len(data) <= budget {
		out := make([]string, 0, len(lines))
		out = append(out, header)
		out = append(out, data...)
		return append(out, total)
	}

	start := max(cursorRow-budget/2, 0)
	if start+budget > len(data) {
		start = len(data) - budget
	}
	start = max(start, 0)
	out := make([]string, 0, budget+2)
	out = append(out, header)
	out = append(out, data[start:start+budget]...)
	return append(out, total)
}

// renderCostsFooter assembles the footer slots: the anomaly/delta note,
// then the grid's currency unit (omitted when the grid mixes currencies —
// body.Currency == ""), then the data-through date (FR-013: the CE-call
// counter is internal bookkeeping only, never rendered).
func renderCostsFooter(body app.CostsBody) string {
	var parts []string
	if body.FooterNote != "" {
		parts = append(parts, body.FooterNote)
	}
	if body.Currency != "" {
		parts = append(parts, body.Currency)
	}
	if body.DataThrough != "" {
		parts = append(parts, "data through "+body.DataThrough)
	}
	return styles.DimText.Render(strings.Join(parts, " · "))
}

// renderCostsError renders the FR-017 explicit error state: a centered
// message block, never an empty grid. body.ErrorMsg is rendered verbatim —
// classification into a user-facing sentence happens upstream.
func renderCostsError(body app.CostsBody, width, height int) string {
	if width <= 0 {
		width = 40
	}
	if height <= 0 {
		height = 3
	}
	maxW := max(width-4, 10)
	wrapped := strings.Split(lipgloss.Wrap(body.ErrorMsg, maxW, " "), "\n")

	lines := make([]string, 0, height)
	padTop := max((height-len(wrapped))/2, 0)
	for i := 0; i < padTop && len(lines) < height; i++ {
		lines = append(lines, "")
	}
	for _, l := range wrapped {
		if len(lines) >= height {
			break
		}
		styled := styles.FlashError.Render(l)
		lines = append(lines, lipgloss.Place(width, 1, lipgloss.Center, lipgloss.Top, styled))
	}
	return strings.Join(lines, "\n")
}

// padLeft right-aligns s within w visible columns, truncating (with an
// ellipsis) rather than overflowing when s is wider than w.
func padLeft(s string, w int) string {
	if w <= 0 {
		return ""
	}
	vw := text.Width(s)
	if vw >= w {
		return text.PadOrTrunc(s, w)
	}
	return strings.Repeat(" ", w-vw) + s
}
