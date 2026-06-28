package app

import (
	"context"
	"maps"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)


// listPageSize is the default cursor jump for PageUp/PageDown on a list screen
// when the renderer does not supply a viewport page size via Action.N.
const listPageSize = 10

// topListState returns the ListState of the top-of-stack screen when the top
// screen is ScreenResourceList or ScreenChildList, nil otherwise.
func (c *Controller) topListState() *ListState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenResourceList && top.ID != runtime.ScreenChildList {
		return nil
	}
	return c.stack[len(c.stack)-1].State.List
}

// ensureListState ensures the top list screen has an initialised ListState
// (push via ApplyIntents only sets the Screen; State.List starts nil). Called
// lazily from applyNavResult after a PushScreen so that action handlers never
// dereference a nil ListState on a freshly-pushed screen.
func (c *Controller) ensureListState() {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenResourceList && top.ID != runtime.ScreenChildList {
		return
	}
	if top.State.List == nil {
		top.State.List = &ListState{Loading: true}
	}
}

// listPageSizeFor returns the page size for a list PageUp/PageDown action.
func listPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return listPageSize
}

// listVisibleCount returns the number of rows visible in the current list
// state (after filter/attention/relatedIDSet). Reads rows from ls.Rows so
// that two stacked screens of the same type stay independent.
func (c *Controller) listVisibleCount(ls *ListState) int {
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType
	if typeName == "" {
		return 0
	}
	resources := c.listScreenResources(ls, typeName)
	visible := c.applyListFilters(ls, typeName, resources)
	return len(visible)
}

// cachedResources returns the resource slice for typeName from the controller's
// type-keyed resource cache, or nil if no data has been received yet.
// Callers that have a per-screen ListState should prefer listScreenResources
// so that stacked same-type screens read their own rows, not a shared slice.
func (c *Controller) cachedResources(typeName string) []resource.Resource {
	if c.resourceCache == nil {
		return nil
	}
	return c.resourceCache[typeName]
}

// listScreenResources returns the resource slice for the given screen's
// ListState.  When ls.Rows is non-nil (the normal case after
// applyResourcesLoaded writes per-screen rows), it is returned directly —
// guaranteeing that two stacked list screens of the same type see their own
// independent row sets.  When ls.Rows is nil (e.g. the screen was freshly
// pushed and no fetch has completed yet), the call falls back to the
// type-keyed cache so that callers that only have a typeName (e.g.
// GetListAllResources, ApplyListFieldUpdates) still work correctly.
func (c *Controller) listScreenResources(ls *ListState, typeName string) []resource.Resource {
	if ls != nil && ls.Rows != nil {
		return ls.Rows
	}
	return c.cachedResources(typeName)
}

// applyListFilters applies the relatedIDSet prefilter, text filter, and
// attention filter to base, returning the visible subset. Mirrors
// ResourceListModel.applyFilter exactly so ListSelected and buildListBody
// agree on which row is "selected".
func (c *Controller) applyListFilters(ls *ListState, typeName string, base []resource.Resource) []resource.Resource {
	td := resource.FindResourceType(typeName)
	if td == nil {
		if fv, ok := c.fallbackTypeDefs[typeName]; ok {
			td = &fv
		}
	}

	// RelatedIDSet prefilter: when non-nil (even if empty), only IDs in the set pass.
	if ls.RelatedIDSet != nil {
		subset := make([]resource.Resource, 0, len(ls.RelatedIDSet))
		for _, r := range base {
			if _, ok := ls.RelatedIDSet[r.ID]; ok {
				subset = append(subset, r)
			}
		}
		base = subset
	}

	// Text filter — matches r.ID, r.Name, r.Fields values, r.Findings[i].Phrase.
	result := listFilterResources(ls.Filter, base)

	// Attention filter: mirrors ResourceListModel.applyFilter §7.
	if ls.AttentionOnly && td != nil {
		findings := c.listEnrichmentFindings(typeName)
		kept := make([]resource.Resource, 0, len(result))
		for _, r := range result {
			if listHasIssueFinding(r) {
				kept = append(kept, r)
				continue
			}
			if len(r.Findings) == 0 {
				if td.ResolveColor(r).IsIssue() {
					kept = append(kept, r)
					continue
				}
				if _, hasFinding := findings[r.ID]; hasFinding {
					kept = append(kept, r)
				}
			}
		}
		result = kept
	}

	return result
}

// listFilterResources is the pure text-filter; mirrors FilterResources in views.
func listFilterResources(query string, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.ID), q) ||
			strings.Contains(strings.ToLower(r.Name), q) {
			result = append(result, r)
			continue
		}
		matched := false
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, r)
			continue
		}
		for _, f := range r.Findings {
			if strings.Contains(strings.ToLower(f.Phrase), q) {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

// listHasIssueFinding mirrors hasIssueFinding in views.
func listHasIssueFinding(r resource.Resource) bool {
	for _, f := range r.Findings {
		if resource.IsIssueSeverity(f.Severity) {
			return true
		}
	}
	return false
}

// listSortResources sorts resources by ls.SortCol/SortDir, mirroring
// sortFiltered in views/sort.go. No-op when SortCol is empty.
// vc is the per-session view config (nil = built-in defaults only); passing
// the controller's viewConfig ensures user-configured sort_key / sort_path
// columns resolve correctly — matching what buildListBody and resourcelist.go
// do (Bug 3 fix).
func listSortResources(vc *config.ViewsConfig, ls *ListState, typeName string, resources []resource.Resource) []resource.Resource {
	if ls.SortCol == "" || len(resources) == 0 {
		return resources
	}

	// Resolve columns from viewConfig first (same priority as buildListBody /
	// resolveColumns in table_render.go) so custom sort_key / sort_path columns
	// are found even when they are not in the built-in defaults.
	vd := config.GetViewDef(vc, typeName)
	if len(vd.List) == 0 {
		vd = config.GetViewDef(nil, typeName)
	}
	var col *config.ListColumn
	sortColLower := strings.ToLower(ls.SortCol)
	for i := range vd.List {
		lc := &vd.List[i]
		if lc.Key == ls.SortCol || lc.Path == ls.SortCol {
			col = lc
			break
		}
		titleUnder := strings.ToLower(strings.ReplaceAll(lc.Title, " ", "_"))
		if titleUnder == sortColLower {
			col = lc
			break
		}
	}

	sortAsc := ls.SortDir != "desc"
	out := make([]resource.Resource, len(resources))
	copy(out, resources)

	sort.SliceStable(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		// Raw struct comparison (numeric/time) when a sortPath or path is present.
		rawPath := ""
		if col != nil {
			rawPath = col.SortPath
			if rawPath == "" {
				rawPath = col.Path
			}
		}
		if rawPath != "" && a.RawStruct != nil && b.RawStruct != nil {
			if cmp, ok := listCompareRaw(a.RawStruct, b.RawStruct, rawPath); ok {
				if sortAsc {
					return cmp < 0
				}
				return cmp > 0
			}
		}

		// Display-value fallback.
		var va, vb string
		if col != nil && col.SortKey != "" {
			va = a.Fields[col.SortKey]
			vb = b.Fields[col.SortKey]
		} else {
			sortColDef := ColumnDef{Key: ls.SortCol}
			if col != nil {
				sortColDef = ColumnDef{Key: col.Key, Title: col.Title, Path: col.Path}
			}
			td := resource.FindResourceType(typeName)
			va = listExtractCellValue(sortColDef, td, a)
			vb = listExtractCellValue(sortColDef, td, b)
		}
		if fa, err := strconv.ParseFloat(va, 64); err == nil {
			if fb, err := strconv.ParseFloat(vb, 64); err == nil {
				if sortAsc {
					return fa < fb
				}
				return fa > fb
			}
		}
		if sortAsc {
			return va < vb
		}
		return va > vb
	})
	return out
}

// listCompareRaw mirrors compareRaw from views/sort.go.
func listCompareRaw(a, b any, path string) (int, bool) {
	va, errA := fieldpath.ExtractValue(a, path)
	vb, errB := fieldpath.ExtractValue(b, path)
	if errA != nil || errB != nil {
		return 0, false
	}
	// Dereference pointers.
	for va.Kind() == reflect.Pointer {
		if va.IsNil() {
			return 0, false
		}
		va = va.Elem()
	}
	for vb.Kind() == reflect.Pointer {
		if vb.IsNil() {
			return 0, false
		}
		vb = vb.Elem()
	}
	// time.Time comparison.
	if va.Type() == reflect.TypeFor[time.Time]() && vb.Type() == reflect.TypeFor[time.Time]() {
		return va.Interface().(time.Time).Compare(vb.Interface().(time.Time)), true
	}
	// Numeric comparison.
	fa, okA := listToFloat(va)
	fb, okB := listToFloat(vb)
	if okA && okB {
		if fa < fb {
			return -1, true
		}
		if fa > fb {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// listToFloat mirrors toFloat from views/sort.go.
func listToFloat(v reflect.Value) (float64, bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	default:
		return 0, false
	}
}

// applyResourcesLoaded stores a page of resources per-screen (on ls.Rows) and
// also in the type-keyed resourceCache for callers that only have a typeName
// (ApplyListFieldUpdates, GetListAllResources). When appendPage is true the
// new slice is appended; otherwise it replaces. Mirrors what
// ResourceListModel.Update does on ResourcesLoaded.
// Called from handleResourcesLoadedEvent (via Handle) and from the public
// ApplyResourcesLoaded test seam in testing.go.
func (c *Controller) applyResourcesLoaded(ls *ListState, typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
	// --- Per-screen storage (Bug 1 fix) -----------------------------------
	// Writing to ls.Rows ensures that two stacked list screens of the same
	// resource type never share a row slice. Each screen's fetch result lands
	// exclusively on that screen's ListState.
	if ls != nil {
		if appendPage {
			ls.Rows = append(ls.Rows, resources...)
		} else {
			ls.Rows = resources
		}
	}

	// --- Type-keyed cache (retained for field-update / all-resources reads) --
	// ApplyListFieldUpdates and GetListAllResources key on typeName, not on a
	// specific screen. Keep them in sync so those callers still work correctly.
	if c.resourceCache == nil {
		c.resourceCache = make(map[string][]resource.Resource)
	}
	if appendPage {
		c.resourceCache[typeName] = append(c.resourceCache[typeName], resources...)
	} else {
		c.resourceCache[typeName] = resources
	}

	if ls != nil {
		ls.Loading = false
		ls.LoadingMore = false
		if pagination != nil {
			ls.HasPagination = pagination.IsTruncated
			ls.PaginationCursor = pagination.NextToken
		} else {
			ls.HasPagination = false
			ls.PaginationCursor = ""
		}
	}
}

// ApplyEnrichmentState stores Wave-2 enrichment results for typeName.
// Mirrors ResourceListModel.SetEnrichmentState.
func (c *Controller) ApplyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string]domain.Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyEnrichmentState(typeName, issueCount, truncated, findings)
}

// applyEnrichmentState is the lock-free implementation of ApplyEnrichmentState.
// Callers must hold c.mu (write).
func (c *Controller) applyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string]domain.Finding) {
	if c.enrichmentStore == nil {
		c.enrichmentStore = make(map[string]map[string]domain.Finding)
	}
	if c.enrichmentTruncated == nil {
		c.enrichmentTruncated = make(map[string]bool)
	}
	c.enrichmentStore[typeName] = findings
	c.enrichmentTruncated[typeName] = truncated
	_ = issueCount // retained for caller parity; issue count is recomputed in buildListBody
}

// listEnrichmentFindings returns the per-resource finding map for typeName, or nil.
func (c *Controller) listEnrichmentFindings(typeName string) map[string]domain.Finding {
	if c.enrichmentStore == nil {
		return nil
	}
	return c.enrichmentStore[typeName]
}

// PatchListRelatedIDSet sets the relatedIDSet on the top list screen.
func (c *Controller) PatchListRelatedIDSet(ids []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	if len(ids) == 0 {
		ls.RelatedIDSet = nil
		return
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	ls.RelatedIDSet = set
}

// reapplyCheckerStore holds the per-type reapply checker + source resource for
// approximate-pivot navigations. Keyed by resource type short name.
type reapplyCheckerEntry struct {
	checker resource.RelatedChecker
	source  resource.Resource
}

// PatchListReapplyChecker registers a RelatedChecker + source resource for the
// top list screen's resource type. When non-nil, subsequent applyResourcesLoaded
// calls re-run the checker to extend RelatedIDSet with newly matched IDs.
func (c *Controller) PatchListReapplyChecker(checker resource.RelatedChecker, src resource.Resource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType
	if typeName == "" {
		return
	}
	if c.reapplyCheckers == nil {
		c.reapplyCheckers = make(map[string]reapplyCheckerEntry)
	}
	if checker == nil {
		delete(c.reapplyCheckers, typeName)
		return
	}
	c.reapplyCheckers[typeName] = reapplyCheckerEntry{checker: checker, source: src}
	// Mirror SetReapplyChecker: activate filter with empty set so zero-match
	// navigations hide all rows immediately rather than showing an unfiltered list.
	if ls.RelatedIDSet == nil {
		ls.RelatedIDSet = make(map[string]struct{})
	}
}

// ApplyReapplyCheckerAgainst re-runs the stored checker for the top list
// screen's resource type against newPage and merges matched IDs into
// RelatedIDSet. This is the public entry point called by
// ResourceListModel.ReapplyCheckerAgainst — the controller owns the actual
// merge logic in reapplyCheckerAgainst.
func (c *Controller) ApplyReapplyCheckerAgainst(newPage []resource.Resource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	if len(c.stack) == 0 {
		return
	}
	typeName := c.stack[len(c.stack)-1].Ctx.ResourceType
	c.reapplyCheckerAgainst(ls, typeName, newPage)
}

// reapplyCheckerAgainst re-runs the stored checker for typeName against newPage
// and merges returned IDs into ls.RelatedIDSet. Mirrors ReapplyCheckerAgainst.
func (c *Controller) reapplyCheckerAgainst(ls *ListState, typeName string, newPage []resource.Resource) {
	if c.reapplyCheckers == nil {
		return
	}
	entry, ok := c.reapplyCheckers[typeName]
	if !ok || entry.checker == nil || len(newPage) == 0 {
		return
	}
	synth := resource.ResourceCache{
		typeName: resource.ResourceCacheEntry{Resources: newPage},
	}
	result := entry.checker(context.Background(), nil, entry.source, synth)
	if len(result.ResourceIDs) == 0 {
		return
	}
	if ls.RelatedIDSet == nil {
		ls.RelatedIDSet = make(map[string]struct{}, len(result.ResourceIDs))
	}
	for _, id := range result.ResourceIDs {
		if id != "" {
			ls.RelatedIDSet[id] = struct{}{}
		}
	}
}

// buildListBody constructs a ListBody from the top list screen's ListState and
// the controller's resource + enrichment caches. Mirrors applySortAndFilter +
// the row/cell extraction in ResourceListModel — producing the same logical
// rows/cells/order/decorators so RenderList parity holds.
func (c *Controller) buildListBody(ctx runtime.ScreenContext, ls *ListState) *ListBody {
	typeName := ctx.ResourceType

	// Resolve typeDef: prefer the fallback (registered via RegisterFallbackTypeDef
	// from the model constructor) over the catalog. The model's typeDef is the
	// authoritative Color classifier and column layout source. For test typeDefs
	// that share a ShortName with a catalog type but have a different Color func
	// or column set, the fallback must win. The catalog is consulted only as a
	// last resort when no fallback is registered.
	var tdVal resource.ResourceTypeDef
	var td *resource.ResourceTypeDef
	if ftd, ok := c.fallbackTypeDefs[typeName]; ok {
		tdVal = ftd
		td = &tdVal
	} else if catalogTD := resource.FindResourceType(typeName); catalogTD != nil {
		td = catalogTD
	}

	// Resolve column definitions mirroring resolveColumns() in table_render.go,
	// using the already-resolved fallback td (not the catalog) for the superset
	// first-column-title check. This ensures test typeDefs with non-standard
	// first columns (e.g. rlTestTypeDef starts with "Instance ID" not "Name")
	// are not silently switched to the 9-column built-in defaults.
	columns := resolveListColumnsForBuild(c.viewConfig, typeName, td)

	// Build the row set from the per-screen store (Bug 1 fix: uses ls.Rows when
	// available so two stacked same-type screens see their own independent rows).
	allResources := c.listScreenResources(ls, typeName)

	// Apply filters (relatedIDSet → text → attention).
	visible := c.applyListFilters(ls, typeName, allResources)

	// Apply sort using viewConfig so user-configured sort_key/sort_path columns
	// resolve correctly (Bug 3 fix).
	visible = listSortResources(c.viewConfig, ls, typeName, visible)

	// Clamp selected row.
	selected := ls.SelectedRow
	if len(visible) > 0 && selected >= len(visible) {
		selected = len(visible) - 1
	}
	if selected < 0 {
		selected = 0
	}

	// Enrichment data.
	findings := c.listEnrichmentFindings(typeName)
	enrichTruncated := map[string]bool{}
	if c.enrichmentTruncated != nil {
		maps.Copy(enrichTruncated, c.enrichmentTruncated)
	}

	// Build rows.
	rows := make([]ListRow, 0, len(visible))
	for _, r := range visible {
		cells := extractListCells(columns, r, td)
		decorator, severity, colorTag := resolveListDecoratorFull(td, r, findings)
		rows = append(rows, ListRow{
			Cells:      cells,
			Decorator:  decorator,
			Severity:   severity,
			ResourceID: r.ID,
			Color:      colorTag,
		})
	}

	// Pagination.
	pagination := PaginationInfo{}
	if ls.HasPagination {
		pagination.HasMore = true
		pagination.Cursor = ls.PaginationCursor
	}

	// Resolve the identity column index (full column list, before hscroll),
	// mirroring resolveIdentityColumn in table_render.go.
	markerCol := resolveListMarkerCol(columns, td)

	return &ListBody{
		Columns:             columns,
		Rows:                rows,
		Selected:            selected,
		ScrollX:             ls.ScrollX,
		Filter:              ls.Filter,
		Sort:                SortSpec{Col: ls.SortCol, Dir: ls.SortDir},
		AttentionOnly:       ls.AttentionOnly,
		Loading:             ls.Loading,
		Truncated:           ls.HasPagination,
		Pagination:          pagination,
		EnrichmentFindings:  findings,
		EnrichmentTruncated: enrichTruncated,
		MarkerCol:           markerCol,
		LoadingMore:         ls.LoadingMore,
	}
}



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

	// Status/lifecycle column — two-layer priority (AS-140).
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

// ListFrameTitle mirrors FrameTitle in ResourceListModel.
func (c *Controller) ListFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	top := c.stack[len(c.stack)-1]
	return c.buildListFrameTitle(top.Ctx, ls)
}

// buildListFrameTitle computes the frame title string for a list screen,
// mirroring FrameTitle() in resourcelist.go.
func (c *Controller) buildListFrameTitle(ctx runtime.ScreenContext, ls *ListState) string {
	typeName := ctx.ResourceType
	name := typeName

	td := resource.FindResourceType(typeName)
	if td != nil && td.ListTitle != "" {
		name = td.ListTitle
	}
	if ls.DisplayName != "" {
		name = ls.DisplayName
	}
	if ls.Loading {
		return name
	}

	allResources := c.listScreenResources(ls, typeName)
	total := len(allResources)
	visible := c.applyListFilters(ls, typeName, allResources)
	filtered := len(visible)
	truncated := ls.HasPagination

	totalStr := itoa(total)
	if truncated {
		totalStr = itoa(total) + "+"
	}

	// Loading-more indicator goes inside the count parentheses, mirroring the
	// baseline FrameTitle: "ec2(200+ loading...)" not "ec2(200+) loading...".
	if ls.LoadingMore {
		return name + "(" + totalStr + " loading...)"
	}

	isAttention := ls.AttentionOnly
	hasTextFilter := ls.Filter != "" && filtered != total

	var title string
	switch {
	case hasTextFilter && isAttention:
		title = name + "(" + itoa(filtered) + " of " + totalStr + ")"
	case hasTextFilter:
		title = name + "(" + itoa(filtered) + "/" + totalStr + ")"
	case isAttention:
		title = name + "(" + itoa(filtered) + " of " + totalStr + ")"
	default:
		title = name + "(" + totalStr + ")"
	}

	if ls.TitleSuffix != "" {
		title += ls.TitleSuffix
	}
	if isAttention {
		title += " [!]"
	}
	return title
}

// ListSelected returns the resource at the current cursor position in the
// visible (filtered+sorted) row set, plus a navigable bool (always true when
// a resource is present — row-dependent guards are wired in the flip step).
// The selection index is clamped to the visible count before indexing (Bug 2
// fix) so that a refresh that shrinks the list never leaves the cursor pointing
// past the end, causing Enter/copy to silently do nothing while a row is
// visually highlighted.
func (c *Controller) ListSelected() (resource.Resource, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return resource.Resource{}, false
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType

	allResources := c.listScreenResources(ls, typeName)
	visible := c.applyListFilters(ls, typeName, allResources)
	visible = listSortResources(c.viewConfig, ls, typeName, visible)

	if len(visible) == 0 {
		return resource.Resource{}, false
	}
	// Clamp to match buildListBody so the caller always gets the row that the
	// UI is actually highlighting, even when a concurrent refresh shrinks the
	// visible count below the stored SelectedRow.
	idx := ls.SelectedRow
	if idx >= len(visible) {
		idx = len(visible) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return visible[idx], true
}

// GetListFilter returns the current filter text of the top list screen.
func (c *Controller) GetListFilter() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	return ls.Filter
}

// GetListSort returns the current sort column and direction of the top list screen.
func (c *Controller) GetListSort() (col, dir string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return "", ""
	}
	return ls.SortCol, ls.SortDir
}

// GetListScrollX returns the horizontal scroll offset of the top list screen.
func (c *Controller) GetListScrollX() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return 0
	}
	return ls.ScrollX
}

// GetListSelectedRow returns the selected-row index of the top list screen.
func (c *Controller) GetListSelectedRow() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return 0
	}
	return ls.SelectedRow
}

// GetListAttentionOnly reports whether attention-only mode is active on the
// top list screen.
func (c *Controller) GetListAttentionOnly() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return ls.AttentionOnly
}

// PatchListDisplayName sets the display name override on the top list screen.
func (c *Controller) PatchListDisplayName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.DisplayName = name
}

// PatchListParentContext sets the parent context map on the top list screen.
// Used by child-list navigation to carry the parent resource's identifiers
// (e.g., bucket name, cluster ARN) into fetch and child-routing calls.
func (c *Controller) PatchListParentContext(ctx map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.ParentContext = ctx
}

// GetListAutoOpenSingle reports whether auto-open-single-detail is active
// on the top list screen.
func (c *Controller) GetListAutoOpenSingle() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return ls.AutoOpenSingle
}

// ClearListAutoOpenSingle resets the auto-open-single-detail flag on the
// top list screen.
func (c *Controller) ClearListAutoOpenSingle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.AutoOpenSingle = false
}

// SetListAutoOpenSingle sets the auto-open-single-detail flag on the top
// list screen.
func (c *Controller) SetListAutoOpenSingle(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.AutoOpenSingle = v
}

// GetListExactRelatedTargetID returns the single ID in RelatedIDSet when the
// set has exactly one non-empty entry, mirroring exactRelatedTargetID in views.
func (c *Controller) GetListExactRelatedTargetID() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil || len(ls.RelatedIDSet) != 1 {
		return "", false
	}
	for id := range ls.RelatedIDSet {
		if id == "" {
			return "", false
		}
		return id, true
	}
	return "", false
}

// SetListLoadingMore sets the LoadingMore flag on the top list screen.
func (c *Controller) SetListLoadingMore(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.LoadingMore = v
}

// ClearListLoading clears both the Loading and LoadingMore flags on the top list
// screen. Called when a fetch or load-more operation fails (error handler path)
// so the title reverts from "name loading..." back to the resource count title.
func (c *Controller) ClearListLoading() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.Loading = false
	ls.LoadingMore = false
}

// GetListPaginationCursor returns the pagination cursor of the top list screen.
func (c *Controller) GetListPaginationCursor() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	return ls.PaginationCursor
}

// GetListParentContext returns the parent context map of the top list screen.
func (c *Controller) GetListParentContext() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	return ls.ParentContext
}

// GetListFetchFilter returns the server-side fetch filter of the top list screen.
func (c *Controller) GetListFetchFilter() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	return ls.FetchFilter
}

// PatchListFetchFilter sets the server-side fetch filter on the top list screen.
func (c *Controller) PatchListFetchFilter(filter map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.FetchFilter = filter
}

// PatchListEscPops sets the EscPops flag on the top list screen.
func (c *Controller) PatchListEscPops(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.EscPops = v
}

// PatchListTitleSuffix sets the title suffix on the top list screen.
func (c *Controller) PatchListTitleSuffix(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.TitleSuffix = s
}

// PatchListShowIssueBadge sets the ShowIssueBadge flag on the top list screen.
func (c *Controller) PatchListShowIssueBadge(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.ShowIssueBadge = v
}

// PatchListAutoOpenSingle is an alias for SetListAutoOpenSingle used by
// navigation adapters that need to configure the flag before resources load.
// Calls the lock-free inner directly to avoid double-locking.
func (c *Controller) PatchListAutoOpenSingle(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.AutoOpenSingle = v
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

// GetListAllResources returns all cached resources for the top list screen's type.
func (c *Controller) GetListAllResources() []resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	return c.cachedResources(top.Ctx.ResourceType)
}

// GetListPagination returns truncated+cursor for the top list screen.
func (c *Controller) GetListPagination() (truncated bool, cursor string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return false, ""
	}
	return ls.HasPagination, ls.PaginationCursor
}

// GetListEscPops reports whether Esc should pop the top list screen.
func (c *Controller) GetListEscPops() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return ls.EscPops
}

// GetListDisplayName returns the display name override of the top list screen.
func (c *Controller) GetListDisplayName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	return ls.DisplayName
}

// GetListTitleSuffix returns the title suffix of the top list screen.
func (c *Controller) GetListTitleSuffix() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	return ls.TitleSuffix
}

// GetListShowIssueBadge reports whether the issue badge is shown.
func (c *Controller) GetListShowIssueBadge() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return ls.ShowIssueBadge
}

// GetListRelatedIDSet returns the relatedIDSet of the top list screen.
func (c *Controller) GetListRelatedIDSet() map[string]struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	return ls.RelatedIDSet
}

// PushChildListScreen pushes a ScreenChildList for the given resource type
// directly onto the controller stack, bypassing menu/command routing. Used by
// NewChildResourceList to ensure topListState() is non-nil before Patch* calls.
// Calls lock-free applyIntents + ensureListState to avoid double-locking.
func (c *Controller) PushChildListScreen(typeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenChildList,
		Context: runtime.ScreenContext{ResourceType: typeName},
	}})
	c.ensureListState()
}

// GetListEnrichmentFindings returns the enrichment findings map for typeName.
// Used by renderDataRow to resolve glyph markers without accessing the deleted
// findingsByID field on ResourceListModel.
func (c *Controller) GetListEnrichmentFindings(typeName string) map[string]domain.Finding {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.listEnrichmentFindings(typeName)
}

// GetListIssueCount returns the number of resources with issue status in the
// top list screen's resource type, mirroring IssueCount() in ResourceListModel.
func (c *Controller) GetListIssueCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil || len(c.stack) == 0 {
		return 0
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType

	// Prefer the fallback typeDef (registered via RegisterFallbackTypeDef from
	// the model constructor) over the catalog: the model's typeDef is the
	// authoritative Color classifier for issue counting. This is critical for
	// test typeDefs that share a ShortName with a catalog type but have a nil
	// Color (falls back to colorFallback(r.Fields["status"])) or a different
	// Color implementation.
	var td resource.ResourceTypeDef
	if ftd, ok := c.fallbackTypeDefs[typeName]; ok {
		td = ftd
	} else if catalogTD := resource.FindResourceType(typeName); catalogTD != nil {
		td = *catalogTD
	} else {
		return 0
	}
	all := c.listScreenResources(ls, typeName)
	findings := c.listEnrichmentFindings(typeName)
	ic := 0
	for _, r := range all {
		if listHasIssueFinding(r) {
			ic++
		} else if len(r.Findings) == 0 {
			if td.ResolveColor(r).IsIssue() {
				ic++
			} else if _, hasFinding := findings[r.ID]; hasFinding {
				ic++
			}
		}
	}
	return ic
}

// GetListVisibleResources returns the visible (filtered+sorted) resource slice
// for the top list screen, for test introspection.
func (c *Controller) GetListVisibleResources() []resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ls := c.topListState()
	if ls == nil || len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType
	all := c.listScreenResources(ls, typeName)
	visible := c.applyListFilters(ls, typeName, all)
	return listSortResources(c.viewConfig, ls, typeName, visible)
}

// ApplyListFieldUpdates merges Wave-2 field updates into the cached resource
// slice for typeName. Keyed by resource ID then field key.
// Updates are applied both to the top list screen's ls.Rows (the primary read
// path after the Bug 1 fix) and to the type-keyed resourceCache (for callers
// such as GetListAllResources that don't have a specific ListState).
func (c *Controller) ApplyListFieldUpdates(typeName string, updates map[string]map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(updates) == 0 {
		return
	}
	// Helper: merge updates into a resource slice in-place.
	applyToSlice := func(rows []resource.Resource) {
		for i := range rows {
			if kvMap, ok := updates[rows[i].ID]; ok {
				if rows[i].Fields == nil {
					rows[i].Fields = make(map[string]string, len(kvMap))
				}
				maps.Copy(rows[i].Fields, kvMap)
			}
		}
	}
	// Update the top list screen's per-screen rows first (primary read path).
	ls := c.topListState()
	if ls != nil {
		applyToSlice(ls.Rows)
	}
	// Also update the type-keyed cache so GetListAllResources etc. see the same values.
	if c.resourceCache != nil {
		applyToSlice(c.resourceCache[typeName])
	}
}

// ApplyListTruncatedIDs stores the per-resource truncation set for typeName.
// Currently retained for API parity with ResourceListModel.SetTruncatedIDs;
// the controller's attention filter does not yet consult this set.
func (c *Controller) ApplyListTruncatedIDs(_ string, _ map[string]bool) {
	// Intentional no-op: truncatedByID was stored but never read in the filter
	// pipeline. Retained for caller parity. No lock needed — no shared state
	// is accessed.
}

