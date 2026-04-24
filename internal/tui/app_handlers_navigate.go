package tui

import (
	"fmt"
	"maps"
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

// handleAvailabilityCacheLoaded applies cached entries to the main menu
// and starts background availability checks.
func (m Model) handleAvailabilityCacheLoaded(msg messages.AvailabilityCacheLoadedMsg) (tea.Model, tea.Cmd) {
	// Canonicalize any alias keys (e.g. "rds" → "dbi") so the menu's filter and
	// issue maps share the same key space as the ResourceTypeDef lookup.
	canonKey := func(k string) string {
		if td := resource.FindResourceType(k); td != nil {
			return td.ShortName
		}
		return k
	}
	canonIntMap := func(src map[string]int) map[string]int {
		dst := make(map[string]int, len(src))
		for k, v := range src {
			dst[canonKey(k)] = v
		}
		return dst
	}
	canonBoolMap := func(src map[string]bool) map[string]bool {
		dst := make(map[string]bool, len(src))
		for k, v := range src {
			dst[canonKey(k)] = v
		}
		return dst
	}
	entries := canonIntMap(msg.Entries)
	truncated := canonBoolMap(msg.Truncated)
	issueCounts := canonIntMap(msg.IssueCounts)
	issueTruncated := canonBoolMap(msg.IssueTruncated)
	issueKnown := canonBoolMap(msg.IssueKnown)

	// Apply cached entries to the main menu
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range truncated {
			menu.SetTruncated(shortName, trunc)
		}
		// Apply cached issue counts (T033).
		if len(issueKnown) > 0 {
			menu.SetIssuesFromCache(issueCounts, issueTruncated, issueKnown)
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
	// Gen guard: drop stale results produced before a profile/region switch.
	// Gen=0 is the zero value (pre-guard dispatch) — accepted unconditionally
	// to preserve backwards-compatible test injection.
	if msg.Gen != 0 && msg.Gen != m.availabilityGen {
		return m, nil
	}
	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		for shortName, count := range msg.Entries {
			menu.SetAvailability(shortName, count)
		}
		for shortName, trunc := range msg.Truncated {
			menu.SetTruncated(shortName, trunc)
		}
		// T034: wire issue counts from prefetch.
		for shortName, count := range msg.IssueCounts {
			trunc := msg.IssueTruncated[shortName]
			menu.SetIssues(shortName, count, trunc)
		}
		menu.SetCheckProgress(0, 0) // signal "done"
	}
	// T034: retain prefetch resources for Wave 2 enrichment (--no-cache live AWS).
	if msg.Resources != nil {
		if m.probeResources == nil {
			m.probeResources = make(map[string][]resource.Resource, len(msg.Resources))
		}
		maps.Copy(m.probeResources, msg.Resources)
		// Seed resourceCache from the prefetch too. Without this, Wave 2
		// FieldUpdates that the enricher merges into the cache entry
		// (handleEnrichmentChecked tail) have no entry to land on when the
		// user hasn't opened the list yet — the enrichment runs before any
		// OpenList, so FieldUpdates would otherwise die in probeResources
		// and vanish on the first fetch. Seeding here keeps the cache
		// entry alive so FieldUpdates survive across navigation.
		if m.resourceCache == nil {
			m.resourceCache = make(map[string]*resourceCacheEntry, len(msg.Resources))
		}
		for rt, resources := range msg.Resources {
			if _, exists := m.resourceCache[rt]; exists {
				continue
			}
			m.resourceCache[rt] = &resourceCacheEntry{
				resources: resources,
			}
		}
	}
	// Start Wave 2 enrichment. Demo mode's typed fakes implement the enricher
	// APIs (DescribePendingMaintenanceActions, etc.), so enrichment runs the
	// same production code path against fixture data — this is what gives the
	// demo its `~` glyphs, `(+N)` suffix, and "maintenance scheduled" status.
	enrichCmd := m.startEnrichment()

	// Surface aggregated per-type prefetch failures to the error log so
	// operators see permission / throttle issues instead of silently missing
	// resource types in the availability counts.
	var flashCmd tea.Cmd
	if msg.PrefetchErr != nil {
		err := msg.PrefetchErr
		flashCmd = func() tea.Msg {
			return messages.FlashMsg{
				Text:    "availability: " + err.Error(),
				IsError: true,
			}
		}
	}

	if enrichCmd != nil && flashCmd != nil {
		return m, tea.Batch(flashCmd, enrichCmd)
	}
	if enrichCmd != nil {
		return m, enrichCmd
	}
	return m, flashCmd
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
			// T032: wire issue counts from probe.
			menu.SetIssues(msg.ResourceType, msg.Issues, msg.Truncated)
		}
		// T032: retain probe resources for Wave 2 enrichment.
		if m.probeResources == nil {
			m.probeResources = make(map[string][]resource.Resource)
		}
		m.probeResources[msg.ResourceType] = msg.Resources
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
	saveCmd := m.saveAvailabilityCache()

	// Start Wave 2 enrichment. Demo mode's typed fakes implement the enricher
	// APIs, so enrichment runs identically against fixture data.
	enrichCmd := m.startEnrichment()
	if enrichCmd != nil {
		return m, tea.Batch(saveCmd, enrichCmd)
	}
	return m, saveCmd
}

// startEnrichment builds the enrichment queue and fires the first batch of probes.
// For each type dispatched, it bumps enrichmentTypeGen, clears any existing
// findings and ran flag (clear-on-rerun-start), then captures the new gen into
// the probeEnrichment call.
func (m *Model) startEnrichment() tea.Cmd {
	m.enrichQueue = m.buildEnrichQueue()
	if len(m.enrichQueue) == 0 {
		return nil
	}
	m.enrichmentGen++
	m.enrichChecked = 0
	m.enrichTotal = len(m.enrichQueue)

	if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
		menu.SetEnrichProgress(0, m.enrichTotal)
	}

	// Dispatch all enrichers at once so priority ordering is observable in a
	// single cmd tree. Each probe is independent; results arrive as individual
	// EnrichmentCheckedMsg values. handleEnrichmentChecked drains any residual
	// queue entries added by future requeue operations.
	var cmds []tea.Cmd
	for len(m.enrichQueue) > 0 {
		name := m.enrichQueue[0]
		m.enrichQueue = m.enrichQueue[1:]
		// Clear-on-rerun-start: bump type gen, wipe stale findings and ran flag.
		m.enrichmentTypeGen[name]++
		delete(m.enrichmentFindings, name)
		delete(m.enrichmentRan, name)
		cmds = append(cmds, m.probeEnrichment(name, m.enrichmentGen))
	}
	return tea.Batch(cmds...)
}

// handleEnrichmentChecked processes a single Wave 2 enrichment result.
func (m Model) handleEnrichmentChecked(msg messages.EnrichmentCheckedMsg) (tea.Model, tea.Cmd) {
	// Normalize to the canonical ShortName so alias-keyed messages (e.g. "rds" for
	// the "dbi" type) match the ResourceType() returned by views in the stack.
	if td := resource.FindResourceType(msg.ResourceType); td != nil {
		msg.ResourceType = td.ShortName
	}
	// Session-wide generation guard — drop stale messages from prior profile/region.
	// Gen=0 is the documented test-injection bypass: accepted regardless of enrichmentGen.
	if msg.Gen != 0 && msg.Gen != m.enrichmentGen {
		return m, nil
	}
	// Per-type generation guard — drop stale probes superseded by a newer rerun.
	// TypeGen=0 is the symmetric test-injection bypass (production always dispatches
	// with TypeGen≥1 because startEnrichment bumps enrichmentTypeGen[name] before
	// capturing typeGen in probeEnrichment).
	if msg.TypeGen != 0 && msg.TypeGen != m.enrichmentTypeGen[msg.ResourceType] {
		return m, nil
	}

	m.enrichChecked++

	// Surface enrichment failures as a flash error so operators see them in the
	// error log (! key). A failed enrichment does not stall the pipeline — the
	// queue continues to drain below.
	var flashCmd tea.Cmd
	if msg.Err != nil {
		err := msg.Err
		rt := msg.ResourceType
		flashCmd = func() tea.Msg {
			return messages.FlashMsg{
				Text:    fmt.Sprintf("enrich %s: %v", rt, err),
				IsError: true,
			}
		}
	}

	// Update findings and menu issue count on success.
	if msg.Err == nil {
		// Persist findings and mark enrichment as ran for this type.
		m.enrichmentFindings[msg.ResourceType] = msg.Findings
		m.enrichmentRan[msg.ResourceType] = true
		// Always replace, including with empty/nil maps — a successful rerun
		// MUST clear prior "?" row markers. Using `if len > 0` would leave
		// stale markers from a previous attempt.
		m.enrichmentTruncatedIDs[msg.ResourceType] = msg.TruncatedIDs

		// Merge FieldUpdates into probeResources so the cached rows carry
		// Wave-2-derived fields. These are then visible to list columns that
		// reference the updated keys.
		if len(msg.FieldUpdates) > 0 {
			slice := m.probeResources[msg.ResourceType]
			for i := range slice {
				if updates, ok := msg.FieldUpdates[slice[i].ID]; ok {
					if slice[i].Fields == nil {
						slice[i].Fields = make(map[string]string, len(updates))
					}
					maps.Copy(slice[i].Fields, updates)
				}
			}
			m.probeResources[msg.ResourceType] = slice
			// Persist FieldUpdates into resourceCache so that navigating away
			// and back restores the Wave-2-derived fields (e.g. last_build,
			// dlq, rotation_enabled) instead of rendering them blank.
			if entry, ok := m.resourceCache[msg.ResourceType]; ok {
				for i := range entry.resources {
					if updates, ok := msg.FieldUpdates[entry.resources[i].ID]; ok {
						if entry.resources[i].Fields == nil {
							entry.resources[i].Fields = make(map[string]string, len(updates))
						}
						maps.Copy(entry.resources[i].Fields, updates)
					}
				}
			}
			// Also propagate into any active ResourceListModel for this type.
			for _, v := range m.stack {
				if rl, ok := v.(*views.ResourceListModel); ok && rl.ResourceType() == msg.ResourceType {
					rl.ApplyFieldUpdates(msg.FieldUpdates)
				}
			}
		}

		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			// Wave 2 is authoritative: compute distinct-instance count across both waves.
			td := resource.FindResourceType(msg.ResourceType)
			var unified int
			if td != nil {
				unified = unifiedIssueCount(m.probeResources[msg.ResourceType], *td, msg.Findings)
			} else {
				unified = msg.Issues
			}
			// Wave 2 truncation with no findings and no wave-1 issues means a
			// sub-call errored but no actual issue was seen. Truncation signals
			// count completeness, not hidden issues — if Wave 2 had seen one, it
			// would have produced a Finding. Don't promote into the attention filter.
			issueTruncated := msg.Truncated
			if unified == 0 && len(msg.Findings) == 0 {
				issueTruncated = false
			}
			// If resource count is already a lower bound (Wave 1 truncated), the
			// issue count is also a lower bound — preserve that signal even when
			// Wave 2 itself did not truncate.
			if menu.GetTruncated()[msg.ResourceType] {
				issueTruncated = true
			}
			menu.SetIssues(msg.ResourceType, unified, issueTruncated)
			menu.SetEnrichProgress(m.enrichChecked, m.enrichTotal)

			// Live-update ALL ResourceListModel views in the stack showing this type.
			for _, v := range m.stack {
				if rl, ok := v.(*views.ResourceListModel); ok && rl.ResourceType() == msg.ResourceType {
					rl.SetEnrichmentState(unified, issueTruncated, msg.Findings)
					rl.SetTruncatedIDs(msg.TruncatedIDs)
				}
			}
		}

		// Live-update ALL DetailModel views in the stack for this resource type.
		// Iterating the full stack ensures stacked (non-active) detail views are also
		// updated when the user has navigated to a second detail view or another screen
		// and enrichment completes while that secondary view is active.
		for _, v := range m.stack {
			if d, ok := v.(*views.DetailModel); ok && d.ResourceType() == msg.ResourceType {
				if f, exists := msg.Findings[d.ResourceID()]; exists {
					d.SetEnrichmentFinding(&f)
				} else {
					d.SetEnrichmentFinding(nil)
				}
			}
		}
	}

	// Fire next from queue — bump per-type gen before each dispatch.
	if len(m.enrichQueue) > 0 {
		next := m.enrichQueue[0]
		m.enrichQueue = m.enrichQueue[1:]
		// Clear-on-rerun-start for the next type.
		m.enrichmentTypeGen[next]++
		delete(m.enrichmentFindings, next)
		delete(m.enrichmentRan, next)
		cmd := m.probeEnrichment(next, m.enrichmentGen)
		return m, tea.Batch(flashCmd, cmd)
	}

	// All enrichment done — clear progress, free retained resources, save cache
	if m.enrichChecked >= m.enrichTotal {
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.SetEnrichProgress(0, 0)
		}
		m.probeResources = nil
		// Save cache with enrichment-updated issue counts.
		cmd := m.saveAvailabilityCache()
		return m, tea.Batch(flashCmd, cmd)
	}
	return m, flashCmd
}

// unifiedIssueCount returns the distinct count of resource IDs with ≥1 issue
// across both Wave-1 (IsIssue() status color) and Wave-2 (enrichment findings).
// Two findings on the same instance count as one.
//
// Only `!`-severity findings contribute to the S1 badge. `~`-severity findings
// are informational and must not bump the count.
//
// Invariant: result ≤ len(wave1Resources). Findings keyed by IDs not present
// in wave1Resources are skipped (orphans) — they would otherwise inflate the
// badge above the visible row count, e.g. an enricher dispatched for cluster
// type writing instance-keyed findings.
func unifiedIssueCount(wave1Resources []resource.Resource, td resource.ResourceTypeDef, findings map[string]resource.EnrichmentFinding) int {
	if td.ExcludeFromIssueBadge {
		return 0
	}
	knownIDs := make(map[string]struct{}, len(wave1Resources))
	for _, r := range wave1Resources {
		knownIDs[r.ID] = struct{}{}
	}
	ids := make(map[string]struct{})
	for _, r := range wave1Resources {
		if td.ResolveColor(r).IsIssue() {
			ids[r.ID] = struct{}{}
		}
	}
	for id, finding := range findings {
		if finding.Severity != "!" {
			continue
		}
		if _, ok := knownIDs[id]; ok {
			ids[id] = struct{}{}
		}
	}
	return len(ids)
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
