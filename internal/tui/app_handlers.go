package tui

import (
	"fmt"
	"os"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// apiErrorFlashDuration controls how long API error messages stay visible.
// Longer than regular flash (2s) because error messages are more important.
const apiErrorFlashDuration = 5 * time.Second

// handleKeyMsg processes all keyboard input: force-quit, input modes, global
// keys, then falls through to the active view.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys handled before delegation
	if key.Matches(msg, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	// Handle input modes
	switch m.inputMode {
	case modeFilter:
		return m.updateFilterMode(msg)
	case modeCommand:
		return m.updateCommandMode(msg)
	}

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
		id := views.NewIdentity(m.profile, m.region, m.keys)
		if m.identity != nil {
			data := m.identityToViewData()
			id.SetIdentity(data)
		}
		id.SetSize(m.innerSize())
		m.pushView(&id)
		// Always re-fetch on i press
		m.identityFetching = true
		cmd := m.fetchIdentity()
		return m, cmd
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

// handleFlash sets the flash message and schedules its auto-clear.
func (m Model) handleFlash(msg messages.FlashMsg) (tea.Model, tea.Cmd) {
	newGen := m.flash.gen + 1
	m.flash = flashState{text: msg.Text, isError: msg.IsError, active: true, gen: newGen}
	gen := m.flash.gen
	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return messages.ClearFlashMsg{Gen: gen}
	})
}

// handleClearFlash clears the flash if the generation matches (not stale).
func (m Model) handleClearFlash(msg messages.ClearFlashMsg) (tea.Model, tea.Cmd) {
	if msg.Gen == m.flash.gen {
		m.flash.active = false
	}
	return m, nil
}

// handleClientsReady stores the new AWS clients and optionally triggers a
// pending refresh (after profile/region switch).
func (m Model) handleClientsReady(msg messages.ClientsReadyMsg) (tea.Model, tea.Cmd) {
	// Ignore stale results from a superseded connect (rapid profile/region switch)
	if msg.Gen != m.connectGen {
		return m, nil
	}

	if msg.Err != nil {
		// Rollback profile/region to previous stable values on connect failure
		if m.hasPrevState {
			m.profile = m.prevProfile
			m.region = m.prevRegion
		}
		m.hasPrevState = false
		m.prevProfile = ""
		m.prevRegion = ""
		m.pendingRefresh = false
		newGen := m.flash.gen + 1
		m.flash = flashState{text: msg.Err.Error(), isError: true, active: true, gen: newGen}
		gen := m.flash.gen
		clearFlash := tea.Tick(apiErrorFlashDuration, func(_ time.Time) tea.Msg {
			return messages.ClearFlashMsg{Gen: gen}
		})

		// The switch attempt cleared identity, resource cache, and availability.
		// Restore them using the still-valid old clients.
		var cmds []tea.Cmd
		cmds = append(cmds, clearFlash)
		if m.clients != nil {
			m.identityFetching = true
			cmds = append(cmds, m.fetchIdentity())
			if m.noCache {
				cmds = append(cmds, m.demoPrefetchCounts())
			} else {
				cmds = append(cmds, m.loadAvailabilityCache())
			}
		}
		return m, tea.Batch(cmds...)
	}
	if msg.Clients == nil {
		if m.clients == nil && m.preSuppliedClients != nil {
			// Fall back to pre-supplied clients (demo path) when msg carries no clients.
			m.clients = m.preSuppliedClients
		}
	} else if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok {
		m.clients = clients
	} else {
		wrongTypeErr := fmt.Errorf("internal: unexpected ClientsReadyMsg.Clients type %T", msg.Clients)
		return m, func() tea.Msg {
			return messages.APIErrorMsg{Err: wrongTypeErr}
		}
	}
	m.hasPrevState = false
	m.prevProfile = ""
	m.prevRegion = ""

	// Emit a one-shot NavigateMsg if the -c flag set an initial command. Only
	// fire while the app is still at the main menu (stack depth 1). If the
	// initial connection was slow and the user already navigated elsewhere,
	// skip the auto-navigation to avoid pushing a view on top of whatever
	// the user is doing. Cleared after first use so that subsequent
	// ClientsReadyMsg (profile/region switch) don't re-navigate.
	var navigateCmd tea.Cmd
	if m.command != "" {
		if len(m.stack) == 1 {
			target := m.command
			navigateCmd = func() tea.Msg {
				return messages.NavigateMsg{
					Target:       messages.TargetResourceList,
					ResourceType: target,
				}
			}
		}
		m.command = "" // always clear, even if skipped, to prevent stale re-navigation
	}

	if m.profile == "" {
		m.profile = "default"
	}
	if m.region == "" {
		if msg.Region != "" {
			m.region = msg.Region
		} else {
			configPath := awsclient.DefaultConfigPath()
			m.region = awsclient.GetDefaultRegion(configPath, m.profile)
		}
	}
	// In demo/no-cache mode, prefetch all counts synchronously in one cmd so the
	// main menu shows counts immediately without the async probe pipeline.
	// Skip identity fetch in demo mode — the profile/region are synthetic.
	if m.noCache {
		availCmd := m.demoPrefetchCounts()
		if m.pendingRefresh {
			m.pendingRefresh = false
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				m.flash = flashState{text: "Connected. Refreshing...", active: true}
				cmd := m.refreshResourceList(*rl)
				return m, tea.Batch(cmd, availCmd, navigateCmd)
			}
		}
		return m, tea.Batch(availCmd, navigateCmd)
	}

	// Fetch identity on every clients-ready event
	m.identityFetching = true
	identityCmd := m.fetchIdentity()

	// Load disk cache which then fires background probes for expired/missing entries.
	availCmd := m.loadAvailabilityCache()

	if m.pendingRefresh {
		m.pendingRefresh = false
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			m.flash = flashState{text: "Connected. Refreshing...", active: true}
			cmd := m.refreshResourceList(*rl)
			return m, tea.Batch(cmd, identityCmd, availCmd, navigateCmd)
		}
	}
	return m, tea.Batch(identityCmd, availCmd, navigateCmd)
}

// handleProfileSelected switches the AWS profile, pops the profile selector,
// and reconnects.
func (m Model) handleProfileSelected(msg messages.ProfileSelectedMsg) (tea.Model, tea.Cmd) {
	m.relatedCache.clear() // always clear on profile switch
	awsclient.ClearPolicyDocumentCache() // clear enricher cache — different account may have same role/policy names
	m.relatedGen++
	m.enrichGen++
	m.connectGen++
	// Only save prev on first switch; rapid A→B→C keeps A as rollback target
	if !m.hasPrevState {
		m.hasPrevState = true
		m.prevProfile = m.profile
		m.prevRegion = m.region
	}
	m.profile = msg.Profile
	m.region = "" // clear so handleClientsReady resolves the new profile's default region
	m.identity = nil
	m.availabilityGen++                                    // cancel in-flight probes
	m.resourceCache = make(map[string]*resourceCacheEntry) // clear all cached resource lists
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.ClearAvailability()
	}
	m.pendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Profile + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(msg.Profile, "", m.connectGen))
}

// handleRegionSelected switches the AWS region, pops the region selector,
// and reconnects.
func (m Model) handleRegionSelected(msg messages.RegionSelectedMsg) (tea.Model, tea.Cmd) {
	m.relatedCache.clear() // always clear on region switch
	m.relatedGen++
	m.enrichGen++
	m.connectGen++
	// Only save prev on first switch; rapid switches keep original as rollback target
	if !m.hasPrevState {
		m.hasPrevState = true
		m.prevProfile = m.profile
		m.prevRegion = m.region
	}
	m.region = msg.Region
	m.identity = nil
	m.availabilityGen++                                    // cancel in-flight probes
	m.resourceCache = make(map[string]*resourceCacheEntry) // clear all cached resource lists
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.ClearAvailability()
	}
	m.pendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Region + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(m.profile, msg.Region, m.connectGen))
}

// handleThemeSelected applies the selected theme, invalidates style caches,
// pops the selector, and persists the choice to config.yaml.
func (m Model) handleThemeSelected(msg messages.ThemeSelectedMsg) (tea.Model, tea.Cmd) {
	path, err := config.ThemePath(msg.Theme)
	if err != nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: "Invalid theme: " + err.Error(), IsError: true}
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: "Cannot read theme: " + err.Error(), IsError: true}
		}
	}
	t, err := styles.ThemeFromYAML(data)
	if err != nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: "Bad theme YAML: " + err.Error(), IsError: true}
		}
	}

	// Persist BEFORE applying — if save fails, abort the change entirely.
	if saveErr := config.SaveTheme(msg.Theme); saveErr != nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: "Cannot save theme config: " + saveErr.Error(), IsError: true}
		}
	}

	// Save succeeded — now apply the theme.
	styles.ApplyTheme(t)
	m.activeTheme = msg.Theme

	// Invalidate header cache.
	m.headerCacheKey = ""

	// Invalidate styledRowCache on all ResourceListModel views in the stack.
	for _, v := range m.stack {
		if rl, ok := v.(*views.ResourceListModel); ok {
			rl.InvalidateStyleCache()
		}
	}

	// Pop the selector view.
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}

	return m, func() tea.Msg {
		return messages.FlashMsg{Text: "Theme: " + msg.Theme}
	}
}

// handleProfilesLoaded pushes the profile selector view onto the stack.
func (m Model) handleProfilesLoaded(msg profilesLoadedMsg) (tea.Model, tea.Cmd) {
	p := views.NewProfile(msg.profiles, m.profile, m.keys)
	p.SetSize(m.innerSize())
	m.pushView(&p)
	return m, nil
}

// handleValueRevealed pushes the reveal view or flashes an error.
func (m Model) handleValueRevealed(msg messages.ValueRevealedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		errText := "reveal failed: " + msg.Err.Error()
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: errText, IsError: true}
		}
	}
	rv := views.NewReveal(msg.ResourceID, msg.Value, m.keys)
	rv.SetSize(m.innerSize())
	m.pushView(&rv)
	return m, nil
}

// handleEnterChildView creates a child resource list view and kicks off a fetch
// using the child type registry. This is the generic handler for all child views.
func (m Model) handleEnterChildView(msg messages.EnterChildViewMsg) (tea.Model, tea.Cmd) {
	childTypeDef := resource.GetChildType(msg.ChildType)
	if childTypeDef == nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: fmt.Sprintf("unknown child type: %s", msg.ChildType), IsError: true}
		}
	}
	rl := views.NewChildResourceList(*childTypeDef, msg.ParentContext, msg.DisplayName, m.viewConfig, m.keys)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	fetchCmd := m.fetchChildResources(msg.ChildType, msg.ParentContext)
	return m, tea.Batch(initCmd, fetchCmd)
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
	if rl, ok := m.activeView().(*views.ResourceListModel); ok {
		rl.ClearLoading()
	}
	gen := m.flash.gen
	return m, tea.Tick(apiErrorFlashDuration, func(_ time.Time) tea.Msg {
		return messages.ClearFlashMsg{Gen: gen}
	})
}
