package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// handleActionOpenHelp handles ActionOpenHelp.
func (c *Controller) handleActionOpenHelp(a Action) (ViewState, []runtime.TaskRequest) {
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionBack handles ActionBack.
func (c *Controller) handleActionBack(_ Action) (ViewState, []runtime.TaskRequest) {
	// Pop a single screen, mirroring the TUI's m.popView() — NOT a full
	// collapse (root-collapse is the "root" Command). Per-view Esc semantics
	// (clear filter/search before popping) arrive with PR-C view state.
	c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
	return c.snapshot(), nil
}

// handleActionOpenIdentity handles ActionOpenIdentity.
func (c *Controller) handleActionOpenIdentity(_ Action) (ViewState, []runtime.TaskRequest) {
	// The runtime has no NavigateTargetIdentity: the TUI opens the identity
	// screen via direct key-handling (not HandleNavigate). The headless
	// controller pushes ScreenIdentity directly so tests can assert the stack
	// without standing up a full TUI.
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenIdentity}})
	c.core.SetIdentityFetching(true)
	c.identityLoading = true
	c.identityResult = nil
	c.identityErrMsg = ""
	fetchTask := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity},
		Payload: runtime.FetchIdentityPayload{},
	}
	return c.snapshot(), []runtime.TaskRequest{fetchTask}
}

// handleActionOpenErrorLog handles ActionOpenErrorLog.
func (c *Controller) handleActionOpenErrorLog(_ Action) (ViewState, []runtime.TaskRequest) {
	// Mirror the TUI's '!' key: flash when no errors recorded; otherwise push
	// a text screen with the log entries newest-first.
	c.showErrorHint = false
	if len(c.errorHistory) == 0 {
		intents, tasks := c.core.HandleFlash(runtime.FlashEvent{
			Text:    "No errors this session",
			IsError: false,
			NewGen:  c.core.ConnectGen(),
		})
		c.applyIntents(intents)
		return c.snapshot(), tasks
	}
	var sb strings.Builder
	for i := len(c.errorHistory) - 1; i >= 0; i-- {
		e := c.errorHistory[i]
		fmt.Fprintf(&sb, "[%s] %s\n", e.t.Format("15:04:05"), e.message)
	}
	lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
	c.applyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenErrorLog}})
	c.ensureTextState(lines)
	return c.snapshot(), nil
}

// handleActionSelectProfile handles ActionSelectProfile.
func (c *Controller) handleActionSelectProfile(a Action) (ViewState, []runtime.TaskRequest) {
	// ConnectGen is read pre-Rotate; HandleProfileSelected calls Rotate internally.
	// NewGen is passed as the bumped flash gen for the "Switching to …" tick.
	// The headless controller has no flash.gen to bump, so we pass the current
	// ConnectGen as a stable stand-in — the ClearFlash tick is adapter-owned.
	intents, tasks := c.core.HandleProfileSelected(runtime.ProfileSelectedEvent{
		Profile: a.Arg,
		NewGen:  c.core.ConnectGen(),
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionSelectRegion handles ActionSelectRegion.
func (c *Controller) handleActionSelectRegion(a Action) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleRegionSelected(runtime.RegionSelectedEvent{
		Region: a.Arg,
		NewGen: c.core.ConnectGen(),
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionSelectTheme handles ActionSelectTheme.
func (c *Controller) handleActionSelectTheme(a Action) (ViewState, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleThemeSelected(runtime.ThemeSelectedEvent{
		Theme: a.Arg,
	})
	c.applyIntents(intents)
	return c.snapshot(), tasks
}

// handleActionCommand handles ActionCommand.
func (c *Controller) handleActionCommand(a Action) (ViewState, []runtime.TaskRequest) {
	// Arg carries a colon-command token (mirrors executeCommand in app_input.go).
	// Only arg-driven tokens are dispatched here; tokens that need selected-row
	// or per-screen state are noted as PR-C TODOs below.
	switch a.Arg {
	case "root", "main":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetMainMenu})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "profile", "ctx":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetProfile})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "region":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetRegion})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "theme":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetTheme})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	case "help":
		res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{Target: runtime.NavigateTargetHelp})
		c.applyNavResult(res)
		return c.snapshot(), tasks

	default:
		// Resource short-name or alias (e.g. "ec2", "s3", "dbi").
		if rt := resource.FindResourceType(a.Arg); rt != nil {
			res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
				Target:       runtime.NavigateTargetResourceList,
				ResourceType: a.Arg,
			})
			c.applyNavResult(res)
			return c.snapshot(), tasks
		}
		// TODO PR-C: "q"/"quit" needs tea.Quit from the renderer, not the controller.
		// Unknown tokens are silently dropped at this layer; the renderer flashes.
	}
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

// handleActionOpenYAML handles ActionOpenYAML.
func (c *Controller) handleActionOpenYAML(_ Action) (ViewState, []runtime.TaskRequest) {
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetYAML,
		ResourceType: typeName,
		Resource:     &r,
	})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionOpenJSON handles ActionOpenJSON.
func (c *Controller) handleActionOpenJSON(_ Action) (ViewState, []runtime.TaskRequest) {
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetJSON,
		ResourceType: typeName,
		Resource:     &r,
	})
	c.applyNavResult(res)
	return c.snapshot(), tasks
}

// handleActionReveal handles ActionReveal.
func (c *Controller) handleActionReveal(_ Action) (ViewState, []runtime.TaskRequest) {
	// Resolve the resource from the active list or detail screen.
	var revealRes *resource.Resource
	var revealType string
	if ds := c.topDetailState(); ds != nil {
		r := ds.Resource
		revealRes = &r
		revealType = ds.ResourceType
	} else if ls := c.topListState(); ls != nil {
		r, ok := c.listSelected()
		if ok {
			revealRes = &r
			if top := c.stack[len(c.stack)-1]; len(c.stack) > 0 {
				revealType = top.Ctx.ResourceType
			}
		}
	}
	if revealRes == nil {
		return c.snapshot(), nil
	}
	res, tasks := c.core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetReveal,
		ResourceType: revealType,
		Resource:     revealRes,
	})
	// KindFetchReveal: no stack push yet — the push happens when
	// Handle receives messages.ValueRevealed and calls HandleValueRevealed.
	_ = res
	return c.snapshot(), tasks
}

// handleActionChildView handles ActionChildView.
func (c *Controller) handleActionChildView(a Action) (ViewState, []runtime.TaskRequest) {
	// Arg carries the trigger key (e, L, R, r, s, Enter, t …).
	triggerKey := a.Arg
	if triggerKey == "" {
		return c.snapshot(), nil
	}
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	td := resource.FindResourceType(typeName)
	if td == nil {
		return c.snapshot(), nil
	}
	// Walk the type's children to find the one registered under this key.
	var matchedChild *resource.ChildViewDef
	for i := range td.Children {
		ch := &td.Children[i]
		if ch.Key != triggerKey {
			continue
		}
		if ch.DrillCondition != nil && !ch.DrillCondition(r) {
			continue
		}
		matchedChild = ch
		break
	}
	if matchedChild == nil {
		return c.snapshot(), nil
	}
	// Build the parent context from ContextKeys.
	ctx := make(map[string]string, len(matchedChild.ContextKeys))
	for param, source := range matchedChild.ContextKeys {
		switch source {
		case "ID":
			ctx[param] = r.ID
		case "Name":
			ctx[param] = r.Name
		default:
			ctx[param] = r.Fields[source]
		}
	}
	displayName := ctx[matchedChild.DisplayNameKey]
	ev := runtime.EnterChildViewEvent{
		ChildType:     matchedChild.ChildType,
		ParentContext: ctx,
		DisplayName:   displayName,
	}
	intents, tasks := c.core.HandleEnterChildView(ev)
	c.applyIntents(intents)
	// Seed the child list screen's context and state after PushScreen.
	if len(c.stack) > 0 {
		top := &c.stack[len(c.stack)-1]
		if top.ID == runtime.ScreenChildList {
			top.Ctx.ResourceType = matchedChild.ChildType
			if top.State.List == nil {
				top.State.List = &ListState{
					Loading:       true,
					ParentContext: ctx,
				}
			}
		}
	}
	return c.snapshot(), tasks
}

// handleActionCloudTrail handles ActionCloudTrail.
func (c *Controller) handleActionCloudTrail(_ Action) (ViewState, []runtime.TaskRequest) {
	// Navigate to the CloudTrail Events ("ct-events") list filtered to the
	// active resource. Mirrors the TUI's 't' key: BuildCloudTrailFilter →
	// RelatedNavigate to "ct-events" with a FetchFilter (server-side filtered
	// fetch). No-ops when the resource type has no CloudTrailKey.
	r, typeName, ok := c.selectedResourceForAction()
	if !ok {
		return c.snapshot(), nil
	}
	ff := resource.BuildCloudTrailFilter(r, typeName)
	if ff == nil {
		return c.snapshot(), nil
	}
	ev := runtime.RelatedNavigateEvent{
		TargetType:     "ct-events",
		SourceResource: r,
		SourceType:     typeName,
		FetchFilter:    ff,
	}
	tasks := c.dispatchRelatedNavigate(ev)
	return c.snapshot(), tasks
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
