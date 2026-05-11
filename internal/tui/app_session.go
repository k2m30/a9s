package tui

// app_session.go — AWS-session lifecycle handlers: clients-ready bootstrap,
// profile/region/theme selectors, and the profile-list loader. Split out of
// internal/tui/app_handlers.go in Phase-05 PR-05a-h1 (AS-147). These handlers
// mutate session.Session fields (Profile, Region, Identity, Clients, ConnectGen,
// PendingRefresh, HasPrevState, PrevProfile, PrevRegion, PreSuppliedClients,
// Command, NoCache — migrated to Session in AS-315a / PR-05a-h2) plus
// tui-Model UI-shell fields (activeTheme, headerCacheKey). Handler bodies
// move to *Core methods returning ([]UIIntent, []TaskRequest) in AS-315b /
// PR-05a-h3.

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleClientsReady stores the new AWS clients and optionally triggers a
// pending refresh (after profile/region switch).
func (m Model) handleClientsReady(msg messages.ClientsReadyMsg) (tea.Model, tea.Cmd) {
	// Ignore stale results from a superseded connect (rapid profile/region switch)
	if msg.Gen != m.ConnectGen {
		return m, nil
	}

	if msg.Err != nil {
		// Rollback profile/region to previous stable values on connect failure
		if m.HasPrevState {
			m.Profile = m.PrevProfile
			m.Region = m.PrevRegion
		}
		m.HasPrevState = false
		m.PrevProfile = ""
		m.PrevRegion = ""
		m.PendingRefresh = false
		newGen := m.flash.gen + 1
		m.flash = flashState{text: msg.Err.Error(), isError: true, active: true, gen: newGen}
		m.errorHistory = append(m.errorHistory, errorEntry{time: time.Now(), message: msg.Err.Error()})
		gen := m.flash.gen
		clearFlash := tea.Tick(apiErrorFlashDuration, func(_ time.Time) tea.Msg {
			return messages.ClearFlashMsg{Gen: gen}
		})

		// The switch attempt cleared identity, resource cache, and availability.
		// Restore them using the still-valid old clients.
		//
		// IMPORTANT: Session.Rotate already swapped in fresh PolicyStore /
		// IdentityStore on m, so the retained transport clients (m.Clients)
		// still point at the PRE-rotate stores. Without rewiring here,
		// Pattern-C related checks (Glue tags, EBS Backup) and IAM lazy-add
		// would read sticky state from the now-discarded old stores — the
		// header's identity reload could succeed against the new fresh
		// stores while related-panel rows stayed broken until the next
		// successful reconnect. Rewire on rollback too. (P3 finding.)
		var cmds []tea.Cmd
		cmds = append(cmds, clearFlash)
		if m.Clients != nil {
			m.Clients.SetIAMPolicies(m.IAMPolicies)
			m.Clients.SetIdentityStore(m.IdentityStore)
			m.Clients.SetRuleSets(m.RuleSets)
			m.IdentityFetching = true
			cmds = append(cmds, m.fetchIdentity())
			if m.NoCache {
				cmds = append(cmds, m.demoPrefetchCounts())
			} else {
				cmds = append(cmds, m.loadAvailabilityCache())
			}
		}
		return m, tea.Batch(cmds...)
	}
	if msg.Clients == nil {
		if m.Clients == nil && m.PreSuppliedClients != nil {
			// Fall back to pre-supplied clients (demo path) when msg carries no clients.
			// Wire per-session capability stores into the transport layer.
			m.PreSuppliedClients.SetIAMPolicies(m.IAMPolicies)
			m.PreSuppliedClients.SetIdentityStore(m.IdentityStore)
			m.PreSuppliedClients.SetRuleSets(m.RuleSets)
			m.Clients = m.PreSuppliedClients
		}
	} else if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok {
		// Per-session stores: SES rule sets, IAM policies, and identity all
		// live on m.{RuleSets,IAMPolicies,IdentityStore} after PR-02b/c/d. The
		// legacy ClearAllSESRuleSetCaches global-clear is gone — fresh
		// session = fresh store, wired here via thread-safe setters.
		clients.SetIAMPolicies(m.IAMPolicies)
		clients.SetIdentityStore(m.IdentityStore)
		clients.SetRuleSets(m.RuleSets)
		m.Clients = clients
	} else {
		wrongTypeErr := fmt.Errorf("internal: unexpected ClientsReadyMsg.Clients type %T", msg.Clients)
		return m, func() tea.Msg {
			return messages.APIErrorMsg{Err: wrongTypeErr}
		}
	}
	m.HasPrevState = false
	m.PrevProfile = ""
	m.PrevRegion = ""

	// Emit a one-shot NavigateMsg if the -c flag set an initial command. Only
	// fire while the app is still at the main menu (stack depth 1). If the
	// initial connection was slow and the user already navigated elsewhere,
	// skip the auto-navigation to avoid pushing a view on top of whatever
	// the user is doing. Cleared after first use so that subsequent
	// ClientsReadyMsg (profile/region switch) don't re-navigate.
	var navigateCmd tea.Cmd
	if m.Command != "" {
		if len(m.stack) == 1 {
			target := m.Command
			navigateCmd = func() tea.Msg {
				return messages.NavigateMsg{
					Target:       messages.TargetResourceList,
					ResourceType: target,
				}
			}
		}
		m.Command = "" // always clear, even if skipped, to prevent stale re-navigation
	}

	if m.Profile == "" {
		m.Profile = "default"
	}
	if m.Region == "" {
		if msg.Region != "" {
			m.Region = msg.Region
		} else {
			configPath := awsclient.DefaultConfigPath()
			m.Region = awsclient.GetDefaultRegion(configPath, m.Profile)
		}
	}
	// In demo/no-cache mode, prefetch all counts synchronously in one cmd so the
	// main menu shows counts immediately without the async probe pipeline.
	// Skip identity fetch in demo mode — the profile/region are synthetic.
	if m.NoCache {
		availCmd := m.demoPrefetchCounts()
		if m.PendingRefresh {
			m.PendingRefresh = false
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				m.flash = flashState{text: "Connected. Refreshing...", active: true}
				cmd := m.refreshResourceList(*rl)
				return m, tea.Batch(cmd, availCmd, navigateCmd)
			}
		}
		return m, tea.Batch(availCmd, navigateCmd)
	}

	// Fetch identity on every clients-ready event
	m.IdentityFetching = true
	identityCmd := m.fetchIdentity()

	// Load disk cache which then fires background probes for expired/missing entries.
	availCmd := m.loadAvailabilityCache()

	if m.PendingRefresh {
		m.PendingRefresh = false
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
	// Capture rollback state BEFORE Rotate (which now clears HasPrevState/
	// PrevProfile/PrevRegion). Only save prev on first switch; rapid A→B→C
	// keeps A as rollback target.
	hadPrev := m.HasPrevState
	prevProf := m.PrevProfile
	prevReg := m.PrevRegion
	if !hadPrev {
		hadPrev = true
		prevProf = m.Profile
		prevReg = m.Region
	}
	m.Rotate() // bumps ConnectGen; clears Identity/IdentityFetching/PendingRefresh/HasPrevState/...
	m.HasPrevState = hadPrev
	m.PrevProfile = prevProf
	m.PrevRegion = prevReg
	m.Profile = msg.Profile
	m.Region = "" // clear so handleClientsReady resolves the new profile's default region
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.ClearAvailability()
	}
	m.PendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Profile + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(msg.Profile, "", m.ConnectGen))
}

// handleRegionSelected switches the AWS region, pops the region selector,
// and reconnects.
func (m Model) handleRegionSelected(msg messages.RegionSelectedMsg) (tea.Model, tea.Cmd) {
	// Capture rollback state BEFORE Rotate (which now clears HasPrevState/
	// PrevProfile/PrevRegion). Only save prev on first switch; rapid switches
	// keep original as rollback target.
	hadPrev := m.HasPrevState
	prevProf := m.PrevProfile
	prevReg := m.PrevRegion
	if !hadPrev {
		hadPrev = true
		prevProf = m.Profile
		prevReg = m.Region
	}
	m.Rotate() // bumps ConnectGen; clears Identity/IdentityFetching/PendingRefresh/HasPrevState/...
	m.HasPrevState = hadPrev
	m.PrevProfile = prevProf
	m.PrevRegion = prevReg
	m.Region = msg.Region
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.ClearAvailability()
	}
	m.PendingRefresh = true
	if _, ok := m.activeView().(*views.SelectorModel); ok {
		m.popView()
	}
	flashCmd := func() tea.Msg {
		return messages.FlashMsg{Text: "Switching to " + msg.Region + "..."}
	}
	return m, tea.Batch(flashCmd, m.connectAWS(m.Profile, msg.Region, m.ConnectGen))
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
	p := views.NewProfile(msg.profiles, m.Profile, m.keys)
	p.SetSize(m.innerSize())
	m.pushView(&p)
	return m, nil
}
