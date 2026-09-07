// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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

// topScreenID returns the ScreenID of the top-of-stack screen, or "" when the
// stack is empty. Used by save-gating logic (C6 scope boundary) that needs to
// distinguish ScreenResourceList (persist-eligible) from ScreenChildList
// (never persisted) without a full Screen reference.
func (c *Controller) topScreenID() runtime.ScreenID {
	if len(c.stack) == 0 {
		return ""
	}
	return c.stack[len(c.stack)-1].ID
}

// isTopLevelCanonicalList reports whether screenID/ls together identify the
// canonical top-level, unfiltered resource list for its type — the C6 scope
// gate maybeSaveResourceListCache/syncExactTotalToMenu use to decide
// disk-cache eligibility, and applyResourcesLoaded's callers use to decide
// RowStore eligibility (task #17 wave 1 stage 4): a ScreenChildList, or a
// ScreenResourceList opened via EscPops/carrying a ParentContext (a
// related-navigation or filtered view), is never the type's global
// population and must not read/write the shared per-type RowStore entry.
func isTopLevelCanonicalList(screenID runtime.ScreenID, ls *ListState) bool {
	if screenID != runtime.ScreenResourceList || ls == nil {
		return false
	}
	return !ls.EscPops && ls.ParentContext == nil
}

// pushByIDPlaceholderList pushes a placeholder ScreenResourceList for
// targetType and always sets EscPops (this screen is a related/filtered/by-ID
// drill, never the type's canonical top-level list — isTopLevelCanonicalList
// above). When targetType has a FetchByIDs helper registered, it additionally
// flags the placeholder as an auto-open-single-detail drill for targetID —
// RelatedIDSet keyed on targetID plus AutoOpenSingle — so Handle's
// autoOpenSingleDetail (handle.go) replaces it with the resolved detail once
// the by-ID fetch delivery lands.
//
// The single constructor for the by-ID placeholder invariant, shared by the
// two lane-neutral by-ID drill seams: the related panel's own single-target
// drill (navigate.go's applyRelatedNavResult, NavigationKindFilteredList
// case) and the costs pivot's resource-open drill (costs_state.go's
// screen.OpenResource case). Returns nil if the push leaves no top list
// screen.
func (c *Controller) pushByIDPlaceholderList(targetType, targetID string) *ListState {
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: targetType},
	}})
	c.ensureListState()
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	ls.EscPops = true
	if targetID != "" && resource.GetFetchByIDs(targetType) != nil {
		ls.RelatedIDSet = map[string]struct{}{targetID: {}}
		ls.AutoOpenSingle = true
	}
	return ls
}

// EnsureListState is the exported surface that TUI builders call immediately
// after a ScreenResourceList/ScreenChildList PushScreen intent has already
// been applied (e.g. via ApplyIntents) so that State.List is non-nil before
// the renderer's builder reads topListState(). Delegates to ensureListState.
// Mirrors EnsureSelectorState (selector.go).
func (c *Controller) EnsureListState() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureListState()
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
		applyListDefaults(top.State.List, top.Ctx.ResourceType)
	}
}

// applyListDefaults seeds per-type defaults on a freshly-created ListState.
// ct-events defaults to newest-first: its Time column, descending. Called by every
// list-screen creation path so the default is applied exactly once, at init,
// and survives the renderer's per-keystroke view reconstruction without a
// constructor-time re-apply that could fight a user-chosen sort.
func applyListDefaults(ls *ListState, resourceType string) {
	if resourceType == "ct-events" {
		ls.SortCol = "time"
		ls.SortDir = "desc"
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

// cachedResources returns the resource slice for typeName from the
// session-owned RowStore (any origin — Fetch, Probe, or Disk), or nil if no
// data has been received yet. Callers that have a per-screen ListState
// should prefer listScreenResources so that stacked same-type screens read
// their own rows, not a shared slice.
func (c *Controller) cachedResources(typeName string) []resource.Resource {
	entry, ok := c.core.AnyOriginResourceCache(typeName)
	if !ok {
		return nil
	}
	return entry.Resources
}

// findCachedResourceByID looks up a single resource by ID within typeName's
// RowStore-backed cache (any origin), for callers that only need one row
// (text/detail screen resolution) rather than the full slice. Mirrors the
// linear-scan-by-ID pattern previously duplicated across
// selectedResourceForAction, buildTextFooterHints, and GetTextResource
// against the deleted Controller.resourceCache map.
func (c *Controller) findCachedResourceByID(typeName, id string) (resource.Resource, bool) {
	for _, r := range c.cachedResources(typeName) {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
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
		ls.rowsVersion++
		return
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	ls.RelatedIDSet = set
	ls.rowsVersion++
}

// seedRelatedExactRows sets the exact-ID related filter on ls and seeds its rows
// from the any-lane RowStore subset matching those IDs, so a cache-hit exact
// filtered list renders immediately — no fetch, no spinner. Returns whether the
// cache fully covers the requested IDs. Shared by both renderers (web
// applyRelatedNavResult directly, TUI newRelatedList via SeedRelatedExactRows)
// so the exact-filtered seed cannot diverge — the single fix for the Partial
// (lazy) lane that render-time pull would otherwise miss. Callers hold c.mu.
func (c *Controller) seedRelatedExactRows(ls *ListState, targetType string, ids []string) bool {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	ls.RelatedIDSet = set
	seen := make(map[string]struct{}, len(set))
	var seeded []resource.Resource
	for _, r := range c.core.AnyLaneResources(targetType) {
		if _, want := set[r.ID]; !want {
			continue
		}
		if _, dup := seen[r.ID]; dup {
			continue
		}
		seen[r.ID] = struct{}{}
		seeded = append(seeded, r)
	}
	if len(seeded) > 0 {
		ls.Rows = seeded
		ls.Loading = false
	}
	ls.rowsVersion++
	return len(seen) == len(set)
}

// SeedRelatedExactRows applies seedRelatedExactRows to the top list screen and
// reports whether the cache fully covers ids. The TUI adapter calls it after
// pushing a related list; the web path reaches the lock-free core directly under
// Apply's already-held lock.
func (c *Controller) SeedRelatedExactRows(targetType string, ids []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return false
	}
	return c.seedRelatedExactRows(ls, targetType, ids)
}

// PatchListReapplyChecker registers a RelatedChecker + source resource for the
// top list screen's resource type. When non-nil, subsequent applyResourcesLoaded
// calls re-run the checker to extend RelatedIDSet with newly matched IDs.
func (c *Controller) PatchListReapplyChecker(checker resource.RelatedChecker, src resource.Resource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.patchListReapplyChecker(checker, src)
}

// patchListReapplyChecker is the lock-free core of PatchListReapplyChecker.
// Callers MUST already hold c.mu — e.g. dispatchRelatedNavigate, which runs
// under Apply's lock; calling the exported wrapper there would re-lock the
// non-reentrant RWMutex and self-deadlock. Mirrors the listIssueCount split.
func (c *Controller) patchListReapplyChecker(checker resource.RelatedChecker, src resource.Resource) {
	ls := c.topListState()
	if ls == nil {
		return
	}
	if checker == nil {
		ls.reapplyChecker = nil
		ls.reapplySource = resource.Resource{}
		return
	}
	ls.reapplyChecker = checker
	ls.reapplySource = src
	// Mirror SetReapplyChecker: activate filter with empty set so zero-match
	// navigations hide all rows immediately rather than showing an unfiltered list.
	if ls.RelatedIDSet == nil {
		ls.RelatedIDSet = make(map[string]struct{})
		ls.rowsVersion++
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

// clearFetchInFlight resets every in-flight fetch-activity flag (Loading,
// LoadingMore, Refreshing) to false. This is the single point both a landed
// fetch result (applyResourcesLoaded, success or partial-failure) and a
// fetch-failure path (ClearListLoading, the headless/web
// ClearActiveListLoadingIntent case in intents.go) route through, so a
// load-more or refresh failure can never strand a flag the other lane already
// knew to clear.
func (ls *ListState) clearFetchInFlight() {
	ls.Loading = false
	ls.LoadingMore = false
	ls.Refreshing = false
}

// ClearListLoading clears the top list screen's in-flight fetch flags. Called
// when a fetch or load-more operation fails (error handler path) so the title
// reverts from "name loading..." back to the resource count title.
func (c *Controller) ClearListLoading() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.clearFetchInFlight()
}

// SetListFetchError records a failed fetch's error text on the top list
// screen, mirroring the headless ClearActiveListLoadingIntent application in
// intents.go (cache contract C4): a fetch failure over cached content stops the
// refreshing marker and swaps in an error marker instead of leaving the list
// with no error surfaced. No-op when err is empty.
func (c *Controller) SetListFetchError(err string) {
	if err == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.Refreshing = false
	ls.LastFetchError = err
}

// SetListRefreshing sets the Refreshing flag on the top list screen. Mirrors
// SetListFetchError's locking/topListState pattern. Used by cache-first
// seeding callers (C3: docs/design/cache-requirements.md) to mark a
// seeded-but-unverified list surface so the renderer's refreshing marker
// (⟳ / "── refreshing... ──") distinguishes it from verified-fresh content —
// renderers read this flag, they never compute it.
func (c *Controller) SetListRefreshing(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.Refreshing = v
}

// SetListTotalCount sets the TotalCount override on the top list screen.
// Mirrors SetListRefreshing's locking/topListState pattern. Used by
// cache-first seeding callers (the seed-time provisional total, #17 wave 2) AFTER
// applyResourcesLoaded so the seed-time value survives the unconditional
// clear inside it — same set-after-seed ordering SetListRefreshing already
// requires. n <= 0 is a no-op: TotalCount's zero value already means
// "not applicable", and buildListFrameTitle only prefers TotalCount when it
// exceeds len(Rows).
func (c *Controller) SetListTotalCount(n int) {
	if n <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ls := c.topListState()
	if ls == nil {
		return
	}
	ls.TotalCount = n
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
