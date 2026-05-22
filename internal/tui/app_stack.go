// app_stack.go — PR-05a-h4-c (AS-963) tui.Model view-stack helpers.
//
// Split out of app.go so the view-stack manipulation surface
// (activeView, pushView, popView, innerSize, propagateSize,
// updateActiveView, cacheTopLevelResourceList, helpContext) lives in
// its own file and app.go stays inside the 300–400 LOC budget that the
// spec acceptance check enforces (`wc -l internal/tui/app.go`).
//
// All eight functions are pure renderer-side view-stack manipulation:
// they update the m.stack slice or read state from the active view.
// updateActiveView is the per-view-kind switch that delegates a
// tea.Msg to the right concrete view model and writes the updated
// model back into the stack.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// activeView returns the top of the view stack.
func (m *Model) activeView() views.View {
	return m.stack[len(m.stack)-1]
}

// pushView adds a new view to the stack.
func (m *Model) pushView(v views.View) {
	m.stack = append(m.stack, v)
}

// popView removes the top view. Returns false if only one entry remains.
func (m *Model) popView() bool {
	if len(m.stack) <= 1 {
		return false
	}
	// Sync list count back to main menu when popping directly from list → menu.
	// Only depth 2 (menu → list) triggers this. Related lists (pushed from a detail
	// view at depth 3+) are marked escPops=true because they show filtered subsets
	// of a resource type, not the global population — syncing their count back to
	// the menu badge would overwrite the real global count with a filter result.
	// See app_related.go for related-list construction.
	if len(m.stack) == 2 {
		if rl, ok := m.stack[1].(*views.ResourceListModel); ok {
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				shortName := rl.ShortName()
				if shortName != "" {
					newCount := rl.LoadedCount()
					newTrunc := rl.IsTruncated()
					curCount, known := menu.GetAvailability()[shortName]
					if !newTrunc || !known || newCount > curCount {
						menu.SetAvailability(shortName, newCount)
						menu.SetTruncated(shortName, newTrunc)
					}
					// Sync-back issue count with only-increase guard (T036, FR-022).
					// The list's Status-based issueCount may be lower than the menu's
					// enriched count. Never overwrite a higher enriched count.
					newIssues := rl.IssueCount()
					curIssues := menu.GetIssueCounts()[shortName]
					curIssueTrunc := menu.GetIssueTruncated()[shortName]
					switch {
					case newIssues > curIssues:
						// higher enriched count from rl: take it
						menu.SetIssues(shortName, newIssues, newTrunc)
					case newIssues == curIssues && curIssueTrunc && !newTrunc:
						// count confirmed but stale "+" must clear
						menu.SetIssues(shortName, newIssues, false)
					}
				}
			}
		}
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

// propagateSize calls SetSize on every view in the stack with inner dimensions.
func (m *Model) propagateSize() {
	w, h := m.innerSize()
	for _, v := range m.stack {
		v.SetSize(w, h)
	}
}

// updateActiveView delegates a message to the active view and merges the result.
func (m Model) updateActiveView(msg tea.Msg) (tea.Model, tea.Cmd) {
	active := m.activeView()
	switch v := active.(type) {
	case *views.MainMenuModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.ResourceListModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		m.cacheTopLevelResourceList(updated)
		return m, cmd
	case *views.DetailModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.YAMLModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.JSONModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.RevealModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.SelectorModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.HelpModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	case *views.IdentityModel:
		updated, cmd := v.Update(msg)
		m.stack[len(m.stack)-1] = &updated
		return m, cmd
	}
	return m, nil
}

// cacheTopLevelResourceList writes the active ResourceListModel's
// interactive state (resources, pagination, filter, sort, cursor,
// h-scroll) into the session's resource-list cache so that re-entering
// the same top-level resource list restores the view exactly. Child
// (parentContext != nil) and EscPops lists are skipped because they
// are filtered subsets and would pollute the global cache.
func (m *Model) cacheTopLevelResourceList(rl views.ResourceListModel) {
	if rl.ParentContext() != nil || rl.EscPops() {
		return
	}
	rt := rl.ResourceType()
	sortColIdx, sortAsc := rl.SortState()
	m.core.Session().ResourceCache[rt] = &domain.ListViewCacheEntry{
		Resources:     rl.AllResources(),
		Pagination:    rl.PaginationState(),
		FilterText:    rl.FilterText(),
		AttentionOnly: rl.AttentionOnly(),
		SortColIdx:    sortColIdx,
		SortAsc:       sortAsc,
		CursorPos:     rl.CursorPosition(),
		HScrollOffset: rl.HScrollOffset(),
	}
}

// helpContext determines the HelpContext from the current active view.
func (m *Model) helpContext() views.HelpContext {
	return m.activeView().GetHelpContext()
}
