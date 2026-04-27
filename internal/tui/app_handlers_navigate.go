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
	case messages.TargetMainMenu:
		for m.popView() {
		}
		return m, nil

	case messages.TargetResourceList:
		rt := resource.FindResourceType(msg.ResourceType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{Text: fmt.Sprintf("unknown resource type: %s", msg.ResourceType), IsError: true}
			}
		}
		// When navigated via an alias (e.g. "rds" → ShortName "dbi"), preserve the
		// alias as the display name so the frame title reflects the user's intent.
		requestedAlias := msg.ResourceType
		if requestedAlias == rt.ShortName {
			requestedAlias = ""
		}
		// Enrichment maps (findings, truncated, unmatched) and the menu's
		// issue maps are keyed by the CANONICAL ShortName. NavigateMsg may
		// carry an alias (e.g. "rds" → ShortName "dbi"); look up under the
		// canonical name so alias navigation shows the same enrichment state
		// as canonical navigation.
		canon := rt.ShortName
		// Check resource cache before creating a new view and fetching.
		if entry, ok := m.resourceCache[msg.ResourceType]; ok {
			rl := views.NewResourceListFromCache(
				*rt, m.viewConfig, m.keys,
				entry.resources, entry.pagination,
				entry.filterText, entry.sortColIdx, entry.sortAsc,
				entry.cursorPos, entry.hScrollOffset,
				entry.attentionOnly,
			)
			if requestedAlias != "" {
				rl.SetDisplayName(requestedAlias)
			}
			rl.SetShowIssueBadge(true) // top-level list from main menu
			rl.SetSize(m.innerSize())
			// Wire enrichment state so markers reflect current findings.
			issueCount, issueTrunc := 0, false
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				issueCount = menu.GetIssueCounts()[canon]
				issueTrunc = menu.GetIssueTruncated()[canon]
			}
			rl.SetEnrichmentState(issueCount, issueTrunc, m.enrichmentFindings[canon])
			rl.SetTruncatedIDs(m.enrichmentTruncatedIDs[canon])
			m.pushView(&rl)
			return m, nil
		}
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		if requestedAlias != "" {
			rl.SetDisplayName(requestedAlias)
		}
		rl.SetShowIssueBadge(true) // top-level list from main menu
		rl.SetSize(m.innerSize())
		// Wire enrichment state so markers reflect current findings.
		issueCount, issueTrunc := 0, false
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			issueCount = menu.GetIssueCounts()[canon]
			issueTrunc = menu.GetIssueTruncated()[canon]
		}
		rl.SetEnrichmentState(issueCount, issueTrunc, m.enrichmentFindings[canon])
		rl.SetTruncatedIDs(m.enrichmentTruncatedIDs[canon])
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
		// Normalize alias to canonical ShortName so the detail view's ResourceType()
		// matches the ShortName used by handleEnrichmentChecked and injectEnrichmentSection.
		if td := resource.FindResourceType(resType); td != nil {
			resType = td.ShortName
		}
		if resType == "" {
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				resType = rl.ResourceType()
			}
		}
		d := views.NewDetail(*msg.Resource, resType, m.viewConfig, m.keys)
		d.SetNavProvider(resource.GetNavigableFields)
		d.SetSize(m.innerSize())
		// Wire enrichment finding if one exists for this resource.
		if findings, ok := m.enrichmentFindings[resType]; ok {
			if f, exists := findings[msg.Resource.ID]; exists {
				d.SetEnrichmentFinding(&f)
			}
		}
		m.pushView(&d)
		var cmds []tea.Cmd

		// Dispatch enrichment if registered for this resource type.
		// Only bump enrichGen when the resource identity changes, so
		// switching to YAML/JSON for the same resource doesn't invalidate
		// an in-flight enrichment from the detail view open.
		if resource.HasDetailEnricher(resType) {
			key := resType + ":" + msg.Resource.ID
			if key != m.enrichResKey {
				m.enrichGen++
				m.enrichResKey = key
			}
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
				d.ApplyRelatedResults(relatedCacheReplay(resType, cached))
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
		if msg.ReplaceCurrent {
			m.popView()
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
		if resource.HasDetailEnricher(resType) {
			key := resType + ":" + msg.Resource.ID
			if key != m.enrichResKey {
				m.enrichGen++
				m.enrichResKey = key
			}
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
		if msg.ReplaceCurrent {
			m.popView()
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
		if resource.HasDetailEnricher(resType) {
			key := resType + ":" + msg.Resource.ID
			if key != m.enrichResKey {
				m.enrichGen++
				m.enrichResKey = key
			}
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
		// Increment gen to cancel any in-flight probes and enrichment
		m.availabilityGen++
		m.enrichmentGen++
		m.enrichmentFindings = make(map[string]map[string]resource.EnrichmentFinding)
		m.enrichmentRan = make(map[string]bool)
		m.enrichmentTypeGen = make(map[string]int)
		m.enrichmentTruncatedIDs = make(map[string]map[string]bool)
		m.probeResources = make(map[string][]resource.Resource)
		m.probeTruncated = make(map[string]bool)
		// Reset the menu's view-side state (availability, issue counts) in
		// lockstep with the model-side maps above.
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.ClearAvailability()
		}
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
		m.relatedGen++      // cancel in-flight results from previous batch
		m.enrichGen++       // cancel in-flight enrichment from previous batch
		m.enrichResKey = "" // force gen bump on next enrichment dispatch
		// Invalidate the SES v1 receipt rule set cache so Ctrl+R on a detail view
		// picks up receipt-rule changes without requiring a profile/region switch.
		if rt == "ses" {
			awsclient.InvalidateSESRuleSetCache(m.clients)
		}
		m.flash = flashState{text: "Refreshing...", isError: false, active: true}

		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return messages.RelatedCheckStartedMsg{
				ResourceType:   rt,
				SourceResource: srcRes,
			}
		})
		if resource.HasDetailEnricher(rt) {
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
	if rt == "ses" {
		awsclient.InvalidateSESRuleSetCache(m.clients)
	}
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}

	// Top-level list with a registered enricher: bump per-type gen, clear
	// findings, and dispatch a wrapped fetch that stamps TypeGen onto the
	// outgoing ResourcesLoadedMsg so the tail branch in app.go can seed
	// probeResources and dispatch probeEnrichment on success.
	if rl.ParentContext() == nil && !rl.EscPops() {
		if _, hasEnricher := awsclient.IssueEnricherRegistry[rt]; hasEnricher {
			m.enrichmentTypeGen[rt]++
			tok := m.enrichmentTypeGen[rt]
			delete(m.enrichmentFindings, rt)
			delete(m.enrichmentRan, rt)
			// Clear per-resource truncation markers too: if the refresh errors
			// out, stale "?" prefixes must not persist across the rerun.
			delete(m.enrichmentTruncatedIDs, rt)
			// Propagate the cleared state to the active ResourceListModel so
			// row markers disappear immediately at Ctrl+R — otherwise stale markers
			// would remain visible until the rerun completes (and indefinitely if
			// the refresh errors out).
			rl.SetEnrichmentState(0, false, nil)
			rl.SetTruncatedIDs(nil)
			cmd := m.refreshResourceListWithEnrichmentRerun(*rl, tok)
			return m, cmd
		}
		// Top-level list without an enricher: clear any stale findings immediately
		// so row markers don't persist across a Ctrl+R refresh.
		if _, hasFindings := m.enrichmentFindings[rt]; hasFindings {
			delete(m.enrichmentFindings, rt)
			rl.SetEnrichmentState(0, false, nil)
		}
	}
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


func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		if err != nil {
			return messages.FlashMsg{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.FlashMsg{Text: successLabel, IsError: false}
	}
}
