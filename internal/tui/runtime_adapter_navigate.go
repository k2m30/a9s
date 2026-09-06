// SPDX-License-Identifier: GPL-3.0-or-later

// runtime_adapter_navigate.go — Bubble Tea adapter glue for runtime.Core's
// HandleNavigate entry point.
//
// handleNavigate calls core.HandleNavigate, applies the navigation decision to
// the view stack, and translates the returned TaskRequests into tea.Cmd values.
//
// handleCopy, handleRefresh / refreshResourceList, and handleReveal stay here as
// TUI-only helpers: their bodies depend on adapter state (view stack, view-typed
// methods, flash, tea.Cmd returns) and read/write the session owned by core.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/atotto/clipboard"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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
		// screen's ListState. This is a top-level, menu-driven list — pushed as
		// ScreenResourceList (not PushChildListScreen's ScreenChildList, which
		// is reserved for actual child/related lists, see list_state.go's
		// "persist-eligible" contract). Using ScreenChildList here silently
		// disabled the C6 disk-cache save gate
		// (maybeSaveResourceListCache checks screen.ID == ScreenResourceList)
		// for every top-level TUI list (#17 wave 2).
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: canon},
		}})
		m.ctrl.EnsureListState()
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
		rl.SetSize(m.innerSize())
		issueCount := m.ctrl.GetMenuIssueCounts()[canon]
		issueTrunc := m.ctrl.GetMenuIssueTruncated()[canon]
		rl.SetEnrichmentState(issueCount, issueTrunc, wave2FindingsByID(entry.Resources), wave2DetailsByID(entry.Resources))
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
		// topListState() inside NewResourceList/NewResourceListFromCache
		// resolves to this screen's ListState. Top-level, menu-driven list —
		// see the ScreenResourceList-not-ScreenChildList note on the
		// NavigateKindPushResourceListCached branch above (#17 wave 2).
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
			ID:      runtime.ScreenResourceList,
			Context: runtime.ScreenContext{ResourceType: canon},
		}})
		m.ctrl.EnsureListState()
		var rl views.ResourceListModel
		var initCmd tea.Cmd
		if result.CachedEntry != nil {
			// Per C1/Goal 4: HandleNavigate attached a synthetic seed from
			// session.ProbeResources/ProbeTruncated on this cache-miss branch —
			// build the list the same way the PushResourceListCached case does
			// (no loading shell, no spinner) so warm list-open renders instantly.
			// The fetch task below (already emitted by HandleNavigate for every
			// PushResourceList result) still runs to verify/replace the seeded
			// rows — render what you know, verify on sight.
			entry := result.CachedEntry
			rl = views.NewResourceListFromCache(
				*rt, m.viewConfig, m.keys,
				entry.Resources, entry.Pagination,
				"", 0, false, 0, 0, false,
				m.ctrl,
			)
			// C3 (docs/design/cache-requirements.md): a seeded-but-unverified
			// surface must carry the refreshing marker so it renders
			// distinguishably from verified-fresh content. NewResourceListFromCache
			// (via ApplyResourcesLoaded) clears Refreshing as part of applying the
			// seeded page, so this must run after construction — mirrors the
			// headless controller's ordering in applyNavResult.
			m.ctrl.SetListRefreshing(true)
			// Seed-time provisional total: same set-after-seed ordering — ApplyResourcesLoaded
			// unconditionally clears TotalCount as part of applying the seeded
			// page, so this must also run after construction.
			m.ctrl.SetListTotalCount(entry.TotalCount)
		} else {
			rl = views.NewResourceList(*rt, m.viewConfig, m.keys, m.ctrl)
			_, initCmd = rl.Init()
		}
		if result.DisplayAlias != "" {
			rl.SetDisplayName(result.DisplayAlias)
		}
		rl.SetSize(m.innerSize())
		issueCount := m.ctrl.GetMenuIssueCounts()[canon]
		issueTrunc := m.ctrl.GetMenuIssueTruncated()[canon]
		rl.SetEnrichmentState(issueCount, issueTrunc, nil, nil)
		rl.SetTruncatedIDs(m.core.EnrichmentTruncatedIDs(canon))
		rs := newListRS(canon)
		w, h := m.innerSize()
		rs.width, rs.height = w, h
		m.pushRS(rs)
		fetchCmd := navigateTasksToCmd(m, tasks)
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
		// No separate ApplyDetailFinding call here: EnsureDetailState above
		// already seeds ds.Findings/ds.AttentionDetails from
		// result.Resource.Findings/AttentionDetails verbatim (a freshly-pushed
		// ScreenDetail always hits ensureDetailState's non-nil branch), which
		// already carries every wave-2 finding ApplyWave2ToRow appended (#52 —
		// a resource with more than one independently-evaluated condition).
		// primaryWave2Finding only ever surfaces the WORST-severity one; re-applying it
		// here would strip the ones EnsureDetailState just correctly seeded
		// down to that single entry.
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
		// BeginDetailWorkload begins the op and returns its complete workload
		// (enrich + related) in one call — cache-replay suppression (D6: no
		// re-fan-out over cached data) is decided INSIDE it (shared with the
		// headless/web NavigateKindPushDetail case in core/app/navigate.go),
		// not re-derived here via a separate ReplayRelatedCache call.
		_, workloadTasks := m.ctrl.BeginDetailWorkload(result.ResolvedType, *result.Resource, false, false)
		if !detailRS.rightColAutoShown {
			// Narrow terminal or no registered related defs: the right
			// column never shows, so drop the related half — the enrich
			// half (detail fields) is unaffected by column visibility.
			workloadTasks = dropTaskKind(workloadTasks, runtime.KindRelatedCheck)
		}
		if len(workloadTasks) == 0 {
			return m, nil
		}
		return m, m.dispatchTaskRequests(workloadTasks)

	case runtime.NavigateKindPushYAML:
		y := views.NewYAMLWithCtrl(*result.Resource, result.ResolvedType, m.keys, m.ctrl)
		return m.pushTextScreen(result, runtime.ScreenYAML, &y)

	case runtime.NavigateKindPushJSON:
		j := views.NewJSONWithCtrl(*result.Resource, result.ResolvedType, m.keys, m.ctrl)
		return m.pushTextScreen(result, runtime.ScreenJSON, &j)

	case runtime.NavigateKindPushHelp:
		ctx := m.helpContext()
		activeShortName := ""
		if rs := m.activeRS(); rs.kind == rsKindList {
			activeShortName = rs.resourceType
		}
		// Every sibling NavigateKindPush* case pushes the matching controller
		// screen before its rendererState (see PushRegion/PushTheme/PushCosts
		// immediately below) — this one didn't, which desynced m.stack from
		// m.ctrl's screen stack the moment Help was opened via this Navigate
		// path (as opposed to the direct '?' key handler in app_input.go,
		// which already pushes both).
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})
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
		return m, navigateTasksToCmd(m, tasks)

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
		return m.pushSelectorScreen(runtime.ScreenRegion, regionCodes, m.core.Region(), "aws-regions", func(s string) tea.Msg {
			return messages.RegionSelected{Region: s}
		})

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
		return m.pushSelectorScreen(runtime.ScreenTheme, themeFiles, m.activeTheme, "themes", func(s string) tea.Msg {
			return messages.ThemeSelected{Theme: s}
		})

	case runtime.NavigateKindFetchReveal:
		return m, navigateTasksToCmd(m, tasks)

	case runtime.NavigateKindPushCosts:
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
		m.ctrl.EnsureCostsState(app.Now())
		// SC-002: HandleNavigate itself never fetches unconditionally —
		// EnsureCostsFetch is the sole cache-first decision point, so a
		// warm cache opens with zero CE calls.
		tasks = append(tasks, m.ctrl.EnsureCostsFetch()...)
		costsRS := newCostsRS()
		w, h := m.innerSize()
		costsRS.width, costsRS.height = w, h
		m.pushRS(costsRS)
		return m, navigateTasksToCmd(m, tasks)
	}
	return m, nil
}

// textScreenView is the shared surface of YAMLModel and JSONModel that
// pushTextScreen needs: a resizable viewport and its rendered content lines.
type textScreenView interface {
	SetSize(w, h int)
	ContentLines() []string
}

// pushTextScreen handles the shared push logic for NavigateKindPushYAML and
// NavigateKindPushJSON: sizes the view, pushes the controller screen with
// resource context, seeds TextState with the syntax-colored content lines
// (must happen after SetSize so ContentLines() uses the fully-initialised
// viewport width), pushes the rendererState, and dispatches DetailEnrich
// when requested. Carries ResourceType + ResourceID so
// selectedResourceForAction resolves the resource from this text screen
// (enables 't', child views, etc.).
func (m Model) pushTextScreen(result runtime.NavigateResult, screenID runtime.ScreenID, view textScreenView) (tea.Model, tea.Cmd) {
	if result.ReplaceCurrent {
		m.popRS()
	}
	view.SetSize(m.innerSize())
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID: screenID,
		Context: runtime.ScreenContext{
			ResourceType: result.ResolvedType,
			ResourceID:   result.Resource.ID,
		},
	}})
	m.ctrl.EnsureTextState(view.ContentLines())
	res := *result.Resource
	m.ctrl.SetTextResource(res)
	textRS := newTextRS()
	textRS.textResource = &res
	w, h := m.innerSize()
	textRS.width, textRS.height = w, h
	m.pushRS(textRS)
	// Dispatch the operation's COMPLETE workload, not just enrich: the
	// related-check task's result still writes RelatedCache for the detail
	// screen beneath this YAML/JSON overlay, or for the next plain-detail
	// open of this same resource — mirrors core/app/navigate.go's
	// NavigateKindPushYAML/JSON cases (applyNavResult).
	_, tasks := m.ctrl.BeginDetailWorkload(result.ResolvedType, res, false, false)
	if len(tasks) == 0 {
		return m, nil
	}
	return m, m.dispatchTaskRequests(tasks)
}

// dropTaskKind returns tasks with every entry whose Key.Kind matches kind
// removed. Used where a renderer-only gate (narrow terminal, right column not
// auto-shown) must suppress one task from the complete workload
// Controller.BeginDetailWorkload returns — the builder decides the workload
// from registration + cache state alone and cannot know about renderer
// visibility, so the adapter filters its opaque result rather than asking
// the builder for a partial one.
func dropTaskKind(tasks []runtime.TaskRequest, kind runtime.TaskKind) []runtime.TaskRequest {
	out := tasks[:0:0]
	for _, t := range tasks {
		if t.Key.Kind != kind {
			out = append(out, t)
		}
	}
	return out
}

// pushSelectorScreen handles the shared push logic for NavigateKindPushRegion
// and NavigateKindPushTheme: pushes the controller screen, seeds
// SelectorState with the option list, current selection, and group key, then
// pushes a selectorRS wired to onSelect.
func (m Model) pushSelectorScreen(screenID runtime.ScreenID, options []string, current, group string, onSelect func(string) tea.Msg) (tea.Model, tea.Cmd) {
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: screenID}})
	m.ctrl.EnsureSelectorState(options, current, group)
	selRS := newSelectorRS(onSelect)
	w, h := m.innerSize()
	selRS.width, selRS.height = w, h
	m.pushRS(selRS)
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
	case messages.TargetCosts:
		return runtime.NavigateTargetCosts
	}
	return runtime.NavigateTargetUnknown
}

// navigateTasksToCmd translates TaskRequests from HandleNavigate into a
// Bubble Tea command. Unknown TaskKind values are dropped for forward-
// compatibility with newer runtime builds.
func navigateTasksToCmd(m Model, tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch t.Key.Kind {
		case runtime.KindFetchResources:
			// The runtime stamps the canonical (alias-resolved) ShortName onto
			// req.Key.Scope when it builds the task, so ExecuteTask uses the same
			// value that the former fetchResources(fetchRT, ...) call used —
			// no navigation-result fallback is needed.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.KindFetchProfiles:
			// ErrAdapterOnlyTask — profiles result is a TUI-private type; keep adapter-local.
			cmds = append(cmds, m.fetchProfiles())

		case runtime.KindFetchReveal:
			// Route through ExecuteTask; fall back to adapter if needed.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.KindFetchCosts:
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

// handleCopy performs context-dependent clipboard copy as a tea.Cmd. The
// reveal screen stays TUI-local — the controller has no reveal screen — so
// it is handled directly from the renderer's own rs.revealValue; every other
// kind delegates to the controller's CopyContent, the single source of truth
// shared with the web renderer.
func (m Model) handleCopy() (tea.Model, tea.Cmd) {
	rs := m.activeRS()
	if rs.kind == rsKindReveal {
		if rs.revealValue == "" {
			return m, nil
		}
		return m, copyToClipboard(rs.revealValue, "Secret copied to clipboard")
	}
	content, label := m.ctrl.CopyContent()
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

	// Main menu: restart availability checks (no-op in no-cache mode) via the
	// shared Controller.RestartAvailabilitySweep mutation-list method
	// (core/app/actions_list.go — see its doc comment for the exact bundle),
	// the single source shared with the headless/web menu-refresh branch
	// (handleActionRefresh). A nil return means --no-cache mode. The returned
	// TaskKindLoadAvailCache task is discarded: the TUI dispatches its own
	// loadAvailabilityCache tea.Cmd instead, reaching the same executor logic
	// through its own Cmd rather than a drained TaskRequest.
	if rs.kind == rsKindMenu {
		if m.ctrl.RestartAvailabilitySweep() == nil {
			return m, nil
		}
		m.flash = flashState{text: "Refreshing availability...", isError: false, active: true}
		cmd := m.loadAvailabilityCache()
		return m, cmd
	}

	// Detail view: begin a fresh DetailOperation and re-dispatch enrich +
	// related, routed through ctrl.Apply(ActionRefresh) exactly like the
	// rsKindCosts branch below routes costs refresh — core/app/actions_list.go's
	// handleActionRefresh owns the RelatedRows reset, the RelatedCache
	// invalidation, and the operation itself; this branch keeps only the
	// renderer-only right-column widget reset, the SES cache swap, and the
	// flash.
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
		_, tasks := m.ctrl.Apply(app.Action{Kind: app.ActionRefresh})
		if len(tasks) == 0 {
			return m, nil
		}
		return m, m.dispatchTaskRequests(tasks)
	}

	if rs.kind == rsKindCosts {
		// Controller.handleActionRefresh's own topCostsState branch owns the
		// force-refetch-open-period logic now (FR-012) — routed generically
		// through Apply so this transport carries no costs-specific logic.
		_, tasks := m.ctrl.Apply(app.Action{Kind: app.ActionRefresh})
		if len(tasks) == 0 {
			return m, nil
		}
		cmds := make([]tea.Cmd, len(tasks))
		for i, t := range tasks {
			cmds[i] = m.executeTaskCmd(t)
		}
		return m, tea.Batch(cmds...)
	}

	if rs.kind != rsKindList {
		return m, nil
	}
	rt := rs.resourceType
	parentCtx := m.ctrl.GetListParentContext()
	escPops := m.ctrl.GetListEscPops()

	// Pre-fetch cleanup: strip stale Wave 2 findings from the session-owned
	// mirrors (ResourceCache[rt]/LazyResourceCache/ProbeResources) BEFORE
	// deleting the cache entry, so a later cache-open (before this rerun's
	// fresh EnrichmentChecked lands) never reseeds from a stale wave2 row.
	// Deleting the cache entry afterwards is still correct (forces a fresh
	// fetch).
	//
	// Deliberately NOT clearing the controller's own rendered rows
	// (ls.Rows / c.resourceCache via ClearRowFindings) here: that used to
	// blank every Wave-2 glyph on screen for the full AWS round-trip between
	// this Update() and the rerun's EnrichmentChecked arrival — a real,
	// user-visible flicker, not just a stale-state risk. Wave-2 state is
	// stale-until-replaced (never blank-until-replaced): ApplyWave2ToRow
	// already strips-then-conditionally-reappends per resource ID against the
	// FULL fresh findings map when the rerun's result lands, so any row
	// missing from that map is correctly cleared at that point — pre-clearing
	// here only widened the visible gap without changing the eventual state.
	if parentCtx == nil && !escPops {
		(&m).applyEnrichment(rt)
	}

	m.core.DeleteResourceCache(rt) // clear cache for refreshed type only
	if rt == "ses" {
		// Swap (see detail-view path above): protects against in-flight blocked
		// DescribeActiveReceiptRuleSet fetchers re-poisoning the cache.
		m.core.ResetRuleSets()
	}
	m.flash = flashState{text: "Refreshing...", isError: false, active: true}

	// Top-level list with a registered enricher: bump per-type gen (via the
	// shared Core.RefreshListEnrichment, the single source also used by the
	// headless/web list-refresh branch in core/app/actions_list.go) and
	// dispatch a wrapped fetch that stamps TypeGen onto the outgoing
	// ResourcesLoadedMsg so the tail branch in app.go can seed probeResources
	// and dispatch probeEnrichment on success. A 0 token means rt has no
	// registered issue enricher — dispatch the normal, unstamped refetch.
	if parentCtx == nil && !escPops {
		if tok := m.core.RefreshListEnrichment(rt); tok != 0 {
			cmd := m.refreshActiveListWithEnrichmentRerun(tok)
			return m, cmd
		}
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
func (m Model) refreshActiveListWithEnrichmentRerun(tok domain.Gen) tea.Cmd {
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
// IdentityError is stamped with AspectConnect; the shim performs
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
		_, tasks := m.ctrl.BeginDetailWorkload(rt, srcRes, false, false)
		if len(tasks) == 0 {
			return m, nil
		}
		return m, m.dispatchTaskRequests(tasks)
	}
	rs.rightCol.SetFocused(false)
	m.ctrl.SetDetailRelatedVisible(false, true)
	return m, nil
}

// clipboardWrite is this package's write to the OS clipboard. Tests replace
// it via SetClipboardWriteForTest and assert on what was captured, because the
// OS clipboard is one shared resource across every parallel worktree and
// reading it back makes the assertion depend on whoever wrote last.
var clipboardWrite = clipboard.WriteAll

// SetClipboardWriteForTest replaces clipboardWrite and returns a function that
// restores the previous one.
func SetClipboardWriteForTest(fn func(string) error) func() {
	prev := clipboardWrite
	clipboardWrite = fn
	return func() { clipboardWrite = prev }
}

// copyToClipboard returns a tea.Cmd that writes content to the system
// clipboard and emits a FlashMsg with the success label or error text.
func copyToClipboard(content, successLabel string) tea.Cmd {
	return func() tea.Msg {
		err := clipboardWrite(content)
		if err != nil {
			return messages.Flash{Text: fmt.Sprintf("Copy failed: %v", err), IsError: true}
		}
		return messages.Flash{Text: successLabel, IsError: false}
	}
}
