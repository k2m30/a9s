package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// updateFilterMode handles keys while in filter input mode.
func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		m.applyFilterToActiveView("")
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	m.applyFilterToActiveView(m.cmdInput.Value())
	return m, cmd
}

// applyFilterToActiveView applies the given filter text to whichever navigable view is active.
func (m *Model) applyFilterToActiveView(text string) {
	if f, ok := m.activeView().(views.Filterable); ok {
		f.SetFilter(text)
	}
}

// updateCommandMode handles keys while in command input mode.
func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		cmd := m.cmdInput.Value()
		m.cmdInput.Blur()
		return m.executeCommand(cmd)
	}

	var teaCmd tea.Cmd
	m.cmdInput, teaCmd = m.cmdInput.Update(msg)
	return m, teaCmd
}

// executeCommand dispatches a colon-command string.
func (m Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return m, nil
	}

	switch cmd {
	case "q", "quit":
		return m, tea.Quit
	case "ctx", "profile":
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}
			}
		}
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetProfile}
		}
	case "region":
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}
			}
		}
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetRegion}
		}
	case "help":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetHelp}
		}
	}

	rt := resource.FindResourceType(cmd)
	if rt != nil {
		return m, func() tea.Msg {
			return messages.NavigateMsg{
				Target:       messages.TargetResourceList,
				ResourceType: rt.ShortName,
			}
		}
	}

	return m, func() tea.Msg {
		return messages.FlashMsg{
			Text:    fmt.Sprintf("unknown command: %s", cmd),
			IsError: true,
		}
	}
}
