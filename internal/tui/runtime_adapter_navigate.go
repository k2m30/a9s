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
// and the identity-error view-update tail stay here as TUI-only helpers
// because every line of their bodies depends on adapter state (view
// stack, view-typed methods, flashState, tea.Cmd returns). Their
// runtime-policy parts (cache-mutation gen bumps, per-type
// enrichment-rerun bookkeeping) are reads/writes against the session
// owned by core — same data the runtime sees through its session field.
//
// PR-05a-h4-b (AS-962) removed the inline handleIdentityLoaded /
// handleIdentityError helpers in favour of HandleEvent-routed dispatch
// + applyIntents (SetIdentityIntent, HeaderInvalidateIntent). The
// IdentityError view-side note now flows through the runtime_adapter
// SetIdentityError path; handleIdentityError-the-method is gone.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
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
func (m Model) handleNavigate(msg messages.Navigate) (tea.Model, tea.Cmd) {
	ev := runtime.NavigateEvent{
		Target:         translateNavigateTarget(msg.Target),
		ResourceType:   msg.ResourceType,
		Resource:       msg.Resource,
		ReplaceCurrent: msg.ReplaceCurrent,
	}
	// Resolve empty ResourceType from the active rs for Detail/YAML/JSON.
	// The runtime has no view stack to consult; canonicalization happens here.
	if ev.ResourceType == "" {
		rs := m.activeRS()
		switch ev.Target {
		case runtime.NavigateTargetDetail:
			if rs.kind == rsKindList {
				ev.ResourceType = rs.resourceType
			}
		case runtime.NavigateTargetYAML, runtime.NavigateTargetJSON:
			if rs.kind == rsKindList || rs.kind == rsKindDetail {
				ev.ResourceType = rs.resourceType
			}
		case runtime.NavigateTargetReveal:
			if rs.kind == rsKindList {
				ev.ResourceType = rs.resourceType
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
			return messages.Flash{Text: flashText, IsError: flashErr}
		}

	case runtime.NavigateKindPopAll:
		for m.popRS() {
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
				return messages.Flash{
					Text:    fmt.Sprintf("internal: unknown resource type after cache hit: %s", canon),
					IsError: true,
				}
			}
		}
		// Sync m.ctrl stack before constructing the transient view so
		// topListState() inside NewResourceListFromCache resolves to this
		// screen's ListState.
		m.ctrl.PushChildListScreen(canon)
		rl := views.NewResourceListFromCache(
			*rt, m.viewConfig, m.keys,
			entry.Resources, entry.Pagination,
			entry.FilterText, entry.SortColIdx, entry.SortAsc,
			entry.CursorPos, entry.HScrollOffset,
			entry.AttentionOnly,
			m.ctrl,
		)
		if result.DisplayAlias != "" {
			rl.SetDisplayName(result.DisplayAlias)
		}
		rl.SetShowIssueBadge(true)
		rl.SetSize(m.innerSize())
		issueCount := m.ctrl.GetMenuIssueCounts()[canon]
		issueTrunc := m.ctrl.GetMenuIssueTruncated()[canon]
		rl.SetEnrichmentState(issueCount, issueTrunc, findingsFromRows(entry.Resources))
		rl.SetTruncatedIDs(m.core.EnrichmentTruncatedIDs(canon))
		rs := newListRS(canon)
		w, h := m.innerSize()
		rs.width, rs.height = w, h
		m.pushRS(rs)
		return m, nil

	case runtime.NavigateKindPushResourceList:
		canon := result.ResolvedType
		rt := resource.FindResourceType(canon)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.Flash{
					Text:    fmt.Sprintf("internal: unknown resource type: %s", canon),
					IsError: true,
				}
			}
		}
		// Sync m.ctrl stack before constructing the transient view so
		// topListState() inside NewResourceList resolves to this screen's ListState.
		m.ctrl.PushChildListScreen(canon)
		rl := views.NewResourceList(*rt, m.viewConfig, m.keys, m.ctrl)
		if result.DisplayAlias != "" {
			rl.SetDisplayName(result.DisplayAlias)
		}
		rl.SetShowIssueBadge(true)
		rl.SetSize(m.innerSize())
		issueCount := m.ctrl.GetMenuIssueCounts()[canon]
		issueTrunc := m.ctrl.GetMenuIssueTruncated()[canon]
		rl.SetEnrichmentState(issueCount, issueTrunc, nil)
		rl.SetTruncatedIDs(m.core.EnrichmentTruncatedIDs(canon))
		_, initCmd := rl.Init()
		rs := newListRS(canon)
		w, h := m.innerSize()
		rs.width, rs.height = w, h
		m.pushRS(rs)
		fetchCmd := navigateTasksToCmd(m, msg, result, tasks)
		return m, tea.Batch(initCmd, fetchCmd)

	case runtime.NavigateKindPushDetail:
		if result.ReplaceCurrent {
			m.popRS()
		}
		// Push ScreenDetail onto the controller stack and seed DetailState so
		// Snapshot().Body.Detail is non-nil from the first render.
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
		m.ctrl.EnsureDetailState(*result.Resource, result.ResolvedType)
		// Initialise related rows from registered defs so the controller body
		// shows loading state immediately (mirrors newRightColumn on SetSize).
		m.ctrl.InitDetailRelatedRows(result.ResolvedType)
		// Seed wave-2 findings into the controller immediately when the resource
		// already carries a finding (wave-1 or pre-loaded wave-2).
		if ef, ad := findingFromResource(*result.Resource); ef != nil {
			m.ctrl.ApplyDetailFinding(ef, ad)
		}
		// Create a transient detail model only to configure the controller state
		// (SetNavProvider seeds navigable-field data into the ctrl).
		d := views.NewDetailWithCtrl(*result.Resource, result.ResolvedType, m.viewConfig, m.keys, m.ctrl)
		d.SetNavProvider(resource.GetNavigableFields)
		d.SetSize(m.innerSize())
		// Push a detail rendererState instead of the view model.
		detailRS := newDetailRS(result.ResolvedType)
		w, h := m.innerSize()
		detailRS.width, detailRS.height = w, h
		// The right column auto-shows when related defs exist — mirror SetSize behaviour.
		defs := resource.GetRelated(result.ResolvedType)
		if len(defs) > 0 {
			detailRS.rightCol = views.NewRightColumn(defs, *result.Resource, result.ResolvedType)
			detailRS.rightColAutoShown = true
			detailRS.rightColVisible = true
		}
		m.pushRS(detailRS)
		var cmds []tea.Cmd
		if result.DispatchEnrich {
			res := *result.Resource
			rt := result.ResolvedType
			cmds = append(cmds, func() tea.Msg {
				return messages.EnrichDetail{ResourceType: rt, Resource: res}
			})
		}
		if result.DispatchRelated && detailRS.rightColAutoShown {
			ck := runtime.RelatedCacheKey(result.ResolvedType, result.Resource.ID)
			if cached, ok := m.core.RelatedCacheGet(ck); ok && len(cached) > 0 {
				// Replay cached related results directly into the controller.
				for _, msg := range runtime.RelatedCacheReplay(result.ResolvedType, cached) {
					errMsg := ""
					if msg.Result.Err != nil {
						errMsg = msg.Result.Err.Error()
					}
					m.ctrl.ApplyDetailRelatedResultForResource(
						result.ResolvedType,
						result.Resource.ID,
						msg.DefDisplayName,
						msg.Result.TargetType,
						msg.Result.Count,
						false,
						errMsg,
						msg.Result.Approximate,
						msg.Result.ResourceIDs,
						msg.Result.FetchFilter,
					)
				}
			} else {
				res := *result.Resource
				rt := result.ResolvedType
				cmds = append(cmds, func() tea.Msg {
					return messages.RelatedCheckStarted{ResourceType: rt, SourceResource: res}
				})
			}
		}
		if len(cmds) == 0 {
			return m, nil
		}
		return m, tea.Batch(cmds...)

	case runtime.NavigateKindPushYAML:
		if result.ReplaceCurrent {
			m.popRS()
		}
		y := views.NewYAMLWithCtrl(*result.Resource, result.ResolvedType, m.keys, m.ctrl)
		y.SetSize(m.innerSize())
		// Push ScreenYAML onto the controller stack and seed TextState with the
		// syntax-colored content lines so Snapshot().Body.Text is non-nil from
		// the first render. Must happen after SetSize so ContentLines() uses the
		// fully-initialised viewport width for any width-dependent output.
		// Carry ResourceType + ResourceID so selectedResourceForAction resolves
		// the resource from this text screen (enables 't', child views, etc.).
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
			ID: runtime.ScreenYAML,
			Context: runtime.ScreenContext{
				ResourceType: result.ResolvedType,
				ResourceID:   result.Resource.ID,
			},
		}})
		m.ctrl.EnsureTextState(y.ContentLines())
		textRS := newTextRS()
		res := *result.Resource
		textRS.textResource = &res
		w, h := m.innerSize()
		textRS.width, textRS.height = w, h
		m.pushRS(textRS)
		if result.DispatchEnrich {
			rt := result.ResolvedType
			return m, func() tea.Msg {
				return messages.EnrichDetail{ResourceType: rt, Resource: res}
			}
		}
		return m, nil

	case runtime.NavigateKindPushJSON:
		if result.ReplaceCurrent {
			m.popRS()
		}
		j := views.NewJSONWithCtrl(*result.Resource, result.ResolvedType, m.keys, m.ctrl)
		j.SetSize(m.innerSize())
		// Push ScreenJSON onto the controller stack and seed TextState with the
		// syntax-colored content lines so Snapshot().Body.Text is non-nil from
		// the first render. Must happen after SetSize so ContentLines() uses the
		// fully-initialised viewport width for any width-dependent output.
		// Carry ResourceType + ResourceID so selectedResourceForAction resolves
		// the resource from this text screen (enables 't', child views, etc.).
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
			ID: runtime.ScreenJSON,
			Context: runtime.ScreenContext{
				ResourceType: result.ResolvedType,
				ResourceID:   result.Resource.ID,
			},
		}})
		m.ctrl.EnsureTextState(j.ContentLines())
		textRS := newTextRS()
		jres := *result.Resource
		textRS.textResource = &jres
		w, h := m.innerSize()
		textRS.width, textRS.height = w, h
		m.pushRS(textRS)
		if result.DispatchEnrich {
			rt := result.ResolvedType
			return m, func() tea.Msg {
				return messages.EnrichDetail{ResourceType: rt, Resource: jres}
			}
		}
		return m, nil

	case runtime.NavigateKindPushHelp:
		ctx := m.helpContext()
		activeShortName := ""
		if rs := m.activeRS(); rs.kind == rsKindList {
			activeShortName = rs.resourceType
		}
		helpRS := newHelpRS(ctx, activeShortName)
		w, h := m.innerSize()
		helpRS.width, helpRS.height = w, h
		m.pushRS(helpRS)
		return m, nil

	case runtime.NavigateKindFetchProfiles:
		if m.core.PreSuppliedClients() != nil {
			return m, func() tea.Msg {
				return messages.Flash{
					Text:    "context switching is disabled in demo mode",
					IsError: true,
				}
			}
		}
		return m, navigateTasksToCmd(m, msg, result, tasks)

	case runtime.NavigateKindPushRegion:
		if m.core.PreSuppliedClients() != nil {
			return m, func() tea.Msg {
				return messages.Flash{
					Text:    "region switching is disabled in demo mode",
					IsError: true,
				}
			}
		}
		regions := m.core.AllRegions()
		regionCodes := make([]string, len(regions))
		for i, r := range regions {
			regionCodes[i] = r.Code
		}
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenRegion}})
		m.ctrl.EnsureSelectorState(regionCodes, m.core.Region(), "aws-regions")
		selRS := newSelectorRS(func(s string) tea.Msg {
			return messages.RegionSelected{Region: s}
		})
		wR, hR := m.innerSize()
		selRS.width, selRS.height = wR, hR
		m.pushRS(selRS)
		return m, nil

	case runtime.NavigateKindPushTheme:
		cfgDir := config.ConfigDir()
		if cfgDir == "" {
			return m, func() tea.Msg {
				return messages.Flash{Text: "Config directory not available", IsError: true}
			}
		}
		themesDir := filepath.Join(cfgDir, "themes")
		entries, err := os.ReadDir(themesDir)
		if err != nil {
			return m, func() tea.Msg {
				return messages.Flash{Text: "Cannot read themes directory: " + err.Error(), IsError: true}
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
				return messages.Flash{Text: "No theme files found in " + themesDir, IsError: true}
			}
		}
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenTheme}})
		m.ctrl.EnsureSelectorState(themeFiles, m.activeTheme, "themes")
		thRS := newSelectorRS(func(s string) tea.Msg {
			return messages.ThemeSelected{Theme: s}
		})
		wT, hT := m.innerSize()
		thRS.width, thRS.height = wT, hT
		m.pushRS(thRS)
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
func navigateTasksToCmd(m Model, msg messages.Navigate, result runtime.NavigateResult, tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch t.Key.Kind {
		case runtime.KindFetchResources:
			// The runtime stamps the canonical (alias-resolved) ShortName onto
			// req.Key.Scope when it builds the task, so ExecuteTask uses the same
			// value that the former fetchResources(fetchRT, ...) call used.
			// result.ResolvedType / msg.ResourceType fallback is no longer needed.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.KindFetchProfiles:
			// ErrAdapterOnlyTask — profiles result is a TUI-private type; keep adapter-local.
			cmds = append(cmds, m.fetchProfiles())

		case runtime.KindFetchReveal:
			// Route through ExecuteTask; fall back to adapter if needed.
			cmds = append(cmds, m.executeTaskCmd(t))
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
// Dispatches by active rs.kind to produce (content, label) pairs.
func (m Model) handleCopy() (tea.Model, tea.Cmd) {
	rs := m.activeRS()
	snap := m.ctrl.Snapshot()
	var content, label string
	switch rs.kind {
	case rsKindList:
		r, ok := m.ctrl.ListSelected()
		if !ok {
			return m, nil
		}
		rt := resource.FindResourceType(rs.resourceType)
		if rt != nil && rt.CopyField != "" {
			if val, ok2 := r.Fields[rt.CopyField]; ok2 && val != "" {
				content, label = val, "Copied: "+val
				break
			}
		}
		content, label = r.ID, "Copied: "+r.ID
	case rsKindDetail:
		if rs.rightCol.IsFocused() {
			name := rs.rightCol.SelectedTypeName()
			if name != "" {
				content, label = name, "Copied: "+name
			}
			break
		}
		if snap.Body.Detail != nil {
			fc := snap.Body.Detail.FieldCursor
			if fc >= 0 && fc < len(snap.Body.Detail.Fields) {
				item := snap.Body.Detail.Fields[fc]
				val := item.Value
				if val == "" {
					val = item.Key
				}
				if val != "" {
					content, label = val, "Copied: "+val
					break
				}
			}
		}
		// Fallback: copy raw YAML of the detail resource.
		if snap.Body.Detail != nil {
			rawLines := snap.Body.Detail.Fields
			_ = rawLines
			// Build raw YAML using the controller resource.
			res := m.ctrl.GetDetailResource()
			if res.ID != "" {
				content = rawYAMLFromResource(res)
				if content != "" {
					label = "Copied detail to clipboard"
				}
			}
		}
	case rsKindReveal:
		content, label = rs.revealValue, "Secret copied to clipboard"
	case rsKindText:
		if snap.Body.Text != nil {
			content = rawContentFromTextBody(snap.Body.Text)
			if content != "" {
				label = "Copied YAML to clipboard"
			}
		}
	case rsKindIdentity:
		if !rs.identityLoading && rs.identityData.ARN != "" {
			content, label = rs.identityData.ARN, "Copied!"
		}
	}
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
// touch the session owned by core — same data the runtime would mutate via
// c.session — so a future split into runtime-side helpers can land
// without re-shaping the call sites here.
func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	rs := m.activeRS()

	// Main menu: restart availability checks (no-op in no-cache mode).
	if rs.kind == rsKindMenu {
		if m.core.NoCache() {
			return m, nil
		}
		// Increment gen to cancel any in-flight probes and enrichment.
		m.core.BumpAvailabilityGen()
		m.core.BumpEnrichmentGen()
		m.core.ResetEnrichmentMaps()
		// Clear stale Wave 2 from all cached rows before resetting ProbeResources.
		// Without this, opening a cached list before the new enrichment completes
		// would show the previous run's attention state (PR #310 CodeRabbit
		// finding B).
		clearAllWave2(&m)
		m.core.ResetProbeMaps()
		// Reset the menu's availability / issue-count state via the controller.
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
		m.flash = flashState{text: "Refreshing availability...", isError: false, active: true}
		cmd := m.loadAvailabilityCache()
		return m, cmd
	}

	// Detail view: re-trigger related resource checks and enrichment.
	if rs.kind == rsKindDetail {
		rt := rs.resourceType
		srcRes := m.ctrl.GetDetailResource()
		// Reset right column widget on the active rendererState.
		if rs.rightColVisible || rs.rightColAutoShown {
			defs := resource.GetRelated(rt)
			rs.rightCol = views.NewRightColumn(defs, srcRes, rt)
			rcw := views.ComputeRightColWidth(rs.width, 32)
			rs.rightCol.SetSize(rcw, rs.height)
		}
		// Reset controller's RelatedRows to loading state so View() shows
		// loading rows immediately rather than stale counts.
		m.ctrl.ResetDetailRelatedRows(rt)
		m.core.RelatedCacheDelete(runtime.RelatedCacheKey(rt, srcRes.ID))
		m.core.BumpRelatedGen() // cancel in-flight results from previous batch
		m.core.BumpEnrichGen()  // cancel in-flight enrichment from previous batch
		m.core.ClearEnrichResKey() // force gen bump on next enrichment dispatch
		// Invalidate the SES v1 receipt rule set cache so Ctrl+R on a detail
		// view picks up receipt-rule changes without requiring a profile/region
		// switch. Swap (not Clear) so that any in-flight blocked
		// DescribeActiveReceiptRuleSet call writes to the orphaned old store on
		// completion rather than repopulating the new active one —
		// sesActiveReceiptRuleSet captures its store reference at entry; we
		// replace the slot here.
		if rt == "ses" {
			m.core.ResetRuleSets()
		}
		m.flash = flashState{text: "Refreshing...", isError: false, active: true}

		var cmds []tea.Cmd
		cmds = append(cmds, func() tea.Msg {
			return messages.RelatedCheckStarted{
				ResourceType:   rt,
				SourceResource: srcRes,
			}
		})
		if resource.HasDetailEnricher(rt) {
			cmds = append(cmds, func() tea.Msg {
				return messages.EnrichDetail{
					ResourceType: rt,
					Resource:     srcRes,
				}
			})
		}
		return m, tea.Batch(cmds...)
	}

	if rs.kind != rsKindList {
		return m, nil
	}
	rt := rs.resourceType
	parentCtx := m.ctrl.GetListParentContext()
	escPops := m.ctrl.GetListEscPops()

	// Pre-fetch cleanup: strip stale Wave 2 findings from all cached rows of
	// this type BEFORE deleting the cache entry. applyEnrichment walks
	// ResourceCache[rt] to find rows; because the active ResourceListModel's
	// allResources slice shares the same backing array as entry.Resources, the
	// mutation clears both the cache copy and the view's rows in one pass.
	// Deleting the cache entry afterwards is still correct (forces a fresh
	// fetch); the view rows are already cleared. This fixes the PR #310
	// CodeRabbit finding A: previously delete() ran first so applyEnrichment
	// found no rows and the rl's slice retained stale wave2 state.
	if parentCtx == nil && !escPops {
		(&m).applyEnrichment(rt, nil, nil)
	}

	m.core.DeleteResourceCache(rt) // clear cache for refreshed type only
	if rt == "ses" {
		// Swap (see detail-view path above): protects against in-flight blocked
		// DescribeActiveReceiptRuleSet fetchers re-poisoning the cache.
		m.core.ResetRuleSets()
	}
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}

	// Top-level list with a registered enricher: bump per-type gen, clear
	// findings, and dispatch a wrapped fetch that stamps TypeGen onto the
	// outgoing ResourcesLoadedMsg so the tail branch in app.go can seed
	// probeResources and dispatch probeEnrichment on success.
	if parentCtx == nil && !escPops {
		if m.core.HasIssueEnricher(rt) {
			tok := m.core.BumpEnrichmentTypeGen(rt)
			m.core.DeleteEnrichmentRan(rt)
			// Clear per-resource truncation markers too: if the refresh errors
			// out, stale "?" prefixes must not persist across the rerun.
			m.core.DeleteEnrichmentTruncatedIDs(rt)
			// Wave2 already stripped above (pre-fetch cleanup). Strip any rows
			// that entered via ProbeResources/LazyResourceCache (those paths
			// are NOT covered by the pre-fetch cleanup above, which only
			// covers the ResourceCache entry before deletion).
			(&m).applyEnrichment(rt, nil, nil)
			// Propagate the cleared enrichment state to the controller so row
			// markers disappear immediately at Ctrl+R.
			m.ctrl.ApplyEnrichmentState(rt, 0, false, nil)
			cmd := m.refreshActiveListWithEnrichmentRerun(rt, tok)
			return m, cmd
		}
		// Top-level list without an enricher: clear the enrichment state.
		// Wave2 was already stripped in the pre-fetch cleanup above.
		m.ctrl.ApplyEnrichmentState(rt, 0, false, nil)
	}
	return m, m.refreshActiveList()
}

// refreshActiveList refreshes the top-of-stack resource list, reading its
// resource type + fetch configuration from the controller rather than from
// a stored view model. Used by handleRefresh and the RefreshActiveListIntent
// handler in runtime_adapter.go (applyIntent).
func (m Model) refreshActiveList() tea.Cmd {
	rs := m.activeRS()
	if rs.kind != rsKindList {
		return nil
	}
	rt := rs.resourceType
	gen := m.core.AvailabilityGen()

	if ff := m.ctrl.GetListFetchFilter(); len(ff) > 0 {
		return m.fetchResourcesFiltered(rt, ff, gen)
	}
	if pc := m.ctrl.GetListParentContext(); pc != nil {
		return m.fetchChildResources(rt, pc)
	}
	return m.fetchResources(rt, gen)
}

// refreshActiveListWithEnrichmentRerun wraps refreshActiveList with an
// enrichment-rerun token stamp — mirrors refreshResourceListWithEnrichmentRerun
// in probe_adapter.go but reads config from the controller rather than a
// stored ResourceListModel.
func (m Model) refreshActiveListWithEnrichmentRerun(rt string, tok domain.Gen) tea.Cmd {
	inner := m.refreshActiveList()
	return func() tea.Msg {
		msg := inner()
		if loaded, ok := msg.(messages.ResourcesLoaded); ok {
			loaded.TypeGen = tok
			return loaded
		}
		return msg
	}
}

// rawYAMLFromResource converts a resource.Resource to YAML for clipboard copy.
// Delegates to the exported views.RawYAMLFromResource to reuse the same
// reflect+yaml.Marshal logic as DetailModel.RawYAML() without needing a stored
// DetailModel instance.
func rawYAMLFromResource(res resource.Resource) string {
	return views.RawYAMLFromResource(res)
}

// rawContentFromTextBody returns the plain-text content from a TextBody for
// clipboard copy. Joins lines with newlines, stripping any ANSI color codes
// that the syntax-colorizer may have embedded.
func rawContentFromTextBody(body *app.TextBody) string {
	if body == nil {
		return ""
	}
	var sb strings.Builder
	for i, line := range body.Lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(stripANSIInline(line))
	}
	return sb.String()
}

// stripANSIInline removes ANSI escape sequences from s. Mirrors the inline
// logic in DetailModel.PlainContent() in internal/tui/views/detail_render.go.
func stripANSIInline(s string) string {
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 'a' || s[j] > 'z') && (s[j] < 'A' || s[j] > 'Z') {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// handleReveal fetches a revealed value using the resource type's registered
// reveal fetcher. Adapter-side because the lookup gates on the active rs.
func (m Model) handleReveal() (tea.Model, tea.Cmd) {
	rs := m.activeRS()
	if rs.kind != rsKindList {
		return m, nil
	}
	rt := rs.resourceType
	if !resource.HasRevealFetcher(rt) {
		return m, nil
	}
	r, ok := m.ctrl.ListSelected()
	if !ok {
		return m, nil
	}
	cmd := m.fetchRevealValue(rt, r.ID, m.core.ConnectGen())
	return m, cmd
}

// handleIdentityError clears the fetching flag (via Core through
// HandleEvent → HandleIdentityError) and additionally updates the
// IdentityModel view if active. The view-side note stays in the
// adapter because IdentityModel.SetError requires inspecting the
// renderer's view stack — out of scope for the platform-agnostic Core.
//
// AS-657 stamped IdentityError with AspectConnect; the shim performs
// the stale-gen check up-front so the view-side SetError() does not
// fire on a stale error from a prior profile/region.
func (m Model) handleIdentityError(msg messages.IdentityError) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	updated, cmd := m.coreUpdate(msg)
	if um, ok := updated.(Model); ok {
		m = um
	}
	// Update the identity rendererState error field if an identity rs is active.
	if rs := m.activeRS(); rs.kind == rsKindIdentity {
		rs.identityLoading = false
		if msg.Err != "" {
			rs.identityErr = msg.Err
		}
	}
	return m, cmd
}

// handleToggleRelated handles the 'r' key on detail screens: toggles the right-column
// related panel. Replaces the equivalent case in DetailModel.Update which is no
// longer stored on the stack. State lives directly on the rendererState.
func (m Model) handleToggleRelated() (tea.Model, tea.Cmd) {
	rs := m.activeRS()
	if rs.kind != rsKindDetail {
		return m, nil
	}
	rs.rightColUserToggled = true
	if rs.width < layout.MinInnerContentWidth {
		return m, nil
	}
	rt := rs.resourceType
	srcRes := m.ctrl.GetDetailResource()
	if rs.rightColAutoShown {
		// First explicit toggle: hide the auto-shown column.
		rs.rightColAutoShown = false
		rs.rightColVisible = false
		rs.rightCol.SetFocused(false)
		m.ctrl.SetDetailRelatedVisible(false, true)
		return m, nil
	}
	// Normal toggle: flip visible state.
	rs.rightColVisible = !rs.rightColVisible
	if rs.rightColVisible {
		defs := resource.GetRelated(rt)
		rs.rightCol = views.NewRightColumn(defs, srcRes, rt)
		m.ctrl.SetDetailRelatedVisible(true, false)
		return m, func() tea.Msg {
			return messages.RelatedCheckStarted{
				ResourceType:   rt,
				SourceResource: srcRes,
			}
		}
	}
	rs.rightCol.SetFocused(false)
	m.ctrl.SetDetailRelatedVisible(false, true)
	return m, nil
}

// copyToClipboard returns a tea.Cmd that writes content to the system
// clipboard and emits a FlashMsg with the success label or error text.
func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.WriteAll(content)
		if err != nil {
			return messages.Flash{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.Flash{Text: successLabel, IsError: false}
	}
}
