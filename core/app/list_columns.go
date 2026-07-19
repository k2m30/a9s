// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
)

// MaterializeListFields returns a copy of r with Fields populated for every
// Path-based, Key-less column in columns, extracting the scalar via
// fieldpath.ExtractScalar(r.RawStruct, col.Path) and writing it under the
// column's resolved Fields key (col.Key when non-empty, else the lowercased
// Title). This is the generic render-sufficiency step Contract B requires:
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
		key := strings.ToLower(col.Title)
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
// shared config.ResolveListColumnCascade, using the caller-supplied td
// (already resolved fallback-first) for the superset first-column-title
// check. This ensures that custom test typeDefs sharing a ShortName with a
// catalog type but having a different column layout (e.g. rlTestTypeDef
// starts with "Instance ID" not "Name") do not get silently switched to the
// built-in 9-column defaults.
func resolveListColumnsForBuild(vc *config.ViewsConfig, typeName string, td *resource.ResourceTypeDef) []ColumnDef {
	lcs := resource.ResolveListColumnCascade(vc, typeName, td)
	cols := make([]ColumnDef, len(lcs))
	for i, lc := range lcs {
		cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
	}
	return cols
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
			if dec := lookupListDecorator(td.CellDecorators, col, td.LifecycleKey); dec != nil {
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
//
// A status-qualifying column (same predicate as listExtractCellValue's isStatusCol)
// additionally tries lifecycleKey (the type's LifecycleKey, "" meaning "state" per
// listExtractCellValue's own default) and the literal "state" as decorator keys,
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
	isStatusCol := col.Key == "status" || col.Key == lifecycleKey ||
		strings.EqualFold(col.Title, "status") || strings.EqualFold(col.Title, "state")
	if isStatusCol {
		if d, ok := decs[lifecycleKey]; ok {
			return d
		}
		if d, ok := decs["state"]; ok {
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

	// Status/lifecycle column — OWNER CONTRACT: a column whose Title equals
	// "Status" (case-insensitive, exact word) IS the status column,
	// regardless of its Key. This covers keyed columns whose data lives
	// under their own key (e.g. cb's {Key:"last_status", Title:"Status"},
	// tg's {Key:"health_summary", Title:"Status"}, sg's
	// {Key:"risk_summary", Title:"Status"}) as well as the pre-existing
	// Key-less, Path-based case (e.g. lambda/ec2's per-session view
	// {Title:"State", Path:"State"} with no Key, or acm/eks/ng's
	// default-view {Path:"Status"} with no Key). Every qualifying column
	// must route through the same phraseFromFindings + HumanizeStatusPhrase
	// chokepoint — otherwise it falls through to the raw fieldpath/Fields
	// value further down and a raw AWS enum (e.g. "FAILED", "STALE")
	// reaches the screen. The Key==status/lifecycleKey checks are kept for
	// defensive parity with columns whose Title doesn't literally say
	// "Status"/"State". The title check mirrors resolveListStatusCol's own
	// cascade so the two functions never disagree about which column is
	// the status column.
	lifecycleKey := "state"
	if td != nil && td.LifecycleKey != "" {
		lifecycleKey = td.LifecycleKey
	}
	isStatusCol := col.Key == "status" || col.Key == lifecycleKey ||
		strings.EqualFold(col.Title, "status") || strings.EqualFold(col.Title, "state")
	if isStatusCol {
		if phrase := listPhraseFromFindings(r.Findings); phrase != "" {
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

	// Title-match loop: lowercased title and space→underscore variant against Fields keys.
	titleLower := strings.ToLower(col.Title)
	titleUnder := strings.ReplaceAll(titleLower, " ", "_")
	for k, v := range r.Fields {
		kl := strings.ToLower(k)
		if kl == titleLower || kl == titleUnder {
			return humanizeListCell(col, v)
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

// listPhraseFromFindings mirrors phraseFromFindings in table_render.go, with
// one addition: SevDim findings are included as the lowest-priority phrase
// source. An issue-severity (SevWarn/SevBroken) finding always wins over a
// SevDim one regardless of slice order, so a row whose only findings are dim
// (e.g. ct-events "routine event", lambda.state.inactive, sns-sub.state.deleted)
// still gets a Status-cell phrase instead of falling through to the raw
// lifecycle field — dim rows are a state, not a problem, so they must not
// count as issues (Attention filter, menu badges, unifiedIssueCount all stay
// severity-gated via domain.Severity.IsIssue()), but the cell should still
// explain why the row is dim.
func listPhraseFromFindings(findings []domain.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	top := 0
	for i, f := range findings {
		if f.Severity.IsIssue() {
			top = i
			break
		}
	}
	if len(findings) == 1 {
		return findings[top].Phrase
	}
	return findings[top].Phrase + " (+" + strconv.Itoa(len(findings)-1) + ")"
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
// DEF-3/C6: a row's OWN persisted findings (r.Findings — what a cold-boot
// reseed from cache.Row.Findings populates, and what the demo/live fold
// layer mutates directly) are consulted FIRST, before falling back to the
// findings map (the session-scoped Wave-2 enrichment store, which is empty
// until a live enrichment probe lands this session). Without this, a
// seeded row's persisted Findings never drive render-time severity — the
// map lookup alone only ever hits after a live probe re-confirms the same
// finding, so a freshly cold-booted list shows no glyph/severity on
// flagged rows until the sweep completes. r.Findings is assumed
// pre-ordered by severity (the same convention listPhraseFromFindings
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
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
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
				cols[i] = ColumnDef{Key: lc.Key, Title: lc.Title, Width: lc.Width, Path: lc.Path, Humanize: lc.Humanize}
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
