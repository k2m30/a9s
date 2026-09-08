// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// handleActionToggleAttention handles ActionToggleAttention.
func (c *Controller) handleActionToggleAttention(_ Action) (ViewState, []runtime.TaskRequest) {
	if ls := c.topListState(); ls != nil {
		ls.AttentionOnly = !ls.AttentionOnly
		ls.SelectedRow = 0
	} else if ms := c.topMenuState(); ms != nil {
		ms.AttentionOnly = !ms.AttentionOnly
		ms.Cursor = 0
	}
	return c.snapshot(), nil
}

// handleActionSetFilter handles ActionSetFilter.
func (c *Controller) handleActionSetFilter(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ls := c.topListState(); ls != nil {
		ls.Filter = a.Arg
		ls.SelectedRow = 0
		ls.ScrollY = 0
	} else if ms := c.topMenuState(); ms != nil {
		ms.Filter = a.Arg
		ms.Cursor = 0
		ms.ScrollOffset = 0
	} else if ss := c.topSelectorState(); ss != nil {
		ss.Filter = a.Arg
		ss.Cursor = 0
	}
	return c.snapshot(), nil
}

// handleActionSort handles ActionSort.
func (c *Controller) handleActionSort(a Action) (ViewState, []runtime.TaskRequest) {
	if ls := c.topListState(); ls != nil && a.Arg != "" {
		if ls.SortCol == a.Arg {
			if ls.SortDir == "asc" {
				ls.SortDir = "desc"
			} else {
				ls.SortDir = "asc"
			}
		} else {
			ls.SortCol = a.Arg
			ls.SortDir = "asc"
		}
		ls.SelectedRow = 0
	}
	return c.snapshot(), nil
}

// handleActionToggleWrap handles ActionToggleWrap.
func (c *Controller) handleActionToggleWrap(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.Wrap = !ts.Wrap
	}
	return c.snapshot(), nil
}

// handleActionToggleFocus handles ActionToggleFocus.
func (c *Controller) handleActionToggleFocus(a Action) (ViewState, []runtime.TaskRequest) {
	// Detail-only: Tab toggles focus between the field and related columns.
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	return c.snapshot(), nil
}

// handleActionSearch handles ActionSearch.
func (c *Controller) handleActionSearch(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.Search = a.Arg
		ts.SearchCursor = 0
	}
	return c.snapshot(), nil
}

// handleActionSearchNext handles ActionSearchNext.
func (c *Controller) handleActionSearchNext(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil && ts.Search != "" {
		matches := buildTextSearchMatches(ts.Lines, ts.Search)
		if len(matches) > 0 {
			ts.SearchCursor = (ts.SearchCursor + 1) % len(matches)
			if ts.SearchCursor < len(matches) {
				ts.ScrollY = matches[ts.SearchCursor].Line
			}
		}
	}
	return c.snapshot(), nil
}

// handleActionSearchPrev handles ActionSearchPrev.
func (c *Controller) handleActionSearchPrev(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil && ts.Search != "" {
		matches := buildTextSearchMatches(ts.Lines, ts.Search)
		if len(matches) > 0 {
			ts.SearchCursor = (ts.SearchCursor - 1 + len(matches)) % len(matches)
			if ts.SearchCursor < len(matches) {
				ts.ScrollY = matches[ts.SearchCursor].Line
			}
		}
	}
	return c.snapshot(), nil
}

// handleActionSearchClear handles ActionSearchClear.
func (c *Controller) handleActionSearchClear(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.Search = ""
		ts.SearchCursor = 0
	}
	return c.snapshot(), nil
}

// handleActionToggleRelated handles ActionToggleRelated.
func (c *Controller) handleActionToggleRelated(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	return c.snapshot(), nil
}

// handleActionLoadMore handles ActionLoadMore.
func (c *Controller) handleActionLoadMore(_ Action) (ViewState, []runtime.TaskRequest) {
	ls := c.topListState()
	if ls == nil || !ls.HasPagination || ls.LoadingMore {
		return c.snapshot(), nil
	}
	ls.LoadingMore = true
	typeName := ""
	if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
		typeName = top.Ctx.ResourceType
	}
	tasks := []runtime.TaskRequest{{
		Key: runtime.TaskKey{Kind: runtime.KindFetchMore, Scope: typeName},
		Payload: runtime.FetchMorePayload{
			ContinuationToken: ls.PaginationCursor,
			ParentContext:     ls.ParentContext,
			FetchFilter:       ls.FetchFilter,
			Provenance:        listLane(c.topScreenID(), ls),
		},
	}}
	return c.snapshot(), tasks
}

// handleActionRefresh handles ActionRefresh.
func (c *Controller) handleActionRefresh(_ Action) (ViewState, []runtime.TaskRequest) {
	// Main menu: restart the availability/enrichment sweep — mirrors the
	// TUI's Ctrl+R-on-menu path (runtime_adapter_navigate.go) via the shared
	// RestartAvailabilitySweep mutation-list method (see its doc comment
	// below for the exact bundle and the returned task's shape).
	if ms := c.topMenuState(); ms != nil {
		tasks := c.restartAvailabilitySweepLocked()
		return c.snapshot(), tasks
	}
	// Cost Explorer: force-refetch only the active shape's open period;
	// closed periods are refetched only if genuinely absent from the store.
	if cs := c.topCostsState(); cs != nil {
		tasks := c.forceRefreshCostsLocked()
		return c.snapshot(), tasks
	}
	// Detail view: begin a fresh (refresh=true) DetailOperation and
	// re-dispatch enrich + related under it. The TUI reaches this same
	// branch via ctrl.Apply(ActionRefresh)
	// (internal/tui/runtime_adapter_navigate.go's handleRefresh), so both
	// lanes share one refresh-dispatch policy.
	if ds := c.topDetailState(); ds != nil {
		rt := ds.ResourceType
		srcRes := ds.Resource
		c.resetDetailRelatedRowsLocked(rt)
		// forceRelated deletes the RelatedCache entry before
		// beginDetailWorkloadLocked's own cache-replay attempt — the
		// RelatedCacheLRU entry is append-only (PatchRelatedCache never
		// overwrites), so without that delete every refresh would pile a
		// duplicate per-def entry onto it and a later cache-hit replay would
		// merge stale rows behind the fresh ones.
		_, tasks := c.beginDetailWorkloadLocked(rt, srcRes, true, true)
		return c.snapshot(), tasks
	}
	// List view: delete cache and re-fetch.
	if ls := c.topListState(); ls != nil {
		typeName := ""
		if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
			typeName = top.Ctx.ResourceType
		}
		if typeName == "" {
			return c.snapshot(), nil
		}
		c.core.DeleteResourceCache(typeName)
		// C8: cached content stays visible under the refreshing marker while
		// the refetch runs — only blank Loading/Rows when there is nothing to
		// show yet (the empty-list case never had a marker to keep rows under).
		if len(ls.Rows) == 0 {
			ls.Loading = true
			ls.Rows = nil
		}
		ls.LastFetchError = ""
		// Mirrors the TUI's list-refresh-with-enricher branch
		// (runtime_adapter_navigate.go): no-op (returns 0) when typeName has no
		// registered issue enricher, in which case the resulting fetch task's
		// FetchResourcesPayload.TypeGen is 0 — a normal, unstamped refetch.
		tok := c.core.RefreshListEnrichment(typeName)
		tasks := c.activeListRefreshTasks(tok)
		return c.snapshot(), tasks
	}
	return c.snapshot(), nil
}

// RestartAvailabilitySweep restarts the main-menu availability/enrichment
// sweep: the single mutation-list method shared by the TUI's main-menu
// Ctrl+R path (internal/tui/runtime_adapter_navigate.go, rsKindMenu branch)
// and the headless/web menu-refresh branch (handleActionRefresh above) — the
// same pattern Core.RefreshListEnrichment applies to a list refresh, applied
// here to the menu-level sweep restart.
//
// No-op (returns nil) in --no-cache mode, mirroring the TUI's own guard.
// Otherwise: bumps AvailabilityGen and EnrichmentGen (cancelling any
// in-flight probes/enrichment so their late completions are rejected as
// stale), clears the per-type enrichment latches (ResetEnrichmentMaps),
// strips stale wave2 findings from every retained row
// (Core.ClearAllWave2Findings — must run before the reset below: a cached
// list opened before the new sweep's fresh EnrichmentChecked lands must
// never show the previous run's attention state), resets the probe-origin
// bookkeeping (ResetProbeMaps), clears the menu's rendered
// availability/issue-count state (MenuClearAvailabilityIntent), and clears
// the swept-pair latch (ClearPairSwept) so an explicit manual refresh always
// re-probes every type, even an already-swept pair.
//
// Returns the TaskKindLoadAvailCache task that restarts the sweep — the same
// task Core.HandleClientsReady/BootstrapLive dispatch on a fresh connect
// (core/app/bootstrap.go). The TUI's own cache-load path
// (m.loadAvailabilityCache, a tea.Cmd) stays TUI-side, reaching the same
// executor logic (TaskKindLoadAvailCache's case) through its own Cmd instead
// of a drained TaskRequest — the mutation bundle above is the single thing
// this method exists to unify, not the fetch mechanism itself.
func (c *Controller) RestartAvailabilitySweep() []runtime.TaskRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stampDispatchSnapshotLocked(c.restartAvailabilitySweepLocked())
}

// restartAvailabilitySweepLocked is the lock-free core of
// RestartAvailabilitySweep. Callers must hold c.mu (write).
func (c *Controller) restartAvailabilitySweepLocked() []runtime.TaskRequest {
	if c.core.NoCache() {
		return nil
	}
	c.core.BumpAvailabilityGen()
	c.core.BumpEnrichmentGen()
	c.core.ResetEnrichmentMaps()
	c.core.ClearAllWave2Findings()
	c.core.ResetProbeMaps()
	c.applyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
	c.core.Session().ClearPairSwept()
	return []runtime.TaskRequest{{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindLoadAvailCache},
		Payload: runtime.LoadAvailCachePayload{},
	}}
}

// activeListRefreshTasks builds the fetch task for the top-of-stack list
// screen without disturbing its currently rendered rows. Callers must hold
// c.mu.
//
// typeGen is the enrichment-rerun token to ride along on the task's
// FetchResourcesPayload (0 for "no rerun intent" — the only value the C10
// replay caller below passes, since a pre-connect replay is not an
// enrichment rerun). handleActionRefresh's list branch passes the token
// Core.RefreshListEnrichment returned so the resulting messages.ResourcesLoaded
// reaches HandleResourcesLoaded's rerun branch instead of its list-open
// fallback — see RefreshListEnrichment's doc comment
// (core/runtime/handlers_resources.go).
//
// C10: this is also the replay path for a navigation issued before AWS
// connect completes — once ClientsReady lands, the pending refresh must
// re-fetch the list the user is already looking at. C8 requires the cached
// content stay visible under the refreshing marker during that replay, so
// this helper only sets Refreshing and never blanks Rows/Loading itself —
// handleActionRefresh's own Loading/Rows reset above is now also
// non-destructive whenever the list already has rows to keep.
func (c *Controller) activeListRefreshTasks(typeGen domain.Gen) []runtime.TaskRequest {
	ls := c.topListState()
	if ls == nil {
		return nil
	}
	typeName := ""
	if len(c.stack) > 0 {
		typeName = c.stack[len(c.stack)-1].Ctx.ResourceType
	}
	if typeName == "" {
		return nil
	}
	ls.Refreshing = true
	// A Ctrl+R issued while the top-of-stack list is a related-navigation
	// drill (filtered/child/by-ID placeholder, never the type's canonical
	// top-level list) must not let the resulting ResourcesLoaded default to
	// FetchProvenanceCanonicalList — the symmetric gate in handle.go's
	// handleResourcesLoadedEvent would then reject it on this exact screen,
	// the same class of defect HandleRelatedNavigate's own drill fetches
	// had (core/runtime/handlers_related.go).
	payload := runtime.FetchResourcesPayload{TypeGen: typeGen, Provenance: listLane(c.topScreenID(), ls)}
	return []runtime.TaskRequest{{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: typeName},
		Cache:   runtime.CacheNone,
		Payload: payload,
	}}
}
