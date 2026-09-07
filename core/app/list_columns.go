// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// MaterializeListFields returns a copy of r with Fields populated for every
// Path-based, Key-less column in columns, extracting the scalar via
// fieldpath.ExtractScalar(r.RawStruct, col.Path) and writing it under the
// column's title key (config.TitleFieldKey — the one spelling the extraction
// cascade reads first). This is the generic render-sufficiency step Contract B requires:
// running it once, before a fetch result is cached or a screen's rows are
// stored, means a later cache replay with RawStruct stripped renders
// identical cells (extractListCells's key-based Fields lookup finds the
// value directly, without falling back to fieldpath on a nil RawStruct).
//
// Key-based columns (col.Key != "") are left untouched — they already
// resolve via the Fields-map lookup and may carry a Wave-2 enrichment
// override that fieldpath cannot see, so materializing over them risks
// clobbering a value RawStruct doesn't know about.
//
// A column is only materialized when Fields does not already carry a
// non-empty value under the resolved key, so a prior explicit value (or an
// earlier materialization pass) is never overwritten.
func MaterializeListFields(r resource.Resource, columns []ColumnDef) resource.Resource {
	if r.RawStruct == nil {
		return r
	}
	out := r
	copied := false
	for _, col := range columns {
		if col.Path == "" || col.Key != "" {
			continue
		}
		key := config.TitleFieldKey(col.Title)
		if key == "" {
			continue
		}
		if v, ok := out.Fields[key]; ok && v != "" {
			continue
		}
		val := fieldpath.ExtractScalar(out.RawStruct, col.Path)
		if val == "" {
			continue
		}
		if !copied {
			fresh := make(map[string]string, len(out.Fields)+1)
			maps.Copy(fresh, out.Fields)
			out.Fields = fresh
			copied = true
		}
		out.Fields[key] = val
	}
	return out
}

// resolveListColumnsForBuild resolves the column set for typeName via the
// shared resource.ResolveListColumnCascade, using the caller-supplied td
// (already resolved fallback-first) for the superset first-column-title
// check. This ensures that custom test typeDefs sharing a ShortName with a
// catalog type but having a different column layout (e.g. rlTestTypeDef
// starts with "Instance ID" not "Name") do not get silently switched to the
// built-in 9-column defaults.
//
// The identity election runs here and nowhere else, so it runs over the set
// that will actually be rendered and travels with it on ColumnDef.Identity —
// the cell extractor, the marker column and the sort comparator all read this
// one election instead of re-deriving it from a set they cannot see.
func resolveListColumnsForBuild(vc *config.ViewsConfig, typeName string, td *resource.ResourceTypeDef) []ColumnDef {
	lcs := resource.ResolveListColumnCascade(vc, typeName, td)
	if len(lcs) == 0 {
		return nil
	}
	cols := make([]ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = ColumnDef{
			Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path,
			Humanize: lc.Humanize, SortKey: lc.SortKey, SortPath: lc.SortPath,
		}
	}
	cols[IdentityColumnIndex(cols, td)].Identity = true
	return cols
}

// extractListCells builds the cell value slice for one row: ExtractCellValue
// plus CellDecorators. DATA only — no Lipgloss styling.
// td must already be resolved (fallback-first) by the caller so that CellDecorators
// from the model's typeDef are applied (e.g. EC2 state impaired/initializing prefix).
func extractListCells(columns []ColumnDef, r resource.Resource, td *resource.ResourceTypeDef) []string {
	cells := make([]string, len(columns))
	for i, col := range columns {
		v := ExtractCellValue(col, td, r)
		if td != nil && len(td.CellDecorators) > 0 {
			if dec := lookupListDecorator(td.CellDecorators, col, td.LifecycleKey); dec != nil {
				v = dec(r, v)
			}
		}
		cells[i] = v
	}
	return cells
}

// lookupListDecorator resolves a ColumnDef's decorator. Tries key, path, path last segment
// (lowercased), and lowercased title — in that order.
//
// A status-qualifying column (same predicate as ExtractCellValue's isStatusCol)
// additionally tries lifecycleKey (the type's LifecycleKey, "" meaning "state" per
// ExtractCellValue's own default) and the literal "state" as decorator keys,
// after the explicit key/path/title matches above. Config-driven status columns
// (e.g. ec2's default-view {Title:"Status", Path:"State.Name"}, no Key) carry none
// of the keys a CellDecorators map is normally registered under (e.g. ec2's
// CellDecorators["state"]), so without this fallback a decorator registered under
// the type's state/lifecycle key is unreachable.
func lookupListDecorator(decs map[string]func(resource.Resource, string) string, col ColumnDef, lifecycleKey string) func(resource.Resource, string) string {
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
	if lifecycleKey == "" {
		lifecycleKey = "state"
	}
	if config.IsStatusColumn(col.Key, col.Title, lifecycleKey) {
		if d, ok := decs[lifecycleKey]; ok {
			return d
		}
		if d, ok := decs["state"]; ok {
			return d
		}
	}
	return nil
}

// ExtractCellValue resolves the value one column shows for one row. It is the
// single cascade the list body, the sort comparator and every render path
// share, and it works from ColumnDef (Key+Title+Path) so that path-only
// columns (e.g. EC2 Name/State/Type with key="") resolve correctly.
func ExtractCellValue(col ColumnDef, td *resource.ResourceTypeDef, r resource.Resource) string {
	if col.Key == "@id" {
		return r.ID
	}

	// Status/lifecycle column — OWNER CONTRACT: a column whose Title equals
	// "Status" (case-insensitive, exact word) IS the status column,
	// regardless of its Key. This covers keyed columns whose data lives
	// under their own key (e.g. cb's {Key:"last_status", Title:"Status"},
	// tg's {Key:"health_summary", Title:"Status"}, sg's
	// {Key:"risk_summary", Title:"Status"}) as well as the pre-existing
	// Key-less, Path-based case (e.g. lambda/ec2's per-session view
	// {Title:"State", Path:"State"} with no Key, or acm/eks/ng's
	// default-view {Path:"Status"} with no Key). Every qualifying column
	// must route through the same domain.StatusPhrase + HumanizeStatusPhrase
	// chokepoint — otherwise it falls through to the raw fieldpath/Fields
	// value further down and a raw AWS enum (e.g. "FAILED", "STALE")
	// reaches the screen. The predicate lives in config.IsStatusColumn, which
	// the decorator lookup and the cache save lane also ask, so none of them
	// can disagree about which column this is.
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	if config.IsStatusColumn(col.Key, col.Title, lifecycleKey) {
		if phrase := domain.StatusPhrase(r.Findings); phrase != "" {
			return phrase
		}
		if v, ok := r.Fields[lifecycleKey]; ok && v != "" {
			return domain.HumanizeStatusPhrase(v)
		}
		if v, ok := r.Fields["status"]; ok && v != "" {
			return domain.HumanizeStatusPhrase(v)
		}
		if col.Key != "" {
			if v, ok := r.Fields[col.Key]; ok && v != "" {
				return domain.HumanizeStatusPhrase(v)
			}
		}
		if col.Path != "" && r.RawStruct != nil {
			return domain.HumanizeStatusPhrase(fieldpath.ExtractScalar(r.RawStruct, col.Path))
		}
		return ""
	}

	// Fields map (key-based) takes priority.
	if col.Key != "" {
		if v, ok := r.Fields[col.Key]; ok && v != "" {
			return humanizeListCell(col, v)
		}
	}

	// Path-based fallback via fieldpath.ExtractScalar. col.Humanize opts a
	// non-status enum column (e.g. acm's Type: "AMAZON_ISSUED") into the same
	// HumanizeStatusPhrase chokepoint the isStatusCol branch above uses,
	// without touching identity-column RawStruct-over-Fields precedence
	// (e.g. EC2 InstanceType, which never sets Humanize).
	if col.Path != "" && r.RawStruct != nil {
		if val := fieldpath.ExtractScalar(r.RawStruct, col.Path); val != "" {
			return humanizeListCell(col, val)
		}
	}

	// Second-pass: accept explicit empty-string values stored in Fields.
	if col.Key != "" {
		if v, ok := r.Fields[col.Key]; ok {
			return humanizeListCell(col, v)
		}
	}

	// Title match, as an ordered preference (config.TitleFieldKeys states the
	// order and why). Accepting either spelling inside a single `range` over
	// Fields makes the cell depend on map order whenever both are present.
	for _, want := range config.TitleFieldKeys(col.Title) {
		for k, v := range r.Fields {
			if strings.EqualFold(k, want) {
				return humanizeListCell(col, v)
			}
		}
	}

	// Name fallback: the row's name belongs in the one column that names the
	// row, and nowhere else. A struct-less row (a warm-cache replay, a
	// degraded fetch) leaves every other path-only column with nothing to
	// say, and blank is what it has to say.
	if r.Name != "" && col.Identity {
		return r.Name
	}

	return ""
}

// humanizeListCell applies domain.HumanizeStatusPhrase when col.Humanize is
// set, so a warm-cache row (RawStruct stripped, value materialized in
// r.Fields) renders the same humanized phrase a live row gets via the
// RawStruct+Humanize branch — the RawStruct/Fields cascade must never change
// what the cell shows, only where the raw value came from.
func humanizeListCell(col ColumnDef, v string) string {
	if col.Humanize {
		return domain.HumanizeStatusPhrase(v)
	}
	return v
}

// hasWave2Finding reports whether findings already contains a Wave-2 entry
// (Source prefixed "wave2:"). Used by buildListBody's S4 status-cell override
// to detect when applyWave2ToRow has already mutated r.Findings directly
// (the demo path and the internal/tui fold-layer live path both do this) —
// in that case extractListCells has already derived the correct, possibly
// stacked ("<top> (+N)") status cell from r.Findings, and the single-Finding
// enrichment-store map must not override it.
func hasWave2Finding(findings []domain.Finding) bool {
	for _, f := range findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			return true
		}
	}
	return false
}

// resolveListDecoratorFull mirrors the marker logic in renderDataRow and extends it
// ("healthy", "warning", "broken", "dim", "") so RenderList can reproduce
// the exact lipgloss.Style that View() derives from td.ResolveColor(r).
//
// Per cache contract C6: a row's OWN persisted findings (r.Findings — what a cold-boot
// reseed from cache.Row.Findings populates, and what the demo/live fold
// layer mutates directly) are consulted FIRST, before falling back to the
// findings map (the session-scoped Wave-2 enrichment store, which is empty
// until a live enrichment probe lands this session). Without this, a
// seeded row's persisted Findings never drive render-time severity — the
// map lookup alone only ever hits after a live probe re-confirms the same
// finding, so a freshly cold-booted list shows no glyph/severity on
// flagged rows until the sweep completes. r.Findings is assumed
// pre-ordered by severity (the same convention domain.StatusPhrase
// relies on via findings[0]), so the first entry is "the top" finding. A
// resource may carry more than one independently-evaluated Wave-2
// condition in the findings map's per-ID slice; the decorator reduces to
// the WORST-severity one via domain.WorstSeverityFinding.
func resolveListDecoratorFull(td *resource.ResourceTypeDef, r resource.Resource, findings map[string][]domain.Finding) (RowDecorator, string, string) {
	if td == nil {
		return DecoratorNormal, "", ""
	}
	color := td.ResolveColor(r)
	colorTag := colorToTag(color)
	if color == resource.ColorHealthy {
		if len(r.Findings) > 0 {
			switch r.Findings[0].Severity {
			case domain.SevBroken:
				return DecoratorError, "broken", colorTag
			case domain.SevWarn:
				return DecoratorWarning, "warn", colorTag
			}
		} else if fs, ok := findings[r.ID]; ok && len(fs) > 0 {
			switch domain.WorstSeverityFinding(fs).Severity {
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

// IdentityColumnIndex returns the 0-based index in columns of the type's
// identity column — the one column that names the row. It is the only
// definition of "this is the name column"; nothing may re-derive it from a
// substring of a key, a title or a path.
func IdentityColumnIndex(columns []ColumnDef, td *resource.ResourceTypeDef) int {
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
	// Step 3: column title equals "Name" (case-insensitive) or the type's display name.
	for i, c := range columns {
		if strings.EqualFold(c.Title, "Name") || (td != nil && strings.EqualFold(c.Title, td.Name)) {
			return i
		}
	}
	// Step 4: fall back to index 0.
	return 0
}

// resolveListStatusCol mirrors the statusColIdx resolution in resourcelist.go
// View(): the column whose key is "status" or the type's LifecycleKey, else a
// case-insensitive "State"/"Status" title match. Returns -1 when no status
// column exists. buildListBody uses it to bake the issue-Finding Phrase (S4)
// into the status cell so the ViewState is render-ready — the TUI does this
// override at render time, but the web renders Cells verbatim, so it must live
// in the ViewState for both renderers (and the enrichment findings map, not the
// resource's embedded Wave-1 Findings, is the authoritative source).
func resolveListStatusCol(columns []ColumnDef, td *resource.ResourceTypeDef) int {
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	for i, c := range columns {
		if c.Key == "status" || c.Key == lifecycleKey {
			return i
		}
	}
	for i, c := range columns {
		if strings.EqualFold(c.Title, "State") || strings.EqualFold(c.Title, "Status") {
			return i
		}
	}
	return -1
}

// ResolveColumnsForType resolves the column set for typeName using this
// controller's viewConfig and fallbackTypeDefs, so that handleSortByCol and
// buildListBody always agree on the column set — and therefore on what key
// "N" maps to.
func (c *Controller) ResolveColumnsForType(typeName string) []ColumnDef {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.resolveColumnsLocked(typeName)
}

// resolveColumnsLocked is the lock-free core of ResolveColumnsForType, and the
// controller's single answer to "what are this type's list columns". Callers
// MUST already hold c.mu — SaveColumnsForType runs under the caller's write
// lock, and Go's RWMutex is not reentrant.
func (c *Controller) resolveColumnsLocked(typeName string) []ColumnDef {
	return resolveListColumnsForBuild(c.viewConfig, typeName, c.typeDefForLocked(typeName))
}

// typeDefForLocked resolves the typeDef behind a short name: a registered
// fallback first (a test or a host may override a catalog type), then the
// catalog parent, then the registered child. The child rung is load-bearing —
// internal/tui's app_stack and views/resourcelist both reach the column
// resolver with a ShortName that may be a child's, and a child that resolves
// no typeDef resolves no columns, so the column a sort names is never found.
// Callers MUST already hold c.mu.
func (c *Controller) typeDefForLocked(shortName string) *resource.ResourceTypeDef {
	if fv, ok := c.fallbackTypeDefs[shortName]; ok {
		return &fv
	}
	if td := resource.FindResourceType(shortName); td != nil {
		return td
	}
	return resource.GetChildType(shortName)
}
