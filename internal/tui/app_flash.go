package tui

// app_flash.go — flash-message lifecycle and API-error surface, owned by the
// TUI adapter. These handlers were split out of internal/tui/app_handlers.go
// in Phase-05 PR-05a-h1 (AS-147). They drive tea.Tick auto-clear timers and
// mutate tui-Model flash/errorHistory state that has not yet migrated to
// session.Session; that migration is a follow-up PR.

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// apiErrorFlashDuration controls how long API error messages stay visible.
// Longer than regular flash (2s) because error messages are more important.
const apiErrorFlashDuration = 5 * time.Second

// handleFlash sets the flash message and schedules its auto-clear.
func (m Model) handleFlash(msg messages.FlashMsg) (tea.Model, tea.Cmd) {
	newGen := m.flash.gen + 1
	m.flash = flashState{text: msg.Text, isError: msg.IsError, active: true, gen: newGen}
	if msg.IsError {
		m.errorHistory = append(m.errorHistory, errorEntry{time: time.Now(), message: msg.Text})
	}
	gen := m.flash.gen
	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return messages.ClearFlashMsg{Gen: gen}
	})
}

// handleClearFlash clears the flash if the generation matches (not stale).
func (m Model) handleClearFlash(msg messages.ClearFlashMsg) (tea.Model, tea.Cmd) {
	if msg.Gen == m.flash.gen {
		if m.flash.isError {
			m.showErrorHint = true
		}
		m.flash.active = false
	}
	return m, nil
}

// handleAPIError shows a flash error and clears loading state on the resource list.
func (m Model) handleAPIError(msg messages.APIErrorMsg) (tea.Model, tea.Cmd) {
	code, message, _ := awsclient.ClassifyAWSError(msg.Err)
	var flashText string
	if code != "" && code != "Unknown" {
		flashText = fmt.Sprintf("[%s] %s", code, message)
	} else {
		flashText = msg.Err.Error()
	}
	newGen := m.flash.gen + 1
	m.flash = flashState{text: flashText, isError: true, active: true, gen: newGen}
	m.errorHistory = append(m.errorHistory, errorEntry{time: time.Now(), message: flashText})
	if rl, ok := m.activeView().(*views.ResourceListModel); ok {
		rl.ClearLoading()
	}
	gen := m.flash.gen
	return m, tea.Tick(apiErrorFlashDuration, func(_ time.Time) tea.Msg {
		return messages.ClearFlashMsg{Gen: gen}
	})
}
