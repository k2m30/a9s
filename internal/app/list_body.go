package app

import (
	"maps"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

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
	return c.listSelected()
}

// listSelected is the lock-free core of ListSelected. Callers MUST already hold
// c.mu (e.g. applyLocked dispatching a row-dependent action) — taking the lock
// again would self-deadlock the non-reentrant RWMutex.
func (c *Controller) listSelected() (resource.Resource, bool) {
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
	// Apply to EVERY list screen of this type in the stack (primary read path).
	// PatchResourceList updates every matching ResourceListModel, so the matching
	// list may be stacked beneath a detail or another list. Targeting only the top
	// screen leaves a stacked same-type list's per-screen Rows stale (or wrongly
	// mutates a top list of a different type), and buildListBody prefers ls.Rows
	// over the type cache, so popping back would render stale cell values.
	canon := typeName
	if td := resource.FindResourceType(typeName); td != nil {
		canon = td.ShortName
	}
	for i := range c.stack {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		st := s.Ctx.ResourceType
		if td := resource.FindResourceType(st); td != nil {
			st = td.ShortName
		}
		if st != canon || s.State.List == nil {
			continue
		}
		applyToSlice(s.State.List.Rows)
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
	// Prefer per-screen rows (ls.Rows) over the shared type-keyed cache so that
	// a related-navigation list (EscPops, same ResourceType) cannot overwrite the
	// type key and corrupt the top-level list's resource count on cache-back.
	return c.listScreenResources(ls, top.Ctx.ResourceType)
}
