// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// handleActionBack handles ActionBack.
func (c *Controller) handleActionBack(_ Action) (ViewState, []runtime.TaskRequest) {
	// Costs screen: Esc pops one drill frame while drilled; only once back
	// at the root frame does it fall through to the generic screen-pop below
	// (leaving the Cost Explorer entirely).
	if cs := c.topCostsState(); cs != nil && c.applyCostsBack(cs) {
		return c.snapshot(), nil
	}

	// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
	// collapse (root-collapse is the "root" Command). Per-view Esc semantics
	// (clear filter/search before popping) are handled in the per-screen
	// Update methods in the TUI adapter.
	c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	// Owner decision #38 (2026-07-06): when the pop reveals a detail screen
	// with registered related defs, re-dispatch its related-resource checks —
	// a pivot left at the transient blank-navigable state (domain.RelatedUnknown,
	// no FetchFilter) must resolve to its real count once the user drills into the target
	// type and returns, without a manual Ctrl+R. Mirrors the shape
	// HandleRelatedCheckStarted (core/runtime/related.go) and
	// openRelatedDetail (core/app/navigate.go) already produce, so this
	// is renderer-agnostic — both TUI and web/headless callers get the
	// recompute from this single ActionBack effect.
	//
	// Deliberately NOT mirrored for a revealed text (YAML/JSON) screen (N2):
	// unlike a detail's related panel, a text screen has no persistent
	// "unresolved" badge state to repair on every reveal — it is either
	// enriched or not, and regenerateTextScreenLocked (detail_state.go) now
	// repairs every matching stacked text screen, not just the top one, the
	// moment ANY sibling operation for the same resource successfully folds
	// (e.g. YAML then JSON: JSON's result now regenerates the buried YAML
	// screen too). The only residual gap is Back pressed before either
	// operation's enrichment ever lands at all — the same kind of transient,
	// self-healing "not yet enriched" state enrichDetail's RawStruct==nil
	// case already treats as normal (detail_enrich_engine.go) — and
	// re-entering the view (detail → y/J) already re-triggers a fresh
	// workload via PushYAML/PushJSON's own beginDetailWorkloadLocked call.
	// Unconditionally recomputing on every text-screen reveal would cost a
	// real AWS re-fetch on the overwhelmingly common case (a screen the user
	// already fully viewed) to cover this rare, self-recovering window.
	var tasks []runtime.TaskRequest
	if ds := c.topDetailState(); ds != nil {
		_, tasks = c.beginDetailWorkloadLocked(ds.ResourceType, ds.Resource, false, true)
	}
	return c.snapshot(), tasks
}

// handleActionMoveUp handles ActionMoveUp.
func (c *Controller) handleActionMoveUp(a Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, handled := c.applyDetailActions(a); handled {
		return vs, tasks
	}
	if cs := c.topCostsState(); cs != nil {
		c.applyCostsMoveRow(cs, -1)
		return c.snapshot(), nil
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
		all := menuAllItems()
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
	if cs := c.topCostsState(); cs != nil {
		c.applyCostsMoveRow(cs, 1)
		return c.snapshot(), nil
	}
	if ts := c.topTextState(); ts != nil {
		ts.ScrollY++
	} else if ls := c.topListState(); ls != nil {
		visible := c.listVisibleCount(ls)
		if ls.SelectedRow < visible-1 {
			ls.SelectedRow++
		}
	} else if ms := c.topMenuState(); ms != nil {
		all := menuAllItems()
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
		all := menuAllItems()
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
		all := menuAllItems()
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
		all := menuAllItems()
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
		all := menuAllItems()
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
	if vs, tasks, ok := c.handleCostsScreenAction(func(cs *CostsState) *runtime.TaskRequest {
		return c.applyCostsMoveCol(cs, -1)
	}); ok {
		return vs, tasks
	}
	if ls := c.topListState(); ls != nil {
		if ls.ScrollX > 0 {
			ls.ScrollX--
		}
	}
	return c.snapshot(), nil
}

// handleActionScrollRight handles ActionScrollRight.
func (c *Controller) handleActionScrollRight(_ Action) (ViewState, []runtime.TaskRequest) {
	if vs, tasks, ok := c.handleCostsScreenAction(func(cs *CostsState) *runtime.TaskRequest {
		return c.applyCostsMoveCol(cs, 1)
	}); ok {
		return vs, tasks
	}
	if ls := c.topListState(); ls != nil {
		ls.ScrollX++
	}
	return c.snapshot(), nil
}

// handleActionSelect handles ActionSelect.
func (c *Controller) handleActionSelect(_ Action) (ViewState, []runtime.TaskRequest) {
	// Costs screen: Enter drills into the cursor's cell (FR-006).
	if vs, tasks, ok := c.handleCostsScreenAction(c.applyCostsSelect); ok {
		return vs, tasks
	}

	// Resource/child list: open the detail of the currently-selected row,
	// identical to ActionOpenDetail. Enter and row-clicks in the web UI both
	// send ActionSelect; the TUI uses ActionOpenDetail from its key handler.
	if ls := c.topListState(); ls != nil {
		return c.openSelectedListDetail()
	}

	// Related-panel Enter: when the top screen is a detail view and
	// RelatedFocus is active, navigate to the focused related row.
	if ds := c.topDetailState(); ds != nil && ds.RelatedFocus {
		focusedRow := ds.focusedRelatedRow()
		if focusedRow != nil && isActionableDetailRow(*focusedRow) {
			// Only a blank RelatedUnknown row (no count, no IDs, no filter)
			// resolves IN PLACE — re-dispatching the source's checks. A truncated
			// "(0+)" is RelatedResolved and navigates to a scoped list exactly like
			// "(N+)"; resource.RelatedEnter is the single arbiter so keyboard, mouse
			// and TUI Enter can never diverge on the zero lower bound.
			if resource.RelatedEnter(focusedRow.State, focusedRow.Count, focusedRow.Truncated) == resource.RelatedEnterResolveInPlace {
				_, tasks := c.beginDetailWorkloadLocked(ds.ResourceType, ds.Resource, false, true)
				return c.snapshot(), tasks
			}
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
				Truncated:      focusedRow.Truncated,
				Checker:        checker,
			}
			tasks := c.dispatchRelatedNavigate(ev)
			return c.snapshot(), tasks
		}
		return c.snapshot(), nil
	}

	if ms := c.topMenuState(); ms != nil {
		visible := menuVisibleItems(ms, menuAllItems())
		if len(visible) > 0 && ms.Cursor < len(visible) {
			selected := visible[ms.Cursor]
			if selected.ShortName == CostsMenuShortName {
				res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetCosts})
				tasks = append(tasks, c.applyNavResult(res)...)
				return c.snapshot(), tasks
			}
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
			tasks = append(tasks, c.applyNavResult(res)...)
			return c.snapshot(), tasks
		}
	}
	return c.snapshot(), nil
}

// handleActionCostZoomIn handles ActionCostZoomIn.
func (c *Controller) handleActionCostZoomIn(_ Action) (ViewState, []runtime.TaskRequest) {
	vs, tasks, _ := c.handleCostsScreenAction(func(cs *CostsState) *runtime.TaskRequest {
		return c.applyCostZoom(cs, true)
	})
	return vs, tasks
}

// handleActionCostZoomOut handles ActionCostZoomOut.
func (c *Controller) handleActionCostZoomOut(_ Action) (ViewState, []runtime.TaskRequest) {
	vs, tasks, _ := c.handleCostsScreenAction(func(cs *CostsState) *runtime.TaskRequest {
		return c.applyCostZoom(cs, false)
	})
	return vs, tasks
}

// handleActionCostMetric handles ActionCostMetric.
func (c *Controller) handleActionCostMetric(_ Action) (ViewState, []runtime.TaskRequest) {
	vs, tasks, _ := c.handleCostsScreenAction(c.applyCostMetricCycle)
	return vs, tasks
}

// handleActionCostPivot handles ActionCostPivot. a.N carries the pressed digit.
func (c *Controller) handleActionCostPivot(a Action) (ViewState, []runtime.TaskRequest) {
	vs, tasks, _ := c.handleCostsScreenAction(func(cs *CostsState) *runtime.TaskRequest {
		return c.applyCostPivot(cs, a.N)
	})
	return vs, tasks
}

// handleCostsScreenAction runs apply against the top costs screen's
// CostsState and wraps the result for Apply callers — the
// topCostsState-guard, apply, snapshot, task-wrap shape every cost action
// handler shares. matched is false (and vs/tasks are zero) when the top
// screen is not costs; ScrollLeft/ScrollRight/Select's costs branch checks
// it before falling through to their own non-costs handling, and the four
// dedicated CostZoom*/Metric/Pivot handlers — which have no fallback at all
// — return it directly.
func (c *Controller) handleCostsScreenAction(apply func(cs *CostsState) *runtime.TaskRequest) (vs ViewState, tasks []runtime.TaskRequest, matched bool) {
	cs := c.topCostsState()
	if cs == nil {
		return c.snapshot(), nil, false
	}
	// apply must run — and finish mutating cs — before snapshot() reads the
	// controller's state: Go evaluates a return statement's operands
	// left-to-right, so inlining apply(cs) directly into snapshot()'s
	// argument list would capture the PRE-mutation state.
	task := apply(cs)
	return c.snapshot(), costsTaskSlice(task), true
}

// costsTaskSlice wraps an ensureCostsShapeFetched result (nil on a cache
// hit or no-op) into the []runtime.TaskRequest shape Apply callers expect.
func costsTaskSlice(t *runtime.TaskRequest) []runtime.TaskRequest {
	if t == nil {
		return nil
	}
	return []runtime.TaskRequest{*t}
}

// handleActionSelectIndex handles ActionSelectIndex: sets the cursor of the
// current screen (resource/child list, main menu, or selector) to the
// visible index carried in a.N, clamped to the visible range, then performs
// the same logic as ActionSelect. This is the atomic replacement for the web
// UI's row/entry click path, which previously replayed move-top + N×move-down
// + select as separate round-trips — a chain that landed on the wrong row
// whenever cursor movement skips entries (e.g. the main menu's
// skip-unavailable stepping over confirmed-empty resource types).
//
// a.N is the same visible index the renderer's template used to iterate the
// screen (ListBody.Rows / MenuBody.Entries / SelectorBody.Items), which is
// exactly what buildListBody/buildMenuBody/buildSelectorBody expose as
// .Selected — so template index == controller index by construction, and no
// cursor-movement replay is needed to reach it.
func (c *Controller) handleActionSelectIndex(a Action) (ViewState, []runtime.TaskRequest) {
	if ls := c.topListState(); ls != nil {
		visible := c.listVisibleCount(ls)
		ls.SelectedRow = clampInt(a.N, 0, visible-1)
		return c.handleActionSelect(a)
	}
	if ms := c.topMenuState(); ms != nil {
		all := menuAllItems()
		visible := menuVisibleItems(ms, all)
		ms.Cursor = clampInt(a.N, 0, len(visible)-1)
		return c.handleActionSelect(a)
	}
	if ss := c.topSelectorState(); ss != nil {
		visible := selectorVisibleItems(ss)
		ss.Cursor = clampInt(a.N, 0, len(visible)-1)
		return c.handleActionSelect(a)
	}
	return c.snapshot(), nil
}

// stepToSelectable advances cur in the given direction (+1/-1) over a list of
// length total, skipping any index for which isSkippable reports true — the
// shared scan behind the main menu's skip-unavailable stepping and the detail
// related-panel's cursor movement (menu.go's menuSkipUnavailable and
// detail_cursor.go's related-cursor stepping both delegate here so the two
// panels can never diverge in behavior).
//
// cur is the position already moved by the caller's ±1/page/top/bottom logic
// (this function does not perform that initial move). It first scans forward
// from cur in direction; if every remaining index in that direction is
// skippable, it falls back to scanning from cur-direction in the opposite
// direction (back toward and past the start), so a direction that runs off
// the end of the list still lands on the nearest selectable item behind it.
// If nothing in the list is selectable, cur is returned unchanged (stay put).
func stepToSelectable(cur, total, direction int, isSkippable func(i int) bool) int {
	if total <= 0 {
		return cur
	}
	start := cur
	for i := cur; i >= 0 && i < total; i += direction {
		if !isSkippable(i) {
			return i
		}
	}
	for i := start - direction; i >= 0 && i < total; i -= direction {
		if !isSkippable(i) {
			return i
		}
	}
	return cur
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
	// Locate the row at clickIdx in the filtered visible list (same walk as
	// buildDetailRelatedBlocks / detailRelatedVisibleCount / cursor stepping).
	targetRow := visibleRelatedRowAt(ds, clickIdx)
	if targetRow == nil || !isActionableDetailRow(*targetRow) {
		// Dead-end row: loading, error, unknown (RelatedUnknown) without
		// FetchFilter, or confirmed zero without FetchFilter/Truncated. No
		// navigation.
		return c.snapshot(), nil
	}
	// Sync cursor state so the selection highlight is consistent with the
	// navigation that follows.
	ds.RelatedFocus = true
	ds.RelatedCursor = clickIdx

	// Only a blank RelatedUnknown row (no count, no IDs, no filter) resolves IN
	// PLACE — re-dispatching this resource's related checks (same shape as
	// handleActionBack's recompute). A truncated "(0+)" is RelatedResolved and
	// navigates to a scoped list exactly like "(N+)"; resource.RelatedEnter is
	// the single arbiter shared with the keyboard and TUI Enter paths.
	if resource.RelatedEnter(targetRow.State, targetRow.Count, targetRow.Truncated) == resource.RelatedEnterResolveInPlace {
		_, tasks := c.beginDetailWorkloadLocked(ds.ResourceType, ds.Resource, false, true)
		return c.snapshot(), tasks
	}

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
		Truncated:      targetRow.Truncated,
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
	body, _ := c.buildDetailBody(ds)
	fields := body.Fields
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
