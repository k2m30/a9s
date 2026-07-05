package app

import (
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
// ct-events defaults to newest-first (event_time DESC). Called by every
// list-screen creation path so the default is applied exactly once, at init,
// and survives the renderer's per-keystroke view reconstruction without a
// constructor-time re-apply that could fight a user-chosen sort.
func applyListDefaults(ls *ListState, resourceType string) {
	if resourceType == "ct-events" {
		ls.SortCol = "event_time"
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

// SetListFetchError records a failed fetch's error text on the top list
// screen, mirroring the headless ClearActiveListLoadingIntent application in
// intents.go (DEF-5/C4): a fetch failure over cached content stops the
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
// seeding callers (DEF-12, C3: docs/design/cache-requirements.md) to mark a
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
// cache-first seeding callers (item B, #17 wave 2, DEF-21) AFTER
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
