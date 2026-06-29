package app

import (
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// handleActionBack handles ActionBack.
func (c *Controller) handleActionBack(_ Action) (ViewState, []runtime.TaskRequest) {
	// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
	// collapse (root-collapse is the "root" Command). Per-view Esc semantics
	// (clear filter/search before popping) are handled in the per-screen
	// Update methods in the TUI adapter.
	c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	return c.snapshot(), nil
}

// handleActionMoveUp handles ActionMoveUp.
func (c *Controller) handleActionMoveUp(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		if ts.ScrollY > 0 {
			ts.ScrollY--
		}
	} else if ls := c.topListState(); ls != nil {
		visible := c.listVisibleCount(ls)
		if ls.SelectedRow > 0 {
			ls.SelectedRow--
		}
		_ = visible
	} else if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		if ms.Cursor > 0 {
			ms.Cursor--
		}
		menuSkipUnavailable(ms, visible, -1)
	} else if ss := c.topSelectorState(); ss != nil {
		visible := selectorVisibleItems(ss)
		if ss.Cursor > 0 {
			ss.Cursor--
		}
		_ = visible
	}
	return c.snapshot(), nil
}

// handleActionMoveDown handles ActionMoveDown.
func (c *Controller) handleActionMoveDown(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.ScrollY++
	} else if ls := c.topListState(); ls != nil {
		visible := c.listVisibleCount(ls)
		if ls.SelectedRow < visible-1 {
			ls.SelectedRow++
		}
	} else if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		if ms.Cursor < len(visible)-1 {
			ms.Cursor++
		}
		menuSkipUnavailable(ms, visible, +1)
	} else if ss := c.topSelectorState(); ss != nil {
		visible := selectorVisibleItems(ss)
		if ss.Cursor < len(visible)-1 {
			ss.Cursor++
		}
	}
	return c.snapshot(), nil
}

// handleActionMoveTop handles ActionMoveTop.
func (c *Controller) handleActionMoveTop(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.ScrollY = 0
	} else if ls := c.topListState(); ls != nil {
		ls.SelectedRow = 0
	} else if ms := c.topMenuState(); ms != nil {
		ms.Cursor = 0
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		menuSkipUnavailable(ms, visible, +1)
	} else if ss := c.topSelectorState(); ss != nil {
		ss.Cursor = 0
	}
	return c.snapshot(), nil
}

// handleActionMoveBottom handles ActionMoveBottom.
func (c *Controller) handleActionMoveBottom(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		if n := len(ts.Lines); n > 0 {
			ts.ScrollY = n - 1
		}
	} else if ls := c.topListState(); ls != nil {
		visible := c.listVisibleCount(ls)
		if visible > 0 {
			ls.SelectedRow = visible - 1
		}
	} else if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		if len(visible) > 0 {
			ms.Cursor = len(visible) - 1
		}
		menuSkipUnavailable(ms, visible, -1)
	} else if ss := c.topSelectorState(); ss != nil {
		visible := selectorVisibleItems(ss)
		if len(visible) > 0 {
			ss.Cursor = len(visible) - 1
		}
	}
	return c.snapshot(), nil
}

// handleActionPageUp handles ActionPageUp.
func (c *Controller) handleActionPageUp(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.ScrollY -= textPageSizeFor(a)
		if ts.ScrollY < 0 {
			ts.ScrollY = 0
		}
	} else if ls := c.topListState(); ls != nil {
		pageSize := listPageSizeFor(a)
		ls.SelectedRow -= pageSize
		if ls.SelectedRow < 0 {
			ls.SelectedRow = 0
		}
	} else if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		ms.Cursor -= menuPageSizeFor(a)
		if ms.Cursor < 0 {
			ms.Cursor = 0
		}
		menuSkipUnavailable(ms, visible, -1)
	} else if ss := c.topSelectorState(); ss != nil {
		pageSize := selectorPageSizeFor(a)
		ss.Cursor -= pageSize
		if ss.Cursor < 0 {
			ss.Cursor = 0
		}
	}
	return c.snapshot(), nil
}

// handleActionPageDown handles ActionPageDown.
func (c *Controller) handleActionPageDown(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if ts := c.topTextState(); ts != nil {
		ts.ScrollY += textPageSizeFor(a)
	} else if ls := c.topListState(); ls != nil {
		pageSize := listPageSizeFor(a)
		visible := c.listVisibleCount(ls)
		ls.SelectedRow += pageSize
		if n := visible; ls.SelectedRow >= n {
			ls.SelectedRow = max(n-1, 0)
		}
	} else if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		ms.Cursor += menuPageSizeFor(a)
		if n := len(visible); ms.Cursor >= n {
			ms.Cursor = max(n-1, 0)
		}
		menuSkipUnavailable(ms, visible, +1)
	} else if ss := c.topSelectorState(); ss != nil {
		pageSize := selectorPageSizeFor(a)
		visible := selectorVisibleItems(ss)
		ss.Cursor += pageSize
		if n := len(visible); ss.Cursor >= n {
			ss.Cursor = max(n-1, 0)
		}
	}
	return c.snapshot(), nil
}

// handleActionScrollLeft handles ActionScrollLeft.
func (c *Controller) handleActionScrollLeft(_ Action) (ViewState, []runtime.TaskRequest) {
	if ls := c.topListState(); ls != nil {
		if ls.ScrollX > 0 {
			ls.ScrollX--
		}
	}
	return c.snapshot(), nil
}

// handleActionScrollRight handles ActionScrollRight.
func (c *Controller) handleActionScrollRight(_ Action) (ViewState, []runtime.TaskRequest) {
	if ls := c.topListState(); ls != nil {
		ls.ScrollX++
	}
	return c.snapshot(), nil
}

// handleActionSelect handles ActionSelect.
func (c *Controller) handleActionSelect(a Action) (ViewState, []runtime.TaskRequest) {
	// Resource/child list: open the detail of the currently-selected row,
	// identical to ActionOpenDetail. Enter and row-clicks in the web UI both
	// send ActionSelect; the TUI uses ActionOpenDetail from its key handler.
	if ls := c.topListState(); ls != nil {
		return c.openSelectedListDetail()
	}

	// Related-panel Enter: when the top screen is a detail view and
	// RelatedFocus is active, navigate to the focused related row.
	if ds := c.topDetailState(); ds != nil && ds.RelatedFocus {
		// Find the row at RelatedCursor using the same filter logic as
		// detailRelatedVisibleCount.
		query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
		var focusedRow *DetailRelatedRow
		idx := 0
		for i := range ds.RelatedRows {
			row := &ds.RelatedRows[i]
			if isSelfPivotZeroDetailRow(*row, ds.ResourceType) {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
				continue
			}
			if idx == ds.RelatedCursor {
				focusedRow = row
				break
			}
			idx++
		}
		if focusedRow != nil && isActionableDetailRow(*focusedRow) {
			// Derive the single target ID when there is exactly one related
			// resource (used by NavigationKindDetail cache-hit path).
			targetID := ""
			if len(focusedRow.ResourceIDs) == 1 {
				targetID = focusedRow.ResourceIDs[0]
			}
			// Look up the checker from the registered RelatedDef; DetailRelatedRow
			// is a serialisable value type (no funcs/checker field).
			var checker resource.RelatedChecker
			for _, def := range resource.GetRelated(ds.ResourceType) {
				if def.TargetType == focusedRow.TargetType {
					checker = def.Checker
					break
				}
			}
			ev := runtime.RelatedNavigateEvent{
				TargetType:     focusedRow.TargetType,
				SourceResource: ds.Resource,
				SourceType:     ds.ResourceType,
				TargetID:       targetID,
				RelatedIDs:     focusedRow.ResourceIDs,
				FetchFilter:    focusedRow.FetchFilter,
				Checker:        checker,
			}
			tasks := c.dispatchRelatedNavigate(ev)
			return c.snapshot(), tasks
		}
		return c.snapshot(), nil
	}

	if ms := c.topMenuState(); ms != nil {
		all := resource.AllResourceTypes()
		visible := menuVisibleItems(ms, all)
		if len(visible) > 0 && ms.Cursor < len(visible) {
			selected := visible[ms.Cursor]
			// Block navigation to confirmed-empty types (count known, zero, not
			// truncated). Availability may be stored under an alias key, so resolve
			// it via menuActiveKey — matching MenuSelected (the TUI Enter path).
			if ms.Availability != nil {
				activeKey := menuActiveKey(ms, selected)
				isTruncated := ms.Truncated != nil && ms.Truncated[activeKey]
				if count, known := ms.Availability[activeKey]; known && count == 0 && !isTruncated {
					return c.snapshot(), nil
				}
			}
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
				Target:       runtime.NavigateTargetResourceList,
				ResourceType: selected.ShortName,
			})
			c.applyNavResult(res)
			return c.snapshot(), tasks
		}
	}
	return c.snapshot(), nil
}

// handleActionRelatedSelect handles ActionRelatedSelect.
func (c *Controller) handleActionRelatedSelect(a Action) (ViewState, []runtime.TaskRequest) {
	// Web UI click path: navigate to the related row at the visible index in
	// Arg. Sets RelatedFocus + RelatedCursor then delegates to the same
	// HandleRelatedNavigate path as the keyboard Enter in ActionSelect.
	ds := c.topDetailState()
	if ds == nil {
		return c.snapshot(), nil
	}
	clickIdx, err := strconv.Atoi(strings.TrimSpace(a.Arg))
	if err != nil || clickIdx < 0 {
		return c.snapshot(), nil
	}
	// Locate the row at clickIdx in the filtered visible list (mirrors
	// buildDetailRelatedBlocks / detailRelatedVisibleCount filter logic).
	query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
	var targetRow *DetailRelatedRow
	visIdx := 0
	for i := range ds.RelatedRows {
		row := &ds.RelatedRows[i]
		if isSelfPivotZeroDetailRow(*row, ds.ResourceType) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
			continue
		}
		if visIdx == clickIdx {
			targetRow = row
			break
		}
		visIdx++
	}
	if targetRow == nil || !isActionableDetailRow(*targetRow) {
		// Dead-end row: loading, error, count==-1 without FetchFilter, or
		// confirmed zero without FetchFilter/Approximate. No navigation.
		return c.snapshot(), nil
	}
	// Sync cursor state so the selection highlight is consistent with the
	// navigation that follows.
	ds.RelatedFocus = true
	ds.RelatedCursor = clickIdx
	// Navigate — identical to the ActionSelect related-Enter path.
	targetID := ""
	if len(targetRow.ResourceIDs) == 1 {
		targetID = targetRow.ResourceIDs[0]
	}
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated(ds.ResourceType) {
		if def.TargetType == targetRow.TargetType {
			checker = def.Checker
			break
		}
	}
	ev := runtime.RelatedNavigateEvent{
		TargetType:     targetRow.TargetType,
		SourceResource: ds.Resource,
		SourceType:     ds.ResourceType,
		TargetID:       targetID,
		RelatedIDs:     targetRow.ResourceIDs,
		FetchFilter:    targetRow.FetchFilter,
		Checker:        checker,
	}
	tasks := c.dispatchRelatedNavigate(ev)
	return c.snapshot(), tasks
}

// handleActionFieldSelect handles ActionFieldSelect.
func (c *Controller) handleActionFieldSelect(a Action) (ViewState, []runtime.TaskRequest) {
	// Web UI click path: navigate to the resource linked by the navigable
	// detail field at the visible index in Arg. Mirrors the TUI Enter-on-
	// navigable-field path (TargetType + NavID/Value → HandleRelatedNavigate).
	ds := c.topDetailState()
	if ds == nil {
		return c.snapshot(), nil
	}
	fieldIdx, err := strconv.Atoi(strings.TrimSpace(a.Arg))
	if err != nil || fieldIdx < 0 {
		return c.snapshot(), nil
	}
	// Build the fields list using the same pipeline as buildDetailBody so
	// $i in the template aligns with the slice index here.
	fields := buildDetailBody(ds, c.viewConfig).Fields
	if fieldIdx >= len(fields) {
		return c.snapshot(), nil
	}
	field := fields[fieldIdx]
	if !field.IsNavigable || field.TargetType == "" {
		return c.snapshot(), nil
	}
	// Mirror TUI: NavID overrides Value when present.
	targetID := field.Value
	if field.NavID != "" {
		targetID = field.NavID
	}
	// No RelatedIDs / FetchFilter / Checker needed: HandleRelatedNavigate
	// routes a single-ID event to a cache-hit detail or a by-ID fetch.
	ev := runtime.RelatedNavigateEvent{
		TargetType:     field.TargetType,
		SourceResource: ds.Resource,
		SourceType:     ds.ResourceType,
		TargetID:       targetID,
	}
	tasks := c.dispatchRelatedNavigate(ev)
	return c.snapshot(), tasks
}
