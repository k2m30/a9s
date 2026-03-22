package views

import (
	"strings"

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
func (m ResourceListModel) fitColumns(cols []listCol) []listCol {
	if m.width <= 0 {
		return cols
	}
	usedWidth := 1 // leading space
	var fit []listCol
	for _, c := range cols {
		needed := c.width + 2 // column width + 2-space gap
		if usedWidth+needed > m.width && len(fit) > 0 {
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

// colHeaderTitle returns the column title with sort indicator if applicable.
func (m ResourceListModel) colHeaderTitle(c listCol, _ int) string {
	title := c.title
	// Add sort indicator based on active sort field.
	// Match by common patterns: first column is often "name-ish", etc.
	var isActive bool
	switch m.sort {
	case SortName:
		// Name sort applies to column with key "name" or title containing "Name"
		isActive = strings.EqualFold(c.key, "name") || strings.Contains(strings.ToLower(c.title), "name")
	case SortID:
		isActive = strings.Contains(strings.ToLower(c.key), "id") || strings.Contains(c.title, "ID")
	case SortAge:
		isActive = strings.Contains(strings.ToLower(c.key), "time") || strings.Contains(strings.ToLower(c.key), "date") ||
			strings.Contains(strings.ToLower(c.key), "launch") || strings.Contains(strings.ToLower(c.key), "creation") ||
			strings.Contains(strings.ToLower(c.title), "time") || strings.Contains(strings.ToLower(c.title), "date")
	}
	if isActive {
		if m.sortAsc {
			title += "\u2191"
		} else {
			title += "\u2193"
		}
	}
	return title
}

// renderDataRow renders a single data row.
func (m ResourceListModel) renderDataRow(cols []listCol, r resource.Resource) string {
	cells := make([]string, len(cols))
	for i, c := range cols {
		val := m.extractCellValue(c, r)
		cells[i] = text.PadOrTrunc(val, c.width)
	}
	return " " + strings.Join(cells, "  ")
}

// extractCellValue gets the cell value for a column from a resource.
func (m ResourceListModel) extractCellValue(c listCol, r resource.Resource) string {
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
	return ""
}
