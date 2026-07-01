package app

import (
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
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
		},
	}}
	return c.snapshot(), tasks
}

// handleActionRefresh handles ActionRefresh.
func (c *Controller) handleActionRefresh(_ Action) (ViewState, []runtime.TaskRequest) {
	// Detail view: re-dispatch enrich + related.
	if ds := c.topDetailState(); ds != nil {
		rt := ds.ResourceType
		srcRes := ds.Resource
		var tasks []runtime.TaskRequest
		if resource.HasDetailEnricher(rt) {
			tasks = append(tasks, runtime.TaskRequest{
				Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: rt},
			})
		}
		// Emit the enrich detail task so the executor re-runs enrichment.
		// The related-check task is emitted separately via Handle(RelatedCheckStarted).
		_ = srcRes
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
		ls.Loading = true
		ls.Rows = nil
		tasks := []runtime.TaskRequest{{
			Key:   runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: typeName},
			Cache: runtime.CacheNone,
		}}
		return c.snapshot(), tasks
	}
	return c.snapshot(), nil
}
