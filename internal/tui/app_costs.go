// app_costs.go — Cost Explorer screen key routing. Translates rsKindCosts
// key events into the costs Action kinds (internal/app/action.go); movement
// and drill/back reuse the same Action vocabulary every other ctrl-backed
// screen uses (ActionMoveUp/Down, ActionScrollLeft/Right, ActionSelect,
// ActionBack), following the same pattern as handleDetailKeyMsg /
// handleTextKeyMsg in app_stack.go.
package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// handleCostsKeyMsg routes key events on the Cost Explorer screen. Pivot,
// metric, zoom, and drill (Enter) can each surface a TaskRequest from
// Controller.Apply — usually KindFetchCosts on a query-shape miss (FR-017),
// but a RESOURCE_ID leaf's Enter can also surface KindFetchByIDDetail
// (navigate to the resource's own a9s detail view, FR-008). Every kind but
// KindFetchByIDDetail goes through m.dispatchTaskRequests, the shared switch
// every screen's adapter uses; KindFetchByIDDetail is routed by
// dispatchCostsByIDTask instead (see its own doc for why).
func (m Model) handleCostsKeyMsg(msg tea.KeyMsg, _ *rendererState) (tea.Model, tea.Cmd) {
	var tasks []runtime.TaskRequest
	switch {
	case key.Matches(msg, m.keys.Up):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
	case key.Matches(msg, m.keys.Down):
		m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
	case key.Matches(msg, m.keys.ScrollLeft):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionScrollLeft})
	case key.Matches(msg, m.keys.ScrollRight):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionScrollRight})
	case key.Matches(msg, m.keys.CostZoomIn):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionCostZoomIn})
	case key.Matches(msg, m.keys.CostZoomOut):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionCostZoomOut})
	case key.Matches(msg, m.keys.CostMetric):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionCostMetric})
	case key.Matches(msg, m.keys.Enter):
		_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionSelect})
	default:
		for i, b := range m.keys.CostPivot {
			if !key.Matches(msg, b) {
				continue
			}
			digit := i + 1
			if i == len(m.keys.CostPivot)-1 {
				digit = 0
			}
			_, tasks = m.ctrl.Apply(app.Action{Kind: app.ActionCostPivot, N: digit})
			break
		}
	}
	for _, t := range tasks {
		if t.Key.Kind == runtime.KindFetchByIDDetail {
			payload, ok := t.Payload.(runtime.FetchByIDDetailPayload)
			if !ok {
				continue
			}
			cmd := m.dispatchCostsByIDTask(payload)
			return m, cmd
		}
	}
	return m, m.dispatchTaskRequests(tasks)
}

// dispatchCostsByIDTask opens the detail view for a RESOURCE_ID leaf's
// KindFetchByIDDetail task. applyCostsSelect already pushed a placeholder
// ScreenResourceList (AutoOpenSingle set) onto the controller-owned stack
// before returning this task — the lane-neutral seam headless/web's
// autoOpenSingleDetail consumes on delivery. The TUI keeps its own separate
// rendererState stack (m.stack), which that controller-side push does not
// touch, so this pushes a matching placeholder rendererState here (mirrors
// m.newRelatedList's ctrl+TUI dual-push for the identical related-panel
// by-ID case) and fetches via a Navigate that carries ReplaceCurrent: true —
// NavigateKindPushDetail's own ReplaceCurrent branch then pops that
// placeholder (both stacks, via popRS' internal ActionBack) before pushing
// the resource's detail, so the two stacks land back in lockstep and a
// single Esc from the opened detail returns straight to the costs screen.
func (m *Model) dispatchCostsByIDTask(payload runtime.FetchByIDDetailPayload) tea.Cmd {
	placeholder := newListRS(payload.TargetType)
	w, h := m.innerSize()
	placeholder.width, placeholder.height = w, h
	m.pushRS(placeholder)

	inner := m.fetchByIDDetail(payload.TargetType, payload.ID)
	if inner == nil {
		return nil
	}
	return func() tea.Msg {
		msg := inner()
		if nav, ok := msg.(messages.Navigate); ok {
			nav.ReplaceCurrent = true
			return nav
		}
		return msg
	}
}

// popStrandedByIDPlaceholder pops the top rendererState when it is a list
// placeholder (dispatchCostsByIDTask's own push, or the generic
// related-navigate one) still waiting on exactly id — so a placeholder
// pushed for one by-ID fetch is never popped by a result belonging to a
// different one.
func (m *Model) popStrandedByIDPlaceholder(id string) bool {
	if m.activeRS().kind != rsKindList {
		return false
	}
	got, ok := m.ctrl.GetListExactRelatedTargetID()
	if !ok || got != id {
		return false
	}
	m.popRS()
	return true
}
