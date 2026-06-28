// app_dispatch.go — TUI-side runtime intent + task dispatchers.
//
// Split out of app.go in PR-05a-h4-b (AS-962) so the runtime↔adapter
// glue (applyIntents, pushScreen, applyTheme, tasksToCmd, coreUpdate)
// lives next to runtime_adapter.go's per-intent helper (applyIntent),
// and so app.go stays under the 700 LOC budget set by the spec
// acceptance check (`wc -l internal/tui/app.go`).
//
// The five new h4-b intents — PatchResourceCache, PatchRelatedCache,
// PatchLazyResourceCache, SetIdentityIntent, HeaderInvalidateIntent —
// each land as a case in applyIntents below. They cross-write
// session-state owned by Core via the typed accessors in
// internal/runtime/accessors.go (SetResourceCache, RelatedCacheSet,
// ExtendLazyResourceCache). PR-05a-h4-c (AS-963) routed the related-cache
// key + result type through the internal/runtime package and migrated
// every session-field access in the renderer onto those typed accessors,
// so the dispatcher no longer reaches into the session struct shape.
package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// applyIntents walks the []runtime.UIIntent slice returned by
// m.core.HandleEvent (or any direct Handle* method call) and applies
// each intent to the TUI view tree + session state. Returns any
// follow-up tea.Cmds the intents themselves require (flash re-emit,
// screen-builder closures, theme-apply errors).
//
// Session-state mutations land through m.core's typed accessors
// (SetResourceCache, RelatedCacheSet, ExtendLazyResourceCache) so the
// dispatcher never reaches through Core into the session struct.
func (m *Model) applyIntents(intents []runtime.UIIntent) []tea.Cmd {
	var cmds []tea.Cmd
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PatchMenuAvailability:
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				menu.SetAvailability(v.ResourceType, v.Count)
				menu.SetTruncated(v.ResourceType, v.Truncated)
			}
		case runtime.PatchMenuIssueBatch:
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				if len(v.Known) > 0 {
					menu.SetIssuesFromCache(v.Counts, v.Truncated, v.Known)
				}
			}
		case runtime.PatchMenuCheckProgress:
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				menu.SetCheckProgress(v.Checked, v.Total)
			}
		case runtime.PatchMenuEnrichProgress:
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				menu.SetEnrichProgress(v.Checked, v.Total)
			}
		case runtime.PatchMenu:
			if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
				menu.SetIssues(v.ResourceType, v.Issues, v.Truncated)
			}
		case runtime.PatchResourceList:
			for _, sv := range m.stack {
				rl, ok := sv.(*views.ResourceListModel)
				if !ok || rl.ResourceType() != v.ResourceType {
					continue
				}
				if v.Issues != nil && v.Enrichment != nil {
					rl.SetEnrichmentState(v.Issues.Count, v.Issues.Truncated, v.Enrichment.Findings)
					rl.SetTruncatedIDs(v.Enrichment.TruncatedIDs)
				} else if v.Issues != nil {
					rl.SetEnrichmentState(v.Issues.Count, v.Issues.Truncated, nil)
				}
				if v.Enrichment != nil && len(v.Enrichment.FieldUpdates) > 0 {
					rl.ApplyFieldUpdates(v.Enrichment.FieldUpdates)
				}
			}
		case runtime.PatchDetail:
			for _, sv := range m.stack {
				d, ok := sv.(*views.DetailModel)
				if !ok || d.ResourceType() != v.ResourceType {
					continue
				}
				if v.ResourceID != "" && d.ResourceID() != v.ResourceID {
					continue
				}
				// AS-1395: runtime ships domain.Finding + domain.AttentionDetail
				// keyed by Resource.ID. nil EnrichmentFindings = clear; non-nil =
				// update from map.
				//
				// Controller-backed models: route to ctrl.ApplyDetailFinding so the
				// controller remains the single source of truth for findings.
				// Legacy models (ctrl == nil): use SetEnrichmentFinding as before.
				if d.IsControllerBacked() {
					// Route by resource ID so a STACKED detail (not the active one)
					// receives its finding — ApplyDetailFinding targets only the top
					// screen, which drops findings for details below the active one.
					if v.EnrichmentFindings == nil {
						m.ctrl.ApplyDetailFindingForResource(d.ResourceType(), d.ResourceID(), nil, nil)
					} else if f, exists := v.EnrichmentFindings[d.ResourceID()]; exists {
						finding := f
						var ad *domain.AttentionDetail
						if got, hasAD := v.EnrichmentAttentionDetails[d.ResourceID()]; hasAD && len(got.Rows) > 0 {
							adVal := got
							ad = &adVal
						}
						m.ctrl.ApplyDetailFindingForResource(d.ResourceType(), d.ResourceID(), &finding, ad)
					} else {
						m.ctrl.ApplyDetailFindingForResource(d.ResourceType(), d.ResourceID(), nil, nil)
					}
				} else {
					if v.EnrichmentFindings == nil {
						d.SetEnrichmentFinding(nil, nil)
					} else if f, exists := v.EnrichmentFindings[d.ResourceID()]; exists {
						finding := f
						var ad *domain.AttentionDetail
						if got, hasAD := v.EnrichmentAttentionDetails[d.ResourceID()]; hasAD && len(got.Rows) > 0 {
							adVal := got
							ad = &adVal
						}
						d.SetEnrichmentFinding(&finding, ad)
					} else {
						d.SetEnrichmentFinding(nil, nil)
					}
				}
			}
		case runtime.FlashIntent:
			// Re-emit as messages.Flash so the flash routes through
			// HandleFlash and picks up the auto-clear tick + history
			// entry. The h3 direct-mutate path is in runtime_adapter.go's
			// applyIntent (singular) used by dispatchHandlerResult only.
			text, isErr := v.Text, v.IsError
			cmds = append(cmds, func() tea.Msg {
				return messages.Flash{Text: text, IsError: isErr}
			})
		case runtime.ClearFlash:
			m.flash.active = false
		case runtime.PushScreen:
			// Keep m.ctrl stack in sync before the builder runs so that
			// topListState() inside NewChildResourceList resolves to this
			// screen's own fresh ListState rather than whatever list was
			// previously on top.
			//
			// ScreenChildList: ChildType is in ChildListPayload (Context is
			// zero-valued for EnterChildView-emitted pushes).
			// ScreenResourceList: ResourceType is in Context.ResourceType.
			switch v.ID {
			case runtime.ScreenChildList:
				resourceType := v.Context.ResourceType
				if resourceType == "" {
					if clp, ok := v.Payload.(runtime.ChildListPayload); ok {
						resourceType = clp.ChildType
					}
				}
				m.ctrl.PushChildListScreen(resourceType)
			case runtime.ScreenResourceList:
				m.ctrl.PushChildListScreen(v.Context.ResourceType)
			case runtime.ScreenProfileSelector:
				// Push the selector screen onto the controller stack BEFORE the
				// builder runs, so EnsureSelectorState (called by the builder) finds
				// the screen already on top of the controller stack.
				m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenProfileSelector}})
			}
			if c := m.pushScreen(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopScreen:
			m.popView()
		case runtime.ApplyThemeIntent:
			if c := m.applyTheme(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopSelectorIntent:
			if _, ok := m.activeView().(*views.SelectorModel); ok {
				m.popView()
			}
		case runtime.PatchResourceCache:
			m.core.SetResourceCache(v.ResourceType, v.Entry)
		case runtime.PatchRelatedCache:
			// PR-05a-h4-b: kept as an applyIntents case for forward-
			// compatibility, even though Core today writes the
			// RelatedCache directly inside HandleRelatedCheckResult
			// (the renderer-agnostic alternative path is unused in
			// production but keeps the intent set complete for tests
			// and future emitters that may surface this branch).
			if v.SourceID != "" {
				key := runtime.RelatedCacheKey(v.ResourceType, v.SourceID)
				existing, _ := m.core.RelatedCacheGet(key)
				m.core.RelatedCacheSet(key, append(existing, runtime.RelatedCacheResult{
					DefDisplayName: v.DefDisplayName,
					Result:         v.Result,
				}))
			}
		case runtime.PatchLazyResourceCache:
			m.core.ExtendLazyResourceCache(v.Adds)
		case runtime.SetIdentityIntent:
			if v.Identity == nil {
				continue
			}
			if idView, ok := m.activeView().(*views.IdentityModel); ok {
				idView.SetIdentity(views.IdentityData{
					AccountID:     v.Identity.AccountID,
					AccountAlias:  v.Identity.AccountAlias,
					ARN:           v.Identity.Arn,
					RoleName:      v.Identity.RoleName,
					UserName:      v.Identity.UserName,
					SessionName:   v.Identity.SessionName,
					IsAssumedRole: v.Identity.IsAssumedRole,
				})
			}
		case runtime.HeaderInvalidateIntent:
			m.headerCacheKey = ""
		}
	}
	return cmds
}

// pushScreen resolves the runtime-emitted PushScreen via the screens
// builder map and pushes the resulting view onto the stack. The live
// *Model is passed to the builder at invocation so it observes the
// current keymap / viewConfig / innerSize() rather than a stale
// snapshot captured at tui.New time. A missing builder (unknown
// ScreenID) surfaces as a flash so the operator sees the misconfig;
// a builder that returns a nil view is silently dropped (the builder
// itself owns the "I refuse to render this payload" decision).
func (m *Model) pushScreen(v runtime.PushScreen) tea.Cmd {
	builder, ok := m.screens[v.ID]
	if !ok {
		id := string(v.ID)
		return func() tea.Msg {
			return messages.Flash{Text: "no screen builder: " + id, IsError: true}
		}
	}
	view, cmd := builder(m, v.Payload)
	if view != nil {
		m.pushView(view)
	}
	return cmd
}

// applyTheme parses the YAML bytes carried by ApplyThemeIntent and, on
// success, swaps the active theme, invalidates the header cache, walks
// the view stack invalidating ResourceListModel style caches, and sets
// m.activeTheme so the next theme selector renders the "(current)"
// indicator correctly.
//
// Post-AS-784: Core's HandleThemeFileRead pre-validates the YAML via
// styles.ThemeFromYAML before emitting ApplyThemeIntent + Save task. The
// adapter parse-error branch below is therefore defensive — under normal
// flow the bytes are guaranteed to parse.
func (m *Model) applyTheme(v runtime.ApplyThemeIntent) tea.Cmd {
	t, err := styles.ThemeFromYAML(v.Bytes)
	if err != nil {
		text := "Bad theme YAML: " + err.Error()
		return func() tea.Msg {
			return messages.Flash{Text: text, IsError: true}
		}
	}
	styles.ApplyTheme(t)
	m.activeTheme = v.Name
	m.headerCacheKey = ""
	for _, sv := range m.stack {
		if rl, ok := sv.(*views.ResourceListModel); ok {
			rl.InvalidateStyleCache()
		}
	}
	return nil
}

// tasksToCmd converts a []runtime.TaskRequest returned by m.core into a
// single tea.Cmd (or nil when the slice is empty). The TaskKind switch
// matches the symmetric runtimeTasksToCmd in runtime_adapter.go used
// by handleEnrichDetail; this dispatcher additionally covers the
// availability/enrich probe + save-cache tasks emitted by the h3 +
// h4-a handlers, which the singular dispatcher does not route.
func (m *Model) tasksToCmd(tasks []runtime.TaskRequest) tea.Cmd {
	var cmds []tea.Cmd
	for _, req := range tasks {
		switch req.Key.Kind {
		case runtime.TaskKindProbeAvailability, runtime.TaskKindProbeEnrich, runtime.TaskKindFetchChildResources:
			// Route through ExecuteTask; fall back to adapter handling for
			// ErrAdapterOnlyTask (defensive — these kinds are not adapter-only).
			cmd := m.executeTaskCmd(req)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case runtime.TaskKindSaveCache:
			// saveAvailabilityCache reads counts from MainMenuModel (the live
			// TUI view), giving more precise values than ExecuteTask's
			// availabilityFromResourceCache path. Keep adapter-local.
			cmd := m.saveAvailabilityCache()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case runtime.TaskKindReadThemeFile:
			// ErrAdapterOnlyTask — renderer concern, keep adapter-local.
			if p, ok := req.Payload.(runtime.ReadThemePayload); ok {
				cmds = append(cmds, readThemeFileCmd(p))
			}

		case runtime.TaskKindSaveThemeConfig:
			// ErrAdapterOnlyTask — renderer concern, keep adapter-local.
			if p, ok := req.Payload.(runtime.SaveThemeConfigPayload); ok {
				cmds = append(cmds, saveThemeConfigCmd(p))
			}
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// executeTaskCmd wraps Core.ExecuteTask in a tea.Cmd. The returned event is
// delivered back into the Update loop as a tea.Msg. Adapter-only tasks fall
// back to nil (they must be handled by the caller's kind-specific branch).
func (m Model) executeTaskCmd(req runtime.TaskRequest) tea.Cmd {
	ctx := m.appCtx
	return func() tea.Msg {
		ev, err := m.core.ExecuteTask(ctx, req)
		if err != nil {
			if errors.Is(err, runtime.ErrAdapterOnlyTask) {
				return nil
			}
			return nil
		}
		return ev
	}
}

// coreUpdate dispatches a tea.Msg through m.core.HandleEvent, applies
// the returned UIIntents to the view tree, and converts TaskRequests
// to tea.Cmds. Used by the Update() switch for messages routed
// entirely through the orchestrator (availability/enrichment events,
// identity loaded/error post-h4-b).
func (m Model) coreUpdate(msg messages.Event) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleEvent(msg)
	cmds := m.applyIntents(intents)
	if tc := m.tasksToCmd(tasks); tc != nil {
		cmds = append(cmds, tc)
	}
	return m, tea.Batch(cmds...)
}
