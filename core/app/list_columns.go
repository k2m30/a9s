// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// MaterializeListFields returns a copy of r with Fields populated for every
// Path-based column in columns, extracting the scalar via
// fieldpath.ExtractScalar(r.RawStruct, col.Path) and writing it under the key
// that column's cell is read from — its own Key, or its title key
// (config.TitleFieldKey, the spelling the extraction cascade reads first) when
// it has none. This is the generic render-sufficiency step Contract B
// requires: running it once, before a fetch result is cached or a screen's
// rows are stored, means a later cache replay with RawStruct stripped renders
// identical cells, without falling back to fieldpath on a nil RawStruct.
//
// A column carrying BOTH a Key and a Path is materialized too. It is the
// ordinary shape since ResolveListColumnCascade started merging the catalog's
// Key onto the defaults' Path, and skipping it left the cached row blank
// wherever the fetcher had not written that key itself. The clobbering worry
// that kept keyed columns out — a Wave-2 override fieldpath cannot see — is
// already answered by the guard below: a key that already holds a non-empty
// value is never written.
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
		if col.Path == "" {
			continue
		}
		key := col.Key
		if key == "" {
			key = config.TitleFieldKey(col.Title)
		}
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
	// The type's own declaration, not the column's: see
	// ResourceTypeDef.HumanizeFields. A column reads it; it does not own it.
	humanized := map[string]bool(nil)
	if td != nil {
		humanized = td.HumanizedFields()
	}
	cols := make([]ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = ColumnDef{
			Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path,
			Humanize: catalog.Humanizes(humanized, lc.Key, lc.Path),
			SortKey:  lc.SortKey,
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
	return config.CanonicalValue(extractCellText(col, td, r))
}

// extractCellText resolves which of the row's several representations answers
// for this column. It says where the value came from; ExtractCellValue, the
// one exit, says how it reads.
func extractCellText(col ColumnDef, td *resource.ResourceTypeDef, r resource.Resource) string {
	if col.Key == "@id" {
		return r.ID
	}

	// Status/lifecycle column — OWNER CONTRACT: the type DECLARES its status
	// column by giving it the type's lifecycle key (config.IsStatusColumn),
	// and this reads that key and no other. It used to read the lifecycle
	// key, then "status", then the column's own key, so a type declaring its
	// status under its own name — tg's health_summary, cb's last_build —
	// showed whichever other spelling was in Fields and its own declaration
	// came third.
	//
	// A finding still outranks every stored value: the phrase is what the
	// operator is being told about this row. Below it, one key, then the
	// struct. Every qualifying column routes through the same
	// domain.StatusPhrase + HumanizeStatusPhrase chokepoint — otherwise it
	// falls through to the raw fieldpath/Fields value further down and a raw
	// AWS enum ("FAILED", "STALE") reaches the screen.
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	if config.IsStatusColumn(col.Key, col.Title, lifecycleKey) {
		if phrase := domain.StatusPhrase(r.Findings); phrase != "" {
			return phrase
		}
		if v, ok := r.Fields[col.Key]; ok && v != "" {
			return domain.HumanizeStatusPhrase(v)
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
	//
	// Within ONE preference the smallest matching key wins, for the same
	// reason: a warm-cache row can hold two keys differing only by case
	// ("Time" beside "time"), both satisfy the fold, and a bare `range` would
	// show whichever one map order reached first — a different cell on two
	// consecutive starts.
	for _, want := range config.TitleFieldKeys(col.Title) {
		best, found := "", false
		for k := range r.Fields {
			if strings.EqualFold(k, want) && (!found || k < best) {
				best, found = k, true
			}
		}
		if found {
			return humanizeListCell(col, r.Fields[best])
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

// humanizeListCell applies the TYPE's declaration as it was resolved onto this
// column: col.Humanize is set by resolveListColumnsForBuild from
// ResourceTypeDef.HumanizeFields, and says that the field this column reads
// carries an AWS constant an operator should not have to read. The column
// carries the answer; it does not own the question, which is why the detail
// row for the same field reads the same words. What every value reads like
// regardless of its column is ExtractCellValue's exit, not this.
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

// resolveListRowSeverity returns the row's issue-severity tag and its
// pre-resolved colour tag ("healthy", "warning", "broken", "dim", "") so
// RenderList can reproduce the exact lipgloss.Style that View() derives from
// td.ResolveColor(r).
func resolveListRowSeverity(td *resource.ResourceTypeDef, r resource.Resource) (string, string) {
	if td == nil {
		return "", ""
	}
	color := td.ResolveColor(r)
	colorTag := colorToTag(color)
	sev := ""
	if color.IsIssue() {
		sev = "issue"
	}
	return sev, colorTag
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

// resolveListStatusCol finds the type's declared status column — the one
// naming its lifecycle key, which config.IsStatusColumn is the one answer to.
// Returns -1 when no status column exists. buildListBody uses it to bake the
// issue-Finding Phrase (S4) into the status cell so the ViewState is
// render-ready — the TUI does this override at render time, but the web
// renders Cells verbatim, so it must live in the ViewState for both renderers
// (and the enrichment findings map, not the resource's embedded Wave-1
// Findings, is the authoritative source).
func resolveListStatusCol(columns []ColumnDef, td *resource.ResourceTypeDef) int {
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	for i, c := range columns {
		if config.IsStatusColumn(c.Key, c.Title, lifecycleKey) {
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
	_, columns := c.listColumnsAndTypeLocked(typeName)
	return columns
}

// listColumnsAndTypeLocked resolves a screen's typeDef and its column set
// together, because they are one answer: the columns come FROM the typeDef.
// Resolving them apart is how a screen renders one type's columns and filters
// with another's — the shape row 39 removed at the typeDef and this removes at
// the column set. Callers MUST already hold c.mu.
func (c *Controller) listColumnsAndTypeLocked(typeName string) (*resource.ResourceTypeDef, []ColumnDef) {
	td := c.typeDefForLocked(typeName)
	return td, resolveListColumnsForBuild(c.viewConfig, typeName, td)
}

// typeDefForLocked resolves the typeDef behind a short name: a registered
// fallback first (a test or a host may override a catalog type), then the
// catalog parent, then the registered child. The child rung is load-bearing —
// internal/tui's app_stack and views/resourcelist both reach the column
// resolver with a ShortName that may be a child's, and a child that resolves
// no typeDef resolves no columns, so the column a sort names is never found.
//
// It is the ONE resolution a screen gets. The columns it renders, the columns
// its filter compares, the classifier its issue count asks and the child key
// its footer offers all come from this typeDef, so a screen cannot render one
// type's columns and filter with another's. Callers MUST already hold c.mu.
func (c *Controller) typeDefForLocked(shortName string) *resource.ResourceTypeDef {
	if fv, ok := c.fallbackTypeDefs[shortName]; ok {
		return &fv
	}
	if td := resource.FindResourceType(shortName); td != nil {
		return td
	}
	return resource.GetChildType(shortName)
}

// SortColKey is the one identifier a sort names a column by. Both directions
// of the list-view cache round trip ask here — leaving a list translates the
// active sort's key into the column's index, re-entering translates the index
// back into a key — so a key saved on one side always finds the same column on
// the other. A second spelling anywhere means a sort the operator set is
// dropped without a word when they come back to the list.
//
// The key is the column's Key, or its Title when it has none. Path is not in
// the formula: two columns of one view legitimately read the same RawStruct
// path (sns shows a topic's ARN under both "Topic Name" and "Topic ARN"), so a
// path names a column ambiguously, while a title is a view file's YAML mapping
// key and is unique by construction.
func (c ColumnDef) SortColKey() string {
	if c.Key != "" {
		return c.Key
	}
	return c.Title
}

// SortColIndex returns the index in columns of the column SortColKey names, or
// -1 when no column answers to key.
func SortColIndex(columns []ColumnDef, key string) int {
	if key == "" {
		return -1
	}
	for i, c := range columns {
		if c.SortColKey() == key {
			return i
		}
	}
	return -1
}
