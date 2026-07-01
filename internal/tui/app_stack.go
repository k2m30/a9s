// app_stack.go — tui.Model renderer-state stack helpers.
//
// The view stack is now a []*rendererState slice: each entry carries only
// viewport, search widget, right-column, scroll-offset, reveal payload, help
// context, and terminal dimensions. No concrete view model pointers are stored.
//
// All logical screen state (menu cursor, list resources, detail fields, …)
// lives in the headless app.Controller and is read from m.ctrl.Snapshot()
// at render time.
package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// activeRS returns the top-of-stack rendererState.
func (m *Model) activeRS() *rendererState {
	return m.stack[len(m.stack)-1]
}

// pushRS appends a new rendererState to the stack.
func (m *Model) pushRS(rs *rendererState) {
	m.stack = append(m.stack, rs)
}

// popRS removes the top rendererState. Returns false when only one entry remains.
//
// At depth 2 (menu → list), we sync the list's loaded count back to the menu
// availability badge if the controller reports this is a top-level list.
// All controller-state reads happen BEFORE calling ActionBack so we observe
// the list's state, not the menu's.
func (m *Model) popRS() bool {
	if len(m.stack) <= 1 {
		return false
	}
	// Sync list counts back to the menu badge when popping directly from list → menu.
	// Only depth 2 (menu → list) triggers this. Related lists (escPops=true) are
	// skipped — they show filtered subsets, not the global population.
	if len(m.stack) == 2 {
		body := m.ctrl.Snapshot().Body
		if body.Kind == app.BodyKindList {
			if !m.ctrl.GetListEscPops() && m.ctrl.GetListParentContext() == nil {
				shortName := m.activeRS().resourceType
				if shortName != "" {
					newCount := len(m.ctrl.GetListAllResources())
					newTrunc, _ := m.ctrl.GetListPagination()
					availability := m.ctrl.GetMenuAvailability()
					curCount, known := availability[shortName]
					if !newTrunc || !known || newCount > curCount {
						m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuAvailability{
							ResourceType: shortName,
							Count:        newCount,
							Truncated:    newTrunc,
						}})
					}
					// Sync-back issue count with only-increase guard (T036, FR-022).
					newIssues := m.ctrl.GetListIssueCount()
					curIssues := m.ctrl.GetMenuIssueCounts()[shortName]
					curIssueTrunc := m.ctrl.GetMenuIssueTruncated()[shortName]
					switch {
					case newIssues > curIssues:
						m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenu{
							ResourceType: shortName,
							Issues:       newIssues,
							Truncated:    newTrunc,
						}})
					case newIssues == curIssues && curIssueTrunc && !newTrunc:
						m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenu{
							ResourceType: shortName,
							Issues:       newIssues,
							Truncated:    false,
						}})
					}
				}
			}
		}
	}
	// Persist sort/cursor/scroll state to session cache before popping a list.
	// Must run before ActionBack so the controller still holds the list state.
	if m.activeRS().kind == rsKindList {
		m.cacheTopLevelResourceList()
	}
	// Keep the headless controller stack in sync: pop controller screen when the
	// rs being removed was ctrl-backed (i.e. a PushScreen was issued when it was
	// pushed). Help, identity-overlay, and error-log overlay are NOT ctrl-backed.
	if m.activeRS().ctrlBacked {
		m.ctrl.Apply(app.Action{Kind: app.ActionBack})
	}
	m.stack = m.stack[:len(m.stack)-1]
	return true
}

// innerSize returns the content area dimensions inside the frame.
func (m *Model) innerSize() (int, int) {
	w := m.width - 2
	h := m.height - 3
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// propagateSize copies inner dimensions onto every rendererState in the stack.
func (m *Model) propagateSize() {
	w, h := m.innerSize()
	for _, rs := range m.stack {
		rs.width = w
		rs.height = h
	}
}

// cacheTopLevelResourceList writes the active list's interactive state into
// the session resource-list cache so that re-entering the same top-level
// resource list restores the view exactly. Child (parentContext != nil) and
// EscPops lists are skipped because they are filtered subsets.
func (m *Model) cacheTopLevelResourceList() {
	if m.ctrl.GetListParentContext() != nil || m.ctrl.GetListEscPops() {
		return
	}
	rs := m.activeRS()
	rt := rs.resourceType
	if rt == "" {
		return
	}
	// Translate sort column key → index for ListViewCacheEntry.
	sortColIdx, sortAsc := -1, true
	col, dir := m.ctrl.GetListSort()
	if col != "" {
		td := resource.FindResourceType(rt)
		if td == nil {
			if ct := resource.GetChildType(rt); ct != nil {
				td = ct
			}
		}
		if td != nil {
			cols := m.ctrl.ResolveColumnsForType(td.ShortName)
			for i, c := range cols {
				effectiveKey := c.Key
				if effectiveKey == "" {
					effectiveKey = c.Path
				}
				if effectiveKey == "" {
					effectiveKey = c.Title
				}
				if effectiveKey == col {
					sortColIdx = i
					sortAsc = dir != "desc"
					break
				}
			}
		}
	}
	trunc, cursor := m.ctrl.GetListPagination()
	var paginationMeta *domain.PaginationMeta
	if trunc || cursor != "" {
		paginationMeta = &domain.PaginationMeta{
			IsTruncated: trunc,
			NextToken:   cursor,
		}
	}
	m.core.SetResourceCache(rt, &domain.ListViewCacheEntry{
		Resources:     m.ctrl.GetListAllResources(),
		Pagination:    paginationMeta,
		FilterText:    m.ctrl.GetListFilter(),
		AttentionOnly: m.ctrl.GetListAttentionOnly(),
		SortColIdx:    sortColIdx,
		SortAsc:       sortAsc,
		CursorPos:     m.ctrl.GetListSelectedRow(),
		HScrollOffset: m.ctrl.GetListScrollX(),
	})
}

// helpContext returns the HelpContext from the active rendererState.
func (m *Model) helpContext() views.HelpContext {
	return m.activeRS().helpContext
}

// updateActiveRS is the renderer-state equivalent of the former
// updateActiveView: it routes messages that the active rs itself needs to
// handle (viewport scrolling inside detail/text screens, search-widget keys,
// and key events on list/menu screens).
// Returns (tea.Model, tea.Cmd) to satisfy the Update signature.
func (m Model) updateActiveRS(msg tea.Msg) (tea.Model, tea.Cmd) {
	rs := m.activeRS()
	switch rs.kind {
	case rsKindDetail:
		// Detail screens: key events drive the controller (field cursor, Enter
		// navigation, YAML/JSON/CloudTrail). Non-key messages (viewport-internal
		// tick/resize) still go to the stored viewport.
		// Search-widget in input mode captures all keys before other detail handling.
		if rs.search.IsInputMode() {
			var cmd tea.Cmd
			rs.search, cmd = rs.search.Update(msg)
			// Commit or clear the query to the controller when input mode exits,
			// mirroring the same sync that handleTextKeyMsg does for text screens.
			if !rs.search.IsInputMode() {
				if rs.search.IsActive() {
					m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: rs.search.Query()})
				} else {
					m.ctrl.Apply(app.Action{Kind: app.ActionSearchClear})
				}
			}
			return m, cmd
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return m.handleDetailKeyMsg(keyMsg, rs)
		}
		var cmd tea.Cmd
		rs.viewport, cmd = rs.viewport.Update(msg)
		return m, cmd
	case rsKindReveal:
		// Viewport-bearing screens: route scroll messages to the stored viewport.
		var cmd tea.Cmd
		rs.viewport, cmd = rs.viewport.Update(msg)
		return m, cmd
	case rsKindText:
		// Text screens (YAML, JSON, error log): all key events (including search
		// input mode) go through handleTextKeyMsg so ctrl can be notified when
		// search input mode exits.
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			return m.handleTextKeyMsg(keyMsg, rs)
		}
		var cmd tea.Cmd
		rs.viewport, cmd = rs.viewport.Update(msg)
		return m, cmd
	case rsKindList:
		// Delegate key events to a full ResourceListModel wired to the live
		// controller. Cursor moves and sort actions write directly through
		// m.ctrl; Enter/YAML/JSON return navigation commands.
		var td resource.ResourceTypeDef
		if rt := resource.FindResourceType(rs.resourceType); rt != nil {
			td = *rt
		} else if child := resource.GetChildType(rs.resourceType); child != nil {
			td = *child
		}
		rl := views.NewResourceList(td, m.viewConfig, m.keys, m.ctrl)
		rl.SetSize(rs.width, rs.height)
		_, cmd := rl.Update(msg)
		return m, cmd
	case rsKindMenu:
		// Delegate key events to a full MainMenuModel wired to the live
		// controller. Movement actions write through m.ctrl; Enter returns
		// a Navigate command. Persist the adjusted scroll offset back to rs.
		menu := views.NewMainMenu(m.keys, m.ctrl)
		menu.SetSize(rs.width, rs.height)
		menu.SetScrollOffset(rs.scrollOffset)
		updated, cmd := menu.Update(msg)
		rs.scrollOffset = updated.GetScrollOffset()
		return m, cmd
	case rsKindSelector:
		// Delegate to a transient SelectorModel backed by the live controller
		// so that Up/Down/Enter work through the same ctrl.Apply paths that tests observe.
		sel := views.NewSelectorWithCtrl(m.ctrl, rs.onSelect, m.keys)
		sel.SetSize(rs.width, rs.height)
		_, cmd := sel.Update(msg)
		return m, cmd
	case rsKindHelp:
		// Any key on the help overlay closes it. Delegate to a transient HelpModel
		// so the PopView message is emitted and handled by the root Update loop.
		h := views.NewHelpWithResource(m.keys, rs.helpContext, rs.helpShortName)
		h.SetSize(rs.width, rs.height)
		_, cmd := h.Update(msg)
		return m, cmd
	case rsKindIdentity:
		// Any key on the identity overlay closes it.
		h := views.NewIdentity(m.core.Profile(), m.core.Region(), m.keys)
		h.SetSize(rs.width, rs.height)
		_, cmd := h.Update(msg)
		return m, cmd
	}
	return m, nil
}

// applyFilterToActiveRS routes a filter text change to the controller for
// ctrl-backed filterable screens (menu, list, selector). For all other screen
// kinds it is a no-op — reveal, help, identity, and text overlays are not filterable.
func (m *Model) applyFilterToActiveRS(text string) {
	rs := m.activeRS()
	switch rs.kind {
	case rsKindMenu, rsKindList, rsKindSelector:
		m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: text})
		if rs.kind == rsKindList {
			m.cacheTopLevelResourceList()
		}
	}
}

// handleDetailKeyMsg routes key events on a detail screen. Field-cursor movement
// and scrolling are driven through the controller; navigation keys (Enter on
// navigable field, YAML, JSON, CloudTrail) emit messages. The stored viewport is
// NOT used for key routing because detail scroll position is owned by the
// controller (DetailBody.ScrollY set by ActionPageDown/Up) and applied in
// RenderDetail via viewport.SetYOffset.
func (m Model) handleDetailKeyMsg(msg tea.KeyMsg, rs *rendererState) (tea.Model, tea.Cmd) {
	// Right-column focus or active filter: route all keys through the right-column
	// widget first, then sync any filter-state changes to the controller.
	if rs.rightCol.IsFocused() || rs.rightCol.IsFiltering() {
		prevFilter := rs.rightCol.FilterQuery()
		prevFiltering := rs.rightCol.IsFiltering()

		switch {
		case key.Matches(msg, m.keys.Tab):
			// Tab unfocuses the right column and moves focus back to the left panel.
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			rs.rightCol.SetFocused(false)
			return m, nil
		case key.Matches(msg, m.keys.Escape):
			if rs.rightCol.IsFiltering() || rs.rightCol.HasFilter() {
				// Let the widget clear its own filter state, then sync to ctrl.
				rs.rightCol, _ = rs.rightCol.Update(msg)
				m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})
				return m, nil
			}
			// Not filtering — Esc unfocuses.
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			rs.rightCol.SetFocused(false)
			return m, nil
		case !rs.rightCol.IsFiltering() && key.Matches(msg, m.keys.Up):
			// Cursor movement (not typing): route to controller and widget.
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
			rs.rightCol, _ = rs.rightCol.Update(msg)
			return m, nil
		case !rs.rightCol.IsFiltering() && key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
			rs.rightCol, _ = rs.rightCol.Update(msg)
			return m, nil
		case !rs.rightCol.IsFiltering() && key.Matches(msg, m.keys.Enter):
			// Related-panel Enter: navigate using ResourceIDs from controller
			// state (the focused ds.RelatedRows row), not the right-column
			// widget — the renderer holds no duplicate copy of the IDs.
			row, ok := m.ctrl.SelectedRelatedRow()
			if !ok || !resource.IsRelatedActionable(row.Count, row.Approximate, len(row.FetchFilter) > 0, row.Loading, row.Err != "") {
				return m, nil
			}
			var checker resource.RelatedChecker
			for _, def := range resource.GetRelated(rs.resourceType) {
				if def.TargetType == row.TargetType {
					checker = def.Checker
					break
				}
			}
			nav := messages.RelatedNavigate{
				TargetType:     row.TargetType,
				SourceResource: m.ctrl.GetDetailResource(),
				RelatedIDs:     row.ResourceIDs,
				FetchFilter:    row.FetchFilter,
				Checker:        checker,
			}
			return m, func() tea.Msg { return nav }
		default:
			// All other keys (including '/', character typing when filtering, Enter
			// for navigation/confirm, Backspace) go to the right-column widget.
			var cmd tea.Cmd
			rs.rightCol, cmd = rs.rightCol.Update(msg)

			// Sync filter state change to the controller so that buildDetailBody
			// uses the updated ds.RelatedFilter when building the snapshot.
			newFilter := rs.rightCol.FilterQuery()
			if newFilter != prevFilter || rs.rightCol.IsFiltering() != prevFiltering {
				m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: newFilter})
			}
			return m, cmd
		}
	}

	// Left-column key handling.
	switch {
	case key.Matches(msg, m.keys.Down):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		pageSize := max(rs.height-4, 1)
		m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: pageSize})
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		pageSize := max(rs.height-4, 1)
		m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: pageSize})
		return m, nil

	case key.Matches(msg, m.keys.ToggleWrap):
		m.ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		// Tab: toggle focus between left column and right column.
		if rs.rightColVisible {
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			rs.rightCol.SetFocused(!rs.rightCol.IsFocused())
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// Navigate to the target of the navigable field at the current cursor.
		body := m.ctrl.Snapshot().Body.Detail
		if body == nil {
			return m, nil
		}
		fc := body.FieldCursor
		if fc < 0 || fc >= len(body.Fields) {
			return m, nil
		}
		field := body.Fields[fc]
		if !field.IsNavigable || field.TargetType == "" {
			return m, nil
		}
		targetID := field.Value
		if field.NavID != "" {
			targetID = field.NavID
		}
		res := m.ctrl.GetDetailResource()
		rt := m.ctrl.GetDetailResourceType()
		return m, func() tea.Msg {
			return messages.RelatedNavigate{
				TargetType:     field.TargetType,
				SourceResource: res,
				SourceType:     rt,
				TargetID:       targetID,
			}
		}

	case key.Matches(msg, m.keys.YAML):
		res := m.ctrl.GetDetailResource()
		rt := m.ctrl.GetDetailResourceType()
		return m, func() tea.Msg {
			return messages.Navigate{
				Target:       messages.TargetYAML,
				Resource:     &res,
				ResourceType: rt,
			}
		}

	case key.Matches(msg, m.keys.JSON):
		res := m.ctrl.GetDetailResource()
		rt := m.ctrl.GetDetailResourceType()
		return m, func() tea.Msg {
			return messages.Navigate{
				Target:       messages.TargetJSON,
				Resource:     &res,
				ResourceType: rt,
			}
		}

	case key.Matches(msg, m.keys.CloudTrail):
		res := m.ctrl.GetDetailResource()
		rt := m.ctrl.GetDetailResourceType()
		if ff := resource.BuildCloudTrailFilter(res, rt); ff != nil {
			return m, func() tea.Msg {
				return messages.RelatedNavigate{
					TargetType:     "ct-events",
					SourceResource: res,
					SourceType:     rt,
					FetchFilter:    ff,
				}
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		// Esc while search is active clears the search widget and notifies the controller.
		if rs.search.IsActive() {
			rs.search.Deactivate()
			m.ctrl.Apply(app.Action{Kind: app.ActionSearchClear})
			return m, nil
		}

	case key.Matches(msg, m.keys.Search):
		// Activate search on the detail screen.
		rs.search.Activate()
		m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: ""})
		return m, nil

	case key.Matches(msg, m.keys.SearchNext):
		if rs.search.IsActive() {
			m.ctrl.Apply(app.Action{Kind: app.ActionSearchNext})
		}
		return m, nil

	case key.Matches(msg, m.keys.SearchPrev):
		if rs.search.IsActive() {
			m.ctrl.Apply(app.Action{Kind: app.ActionSearchPrev})
		}
		return m, nil

	case key.Matches(msg, m.keys.ScrollRight):
		// l: focus right column.
		if rs.rightColVisible && !rs.rightCol.IsFocused() {
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			rs.rightCol.SetFocused(true)
		}
		return m, nil

	case key.Matches(msg, m.keys.ScrollLeft):
		// h: focus left column.
		if rs.rightCol.IsFocused() {
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			rs.rightCol.SetFocused(false)
		}
		return m, nil
	}

	// Unhandled keys on detail screens are silently dropped (no viewport routing).
	return m, nil
}

// handleTextKeyMsg routes key events on a text screen (YAML, JSON, error log).
// Search, scroll, and wrap keys are handled; all others fall through to the
// viewport. This mirrors the key handling that YAMLModel.Update() performed
// before the renderer-state stack architecture removed stored view models.
func (m Model) handleTextKeyMsg(msg tea.KeyMsg, rs *rendererState) (tea.Model, tea.Cmd) {
	// Search input mode: capture all keys for the search widget.
	// When input mode exits (Enter/Esc), sync the new state to the controller
	// if the screen is ctrl-backed.
	if rs.search.IsInputMode() {
		wasInputMode := true
		var cmd tea.Cmd
		rs.search, cmd = rs.search.Update(msg)
		if wasInputMode && !rs.search.IsInputMode() && rs.ctrlBacked {
			if rs.search.IsActive() {
				// Enter was pressed — commit query.
				m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: rs.search.Query()})
			} else {
				// Esc was pressed — clear search.
				m.ctrl.Apply(app.Action{Kind: app.ActionSearchClear})
			}
		}
		return m, cmd
	}

	switch {
	case key.Matches(msg, m.keys.Search):
		rs.search.Activate()
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: ""})
		}
		return m, nil

	case key.Matches(msg, m.keys.SearchNext):
		if rs.search.IsActive() {
			if rs.ctrlBacked {
				m.ctrl.Apply(app.Action{Kind: app.ActionSearchNext})
			} else {
				rs.search.NextMatch()
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.SearchPrev):
		if rs.search.IsActive() {
			if rs.ctrlBacked {
				m.ctrl.Apply(app.Action{Kind: app.ActionSearchPrev})
			} else {
				rs.search.PrevMatch()
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		if rs.search.IsActive() {
			rs.search.Deactivate()
			if rs.ctrlBacked {
				m.ctrl.Apply(app.Action{Kind: app.ActionSearchClear})
			}
			return m, nil
		}

	case key.Matches(msg, m.keys.ToggleWrap):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
			return m, nil
		}

	case key.Matches(msg, m.keys.Down):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
			return m, nil
		}

	case key.Matches(msg, m.keys.Top):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
			return m, nil
		}

	case key.Matches(msg, m.keys.Bottom):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
			return m, nil
		}

	case key.Matches(msg, m.keys.PageUp):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: max(rs.height-1, 1)})
			return m, nil
		}

	case key.Matches(msg, m.keys.PageDown):
		if rs.ctrlBacked {
			m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: max(rs.height-1, 1)})
			return m, nil
		}
	}

	// For non-ctrl-backed text screens (e.g. error log) and unhandled keys,
	// fall through to the stored viewport.
	var cmd tea.Cmd
	rs.viewport, cmd = rs.viewport.Update(msg)
	return m, cmd
}
