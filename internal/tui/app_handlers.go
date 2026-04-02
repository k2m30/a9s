package tui

import (
	"context"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
		help := views.NewHelp(m.keys, ctx)
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
			if !m.noCache {
				cmds = append(cmds, m.loadAvailabilityCache())
			}
		}
		return m, tea.Batch(cmds...)
	}
	if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok {
		m.clients = clients
	}
	m.hasPrevState = false
	m.prevProfile = ""
	m.prevRegion = ""
	if m.profile == "" && !m.demoMode {
		m.profile = "default"
	}
	if m.region == "" && !m.demoMode {
		if msg.Region != "" {
			m.region = msg.Region
		} else {
			configPath := awsclient.DefaultConfigPath()
			m.region = awsclient.GetDefaultRegion(configPath, m.profile)
		}
	}
	// Fetch identity on every clients-ready event
	m.identityFetching = true
	identityCmd := m.fetchIdentity()

	// Start availability probes (unless disabled)
	var availCmd tea.Cmd
	if !m.noCache {
		availCmd = m.loadAvailabilityCache()
	}

	if m.pendingRefresh {
		m.pendingRefresh = false
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			rt := rl.ResourceType()
			m.flash = flashState{text: "Connected. Refreshing...", active: true}
			cmd := m.fetchResources(rt)
			return m, tea.Batch(cmd, identityCmd, availCmd)
		}
	}
	return m, tea.Batch(identityCmd, availCmd)
}

// handleProfileSelected switches the AWS profile, pops the profile selector,
// and reconnects.
func (m Model) handleProfileSelected(msg messages.ProfileSelectedMsg) (tea.Model, tea.Cmd) {
	if m.demoMode {
		return m, nil
	}
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
	if m.demoMode {
		return m, nil
	}
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
	// ResourceID is the canonical field; fall back to SecretName for backward compatibility.
	id := msg.ResourceID
	if id == "" {
		id = msg.SecretName
	}
	rv := views.NewReveal(id, msg.Value, m.keys)
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

// handleNavigate pushes the appropriate view onto the stack.
func (m Model) handleNavigate(msg messages.NavigateMsg) (tea.Model, tea.Cmd) {
	switch msg.Target {
	case messages.TargetResourceList:
		rt := resource.FindResourceType(msg.ResourceType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: fmt.Sprintf("unknown resource type: %s", msg.ResourceType), IsError: true}
			}
		}
		// Check resource cache before creating a new view and fetching.
		if entry, ok := m.resourceCache[msg.ResourceType]; ok {
			rl := views.NewResourceListFromCache(
				*rt, m.viewConfig, m.keys,
				entry.resources, entry.pagination,
				entry.filterText, entry.sortField, entry.sortAsc,
				entry.cursorPos, entry.hScrollOffset,
			)
			rl.SetSize(m.innerSize())
			m.pushView(&rl)
			return m, nil
		}
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResources(msg.ResourceType))

	case messages.TargetDetail:
		if msg.Resource == nil {
			return m, nil
		}
		if msg.ReplaceCurrent {
			m.popView()
		}
		resType := msg.ResourceType
		if resType == "" {
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				resType = rl.ResourceType()
			}
		}
		d := views.NewDetail(*msg.Resource, resType, m.viewConfig, m.keys)
		d.SetSize(m.innerSize())
		m.pushView(&d)
		// Dispatch related checkers if the right column was auto-shown
		if d.NeedsRelatedCheck() {
			return m, func() tea.Msg {
				return messages.RelatedCheckStartedMsg{
					ResourceType:   resType,
					SourceResource: *msg.Resource,
				}
			}
		}
		return m, nil

	case messages.TargetYAML:
		if msg.Resource == nil {
			return m, nil
		}
		y := views.NewYAML(*msg.Resource, m.keys)
		y.SetSize(m.innerSize())
		m.pushView(&y)
		return m, nil

	case messages.TargetHelp:
		ctx := m.helpContext()
		h := views.NewHelp(m.keys, ctx)
		h.SetSize(m.innerSize())
		m.pushView(&h)
		return m, nil

	case messages.TargetProfile:
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}
			}
		}
		cmd := m.fetchProfiles()
		return m, cmd

	case messages.TargetRegion:
		if m.demoMode {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}
			}
		}
		regions := awsclient.AllRegions()
		regionCodes := make([]string, len(regions))
		for i, r := range regions {
			regionCodes[i] = r.Code
		}
		rg := views.NewRegion(regionCodes, m.region, m.keys)
		rg.SetSize(m.innerSize())
		m.pushView(&rg)
		return m, nil

	case messages.TargetReveal:
		if msg.Resource == nil {
			return m, nil
		}
		rt := msg.ResourceType
		if rt == "" {
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				rt = rl.ResourceType()
			}
		}
		cmd := m.fetchRevealValue(rt, msg.Resource.ID)
		return m, cmd
	}
	return m, nil
}

// handleCopy performs context-dependent clipboard copy as a tea.Cmd.
// Each view implements CopyContent() to provide its own content and label.
func (m Model) handleCopy() (tea.Model, tea.Cmd) {
	content, label := m.activeView().CopyContent()
	if content == "" {
		return m, nil
	}
	return m, copyToClipboard(content, label)
}

// handleRefresh re-fetches resources when on a resource list view,
// or restarts availability checks when on the main menu.
// For detail views, re-triggers related resource checks.
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	// Main menu: restart availability checks
	if _, ok := m.activeView().(*views.MainMenuModel); ok {
		if m.noCache {
			return m, nil
		}
		// Increment gen to cancel any in-flight probes
		m.availabilityGen++
		m.flash = flashState{text: "Refreshing availability...", isError: false, active: true}
		cmd := m.loadAvailabilityCache()
		return m, cmd
	}

	// Detail view: re-trigger related resource checks
	if d, ok := m.activeView().(*views.DetailModel); ok {
		rt := d.ResourceType()
		srcRes := d.SourceResource()
		m.flash = flashState{text: "Refreshing...", isError: false, active: true}
		return m, func() tea.Msg {
			return messages.RelatedCheckStartedMsg{
				ResourceType:   rt,
				SourceResource: srcRes,
			}
		}
	}

	rl, ok := m.activeView().(*views.ResourceListModel)
	if !ok {
		return m, nil
	}
	rt := rl.ResourceType()
	delete(m.resourceCache, rt) // clear cache for refreshed type only
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}

	// If the view has a parent context, it's a child view — use child fetch path.
	if pc := rl.ParentContext(); pc != nil {
		cmd := m.fetchChildResources(rt, pc)
		return m, cmd
	}

	// Top-level resource list — fetch via registry.
	cmd := m.fetchResources(rt)
	return m, cmd
}

// handleReveal fetches a revealed value using the resource type's registered reveal fetcher.
func (m Model) handleReveal() (tea.Model, tea.Cmd) {
	rl, ok := m.activeView().(*views.ResourceListModel)
	if !ok {
		return m, nil
	}
	rt := rl.ResourceType()
	if !resource.HasRevealFetcher(rt) {
		return m, nil
	}
	r := rl.SelectedResource()
	if r == nil {
		return m, nil
	}
	cmd := m.fetchRevealValue(rt, r.ID)
	return m, cmd
}

// handleIdentityLoaded caches the identity and updates the identity view if active.
func (m Model) handleIdentityLoaded(msg messages.IdentityLoadedMsg) (tea.Model, tea.Cmd) {
	m.identityFetching = false
	if id, ok := msg.Identity.(*awsclient.CallerIdentity); ok {
		m.identity = id
	}
	// Update identity view if it's on top of the stack
	if idView, ok := m.activeView().(*views.IdentityModel); ok {
		data := m.identityToViewData()
		idView.SetIdentity(data)
	}
	return m, nil
}

// handleIdentityError clears the fetching flag and updates the identity view if active.
func (m Model) handleIdentityError(msg messages.IdentityErrorMsg) (tea.Model, tea.Cmd) {
	m.identityFetching = false
	if idView, ok := m.activeView().(*views.IdentityModel); ok {
		idView.SetError(msg.Err)
	}
	return m, nil
}

// handleAvailabilityCacheLoaded applies cached entries to the main menu
// and starts background availability checks.
func (m Model) handleAvailabilityCacheLoaded(msg messages.AvailabilityCacheLoadedMsg) (tea.Model, tea.Cmd) {
	// Apply cached entries to the main menu
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range msg.Entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range msg.Truncated {
			menu.SetTruncated(shortName, trunc)
		}
	}

	// Build queue of all resource types to check in background
	allNames := resource.AllShortNames()
	m.availQueue = allNames
	m.availChecked = 0
	m.availTotal = len(allNames)

	// Update menu progress
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(0, m.availTotal)
	}

	// Fire first batch of concurrent probes (up to 3)
	var cmds []tea.Cmd
	for i := 0; i < 3 && len(m.availQueue) > 0; i++ {
		shortName := m.availQueue[0]
		m.availQueue = m.availQueue[1:]
		cmds = append(cmds, m.probeResourceAvailability(shortName, m.availabilityGen))
	}

	return m, tea.Batch(cmds...)
}

// handleAvailabilityChecked processes a single resource type's probe result.
func (m Model) handleAvailabilityChecked(msg messages.AvailabilityCheckedMsg) (tea.Model, tea.Cmd) {
	// Ignore stale results from a previous generation (profile/region switch)
	if msg.Gen != m.availabilityGen {
		return m, nil
	}

	m.availChecked++

	// Update menu availability (only if no error — errors mean "unknown")
	if msg.Err == nil {
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.SetAvailability(msg.ResourceType, msg.Count)
			menu.SetTruncated(msg.ResourceType, msg.Truncated)
		}
	}

	// Update progress on menu
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(m.availChecked, m.availTotal)
	}

	// If queue has more items, fire next probe
	if len(m.availQueue) > 0 {
		next := m.availQueue[0]
		m.availQueue = m.availQueue[1:]
		cmd := m.probeResourceAvailability(next, m.availabilityGen)
		return m, cmd
	}

	// Queue is drained but other probes may still be in flight.
	// Only finalize when ALL probes have returned.
	if m.availChecked < m.availTotal {
		return m, nil
	}

	// All checks done — clear progress indicator, flash, and save cache
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetCheckProgress(0, 0) // 0,0 signals "done"
	}
	m.flash.active = false

	// Save cache to disk
	cmd := m.saveAvailabilityCache()
	return m, cmd
}

func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		if err != nil {
			return messages.FlashMsg{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.FlashMsg{Text: successLabel, IsError: false}
	}
}

// handleRelatedCheckStarted dispatches one async tea.Cmd per registered RelatedDef
// for the given resource type. In demo mode it calls the registered demo checker;
// in live mode it calls def.Checker with a 10-second timeout.
func (m Model) handleRelatedCheckStarted(msg messages.RelatedCheckStartedMsg) (tea.Model, tea.Cmd) {
	defs := resource.GetRelated(msg.ResourceType)
	if len(defs) == 0 {
		return m, nil
	}

	cache := m.buildResourceCacheSnapshot()
	cmds := make([]tea.Cmd, 0, len(defs))

	for _, def := range defs {
		def := def // capture for closure
		cmds = append(cmds, func() tea.Msg {
			if m.demoMode {
				demoFn := resource.GetRelatedDemo(msg.ResourceType)
				if demoFn != nil {
					for _, r := range demoFn(msg.SourceResource) {
						if r.TargetType == def.TargetType {
							return messages.RelatedCheckResultMsg{
								ResourceType: msg.ResourceType,
								Result:       r,
							}
						}
					}
				}
				return messages.RelatedCheckResultMsg{
					ResourceType: msg.ResourceType,
					Result:       resource.RelatedCheckResult{TargetType: def.TargetType, Count: -1},
				}
			}

			if def.Checker == nil {
				return messages.RelatedCheckResultMsg{
					ResourceType: msg.ResourceType,
					Result:       resource.RelatedCheckResult{TargetType: def.TargetType, Count: -1},
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result := def.Checker(ctx, m.clients, msg.SourceResource, cache)
			result.TargetType = def.TargetType
			return messages.RelatedCheckResultMsg{
				ResourceType: msg.ResourceType,
				Result:       result,
			}
		})
	}

	return m, tea.Batch(cmds...)
}

// handleRelatedNavigate pushes a resource list view for the related target type.
// Pre-filters the list when a specific target ID or related IDs are available.
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	rt := resource.FindResourceType(msg.TargetType)
	if rt == nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
				IsError: true,
			}
		}
	}

	// Bug 1 fix: TargetID set → find resource in cache and push detail directly.
	if msg.TargetID != "" {
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			for _, r := range entry.resources {
				if r.ID == msg.TargetID {
					detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
					detail.SetSize(m.innerSize())
					m.pushView(&detail)
					return m, nil
				}
			}
		}
		// Resource not in cache — fetch target list and preserve exact-ID filtering.
		m.flash = flashState{
			text:    fmt.Sprintf("Resource %s not in cache; loading %s list", msg.TargetID, msg.TargetType),
			isError: false,
			active:  true,
		}
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetDisplayName(relatedListBaseName(*rt))
		rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
		rl.SetPendingFilter(msg.TargetID)
		rl.SetRelatedIDFilter([]string{msg.TargetID})
		rl.SetAutoOpenSingleDetail(true)
		rl.SetEscPops(true)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Bug 2 fix: single RelatedID → push detail directly (same as TargetID).
	if len(msg.RelatedIDs) == 1 {
		targetID := msg.RelatedIDs[0]
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			for _, r := range entry.resources {
				if r.ID == targetID {
					detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
					detail.SetSize(m.innerSize())
					m.pushView(&detail)
					return m, nil
				}
			}
		}
		// Not in cache — fall through to list with pending filter
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetDisplayName(relatedListBaseName(*rt))
		rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
		rl.SetPendingFilter(targetID)
		rl.SetRelatedIDFilter([]string{targetID})
		rl.SetAutoOpenSingleDetail(true)
		rl.SetEscPops(true)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Bug 3 fix: multiple RelatedIDs → filter cache to only matching resources.
	if len(msg.RelatedIDs) > 1 {
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			idSet := make(map[string]bool, len(msg.RelatedIDs))
			for _, id := range msg.RelatedIDs {
				idSet[id] = true
			}
			var filtered []resource.Resource
			for _, r := range entry.resources {
				if idSet[r.ID] {
					filtered = append(filtered, r)
				}
			}
			rl := views.NewResourceListFromCache(
				*rt, m.viewConfig, m.keys,
				filtered, entry.pagination,
				"", // no text filter needed, already filtered by ID
				entry.sortField, entry.sortAsc,
				0, 0,
			)
			rl.SetDisplayName(relatedListBaseName(*rt))
			rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
			rl.SetEscPops(true)
			rl.SetSize(m.innerSize())
			m.pushView(&rl)
			return m, nil
		}
		// Cache miss: fetch and preserve exact-ID filtering.
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		rl.SetDisplayName(relatedListBaseName(*rt))
		rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
		rl.SetRelatedIDFilter(msg.RelatedIDs)
		rl.SetEscPops(true)
		rl.SetSize(m.innerSize())
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
	}

	// Fallback: no IDs specified or cache miss for multiple IDs — push unfiltered list.
	rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
	rl.SetDisplayName(relatedListBaseName(*rt))
	rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
	rl.SetEscPops(true)
	rl.SetSize(m.innerSize())
	rl, initCmd := rl.Init()
	m.pushView(&rl)
	return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
}

func relatedTitleSuffix(src resource.Resource) string {
	if src.ID == "" {
		return ""
	}
	if src.Name != "" {
		return fmt.Sprintf(" -- %s (%s)", src.ID, src.Name)
	}
	return " -- " + src.ID
}

func relatedListBaseName(rt resource.ResourceTypeDef) string {
	// Match design/UI convention for alarms list title.
	if rt.ShortName == "alarm" {
		return "alarms"
	}
	return rt.ShortName
}

// buildResourceCacheSnapshot returns a read-only snapshot of currently-loaded
// resource lists, keyed by resource short name. Used by related checkers.
func (m *Model) buildResourceCacheSnapshot() resource.ResourceCache {
	snap := make(resource.ResourceCache, len(m.resourceCache))
	for shortName, entry := range m.resourceCache {
		snap[shortName] = entry.resources
	}
	return snap
}
