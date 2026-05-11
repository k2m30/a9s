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

// handleKeyMsg processes all keyboard input: force-quit, input modes, global
// keys, then falls through to the active view. Moved from app_handlers.go in
// Phase-05 PR-05a-h1 (AS-147).
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys handled before delegation
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	// Handle input modes — don't clear error hint during text input.
	switch m.inputMode {
	case modeFilter:
		return m.updateFilterMode(msg)
	case modeCommand:
		return m.updateCommandMode(msg)
	}

	// Clear error hint on any navigation keypress (not during text input).
	m.showErrorHint = false

	// If the active view is in search input mode, delegate all keys to it.
	// This prevents global keys (q, ?, i, etc.) from firing while typing a search query.
	if s, ok := m.activeView().(views.Searchable); ok && s.IsSearchInputMode() {
		return m.updateActiveView(msg)
	}

	// Global keys in normal mode
	if key.Matches(msg, m.keys.Help) {
		// If already on help, let the help view handle it (closes help)
		if _, ok := m.activeView().(*views.HelpModel); ok {
			return m.updateActiveView(msg)
		}
		ctx := m.helpContext()
		activeShortName := ""
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			activeShortName = rl.ShortName()
		}
		help := views.NewHelpWithResource(m.keys, ctx, activeShortName)
		help.SetSize(m.innerSize())
		m.pushView(&help)
		return m, nil
	}
	if key.Matches(msg, m.keys.Identity) {
		// If already on identity view, let it handle the key (dismisses)
		if _, ok := m.activeView().(*views.IdentityModel); ok {
			return m.updateActiveView(msg)
		}
		id := views.NewIdentity(m.Session.Profile, m.Session.Region, m.keys)
		if m.Session.Identity != nil {
			data := m.identityToViewData()
			id.SetIdentity(data)
		}
		id.SetSize(m.innerSize())
		m.pushView(&id)
		// Always re-fetch on i press
		m.Session.IdentityFetching = true
		cmd := m.fetchIdentity()
		return m, cmd
	}
	if key.Matches(msg, m.keys.ErrorLog) {
		// If already viewing the error log, let the view handle the key.
		if ym, ok := m.activeView().(*views.YAMLModel); ok && ym.IsTextViewer() {
			return m.updateActiveView(msg)
		}
		if len(m.errorHistory) == 0 {
			return m.handleFlash(messages.FlashMsg{Text: "No errors this session"})
		}
		var sb strings.Builder
		for i := len(m.errorHistory) - 1; i >= 0; i-- {
			e := m.errorHistory[i]
			fmt.Fprintf(&sb, "[%s] %s\n", e.time.Format("15:04:05"), e.message)
		}
		tv := views.NewTextViewer("errors", sb.String(), m.keys)
		tv.SetSize(m.innerSize())
		m.pushView(&tv)
		return m, nil
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Escape) {
		if d, ok := m.activeView().(*views.DetailModel); ok && d.ConsumesEscapeLocally() {
			return m.updateActiveView(msg)
		}
		// If active view has active search (confirmed highlights), delegate Esc to clear it.
		if s, ok := m.activeView().(views.Searchable); ok && s.IsSearchActive() {
			return m.updateActiveView(msg)
		}
		// Related-navigation resource lists should pop immediately on Esc.
		if rl, ok := m.activeView().(*views.ResourceListModel); ok && rl.EscPops() {
			m.popView()
			return m, nil
		}
		// If active view has a confirmed filter, clear it first
		if f, ok := m.activeView().(views.Filterable); ok && f.GetFilter() != "" {
			f.SetFilter("")
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				m.cacheTopLevelResourceList(*rl)
			}
			return m, nil
		}
		// Otherwise pop view; no-op on main menu (never quit from Esc)
		m.popView()
		return m, nil
	}
	if key.Matches(msg, m.keys.Colon) {
		m.inputMode = modeCommand
		m.cmdInput.Reset()
		m.cmdInput.Focus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Filter) {
		// Only activate filter mode on filterable views
		if _, ok := m.activeView().(views.Filterable); ok {
			m.inputMode = modeFilter
			m.cmdInput.Reset()
			m.cmdInput.Focus()
			return m, nil
		}
		// On help views, delegate / to the view (which sends PopViewMsg to close).
		if _, ok := m.activeView().(*views.HelpModel); ok {
			return m.updateActiveView(msg)
		}
		// On searchable views (detail, YAML), delegate / for search activation.
		if _, ok := m.activeView().(views.Searchable); ok {
			return m.updateActiveView(msg)
		}
		// On other static views (reveal), consume / without action.
		return m, nil
	}

	// Copy (c) — context-dependent clipboard copy
	if key.Matches(msg, m.keys.Copy) {
		return m.handleCopy()
	}

	// Refresh (ctrl+r) — re-fetch resources in resource list
	if key.Matches(msg, m.keys.Refresh) {
		return m.handleRefresh()
	}

	// Reveal (x) — fetch and display value via registered reveal fetcher
	if key.Matches(msg, m.keys.Reveal) {
		return m.handleReveal()
	}

	return m.updateActiveView(msg)
}

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
		m.resetTabCycle()
		return m, nil
	}
	if key.Matches(msg, m.keys.Tab) {
		m.advanceTabCycle()
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		cmd := m.cmdInput.Value()
		m.cmdInput.Blur()
		m.resetTabCycle()
		return m.executeCommand(cmd)
	}

	// Any other keystroke (typing, backspace, etc.) invalidates the cycle.
	// The current completion is treated as a preview: revert the buffer to
	// the user's original prefix before forwarding the key, so backspace and
	// character insertion act on the prefix, not on the completion text.
	if m.tabPrefix != "" {
		m.cmdInput.SetValue(m.tabPrefix)
		m.cmdInput.SetCursor(len(m.tabPrefix))
	}
	m.resetTabCycle()
	var teaCmd tea.Cmd
	m.cmdInput, teaCmd = m.cmdInput.Update(msg)
	return m, teaCmd
}

// resetTabCycle clears the tab-completion cycle state so the next Tab press
// starts fresh from whatever the user has currently typed.
func (m *Model) resetTabCycle() {
	m.tabPrefix = ""
	m.tabMatches = nil
	m.tabIndex = 0
}

// advanceTabCycle handles a Tab keypress in command mode. On the first Tab
// for a given input it computes all matching candidates and shows the first
// one; on subsequent Tabs it rotates through the candidates. With a single
// candidate it behaves like the old single-shot completion.
func (m *Model) advanceTabCycle() {
	if m.tabPrefix == "" {
		prefix := strings.TrimSpace(m.cmdInput.Value())
		if prefix == "" {
			return
		}
		matches := commandMatches(prefix)
		if len(matches) == 0 {
			return
		}
		m.tabPrefix = prefix
		m.tabMatches = matches
		m.tabIndex = 0
		m.cmdInput.SetValue(matches[0])
		return
	}
	if len(m.tabMatches) == 0 {
		return
	}
	m.tabIndex = (m.tabIndex + 1) % len(m.tabMatches)
	m.cmdInput.SetValue(m.tabMatches[m.tabIndex])
}

// commandMatches returns all command/resource-type names (and aliases) whose
// names start with the given prefix, in a stable order: built-in commands
// first, then resource ShortNames, then aliases. Used by Tab cycling.
func commandMatches(prefix string) []string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}
	matches := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || !strings.HasPrefix(candidate, prefix) {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		matches = append(matches, candidate)
	}
	for _, cmd := range []string{"q", "quit", "ctx", "profile", "region", "theme", "help", "root", "main"} {
		add(cmd)
	}
	for _, rt := range resource.AllResourceTypes() {
		add(rt.ShortName)
		for _, alias := range rt.Aliases {
			add(alias)
		}
	}
	return matches
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
	case "root", "main":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetMainMenu}
		}
	case "ctx", "profile":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetProfile}
		}
	case "region":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetRegion}
		}
	case "theme":
		return m, func() tea.Msg {
			return messages.NavigateMsg{Target: messages.TargetTheme}
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
