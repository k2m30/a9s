package tui

// app_screens.go — view-stack push handlers for screens that are
// constructed in response to a runtime/event signal (reveal result,
// child-view enter). Split out of internal/tui/app_handlers.go in
// Phase-05 PR-05a-h1 (AS-147). PR-05a-h4-a (AS-769) ported both bodies
// to (c *Core) Handle* methods backed by the screen-builder registry
// in internal/tui/screens.go; each adapter below is a ≤12-line shim
// that translates the message to its runtime.*Event, calls the Core
// method, and dispatches the returned intents/tasks.

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// handleValueRevealed delegates to runtime.Core.HandleValueRevealed.
// On Err the Core emits a flash error; on success it emits
// PushScreen{ScreenReveal, RevealPayload{...}} which the screens.go
// builder materialises as a RevealModel.
func (m Model) handleValueRevealed(msg messages.ValueRevealed) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleValueRevealed(runtime.ValueRevealedEvent{
		ResourceID: msg.ResourceID, Value: msg.Value, Err: msg.Err,
	})
	cmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, cmd
}

// handleEnterChildView delegates to runtime.Core.HandleEnterChildView.
// The Core validates ChildType via resource.GetChildType; unknown
// types flash an error, known types emit PushScreen{ScreenChildList}
// paired with a TaskKindFetchChildResources whose payload the adapter
// translates into the existing fetchChildResources closure.
func (m Model) handleEnterChildView(msg messages.EnterChildView) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleEnterChildView(runtime.EnterChildViewEvent{
		ChildType: msg.ChildType, ParentContext: msg.ParentContext, DisplayName: msg.DisplayName,
	})
	cmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, cmd
}
