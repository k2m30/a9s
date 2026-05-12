package tui

// app_screens.go — view-stack push handlers for screens that are constructed
// in response to a runtime/event signal (reveal result, child-view enter).
// Split out of internal/tui/app_handlers.go in Phase-05 PR-05a-h1 (AS-147).
// Both bodies call views.New<Screen>(...) Bubble Tea constructors and
// manipulate the adapter-owned view stack — they stay in tui by design.
// A follow-up PR can replace direct construction with PushScreen UIIntent
// emission once ScreenID/ScreenContext cover Reveal/ChildList contexts.

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleValueRevealed pushes the reveal view or flashes an error.
func (m Model) handleValueRevealed(msg messages.ValueRevealed) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		errText := "reveal failed: " + msg.Err.Error()
		return m, func() tea.Msg {
			return messages.Flash{Text: errText, IsError: true}
		}
	}
	rv := views.NewReveal(msg.ResourceID, msg.Value, m.keys)
	rv.SetSize(m.innerSize())
	m.pushView(&rv)
	return m, nil
}

// handleEnterChildView creates a child resource list view and kicks off a fetch
// using the child type registry. This is the generic handler for all child views.
func (m Model) handleEnterChildView(msg messages.EnterChildView) (tea.Model, tea.Cmd) {
	childTypeDef := resource.GetChildType(msg.ChildType)
	if childTypeDef == nil {
		return m, func() tea.Msg {
			return messages.Flash{Text: fmt.Sprintf("unknown child type: %s", msg.ChildType), IsError: true}
		}
	}
	rl := views.NewChildResourceList(*childTypeDef, msg.ParentContext, msg.DisplayName, m.viewConfig, m.keys)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	fetchCmd := m.fetchChildResources(msg.ChildType, msg.ParentContext)
	return m, tea.Batch(initCmd, fetchCmd)
}
