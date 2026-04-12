package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

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
				entry.attentionOnly,
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
		var cmds []tea.Cmd

		// Dispatch enrichment if registered for this resource type.
		// Increment enrichGen so in-flight results from a previous detail view
		// (e.g., same inline policy name under a different role) are discarded.
		if resource.HasEnricher(resType) {
			m.enrichGen++
			cmds = append(cmds, func() tea.Msg {
				return messages.EnrichDetailMsg{
					ResourceType: resType,
					Resource:     *msg.Resource,
				}
			})
		}

		// Dispatch related checks (existing logic)
		if d.NeedsRelatedCheck() {
			ck := relatedCacheKey(resType, msg.Resource.ID)
			if cached, ok := m.relatedCache.get(ck); ok && len(cached) > 0 {
				d.ApplyRelatedResults(cached)
			} else {
				cmds = append(cmds, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   resType,
						SourceResource: *msg.Resource,
					}
				})
			}
		}

		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)

	case messages.TargetYAML:
		if msg.Resource == nil {
			return m, nil
		}
		resType := msg.ResourceType
		if resType == "" {
			switch av := m.activeView().(type) {
			case *views.ResourceListModel:
				resType = av.ResourceType()
			case *views.DetailModel:
				resType = av.ResourceType()
			}
		}
		y := views.NewYAML(*msg.Resource, resType, m.keys)
		y.SetSize(m.innerSize())
		m.pushView(&y)
		// Dispatch enrichment so YAML view updates when result arrives.
		if resource.HasEnricher(resType) {
			m.enrichGen++
			return m, func() tea.Msg {
				return messages.EnrichDetailMsg{
					ResourceType: resType,
					Resource:     *msg.Resource,
				}
			}
		}
		return m, nil

	case messages.TargetJSON:
		if msg.Resource == nil {
			return m, nil
		}
		resType := msg.ResourceType
		if resType == "" {
			switch av := m.activeView().(type) {
			case *views.ResourceListModel:
				resType = av.ResourceType()
			case *views.DetailModel:
				resType = av.ResourceType()
			}
		}
		j := views.NewJSON(*msg.Resource, resType, m.keys)
		j.SetSize(m.innerSize())
		m.pushView(&j)
		// Dispatch enrichment so JSON view updates when result arrives.
		if resource.HasEnricher(resType) {
			m.enrichGen++
			return m, func() tea.Msg {
				return messages.EnrichDetailMsg{
					ResourceType: resType,
					Resource:     *msg.Resource,
				}
			}
		}
		return m, nil

	case messages.TargetHelp:
		ctx := m.helpContext()
		activeShortName := ""
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			activeShortName = rl.ShortName()
		}
		h := views.NewHelpWithResource(m.keys, ctx, activeShortName)
		h.SetSize(m.innerSize())
		m.pushView(&h)
		return m, nil

	case messages.TargetProfile:
		if m.preSuppliedClients != nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    "context switching is disabled in demo mode",
					IsError: true,
				}
			}
		}
		cmd := m.fetchProfiles()
		return m, cmd

	case messages.TargetRegion:
		if m.preSuppliedClients != nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    "region switching is disabled in demo mode",
					IsError: true,
				}
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

	case messages.TargetTheme:
		cfgDir := config.ConfigDir()
		if cfgDir == "" {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Config directory not available", IsError: true}
			}
		}
		themesDir := filepath.Join(cfgDir, "themes")
		entries, err := os.ReadDir(themesDir)
		if err != nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "Cannot read themes directory: " + err.Error(), IsError: true}
			}
		}
		var themeFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
				themeFiles = append(themeFiles, e.Name())
			}
		}
		if len(themeFiles) == 0 {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: "No theme files found in " + themesDir, IsError: true}
			}
		}
		th := views.NewTheme(themeFiles, m.activeTheme, m.keys)
		th.SetSize(m.innerSize())
		m.pushView(&th)
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
	// Main menu: restart availability checks (no-op in no-cache mode)
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

	// Detail view: re-trigger related resource checks and enrichment
	if d, ok := m.activeView().(*views.DetailModel); ok {
		d.ResetRightColumn()
		rt := d.ResourceType()
		srcRes := d.SourceResource()
		m.relatedCache.delete(relatedCacheKey(rt, srcRes.ID))
		m.relatedGen++ // cancel in-flight results from previous batch
		m.enrichGen++  // cancel in-flight enrichment from previous batch
		m.flash = flashState{text: "Refreshing...", isError: false, active: true}

		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return messages.RelatedCheckStartedMsg{
				ResourceType:   rt,
				SourceResource: srcRes,
			}
		})
		if resource.HasEnricher(rt) {
			cmds = append(cmds, func() tea.Msg {
				return messages.EnrichDetailMsg{
					ResourceType: rt,
					Resource:     srcRes,
				}
			})
		}
		return m, tea.Batch(cmds...)
	}

	rl, ok := m.activeView().(*views.ResourceListModel)
	if !ok {
		return m, nil
	}
	rt := rl.ResourceType()
	delete(m.resourceCache, rt) // clear cache for refreshed type only
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}
	return m, m.refreshResourceList(*rl)
}

func (m Model) refreshResourceList(rl views.ResourceListModel) tea.Cmd {
	rt := rl.ResourceType()

	// Filtered lists must refresh through the same filtered fetcher so their
	// pagination token remains valid for subsequent load-more requests.
	if ff := rl.FetchFilter(); len(ff) > 0 {
		return m.fetchResourcesFiltered(rt, ff)
	}

	// Child lists refresh through the child fetcher using their parent context.
	if pc := rl.ParentContext(); pc != nil {
		return m.fetchChildResources(rt, pc)
	}

	return m.fetchResources(rt)
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

	// Fire first batch of concurrent probes (up to 4)
	var cmds []tea.Cmd
	for i := 0; i < 4 && len(m.availQueue) > 0; i++ {
		shortName := m.availQueue[0]
		m.availQueue = m.availQueue[1:]
		cmds = append(cmds, m.probeResourceAvailability(shortName, m.availabilityGen))
	}

	return m, tea.Batch(cmds...)
}

// handleAvailabilityPrefetched applies synchronously-prefetched counts to the
// main menu. Used in no-cache + pre-supplied-clients mode so counts appear
// immediately without background probes.
func (m Model) handleAvailabilityPrefetched(msg messages.AvailabilityPrefetchedMsg) (tea.Model, tea.Cmd) {
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range msg.Entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range msg.Truncated {
			menu.SetTruncated(shortName, trunc)
		}
		menu.SetCheckProgress(0, 0) // signal "done"
	}
	return m, nil
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
