package views

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

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

// resolveColumns determines the column definitions to use.
func (m ResourceListModel) resolveColumns() []listCol {
	// Check config-driven columns first.
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.typeDef.ShortName)
		if len(vd.List) > 0 {
			cols := make([]listCol, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = listCol{
					title: lc.Title,
					width: lc.Width,
					path:  lc.Path,
					key:   lc.Key,
				}
			}
			return cols
		}
	}

	// Fall back to typeDef columns.
	cols := make([]listCol, len(m.typeDef.Columns))
	for i, c := range m.typeDef.Columns {
		cols[i] = listCol{
			title: c.Title,
			width: c.Width,
			key:   c.Key,
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
func (m ResourceListModel) renderHeaderRow(cols []listCol) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		title := m.colHeaderTitle(c, i)
		parts[i] = text.PadOrTrunc(title, c.width)
	}
	headerText := " " + strings.Join(parts, "  ")
	return styles.TableHeader.Render(headerText)
}

// colHeaderTitle returns the column title with a sort indicator if this column
// is the active sort column. Per §6, the indicator is bound to exactly one
// column via ResourceListModel.sortColKey — set when the sort mode changes.
// Substring matching is intentionally removed to prevent double-glyph bugs
// (e.g. ct-events: both TIME and EVENT previously matched SortAge via isAgeKey).
func (m ResourceListModel) colHeaderTitle(c listCol, _ int) string {
	title := c.title
	if m.sortColKey != "" && colSortKey(c) == m.sortColKey {
		if m.sortAsc {
			title += "\u2191"
		} else {
			title += "\u2193"
		}
	}
	return title
}

// renderDataRow renders a single data row.
func (m ResourceListModel) renderDataRow(cols []listCol, r resource.Resource, base lipgloss.Style, totalWidth int, isSelected bool) string {
	var b strings.Builder
	// Leading single space carries base style.
	b.WriteString(base.Render(" "))
	used := 1
	for i, c := range cols {
		if i > 0 {
			b.WriteString(base.Render("  "))
			used += 2
		}
		val := m.extractCellValue(c, r)
		if (c.key == "state" || c.path == "State.Name") && val == "running" {
			sysStatus := r.Fields["system_status"]
			instStatus := r.Fields["instance_status"]
			if sysStatus == "impaired" || instStatus == "impaired" {
				val = "! " + val
			} else if sysStatus == "initializing" || instStatus == "initializing" {
				val = "~ " + val
			}
		}
		padded := text.PadOrTrunc(val, c.width)
		used += c.width
		b.WriteString(base.Render(padded))
	}
	// Trailing pad to totalWidth for the cursor row so the cursor bg fills the entire line.
	// Non-cursor rows are not padded (preserving the same plain-text length as the
	// pre-fix RowColorStyle.Render approach, which did not add Width padding).
	if isSelected && totalWidth > used {
		b.WriteString(base.Render(strings.Repeat(" ", totalWidth-used)))
	}
	return b.String()
}

// extractCellValue gets the cell value for a column from a resource.
func (m ResourceListModel) extractCellValue(c listCol, r resource.Resource) string {
	// Special key "@id" maps to the resource's canonical ID field.
	if c.key == "@id" {
		return r.ID
	}
	// Try config-driven path via ExtractScalar first (if path set and RawStruct available).
	if c.path != "" && r.RawStruct != nil {
		val := fieldpath.ExtractScalar(r.RawStruct, c.path)
		if val != "" {
			return val
		}
	}
	// Fall back to Fields map.
	if c.key != "" {
		if v, ok := r.Fields[c.key]; ok {
			return v
		}
	}
	// Try matching column title (lowercased) against Fields keys.
	titleLower := strings.ToLower(c.title)
	for k, v := range r.Fields {
		if strings.ToLower(k) == titleLower {
			return v
		}
	}
	// Final fallback: use resource Name for name-style columns when Fields has no value.
	// This handles test fixtures and resources where Fields is sparse but r.Name is set.
	// Matches columns whose key, title, or path contains "name"
	// (e.g., "alarm_name", "Alarm Name", "EventName").
	if r.Name != "" &&
		(strings.Contains(strings.ToLower(c.key), "name") ||
			strings.Contains(strings.ToLower(c.title), "name") ||
			strings.Contains(strings.ToLower(c.path), "name")) {
		return r.Name
	}
	return ""
}
