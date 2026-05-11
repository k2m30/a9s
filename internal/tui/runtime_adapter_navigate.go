// runtime_adapter_navigate.go — Bubble Tea adapter glue for runtime.Core's
// HandleNavigate entry point (Phase 05 PR-05a-h3, AS-149).
//
// handleNavigate replaces the deleted entry point from
// internal/tui/app_handlers_navigate.go. The signature is identical so the
// existing app.go dispatch line is unchanged.
//
// It constructs a transient runtime.Core, calls core.HandleNavigate, then
// applies the navigation decision to the view stack and translates any
// returned TaskRequests into tea.Cmd values.
//
// handleCopy, handleRefresh / refreshResourceList, handleReveal,
// handleIdentityLoaded, and handleIdentityError stay here as TUI-only
// helpers because every line of their bodies depends on adapter state
// (view stack, view-typed methods, flashState, m.identity, tea.Cmd
// returns). Their runtime-policy parts (cache-mutation gen bumps,
// per-type enrichment-rerun bookkeeping) are reads/writes against the
// embedded *Session — same data the runtime sees through c.session.
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
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleNavigate replaces the entry point previously in
// internal/tui/app_handlers_navigate.go. The signature is identical so the
// existing app.go dispatch line is unchanged.
//
// It calls runtime.Core.HandleNavigate to get the navigation decision, then
// constructs the requested view (when applicable) and translates any
// TaskRequests into tea.Cmd values. View construction and tea.Cmd wrapping
// stay here so the runtime is renderer-agnostic.
func (m Model) handleNavigate(msg messages.NavigateMsg) (tea.Model, tea.Cmd) {
	ev := runtime.NavigateEvent{
		Target:         translateNavigateTarget(msg.Target),
		ResourceType:   msg.ResourceType,
		Resource:       msg.Resource,
		ReplaceCurrent: msg.ReplaceCurrent,
	}
	// Resolve empty ResourceType from the active view for Detail/YAML/JSON.
	// The runtime has no view stack to consult; canonicalization happens here.
	if ev.ResourceType == "" {
		switch ev.Target {
		case runtime.NavigateTargetDetail:
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				ev.ResourceType = rl.ResourceType()
			}
		case runtime.NavigateTargetYAML, runtime.NavigateTargetJSON:
			switch av := m.activeView().(type) {
			case *views.ResourceListModel:
				ev.ResourceType = av.ResourceType()
			case *views.DetailModel:
				ev.ResourceType = av.ResourceType()
			}
		case runtime.NavigateTargetReveal:
			if rl, ok := m.activeView().(*views.ResourceListModel); ok {
				ev.ResourceType = rl.ResourceType()
			}
		}
	}

	result, tasks := m.core.HandleNavigate(ev)

	switch result.Kind {
	case runtime.NavigateKindNoop:
		return m, nil

	case runtime.NavigateKindFlash:
		flashText := result.FlashMessage
		flashErr := result.FlashIsError
		return m, func() tea.Msg {
			return messages.FlashMsg{Text: flashText, IsError: flashErr}
		}

	case runtime.NavigateKindPopAll:
		for m.popView() {
		}
		return m, nil

	case runtime.NavigateKindPushResourceListCached:
		canon := result.ResolvedType
		entry := result.CachedEntry
		rt := resource.FindResourceType(canon)
		if rt == nil {
			// Should be impossible — runtime canonicalised against the same
			// registry — but fail loud if it ever happens.
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("internal: unknown resource type after cache hit: %s", canon),
					IsError: true,
				}
			}
		}
		rl := views.NewResourceListFromCache(
			*rt, m.viewConfig, m.keys,
			entry.Resources, entry.Pagination,
			entry.FilterText, entry.SortColIdx, entry.SortAsc,
			entry.CursorPos, entry.HScrollOffset,
			entry.AttentionOnly,
		)
		if result.DisplayAlias != "" {
			rl.SetDisplayName(result.DisplayAlias)
		}
		rl.SetShowIssueBadge(true) // top-level list from main menu
		rl.SetSize(m.innerSize())
		issueCount, issueTrunc := 0, false
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			issueCount = menu.GetIssueCounts()[canon]
			issueTrunc = menu.GetIssueTruncated()[canon]
		}
		rl.SetEnrichmentState(issueCount, issueTrunc, findingsFromRows(entry.Resources))
		rl.SetTruncatedIDs(m.EnrichmentTruncatedIDs[canon])
		m.pushView(&rl)
		return m, nil

	case runtime.NavigateKindPushResourceList:
		canon := result.ResolvedType
		rt := resource.FindResourceType(canon)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("internal: unknown resource type: %s", canon),
					IsError: true,
				}
			}
		}
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
		if result.DisplayAlias != "" {
			rl.SetDisplayName(result.DisplayAlias)
		}
		rl.SetShowIssueBadge(true)
		rl.SetSize(m.innerSize())
		issueCount, issueTrunc := 0, false
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			issueCount = menu.GetIssueCounts()[canon]
			issueTrunc = menu.GetIssueTruncated()[canon]
		}
		rl.SetEnrichmentState(issueCount, issueTrunc, nil)
		rl.SetTruncatedIDs(m.EnrichmentTruncatedIDs[canon])
		rl, initCmd := rl.Init()
		m.pushView(&rl)
		fetchCmd := navigateTasksToCmd(m, msg, result, tasks)
		return m, tea.Batch(initCmd, fetchCmd)

	case runtime.NavigateKindPushDetail:
		if result.ReplaceCurrent {
			m.popView()
		}
		d := views.NewDetail(*result.Resource, result.ResolvedType, m.viewConfig, m.keys)
		d.SetNavProvider(resource.GetNavigableFields)
		d.SetSize(m.innerSize())
		if ef := findingFromResource(*result.Resource); ef != nil {
			d.SetEnrichmentFinding(ef)
		}
		m.pushView(&d)
		var cmds []tea.Cmd
		if result.DispatchEnrich {
			res := *result.Resource
			rt := result.ResolvedType
			cmds = append(cmds, func() tea.Msg {
				return messages.EnrichDetailMsg{ResourceType: rt, Resource: res}
			})
		}
		if result.DispatchRelated && d.NeedsRelatedCheck() {
			ck := session.RelatedCacheKey(result.ResolvedType, result.Resource.ID)
			if cached, ok := m.RelatedCache.Get(ck); ok && len(cached) > 0 {
				d.ApplyRelatedResults(session.RelatedCacheReplay(result.ResolvedType, cached))
			} else {
				res := *result.Resource
				rt := result.ResolvedType
				cmds = append(cmds, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{ResourceType: rt, SourceResource: res}
				})
			}
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)

	case runtime.NavigateKindPushYAML:
		if result.ReplaceCurrent {
			m.popView()
		}
		y := views.NewYAML(*result.Resource, result.ResolvedType, m.keys)
		y.SetSize(m.innerSize())
		m.pushView(&y)
		if result.DispatchEnrich {
			res := *result.Resource
			rt := result.ResolvedType
			return m, func() tea.Msg {
				return messages.EnrichDetailMsg{ResourceType: rt, Resource: res}
			}
		}
		return m, nil

	case runtime.NavigateKindPushJSON:
		if result.ReplaceCurrent {
			m.popView()
		}
		j := views.NewJSON(*result.Resource, result.ResolvedType, m.keys)
		j.SetSize(m.innerSize())
		m.pushView(&j)
		if result.DispatchEnrich {
			res := *result.Resource
			rt := result.ResolvedType
			return m, func() tea.Msg {
				return messages.EnrichDetailMsg{ResourceType: rt, Resource: res}
			}
		}
		return m, nil

	case runtime.NavigateKindPushHelp:
		ctx := m.helpContext()
		activeShortName := ""
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			activeShortName = rl.ShortName()
		}
		h := views.NewHelpWithResource(m.keys, ctx, activeShortName)
		h.SetSize(m.innerSize())
		m.pushView(&h)
		return m, nil

	case runtime.NavigateKindFetchProfiles:
		if m.preSuppliedClients != nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    "context switching is disabled in demo mode",
					IsError: true,
				}
			}
		}
		return m, navigateTasksToCmd(m, msg, result, tasks)

	case runtime.NavigateKindPushRegion:
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

	case runtime.NavigateKindPushTheme:
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

	case runtime.NavigateKindFetchReveal:
		return m, navigateTasksToCmd(m, msg, result, tasks)
	}
	return m, nil
}

// translateNavigateTarget maps the TUI's messages.ViewTarget enum to the
// runtime's NavigateTarget enum. Returning NavigateTargetUnknown for an
// unrecognised input causes HandleNavigate to noop, matching the original
// handler's silent default branch.
func translateNavigateTarget(t messages.ViewTarget) runtime.NavigateTarget {
	switch t {
	case messages.TargetMainMenu:
		return runtime.NavigateTargetMainMenu
	case messages.TargetResourceList:
		return runtime.NavigateTargetResourceList
	case messages.TargetDetail:
		return runtime.NavigateTargetDetail
	case messages.TargetYAML:
		return runtime.NavigateTargetYAML
	case messages.TargetJSON:
		return runtime.NavigateTargetJSON
	case messages.TargetReveal:
		return runtime.NavigateTargetReveal
	case messages.TargetProfile:
		return runtime.NavigateTargetProfile
	case messages.TargetRegion:
		return runtime.NavigateTargetRegion
	case messages.TargetTheme:
		return runtime.NavigateTargetTheme
	case messages.TargetHelp:
		return runtime.NavigateTargetHelp
	}
	return runtime.NavigateTargetUnknown
}

// navigateTasksToCmd translates TaskRequests from HandleNavigate into a
// Bubble Tea command. Unknown TaskKind values are dropped for forward-
// compatibility with newer runtime builds.
func navigateTasksToCmd(m Model, msg messages.NavigateMsg, result runtime.NavigateResult, tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch t.Key.Kind {
		case runtime.KindFetchResources:
			cmds = append(cmds, m.fetchResources(msg.ResourceType))
		case runtime.KindFetchProfiles:
			cmds = append(cmds, m.fetchProfiles())
		case runtime.KindFetchReveal:
			if p, ok := t.Payload.(runtime.FetchRevealPayload); ok {
				cmds = append(cmds, m.fetchRevealValue(p.ResourceType, p.ResourceID))
			}
		}
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	default:
		return tea.Batch(cmds...)
	}
}

// handleCopy performs context-dependent clipboard copy as a tea.Cmd.
// Each view implements CopyContent() to provide its own content and label.
//
// Pure adapter: depends only on m.activeView() and tea.Cmd.
func (m Model) handleCopy() (tea.Model, tea.Cmd) {
	content, label := m.activeView().CopyContent()
	if content == "" {
		return m, nil
	}
	return m, copyToClipboard(content, label)
}

// handleRefresh re-fetches resources when on a resource list view, restarts
// availability checks on the main menu, or re-triggers related-resource
// checks plus enrichment on a detail view.
//
// Stays in the adapter: every branch inspects view-typed state (active
// view kind, MainMenuModel, ResourceListModel, DetailModel) and returns
// tea.Cmd values. The runtime-owned mutations (gen bumps, cache deletes,
// applyEnrichment, ProbeResources/ProbeTruncated reset, RuleSets swap) all
// touch the embedded *Session — same data the runtime would mutate via
// c.session — so a future split into runtime-side helpers can land
// without re-shaping the call sites here.
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	// Main menu: restart availability checks (no-op in no-cache mode).
	if _, ok := m.activeView().(*views.MainMenuModel); ok {
		if m.noCache {
			return m, nil
		}
		// Increment gen to cancel any in-flight probes and enrichment.
		m.AvailabilityGen++
		m.Session.EnrichmentGen++
		m.EnrichmentRan = make(map[string]bool)
		m.EnrichmentTypeGen = make(map[string]int)
		m.EnrichmentTruncatedIDs = make(map[string]map[string]bool)
		// Clear stale Wave 2 from all cached rows before resetting ProbeResources.
		// Without this, opening a cached list before the new enrichment completes
		// would show the previous run's attention state (PR #310 CodeRabbit
		// finding B).
		clearAllWave2(&m)
		m.ProbeResources = make(map[string][]resource.Resource)
		m.ProbeTruncated = make(map[string]bool)
		// Reset the menu's view-side state (availability, issue counts) in
		// lockstep with the model-side maps above.
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.ClearAvailability()
		}
		m.flash = flashState{text: "Refreshing availability...", isError: false, active: true}
		cmd := m.loadAvailabilityCache()
		return m, cmd
	}

	// Detail view: re-trigger related resource checks and enrichment.
	if d, ok := m.activeView().(*views.DetailModel); ok {
		d.ResetRightColumn()
		rt := d.ResourceType()
		srcRes := d.SourceResource()
		m.RelatedCache.Delete(session.RelatedCacheKey(rt, srcRes.ID))
		m.RelatedGen++      // cancel in-flight results from previous batch
		m.EnrichGen++       // cancel in-flight enrichment from previous batch
		m.EnrichResKey = "" // force gen bump on next enrichment dispatch
		// Invalidate the SES v1 receipt rule set cache so Ctrl+R on a detail
		// view picks up receipt-rule changes without requiring a profile/region
		// switch. Swap (not Clear) so that any in-flight blocked
		// DescribeActiveReceiptRuleSet call writes to the orphaned old store on
		// completion rather than repopulating the new active one —
		// sesActiveReceiptRuleSet captures its store reference at entry; we
		// replace the slot here.
		if rt == "ses" {
			m.RuleSets = session.NewRuleSetStore()
			if m.clients != nil {
				m.clients.SetRuleSets(m.RuleSets)
			}
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

	// Pre-fetch cleanup: strip stale Wave 2 findings from all cached rows of
	// this type BEFORE deleting the cache entry. applyEnrichment walks
	// ResourceCache[rt] to find rows; because the active ResourceListModel's
	// allResources slice shares the same backing array as entry.Resources, the
	// mutation clears both the cache copy and the view's rows in one pass.
	// Deleting the cache entry afterwards is still correct (forces a fresh
	// fetch); the view rows are already cleared. This fixes the PR #310
	// CodeRabbit finding A: previously delete() ran first so applyEnrichment
	// found no rows and the rl's slice retained stale wave2 state.
	if rl.ParentContext() == nil && !rl.EscPops() {
		(&m).applyEnrichment(rt, nil)
	}

	delete(m.ResourceCache, rt) // clear cache for refreshed type only
	if rt == "ses" {
		// Swap (see detail-view path above): protects against in-flight blocked
		// DescribeActiveReceiptRuleSet fetchers re-poisoning the cache.
		m.RuleSets = session.NewRuleSetStore()
		if m.clients != nil {
			m.clients.SetRuleSets(m.RuleSets)
		}
	}
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}

	// Top-level list with a registered enricher: bump per-type gen, clear
	// findings, and dispatch a wrapped fetch that stamps TypeGen onto the
	// outgoing ResourcesLoadedMsg so the tail branch in app.go can seed
	// probeResources and dispatch probeEnrichment on success.
	if rl.ParentContext() == nil && !rl.EscPops() {
		if _, hasEnricher := awsclient.IssueEnricherRegistry[rt]; hasEnricher {
			m.EnrichmentTypeGen[rt]++
			tok := m.EnrichmentTypeGen[rt]
			delete(m.EnrichmentRan, rt)
			// Clear per-resource truncation markers too: if the refresh errors
			// out, stale "?" prefixes must not persist across the rerun.
			delete(m.EnrichmentTruncatedIDs, rt)
			// Wave2 already stripped above (pre-fetch cleanup). Strip any rows
			// that entered via ProbeResources/LazyResourceCache (those paths
			// are NOT covered by the pre-fetch cleanup above, which only
			// covers the ResourceCache entry before deletion).
			(&m).applyEnrichment(rt, nil)
			// Propagate the cleared state to the active ResourceListModel so
			// row markers disappear immediately at Ctrl+R — otherwise stale
			// markers would remain visible until the rerun completes (and
			// indefinitely if the refresh errors out).
			rl.SetEnrichmentState(0, false, nil)
			rl.SetTruncatedIDs(nil)
			cmd := m.refreshResourceListWithEnrichmentRerun(*rl, tok)
			return m, cmd
		}
		// Top-level list without an enricher: propagate the cleared state.
		// Wave2 was already stripped in the pre-fetch cleanup above.
		rl.SetEnrichmentState(0, false, nil)
	}
	return m, m.refreshResourceList(*rl)
}

// refreshResourceList chooses the right fetcher entry point for rl: filtered,
// child, or top-level paginated. Adapter-side helper used by handleRefresh
// and by other callers in the TUI.
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

// handleReveal fetches a revealed value using the resource type's registered
// reveal fetcher. Adapter-side because the lookup gates on the active view.
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
	return m, m.fetchRevealValue(rt, r.ID)
}

// handleIdentityLoaded caches the identity and updates the identity view if
// active. Adapter-side because m.identity / m.identityFetching live on the
// TUI Model today.
func (m Model) handleIdentityLoaded(msg messages.IdentityLoadedMsg) (tea.Model, tea.Cmd) {
	m.identityFetching = false
	if id, ok := msg.Identity.(*awsclient.CallerIdentity); ok {
		m.identity = id
	}
	if idView, ok := m.activeView().(*views.IdentityModel); ok {
		data := m.identityToViewData()
		idView.SetIdentity(data)
	}
	return m, nil
}

// handleIdentityError clears the fetching flag and updates the identity view
// if active. Adapter-side for the same reason as handleIdentityLoaded.
func (m Model) handleIdentityError(msg messages.IdentityErrorMsg) (tea.Model, tea.Cmd) {
	m.identityFetching = false
	if idView, ok := m.activeView().(*views.IdentityModel); ok {
		idView.SetError(msg.Err)
	}
	return m, nil
}

// copyToClipboard returns a tea.Cmd that writes content to the system
// clipboard and emits a FlashMsg with the success label or error text.
func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		if err != nil {
			return messages.FlashMsg{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.FlashMsg{Text: successLabel, IsError: false}
	}
}
