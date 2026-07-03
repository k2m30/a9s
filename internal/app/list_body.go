package app

import (
	"maps"
	"strings"

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
//
// Every incoming page is run through MaterializeListFields first (Contract B
// render-sufficiency): Path-based, Key-less columns get their scalar value
// written into Fields while RawStruct is still present, so a later on-disk
// cache replay (which never carries RawStruct) renders identical cells.
// Resources that already have Fields populated (e.g. a cache-replay caller
// passing rows with RawStruct==nil) pass through unchanged — see
// MaterializeListFields's early-return.
func (c *Controller) applyResourcesLoaded(ls *ListState, typeName string, resources []resource.Resource, pagination *resource.PaginationMeta, appendPage bool) {
	resources = c.materializeListFieldsForType(typeName, resources)

	// --- Per-screen storage (Bug 1 fix) -----------------------------------
	// Writing to ls.Rows ensures that two stacked list screens of the same
	// resource type never share a row slice. Each screen's fetch result lands
	// exclusively on that screen's ListState.
	//
	// DEF-17 backstop: an append must never introduce a row whose ID already
	// exists on the screen. This is a backstop, not the fix for the root
	// cause below — it only prevents a duplicate that already reached this
	// call from becoming visible; the empty-cursor guard is what stops the
	// duplicate fetch from happening in the first place.
	if ls != nil {
		if appendPage {
			ls.Rows = append(ls.Rows, dedupAgainstExisting(ls.Rows, resources)...)
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
		existing := c.resourceCache[typeName]
		c.resourceCache[typeName] = append(existing, dedupAgainstExisting(existing, resources)...)
	} else {
		c.resourceCache[typeName] = resources
	}

	if ls != nil {
		ls.Loading = false
		ls.LoadingMore = false
		// Contract A: a fetch result landing clears Refreshing — the seeded
		// (or now-replaced) rows are confirmed. Callers that seed rows from a
		// cache-first source set Refreshing=true themselves AFTER calling this
		// method, so this unconditional clear only ever fires for a genuine
		// fetch-result swap, never undoing the seed-time flag.
		ls.Refreshing = false
		// DEF-5/C4: a successful fetch result clears any outstanding error
		// marker from a previous failed attempt.
		ls.LastFetchError = ""
		if pagination != nil {
			ls.HasPagination = pagination.IsTruncated
			ls.PaginationCursor = pagination.NextToken
		} else {
			ls.HasPagination = false
			ls.PaginationCursor = ""
		}
	}

	// Fresh rows arrive without Wave-2 findings; re-apply the latest known
	// findings from the enrichment store so a silent-swap refetch never leaves
	// the controller rows (and therefore the list-open save path, DEF-8)
	// glyph-blind until the next EnrichmentChecked. Mirrors the session-side
	// fold, which re-applies onto Core stores after every result lands.
	if known := c.listEnrichmentFindings(typeName); len(known) > 0 {
		c.applyRowFindings(typeName, known, nil)
	}
}

// dedupAgainstExisting returns the subset of incoming whose ID is not already
// present in existing. Rows are keyed by their stable resource ID (C2/C6):
// an append that would introduce a row already on the screen is dropped
// rather than shown twice. Order of the surviving rows is preserved.
func dedupAgainstExisting(existing, incoming []resource.Resource) []resource.Resource {
	if len(incoming) == 0 {
		return incoming
	}
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[r.ID] = struct{}{}
	}
	out := make([]resource.Resource, 0, len(incoming))
	for _, r := range incoming {
		if _, dup := seen[r.ID]; dup {
			continue
		}
		seen[r.ID] = struct{}{}
		out = append(out, r)
	}
	return out
}

// materializeListFieldsForType resolves the column set for typeName the same
// way buildListBody does (fallback typeDef first, then catalog) and runs
// every resource through MaterializeListFields. Resources without a matching
// column set (typeName unregistered, no columns resolved) pass through
// unchanged.
func (c *Controller) materializeListFieldsForType(typeName string, resources []resource.Resource) []resource.Resource {
	if len(resources) == 0 {
		return resources
	}
	var tdVal resource.ResourceTypeDef
	var td *resource.ResourceTypeDef
	if ftd, ok := c.fallbackTypeDefs[typeName]; ok {
		tdVal = ftd
		td = &tdVal
	} else if catalogTD := resource.FindResourceType(typeName); catalogTD != nil {
		td = catalogTD
	}
	columns := resolveListColumnsForBuild(c.viewConfig, typeName, td)
	if len(columns) == 0 {
		return resources
	}
	out := make([]resource.Resource, len(resources))
	for i, r := range resources {
		out[i] = MaterializeListFields(r, columns)
	}
	return out
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
	statusCol := resolveListStatusCol(columns, td)
	rows := make([]ListRow, 0, len(visible))
	for _, r := range visible {
		cells := extractListCells(columns, r, td)
		decorator, severity, colorTag := resolveListDecoratorFull(td, r, findings)
		// S4: bake the Wave-2 issue-Finding Phrase into the status cell, from the
		// same enrichment findings map that drives the glyph. Without this the web
		// renders a blank Status for flagged rows in live mode (the cell only
		// reflects Wave-1 r.Findings / a FieldUpdate that live never applies),
		// while the glyph still shows — a web/TUI parity break. Mirrors the
		// render-time override in resourcelist.go's renderListDataRow.
		//
		// This override is only a fallback for the case where Wave-2 enrichment
		// has landed in the enrichment-store map but has NOT yet been mutated
		// onto r.Findings (the live-mode lag the comment above describes). When
		// r.Findings already carries a Wave-2 entry for this resource (the demo
		// path and the fold-layer live path both mutate r.Findings directly via
		// applyWave2ToRow), extractListCells has already derived the correct
		// cell from listPhraseFromFindings(r.Findings) — including the "<top>
		// (+N)" stacking notation for multi-finding rows. The enrichment-store
		// map holds at most one Wave-2 entry per resource ID (see
		// findingsFromRows) and carries no stacking information, so applying it
		// on top of an already-stacked cell would silently drop the "(+N)"
		// suffix and/or clobber a higher-priority Wave-1 phrase with a
		// same-or-lower-severity Wave-2 one.
		if statusCol >= 0 && statusCol < len(cells) && !hasWave2Finding(r.Findings) {
			if f, ok := findings[r.ID]; ok && f.Severity.IsIssue() && f.Phrase != "" {
				cells[statusCol] = f.Phrase
			}
		}
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
		StatusCol:           statusCol,
		LoadingMore:         ls.LoadingMore,
		Refreshing:          ls.Refreshing,
		LastFetchError:      ls.LastFetchError,
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
//
// Per docs/attention-signals.md §Visualization Surfaces: the " !N" issue
// suffix is UNCONDITIONAL — it renders after the count parentheses on any
// list screen (top-level or ScreenChildList), on any renderer (TUI or web),
// whenever N > 0 and the screen is not in attention-only mode. N is the
// current list's issue count — aggregated the same way as the menu badge
// (Controller.GetListIssueCount's algorithm, mirrored lock-free below since
// buildListFrameTitle already runs under c.mu). N=0 renders no suffix. When
// the Wave-2 enrichment issue count itself is a truncated lower bound
// (c.enrichmentTruncated[typeName]), the suffix becomes " !N+" instead of
// " !N". The suffix is OMITTED entirely in attention-only (ctrl+z) mode,
// where the filtered count already IS the issue count and an " !N" suffix
// would be redundant.
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
	if !isAttention {
		if issueCount := c.listIssueCount(ls, typeName); issueCount > 0 {
			title += " !" + itoa(issueCount)
			if c.enrichmentTruncated[typeName] {
				title += "+"
			}
		}
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
	return c.listIssueCount(ls, top.Ctx.ResourceType)
}

// listIssueCount is the lock-free core of GetListIssueCount. Callers MUST
// already hold c.mu (e.g. buildListFrameTitle, which runs under c.mu) —
// taking the lock again would self-deadlock the non-reentrant RWMutex.
// Mirrors the ListSelected()/listSelected() split in this file.
func (c *Controller) listIssueCount(ls *ListState, typeName string) int {
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
	// S1 contract: the list-title suffix and the menu sync-back use the SAME
	// aggregation as the menu badge — and the badge never counts types with
	// ExcludeFromIssueBadge (e.g. ct-events, where "issue-colored" rows are
	// historical events, not live problems). Without this, an excluded type
	// would grow a title suffix and, via the app_stack sync-back, a forbidden
	// menu badge after its list was visited.
	if td.ExcludeFromIssueBadge {
		return 0
	}
	all := c.listScreenResources(ls, typeName)
	findings := c.listEnrichmentFindings(typeName)
	ic := 0
	for _, r := range all {
		switch {
		case listHasBadgeFinding(r):
			ic++
		case td.ResolveColor(r).IsIssue():
			// DEF-8: rows carry Wave-2 findings directly, so a row that is a
			// Wave-1 issue by td.ResolveColor but whose only findings are
			// non-badge (e.g. a lone Wave-2 "~" warn) must still count here —
			// gating this fallback on len(r.Findings) == 0 undercounted any
			// such row. The color check is independent of r.Findings content.
			ic++
		case len(r.Findings) == 0:
			if f, hasFinding := findings[r.ID]; hasFinding && f.Severity == domain.SevBroken {
				// S1: Wave-2 findings bump the count only at "!" severity —
				// "~ findings do not bump" (docs/attention-signals.md). Wave-1
				// yellow/red rows are already counted by the color branch above,
				// matching the menu badge's probe-side aggregation.
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
	c.applyListFieldUpdates(typeName, updates)
}

// applyListFieldUpdates is the lock-free body of ApplyListFieldUpdates so it can
// be called from the intents handler, which already holds c.mu. Callers must
// hold c.mu (write).
func (c *Controller) applyListFieldUpdates(typeName string, updates map[string]map[string]string) {
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

// ClearRowFindings strips every Wave-2 finding (domain.Finding with Source
// prefixed "wave2:") from the cached resource rows for typeName, on both
// per-screen ls.Rows and the type-keyed resourceCache — mirroring the
// dual-store walk in applyListFieldUpdates. AttentionDetails entries keyed by
// a stripped Wave-2 Finding's Code are dropped alongside it.
//
// This is the controller-side half of a Ctrl+R refresh's stale-findings
// cleanup. internal/tui's Model.applyEnrichment strips Wave-2 findings from
// the session-owned stores (Core.ResourceCache / LazyResourceCache /
// ProbeResources); those stores no longer alias the controller's rows since
// applyResourcesLoaded materializes copies (MaterializeListFields) into
// ls.Rows / c.resourceCache. Without this explicit companion call, a
// ResourceListModel constructed from the (now-copied) cache-hit rows retains
// stale Wave-2 findings across Ctrl+R even after the session-side clear.
func (c *Controller) ClearRowFindings(typeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearRowFindings(typeName)
}

// clearRowFindings is the lock-free body of ClearRowFindings so it can be
// called from contexts that already hold c.mu (write).
func (c *Controller) clearRowFindings(typeName string) {
	clearSlice := func(rows []resource.Resource) {
		for i := range rows {
			rows[i].Findings = stripWave2Findings(rows[i].Findings)
			rows[i].AttentionDetails = nil
		}
	}

	canon := typeName
	if td := resource.FindResourceType(typeName); td != nil {
		canon = td.ShortName
	}

	// Every list screen of this type in the stack (mirrors applyListFieldUpdates).
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
		clearSlice(s.State.List.Rows)
	}

	// Type-keyed cache (GetListAllResources and other typeName-only callers).
	if c.resourceCache != nil {
		clearSlice(c.resourceCache[typeName])
		if typeName != canon {
			clearSlice(c.resourceCache[canon])
		}
	}
}

// applyRowFindings is the applying-direction mirror of clearRowFindings: it
// writes Wave-2 findings (and their attention details) onto the controller's
// own row stores — every matching list screen's ls.Rows plus c.resourceCache.
// Without it, enrichment applied to an OPEN live list reaches only the
// session-owned stores (Core.applyEnrichment) and the controller's
// enrichmentStore glyph map, so the list-open save path (which persists from
// ls.Rows) writes rows with no findings — cached rows then reseed glyphless
// (S3-pilot DEF-8). Callers must hold c.mu (write); the PatchResourceList
// intent case is the production entry point. A nil findings map clears Wave-2
// entries, matching runtime.ApplyWave2ToRow's contract.
func (c *Controller) applyRowFindings(typeName string, findings map[string]domain.Finding, details map[string]domain.AttentionDetail) {
	canon := typeName
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(typeName); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	applySlice := func(rows []resource.Resource) {
		for i := range rows {
			runtime.ApplyWave2ToRow(&rows[i], td, findings, details)
		}
	}

	for i := range c.stack {
		s := &c.stack[i]
		if s.ID != runtime.ScreenResourceList && s.ID != runtime.ScreenChildList {
			continue
		}
		st := s.Ctx.ResourceType
		if t := resource.FindResourceType(st); t != nil {
			st = t.ShortName
		}
		if st != canon || s.State.List == nil {
			continue
		}
		applySlice(s.State.List.Rows)
	}

	if c.resourceCache != nil {
		applySlice(c.resourceCache[typeName])
		if typeName != canon {
			applySlice(c.resourceCache[canon])
		}
	}
}

// listHasBadgeFinding reports whether a row's own findings bump the S1 issue
// count. Wave-1 findings (fetcher-written, no "wave2:" Source prefix) count at
// any issue severity — a Wave-1 warning IS the yellow row the menu badge
// counts by color. Wave-2 findings count only at "!" severity: per the S1
// contract "~ findings do not bump", and now that applyRowFindings writes
// Wave-2 findings onto controller rows, counting them at warn severity here
// would reintroduce the very drift the severity gate on the enrichment-store
// branch fixed.
func listHasBadgeFinding(r resource.Resource) bool {
	for _, f := range r.Findings {
		if f.Severity == domain.SevBroken {
			return true
		}
		if resource.IsIssueSeverity(f.Severity) && !strings.HasPrefix(f.Source, "wave2:") {
			return true
		}
	}
	return false
}

// stripWave2Findings returns findings with every Wave-2 entry (Source
// prefixed "wave2:") removed, preserving the order of the remaining entries.
// Mirrors stripWave2 in internal/tui/app_enrich_fold.go and applyWave2ToRow's
// strip step in internal/runtime/helpers.go — kept as a sibling here (rather
// than imported) because internal/app must not depend on internal/tui
// (internal/tui already depends on internal/app) or internal/runtime's
// unexported helpers.
func stripWave2Findings(findings []domain.Finding) []domain.Finding {
	if len(findings) == 0 {
		return findings
	}
	out := findings[:0:0]
	for _, f := range findings {
		if !strings.HasPrefix(f.Source, "wave2:") {
			out = append(out, f)
		}
	}
	return out
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
