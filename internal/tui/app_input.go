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
	if rl, ok := m.activeView().(*views.ResourceListModel); ok {
		m.cacheTopLevelResourceList(*rl)
	}
}

// updateCommandMode handles keys while in command input mode.
func (m Model) updateCommandMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}
	if key.Matches(msg, m.keys.Tab) {
		m.cmdInput.SetValue(m.autocompleteCommand(m.cmdInput.Value()))
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

func (m Model) autocompleteCommand(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	matches := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !strings.HasPrefix(candidate, input) {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		matches = append(matches, candidate)
	}

	for _, cmd := range []string{"q", "quit", "ctx", "profile", "region", "help"} {
		add(cmd)
	}
	for _, rt := range resource.AllResourceTypes() {
		add(rt.ShortName)
		for _, alias := range rt.Aliases {
			add(alias)
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}
	return input
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
