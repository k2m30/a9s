package views

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// lookupDecorator resolves a CellDecorator for column c by trying key, path,
// lowercased title, and path's final segment. Returns nil if no match.
func lookupDecorator(decs map[string]func(resource.Resource, string) string, c listCol) func(resource.Resource, string) string {
	if len(decs) == 0 {
		return nil
	}
	if c.key != "" {
		if d, ok := decs[c.key]; ok {
			return d
		}
	}
	if c.path != "" {
		if d, ok := decs[c.path]; ok {
			return d
		}
		if i := strings.LastIndex(c.path, "."); i >= 0 {
			if d, ok := decs[strings.ToLower(c.path[i+1:])]; ok {
				return d
			}
		} else if d, ok := decs[strings.ToLower(c.path)]; ok {
			return d
		}
	}
	if c.title != "" {
		if d, ok := decs[strings.ToLower(c.title)]; ok {
			return d
		}
	}
	return nil
}

// listCol is a resolved column definition for rendering.
type listCol struct {
	title    string
	width    int
	key      string // resource.Fields key (fallback)
	path     string // config-driven path for ExtractScalar
	sortKey  string // optional: Fields key to use for sorting instead of display value
	sortPath string // optional: RawStruct path for raw numeric/time sort comparison
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
	// When viewConfig is explicitly set, use it (merged with defaults via GetViewDef).
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.typeDef.ShortName)
		if len(vd.List) > 0 {
			cols := make([]listCol, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = listCol{
					title:    lc.Title,
					width:    lc.Width,
					path:     lc.Path,
					key:      lc.Key,
					sortKey:  lc.SortKey,
					sortPath: lc.SortPath,
				}
			}
			return cols
		}
	}

	// When viewConfig is nil, fall back to built-in defaults for this resource
	// type when the defaults are a superset of the typeDef columns. This ensures
	// that resource types whose typeDef.Columns is a subset of the defaults (e.g.
	// S3 which adds a Region column in defaults) render the full column set even
	// in contexts where no config file is loaded (tests, demo mode). The superset
	// check uses first-column title equality so that custom test typeDefs that
	// share a ShortName but define different column layouts (e.g. ec2 sort tests)
	// are not accidentally switched to defaults.
	defaultVD := config.GetViewDef(nil, m.typeDef.ShortName)
	if len(defaultVD.List) > len(m.typeDef.Columns) {
		firstMatch := len(m.typeDef.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == m.typeDef.Columns[0].Title)
		if firstMatch {
			cols := make([]listCol, len(defaultVD.List))
			for i, lc := range defaultVD.List {
				cols[i] = listCol{
					title:    lc.Title,
					width:    lc.Width,
					path:     lc.Path,
					key:      lc.Key,
					sortKey:  lc.SortKey,
					sortPath: lc.SortPath,
				}
			}
			return cols
		}
	}

	// Fall back to typeDef columns.
	// Build a lookup of default columns by title so we can carry over SortKey/SortPath
	// from defaults for any column that matches by title.
	defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
	for _, lc := range defaultVD.List {
		defaultByTitle[lc.Title] = lc
	}
	cols := make([]listCol, len(m.typeDef.Columns))
	for i, c := range m.typeDef.Columns {
		lc := listCol{
			title: c.Title,
			width: c.Width,
			key:   c.Key,
		}
		if def, ok := defaultByTitle[c.Title]; ok {
			if lc.sortKey == "" {
				lc.sortKey = def.SortKey
			}
			if lc.sortPath == "" {
				lc.sortPath = def.SortPath
			}
		}
		cols[i] = lc
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
// shows as "0". The prefix is only shown when len("N:"+title) <= c.width so
// that narrow columns remain legible (fall back to plain title when there is no room).
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
	// Add position number prefix (1-based, max 10 columns for sort),
	// only when the prefixed title fits within the column width.
	if absIdx < 10 {
		displayNum := absIdx + 1 // 0-based → 1-based
		if displayNum == 10 {
			displayNum = 0 // key "0" = column 10
		}
		prefixed := fmt.Sprintf("%d:%s", displayNum, title)
		if len([]rune(prefixed)) <= c.width {
			return prefixed
		}
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

// renderDataRow renders a single data row. markerColIdx is the precomputed
// identity-column index (via resolveIdentityColumn) to avoid recomputing the
// cascade on every row.
func (m ResourceListModel) renderDataRow(cols []listCol, r resource.Resource, base lipgloss.Style, totalWidth int, isSelected bool, markerColIdx int) string {
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
		// Try decorator lookup by column key, then path, then lowercased title.
		// viewConfig-loaded columns often lack Key; fallbacks keep decorators
		// robust to the column-definition source.
		if dec := lookupDecorator(m.typeDef.CellDecorators, c); dec != nil {
			val = dec(r, val)
		}
		// Enrichment row marker: prepend a plain-text severity prefix to the identity
		// column when this resource has a finding. The whole cell (prefix + value) is
		// painted by the base row style so cursor highlight is uninterrupted.
		if i == markerColIdx {
			if finding, ok := m.findingsByID[r.ID]; ok {
				switch finding.Severity {
				case "!":
					val = "! " + val
				case "~":
					val = "~ " + val
				}
			} else if m.truncatedByID[r.ID] {
				val = "? " + val
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
	// Fields map (key-based columns) takes priority over raw struct fields.
	// This ensures Wave-2 enriched values always win over struct literals,
	// and allows columns to carry both a Key (enriched value) and a Path
	// (raw-struct fallback for sorting / column introspection).
	if c.key != "" {
		if v, ok := r.Fields[c.key]; ok && v != "" {
			return v
		}
	}
	// Fall back to config-driven path via ExtractScalar (struct field extraction).
	if c.path != "" && r.RawStruct != nil {
		val := fieldpath.ExtractScalar(r.RawStruct, c.path)
		if val != "" {
			return val
		}
	}
	// Fields map second pass: accept empty-string values stored explicitly.
	// This covers keys that were set but happen to be empty strings.
	if c.key != "" {
		if v, ok := r.Fields[c.key]; ok {
			return v
		}
	}
	// Try matching column title (lowercased) against Fields keys.
	// Also try with spaces replaced by underscores (e.g. "Instance ID" → "instance_id").
	titleLower := strings.ToLower(c.title)
	titleUnder := strings.ReplaceAll(titleLower, " ", "_")
	for k, v := range r.Fields {
		kl := strings.ToLower(k)
		if kl == titleLower || kl == titleUnder {
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
