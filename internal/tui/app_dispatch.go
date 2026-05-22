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
// session-state owned by Core via the handle returned by
// m.core.Session() (no separate sync needed — the session is the same
// instance, not a copy). PR-05a-h4-c (AS-963) routed the related-cache
// key + result type through the internal/runtime package so the
// renderer-side dispatcher no longer imports the internal/session
// package.
package tui

import (
	"maps"

	tea "charm.land/bubbletea/v2"

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
				// nil EnrichmentFindings = clear; non-nil = update from map.
				if v.EnrichmentFindings == nil {
					d.SetEnrichmentFinding(nil)
				} else if f, exists := v.EnrichmentFindings[d.ResourceID()]; exists {
					d.SetEnrichmentFinding(&f)
				} else {
					d.SetEnrichmentFinding(nil)
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
			m.core.Session().ResourceCache[v.ResourceType] = v.Entry
		case runtime.PatchRelatedCache:
			// PR-05a-h4-b: kept as an applyIntents case for forward-
			// compatibility, even though Core today writes the
			// RelatedCache directly inside HandleRelatedCheckResult
			// (the renderer-agnostic alternative path is unused in
			// production but keeps the intent set complete for tests
			// and future emitters that may surface this branch).
			if v.SourceID != "" {
				key := runtime.RelatedCacheKey(v.ResourceType, v.SourceID)
				existing, _ := m.core.Session().RelatedCache.Get(key)
				m.core.Session().RelatedCache.Set(key, append(existing, runtime.RelatedCacheResult{
					DefDisplayName: v.DefDisplayName,
					Result:         v.Result,
				}))
			}
		case runtime.PatchLazyResourceCache:
			maps.Copy(m.core.Session().LazyResourceCache, v.Adds)
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
		case runtime.TaskKindProbeAvailability:
			cmd := m.probeResourceAvailability(req.Key.Scope, m.core.Session().AvailabilityGen)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case runtime.TaskKindProbeEnrich:
			cmd := m.probeEnrichment(req.Key.Scope, m.core.Session().EnrichmentGen)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case runtime.TaskKindSaveCache:
			cmd := m.saveAvailabilityCache()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case runtime.TaskKindFetchChildResources:
			if p, ok := req.Payload.(runtime.FetchChildResourcesPayload); ok {
				if cmd := m.fetchChildResources(p.ChildType, p.ParentContext); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		case runtime.TaskKindReadThemeFile:
			if p, ok := req.Payload.(runtime.ReadThemePayload); ok {
				cmds = append(cmds, readThemeFileCmd(p))
			}
		case runtime.TaskKindSaveThemeConfig:
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

// coreUpdate dispatches a tea.Msg through m.core.HandleEvent, applies
// the returned UIIntents to the view tree, and converts TaskRequests
// to tea.Cmds. Used by the Update() switch for messages routed
// entirely through the orchestrator (availability/enrichment events,
// identity loaded/error post-h4-b).
func (m Model) coreUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleEvent(msg)
	cmds := m.applyIntents(intents)
	if tc := m.tasksToCmd(tasks); tc != nil {
		cmds = append(cmds, tc)
	}
	return m, tea.Batch(cmds...)
}
