package tui

// app_flash.go — flash-message lifecycle and API-error surface, owned by the
// TUI adapter. These handlers were split out of internal/tui/app_handlers.go
// in Phase-05 PR-05a-h1 (AS-147), and their handler bodies were ported to
// runtime.Core in PR-05a-h3 (AS-324). The functions below are thin
// (≤12-line) adapters: they pre-bump the tui-side flashState.gen counter
// (which the Core echoes back via FlashTickPayload.Gen), translate the
// messages.* into the runtime.*Event, call the Core method, apply the
// returned intents, and translate the returned tasks into tea.Cmds.

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// handleFlash bumps the flash gen, then defers to runtime.Core.HandleFlash
// for the state computation. Auto-clear tick is dispatched via the
// returned FlashTickPayload.
func (m Model) handleFlash(msg messages.Flash) (tea.Model, tea.Cmd) {
	m.flash.gen++
	intents, tasks := m.core.HandleFlash(runtime.FlashEvent{
		Text: msg.Text, IsError: msg.IsError, NewGen: m.flash.gen,
	})
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}

// handleClearFlash defers to runtime.Core.HandleClearFlash, which honours
// the gen guard and emits the SetErrorHintIntent when the cleared flash
// was an error flash.
func (m Model) handleClearFlash(msg messages.ClearFlash) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleClearFlash(runtime.ClearFlashEvent{
		Gen: msg.Gen, CurrentGen: m.flash.gen, IsError: m.flash.isError,
	})
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}

// handleAPIError bumps the flash gen and defers to runtime.Core.HandleAPIError
// for the classification + flash text computation. The active resource
// list's loading spinner is cleared via ClearActiveListLoadingIntent.
func (m Model) handleAPIError(msg messages.APIError) (tea.Model, tea.Cmd) {
	m.flash.gen++
	intents, tasks := m.core.HandleAPIError(runtime.APIErrorEvent{
		Err: msg.Err, NewGen: m.flash.gen,
	})
	cmd := m.dispatchHandlerResult(intents, tasks)
	return m, cmd
}
