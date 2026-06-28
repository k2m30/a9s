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
// state (after filter/attention/relatedIDSet). Uses the resource cache stored
// on the controller.
func (c *Controller) listVisibleCount(ls *ListState) int {
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType
	if typeName == "" {
		return 0
	}
	resources := c.cachedResources(typeName)
	visible := c.applyListFilters(ls, typeName, resources)
	return len(visible)
}

// cachedResources returns the resource slice for typeName from the controller's
// resource cache, or nil if no data has been received yet.
func (c *Controller) cachedResources(typeName string) []resource.Resource {
	if c.resourceCache == nil {
		return nil
	}
	return c.resourceCache[typeName]
}

// applyListFilters applies the relatedIDSet prefilter, text filter, and
// attention filter to base, returning the visible subset. Mirrors
// ResourceListModel.applyFilter exactly so ListSelected and buildListBody
// agree on which row is "selected".
func (c *Controller) applyListFilters(ls *ListState, typeName string, base []resource.Resource) []resource.Resource {
	td := resource.FindResourceType(typeName)

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
func listSortResources(ls *ListState, typeName string, resources []resource.Resource) []resource.Resource {
	if ls.SortCol == "" || len(resources) == 0 {
		return resources
	}

	vd := config.GetViewDef(nil, typeName)
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

// applyResourcesLoaded stores a page of resources into the controller's cache
// for typeName. When appendPage is true the new slice is appended; otherwise
// it replaces. Mirrors what ResourceListModel.Update does on ResourcesLoaded.
//
//nolint:unused // wired in PR-C task-result lane (called from Handle on ResourcesLoaded events)
func (c *Controller) applyResourcesLoaded(ls *ListState, typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
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

// reapplyCheckerAgainst re-runs the stored checker for typeName against newPage
// and merges returned IDs into ls.RelatedIDSet. Mirrors ReapplyCheckerAgainst.
//
//nolint:unused // wired in PR-C task-result lane (called from applyResourcesLoaded on each page)
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
	td := resource.FindResourceType(typeName)

	// Resolve column definitions from the view config (mirrors resolveColumns).
	columns := resolveListColumns(typeName)

	// Build the row set from cache.
	allResources := c.cachedResources(typeName)

	// Apply filters (relatedIDSet → text → attention).
	visible := c.applyListFilters(ls, typeName, allResources)

	// Apply sort.
	visible = listSortResources(ls, typeName, visible)

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
		cells := extractListCells(columns, r, typeName)
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
	td := resource.FindResourceType(typeName)
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
// extractCellValue in table_render.go. DATA only — no Lipgloss styling.
func extractListCells(columns []ColumnDef, r resource.Resource, typeName string) []string {
	cells := make([]string, len(columns))
	td := resource.FindResourceType(typeName)
	for i, col := range columns {
		cells[i] = listExtractCellValue(col, td, r)
	}
	return cells
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

	allResources := c.cachedResources(typeName)
	total := len(allResources)
	visible := c.applyListFilters(ls, typeName, allResources)
	filtered := len(visible)
	truncated := ls.HasPagination

	totalStr := itoa(total)
	if truncated {
		totalStr = itoa(total) + "+"
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
func (c *Controller) ListSelected() (resource.Resource, bool) {
	ls := c.topListState()
	if ls == nil {
		return resource.Resource{}, false
	}
	top := c.stack[len(c.stack)-1]
	typeName := top.Ctx.ResourceType

	allResources := c.cachedResources(typeName)
	visible := c.applyListFilters(ls, typeName, allResources)
	visible = listSortResources(ls, typeName, visible)

	if len(visible) == 0 || ls.SelectedRow >= len(visible) {
		return resource.Resource{}, false
	}
	return visible[ls.SelectedRow], true
}

// GetListFilter returns the current filter text of the top list screen.
func (c *Controller) GetListFilter() string {
	ls := c.topListState()
	if ls == nil {
		return ""
	}
	return ls.Filter
}

// GetListSort returns the current sort column and direction of the top list screen.
func (c *Controller) GetListSort() (col, dir string) {
	ls := c.topListState()
	if ls == nil {
		return "", ""
	}
	return ls.SortCol, ls.SortDir
}

// GetListScrollX returns the horizontal scroll offset of the top list screen.
func (c *Controller) GetListScrollX() int {
	ls := c.topListState()
	if ls == nil {
		return 0
	}
	return ls.ScrollX
}

// GetListSelectedRow returns the selected-row index of the top list screen.
func (c *Controller) GetListSelectedRow() int {
	ls := c.topListState()
	if ls == nil {
		return 0
	}
	return ls.SelectedRow
}

// GetListAttentionOnly reports whether attention-only mode is active on the
// top list screen.
func (c *Controller) GetListAttentionOnly() bool {
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return ls.AttentionOnly
}
