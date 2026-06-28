package views

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// phraseFromFindings returns the merged S4 status phrase for a resource's
// findings: the first finding's Phrase alone when only one is present, or
// "<top> (+N)" when N additional findings are stacked. Returns "" when
// findings is empty.
//
// AS-140 collapsed the 3-layer status priority in extractCellValue to two
// layers by computing the merged phrase here at render time instead of
// during Wave-2 enrichment. Both Wave-1 (fetcher-emitted) and Wave-2
// (applyEnrichment-derived) findings live on r.Findings by the time a row
// reaches the renderer.
func phraseFromFindings(findings []domain.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	if len(findings) == 1 {
		return findings[0].Phrase
	}
	return fmt.Sprintf("%s (+%d)", findings[0].Phrase, len(findings)-1)
}

// listCol is a resolved column definition for rendering.
type listCol struct {
	title string
	width int
	key   string // resource.Fields key (fallback)
	path  string // config-driven path for ExtractScalar
}

// colSortKey returns the stable identifier for a column used to match against
// ResourceListModel.sortColKey. Key is preferred, path fallback, title last resort.
func colSortKey(c listCol) string {
	if c.key != "" {
		return c.key
	}
	if c.path != "" {
		return c.path
	}
	return c.title
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
	if m.sortColKey != "" && colSortKey(c) == m.sortColKey {
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

// resolveIdentityColumn returns the index of the column that should carry the
// enrichment-finding row marker. Cascade:
//  1. td.IdentityKey matches a column's Key
//  2. column Key == "name"
//  3. column Path contains "Name" or "Identifier"
//  4. column Title equals "Name" (case-insensitive) or equals td.Name
//  5. fall back to 0
func resolveIdentityColumn(cols []listCol, td resource.ResourceTypeDef) int {
	// Step 1: explicit IdentityKey set on the type definition.
	if td.IdentityKey != "" {
		for i, c := range cols {
			if c.key == td.IdentityKey {
				return i
			}
		}
	}
	// Step 2: column key is literally "name".
	for i, c := range cols {
		if c.key == "name" {
			return i
		}
	}
	// Step 3: column path contains "Name" or "Identifier".
	for i, c := range cols {
		if strings.Contains(c.path, "Name") || strings.Contains(c.path, "Identifier") {
			return i
		}
	}
	// Step 4: column title equals "Name" (case-insensitive) or equals the type's display name.
	for i, c := range cols {
		if strings.EqualFold(c.title, "Name") || strings.EqualFold(c.title, td.Name) {
			return i
		}
	}
	// Step 5: fall back to index 0.
	return 0
}

func lifecycleColumnKey(td resource.ResourceTypeDef) string {
	if td.LifecycleKey != "" {
		return td.LifecycleKey
	}
	return "state"
}

// widenLifecycleColumn scans all rows and widens the lifecycle/status column
// so its declared width covers the longest Findings phrase across every row.
// Returns a new slice; the original is unmodified.
//
// This must run BEFORE fitColumns and renderHeaderRow so that the header and
// all data rows use the same (widened) column width. Without this pre-pass,
// renderDataRow would widen per-row in isolation, causing the header to show
// the original (narrower) width while data rows overflow — a visible desync
// between header labels and data cells.
func (m ResourceListModel) widenLifecycleColumn(cols []listCol, rows []resource.Resource) []listCol {
	if len(cols) == 0 || len(rows) == 0 {
		return cols
	}
	lifecycleKey := lifecycleColumnKey(m.typeDef)
	idx := -1
	for i, c := range cols {
		if c.key == "status" || c.key == lifecycleKey {
			idx = i
			break
		}
	}
	if idx < 0 {
		return cols
	}
	maxW := cols[idx].width
	for _, r := range rows {
		// Mirror extractCellValue's AS-140 two-layer priority:
		// phraseFromFindings(r.Findings) — which composes "<top> (+N)" for
		// stacked findings — first, then Fields[lifecycleKey]. Sizing on
		// Findings[0].Phrase alone would under-size the column whenever a
		// row stacks wave-1+wave-2 (e.g. "stopped (+1)" rendered into a
		// "stopped"-width slot, then truncated).
		phrase := phraseFromFindings(r.Findings)
		if phrase == "" {
			phrase = r.Fields[lifecycleKey]
		}
		if phrase == "" {
			continue
		}
		if nat := lipgloss.Width(phrase); nat > maxW {
			maxW = nat
		}
	}
	if maxW == cols[idx].width {
		return cols // no widening needed
	}
	out := make([]listCol, len(cols))
	copy(out, cols)
	out[idx].width = maxW
	return out
}
