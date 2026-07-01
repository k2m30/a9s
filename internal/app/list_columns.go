package app

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// resolveListColumns mirrors resolveColumns from table_render.go exactly,
// including the superset check, so the controller column set is always
// identical to what the TUI renders.
func resolveListColumns(typeName string) []ColumnDef {
	return resolveListColumnsWithConfig(nil, typeName)
}

// resolveListColumnsForBuild mirrors resolveColumns() in table_render.go, using
// the caller-supplied td (already resolved fallback-first) for the superset
// first-column-title check. This ensures that custom test typeDefs sharing a
// ShortName with a catalog type but having a different column layout (e.g.
// rlTestTypeDef starts with "Instance ID" not "Name") do not get silently
// switched to the built-in 9-column defaults.
func resolveListColumnsForBuild(vc *config.ViewsConfig, typeName string, td *resource.ResourceTypeDef) []ColumnDef {
	if vc != nil {
		vd := config.GetViewDef(vc, typeName)
		if len(vd.List) > 0 {
			cols := make([]ColumnDef, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
			}
			return cols
		}
	}

	defaultVD := config.GetViewDef(nil, typeName)

	// Superset check using the supplied td (fallback-first, not catalog).
	if td != nil && len(defaultVD.List) > len(td.Columns) {
		firstMatch := len(td.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == td.Columns[0].Title)
		if firstMatch {
			cols := make([]ColumnDef, len(defaultVD.List))
			for i, lc := range defaultVD.List {
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
			}
			return cols
		}
	}

	// Fall back to td.Columns, carrying Path from defaults by title match.
	if td != nil && len(td.Columns) > 0 {
		defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
		for _, lc := range defaultVD.List {
			defaultByTitle[lc.Title] = lc
		}
		cols := make([]ColumnDef, len(td.Columns))
		for i, c := range td.Columns {
			cd := ColumnDef{Key: c.Key, Title: c.Title, Width: c.Width}
			if def, ok := defaultByTitle[c.Title]; ok && cd.Path == "" {
				cd.Path = def.Path
			}
			cols[i] = cd
		}
		return cols
	}

	// No td — fall back to raw built-in defaults.
	if len(defaultVD.List) > 0 {
		cols := make([]ColumnDef, len(defaultVD.List))
		for i, lc := range defaultVD.List {
			cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
		}
		return cols
	}
	return nil
}

// resolveListColumnsWithConfig resolves the column set for typeName, using vc
// as the per-session view config (nil = built-in defaults only). Mirrors
// ResourceListModel.resolveColumns so that buildListBody and View() agree.
func resolveListColumnsWithConfig(vc *config.ViewsConfig, typeName string) []ColumnDef {
	td := resource.FindResourceType(typeName)

	// When a per-session view config is provided, use it (mirrors the viewConfig
	// branch in ResourceListModel.resolveColumns). This ensures path-based columns
	// (e.g. ENI Status with Key="" Path="Status") are returned with the correct
	// Key/Path from the config, matching what extractCellValue sees at render time.
	if vc != nil {
		vd := config.GetViewDef(vc, typeName)
		if len(vd.List) > 0 {
			cols := make([]ColumnDef, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = ColumnDef{
					Key:   lc.Key,
					Title: lc.Title,
					Width: lc.Width,
					Path:  lc.Path,
				}
			}
			return cols
		}
	}

	defaultVD := config.GetViewDef(nil, typeName)

	// Superset check: use default view config only when it is strictly larger
	// than td.Columns AND the first column title matches.
	if td != nil && len(defaultVD.List) > len(td.Columns) {
		firstMatch := len(td.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == td.Columns[0].Title)
		if firstMatch {
			cols := make([]ColumnDef, len(defaultVD.List))
			for i, lc := range defaultVD.List {
				cols[i] = ColumnDef{
					Key:   lc.Key,
					Title: lc.Title,
					Width: lc.Width,
					Path:  lc.Path,
				}
			}
			return cols
		}
	}

	// Fall back to td.Columns, carrying Path from defaults by title match.
	if td != nil {
		defaultByTitle := make(map[string]config.ListColumn, len(defaultVD.List))
		for _, lc := range defaultVD.List {
			defaultByTitle[lc.Title] = lc
		}
		cols := make([]ColumnDef, len(td.Columns))
		for i, c := range td.Columns {
			cd := ColumnDef{
				Key:   c.Key,
				Title: c.Title,
				Width: c.Width,
			}
			if def, ok := defaultByTitle[c.Title]; ok && cd.Path == "" {
				cd.Path = def.Path
			}
			cols[i] = cd
		}
		return cols
	}

	// No td registered: fall back to raw view-config list if available.
	if len(defaultVD.List) > 0 {
		cols := make([]ColumnDef, len(defaultVD.List))
		for i, lc := range defaultVD.List {
			cols[i] = ColumnDef{
				Key:   lc.Key,
				Title: lc.Title,
				Width: lc.Width,
				Path:  lc.Path,
			}
		}
		return cols
	}
	return nil
}

// extractListCells builds the cell value slice for one row, mirroring
// extractCellValue + CellDecorators application in table_render.go. DATA only — no Lipgloss styling.
// td must already be resolved (fallback-first) by the caller so that CellDecorators
// from the model's typeDef are applied (e.g. EC2 state impaired/initializing prefix).
func extractListCells(columns []ColumnDef, r resource.Resource, td *resource.ResourceTypeDef) []string {
	cells := make([]string, len(columns))
	for i, col := range columns {
		v := listExtractCellValue(col, td, r)
		if td != nil && len(td.CellDecorators) > 0 {
			if dec := lookupListDecorator(td.CellDecorators, col); dec != nil {
				v = dec(r, v)
			}
		}
		cells[i] = v
	}
	return cells
}

// lookupListDecorator mirrors lookupDecorator in table_render.go but operates on
// ColumnDef (Key+Title+Path) instead of listCol. Tries key, path, path last segment
// (lowercased), and lowercased title — in that order.
func lookupListDecorator(decs map[string]func(resource.Resource, string) string, col ColumnDef) func(resource.Resource, string) string {
	if len(decs) == 0 {
		return nil
	}
	if col.Key != "" {
		if d, ok := decs[col.Key]; ok {
			return d
		}
	}
	if col.Path != "" {
		if d, ok := decs[col.Path]; ok {
			return d
		}
		if i := strings.LastIndex(col.Path, "."); i >= 0 {
			if d, ok := decs[strings.ToLower(col.Path[i+1:])]; ok {
				return d
			}
		} else if d, ok := decs[strings.ToLower(col.Path)]; ok {
			return d
		}
	}
	if col.Title != "" {
		if d, ok := decs[strings.ToLower(col.Title)]; ok {
			return d
		}
	}
	return nil
}

// listExtractCellValue replicates the full extractCellValue cascade from
// table_render.go byte-for-byte, using ColumnDef (Key+Title+Path) so that
// path-only columns (e.g. EC2 Name/State/Type with key="") resolve correctly.
func listExtractCellValue(col ColumnDef, td *resource.ResourceTypeDef, r resource.Resource) string {
	if col.Key == "@id" {
		return r.ID
	}

	// Status/lifecycle column — two-layer priority.
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	isStatusCol := col.Key == "status" || col.Key == lifecycleKey
	if isStatusCol {
		if phrase := listPhraseFromFindings(r.Findings); phrase != "" {
			return phrase
		}
		return r.Fields[lifecycleKey]
	}

	// Fields map (key-based) takes priority.
	if col.Key != "" {
		if v, ok := r.Fields[col.Key]; ok && v != "" {
			return v
		}
	}

	// Path-based fallback via fieldpath.ExtractScalar.
	if col.Path != "" && r.RawStruct != nil {
		if val := fieldpath.ExtractScalar(r.RawStruct, col.Path); val != "" {
			return val
		}
	}

	// Second-pass: accept explicit empty-string values stored in Fields.
	if col.Key != "" {
		if v, ok := r.Fields[col.Key]; ok {
			return v
		}
	}

	// Title-match loop: lowercased title and space→underscore variant against Fields keys.
	titleLower := strings.ToLower(col.Title)
	titleUnder := strings.ReplaceAll(titleLower, " ", "_")
	for k, v := range r.Fields {
		kl := strings.ToLower(k)
		if kl == titleLower || kl == titleUnder {
			return v
		}
	}

	// Name fallback: title OR key OR path contains "name" → r.Name.
	if r.Name != "" &&
		(strings.Contains(strings.ToLower(col.Key), "name") ||
			strings.Contains(strings.ToLower(col.Title), "name") ||
			strings.Contains(strings.ToLower(col.Path), "name")) {
		return r.Name
	}

	return ""
}

// listPhraseFromFindings mirrors phraseFromFindings in table_render.go.
func listPhraseFromFindings(findings []domain.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	if len(findings) == 1 {
		return findings[0].Phrase
	}
	return findings[0].Phrase + " (+" + itoa(len(findings)-1) + ")"
}

// resolveListDecoratorFull mirrors the marker logic in renderDataRow and extends it
// ("healthy", "warning", "broken", "dim", "") so RenderList can reproduce
// the exact lipgloss.Style that View() derives from td.ResolveColor(r).
func resolveListDecoratorFull(td *resource.ResourceTypeDef, r resource.Resource, findings map[string]domain.Finding) (RowDecorator, string, string) {
	if td == nil {
		return DecoratorNormal, "", ""
	}
	color := td.ResolveColor(r)
	colorTag := colorToTag(color)
	if color == resource.ColorHealthy {
		if f, ok := findings[r.ID]; ok {
			switch f.Severity {
			case domain.SevBroken:
				return DecoratorError, "broken", colorTag
			case domain.SevWarn:
				return DecoratorWarning, "warn", colorTag
			}
		}
	}
	sev := ""
	if color.IsIssue() {
		sev = "issue"
	}
	return DecoratorNormal, sev, colorTag
}

// colorToTag converts a domain.Color to the string tag carried by ListRow.Color.
func colorToTag(c domain.Color) string {
	switch c {
	case domain.ColorHealthy:
		return "healthy"
	case domain.ColorWarning:
		return "warning"
	case domain.ColorBroken:
		return "broken"
	case domain.ColorDim:
		return "dim"
	}
	return ""
}

// resolveListMarkerCol mirrors resolveIdentityColumn in table_render.go.
// Returns the 0-based index in columns of the identity column.
// Cascade must match resolveIdentityColumn exactly (steps 1-5).
func resolveListMarkerCol(columns []ColumnDef, td *resource.ResourceTypeDef) int {
	// Step 1: explicit IdentityKey on the type definition.
	if td != nil && td.IdentityKey != "" {
		for i, c := range columns {
			if c.Key == td.IdentityKey {
				return i
			}
		}
	}
	// Step 2: column key is literally "name".
	for i, c := range columns {
		if c.Key == "name" {
			return i
		}
	}
	// Step 3: column path contains "Name" or "Identifier" (mirrors resolveIdentityColumn step 3).
	for i, c := range columns {
		if strings.Contains(c.Path, "Name") || strings.Contains(c.Path, "Identifier") {
			return i
		}
	}
	// Step 4: column title equals "Name" (case-insensitive) or the type's display name.
	for i, c := range columns {
		if strings.EqualFold(c.Title, "Name") || (td != nil && strings.EqualFold(c.Title, td.Name)) {
			return i
		}
	}
	// Step 5: fall back to index 0.
	return 0
}

// ResolveListColumns exports resolveListColumns for use by constructors that
// need to translate a 0-based column index to a column key (e.g., sort restore).
func ResolveListColumns(typeName string) []ColumnDef {
	return resolveListColumns(typeName)
}

// ResolveColumnsForType resolves the column set for typeName using this
// controller's viewConfig and fallbackTypeDefs. Mirrors resolveColumns in
// table_render.go so that handleSortByCol and buildListBody always agree on
// the column set — and therefore on what key "N" maps to.
func (c *Controller) ResolveColumnsForType(typeName string) []ColumnDef {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// viewConfig takes highest priority (same as resolveColumns).
	if c.viewConfig != nil {
		vd := config.GetViewDef(c.viewConfig, typeName)
		if len(vd.List) > 0 {
			cols := make([]ColumnDef, len(vd.List))
			for i, lc := range vd.List {
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
			}
			return cols
		}
	}

	// No viewConfig — resolve the fallback typeDef so we can compare against defaults.
	var ftd *resource.ResourceTypeDef
	if fv, ok := c.fallbackTypeDefs[typeName]; ok {
		ftd = &fv
	} else if ct := resource.FindResourceType(typeName); ct != nil {
		ftd = ct
	}

	// Apply the same superset + first-column-title guard as resolveColumns:
	// use built-in defaults only when they are strictly larger AND the first
	// column title matches — this ensures custom test typeDefs that share a
	// ShortName but have different column layouts (e.g. pgTestTypeDef uses
	// ShortName="ec2" with first col "Instance ID" vs defaults' "Name") are
	// not silently switched to the defaults.
	defaultVD := config.GetViewDef(nil, typeName)
	if ftd != nil && len(defaultVD.List) > len(ftd.Columns) {
		firstMatch := len(ftd.Columns) == 0 ||
			(len(defaultVD.List) > 0 && defaultVD.List[0].Title == ftd.Columns[0].Title)
		if firstMatch {
			cols := make([]ColumnDef, len(defaultVD.List))
			for i, lc := range defaultVD.List {
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path}
			}
			return cols
		}
	}

	// Fall back to typeDef columns (covers test typeDefs with non-matching first title).
	if ftd != nil && len(ftd.Columns) > 0 {
		cols := make([]ColumnDef, len(ftd.Columns))
		for i, col := range ftd.Columns {
			cols[i] = ColumnDef{Key: col.Key, Title: col.Title, Width: col.Width}
		}
		return cols
	}

	return nil
}
