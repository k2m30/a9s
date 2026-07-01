package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// handleKeyMsg processes all keyboard input: force-quit, input modes, global
// keys, then falls through to the active view.
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

	rs := m.activeRS()

	// If the active screen has search in input mode, delegate all keys to it.
	// This prevents global keys (q, ?, i, etc.) from firing while typing a search.
	if rs.search.IsInputMode() {
		return m.updateActiveRS(msg)
	}

	// When the right-column filter is active on a detail screen, character keys
	// must reach the filter widget — not trigger global bindings like ToggleRelated
	// ('r'), Quit ('q'), Copy ('c'), etc. Esc is already caught above by the Escape
	// handler; ForceQuit is intentionally preserved as a universal exit.
	if rs.kind == rsKindDetail && rs.rightCol.IsFiltering() {
		return m.updateActiveRS(msg)
	}

	// Global keys in normal mode
	if key.Matches(msg, m.keys.Help) {
		// If already on help, route to rs update (which handles close).
		if rs.kind == rsKindHelp {
			return m.updateActiveRS(msg)
		}
		ctx := m.helpContext()
		activeShortName := ""
		if rs.kind == rsKindList {
			activeShortName = rs.resourceType
		}
		helpRS := newHelpRS(ctx, activeShortName)
		w, h := m.innerSize()
		helpRS.width, helpRS.height = w, h
		m.pushRS(helpRS)
		return m, nil
	}
	if key.Matches(msg, m.keys.Identity) {
		// If already on identity view, let the rs update handle it (dismisses).
		if rs.kind == rsKindIdentity {
			return m.updateActiveRS(msg)
		}
		idRS := newIdentityRS()
		// Seed with current identity if available so it shows immediately.
		if m.core.Identity() != nil {
			data := m.identityToViewData()
			idRS.identityData = data
			idRS.identityLoading = false
		}
		w, h := m.innerSize()
		idRS.width, idRS.height = w, h
		m.pushRS(idRS)
		// Always re-fetch on i press.
		m.core.SetIdentityFetching(true)
		cmd := m.fetchIdentity(m.core.ConnectGen())
		return m, cmd
	}
	if key.Matches(msg, m.keys.ErrorLog) {
		// If already viewing the error log (non-ctrl-backed text screen), let rs update handle it.
		if rs.kind == rsKindText && !rs.ctrlBacked {
			return m.updateActiveRS(msg)
		}
		if len(m.errorHistory) == 0 {
			return m.handleFlash(messages.Flash{Text: "No errors this session"})
		}
		var sb strings.Builder
		for i := len(m.errorHistory) - 1; i >= 0; i-- {
			e := m.errorHistory[i]
			fmt.Fprintf(&sb, "[%s] %s\n", e.time.Format("15:04:05"), e.message)
		}
		errRS := newErrorLogRS(sb.String())
		w, h := m.innerSize()
		errRS.width, errRS.height = w, h
		m.pushRS(errRS)
		return m, nil
	}
	if key.Matches(msg, m.keys.Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, m.keys.Escape) {
		// Detail: if right-column is focused or filtering, consume Esc locally.
		if rs.kind == rsKindDetail && (rs.rightCol.IsFocused() || rs.rightCol.IsFiltering()) {
			return m.updateActiveRS(msg)
		}
		// If active screen has active search (confirmed highlights), delegate Esc to clear it.
		if rs.search.IsActive() {
			return m.updateActiveRS(msg)
		}
		// Related-navigation resource lists should pop immediately on Esc.
		if rs.kind == rsKindList && m.ctrl.GetListEscPops() {
			m.popRS()
			return m, nil
		}
		// If active screen has a confirmed filter, clear it first.
		if rs.kind == rsKindMenu || rs.kind == rsKindList {
			filter := m.ctrl.GetListFilter()
			if rs.kind == rsKindMenu {
				// Menu filter state is in the menu body — check via snapshot.
				body := m.ctrl.Snapshot().Body
				if body.Menu != nil {
					filter = body.Menu.Filter
				}
			}
			if filter != "" {
				m.applyFilterToActiveRS("")
				return m, nil
			}
		}
		// Otherwise pop; no-op on main menu (never quit from Esc).
		m.popRS()
		return m, nil
	}
	if key.Matches(msg, m.keys.Colon) {
		m.inputMode = modeCommand
		m.cmdInput.Reset()
		m.cmdInput.Focus()
		return m, nil
	}
	if key.Matches(msg, m.keys.Filter) {
		// Activate filter mode on filterable screen kinds (menu, list, selector).
		if rs.kind == rsKindMenu || rs.kind == rsKindList || rs.kind == rsKindSelector {
			m.inputMode = modeFilter
			m.cmdInput.Reset()
			m.cmdInput.Focus()
			return m, nil
		}
		// On help screens, delegate / to the screen (which sends PopViewMsg to close).
		if rs.kind == rsKindHelp {
			return m.updateActiveRS(msg)
		}
		// On searchable screens (detail, text), delegate / for search activation.
		if rs.kind == rsKindDetail || rs.kind == rsKindText {
			return m.updateActiveRS(msg)
		}
		// On reveal — / is ignored (no filter, no search in reveal view).
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

	// ToggleRelated (r) — show/hide the right-column related panel on detail screens.
	// Handled here (not delegated to updateActiveRS) because there is no stored
	// DetailModel on the stack; all right-column state lives directly on the rs.
	if key.Matches(msg, m.keys.ToggleRelated) && rs.kind == rsKindDetail {
		return m.handleToggleRelated()
	}

	return m.updateActiveRS(msg)
}

// updateFilterMode handles keys while in filter input mode.
func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Escape) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		m.applyFilterToActiveRS("")
		return m, nil
	}
	if key.Matches(msg, m.keys.Enter) {
		m.inputMode = modeNormal
		m.cmdInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.cmdInput, cmd = m.cmdInput.Update(msg)
	m.applyFilterToActiveRS(m.cmdInput.Value())
	return m, cmd
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
			return messages.Navigate{Target: messages.TargetMainMenu}
		}
	case "ctx", "profile":
		return m, func() tea.Msg {
			return messages.Navigate{Target: messages.TargetProfile}
		}
	case "region":
		return m, func() tea.Msg {
			return messages.Navigate{Target: messages.TargetRegion}
		}
	case "theme":
		return m, func() tea.Msg {
			return messages.Navigate{Target: messages.TargetTheme}
		}
	case "help":
		return m, func() tea.Msg {
			return messages.Navigate{Target: messages.TargetHelp}
		}
	}

	rt := resource.FindResourceType(cmd)
	if rt != nil {
		return m, func() tea.Msg {
			return messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: rt.ShortName,
			}
		}
	}

	return m, func() tea.Msg {
		return messages.Flash{
			Text:    fmt.Sprintf("unknown command: %s", cmd),
			IsError: true,
		}
	}
}


