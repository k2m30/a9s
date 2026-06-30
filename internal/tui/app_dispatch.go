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
// Menu and enrichment state mutations land through m.ctrl.ApplyIntents so
// the controller remains the single source of truth. No concrete view
// model instances are stored — the stack holds only *rendererState.
func (m *Model) applyIntents(intents []runtime.UIIntent) []tea.Cmd {
	var cmds []tea.Cmd
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.PatchMenuAvailability,
			runtime.PatchMenuIssueBatch,
			runtime.PatchMenuCheckProgress,
			runtime.PatchMenuEnrichProgress,
			runtime.PatchMenu,
			runtime.PatchResourceList,
			runtime.MenuClearAvailabilityIntent:
			// All menu + list enrichment patches route directly to the controller
			// which owns the MenuState / ListState. No stored view model exists.
			m.ctrl.ApplyIntents([]runtime.UIIntent{intent})

		case runtime.PatchDetail:
			// Apply enrichment findings to every stacked detail screen of this
			// resource type — not just the currently active one. When a user has
			// navigated from detail-A to detail-B, enrichment results for both
			// must reach both screens, so popping back to detail-A shows the
			// correct Attention section immediately.
			if len(v.EnrichmentFindings) == 0 {
				// Nil or empty Findings means all resources of this type have
				// recovered: clear enrichment from every stacked detail screen.
				m.ctrl.ClearDetailFindingsForType(v.ResourceType)
			} else {
				// Apply each per-resource finding independently. The controller's
				// ApplyDetailFindingForResource searches all stacked screens by
				// (type, id), so a resource that is stacked but not active is
				// still updated. Resources NOT in the map have recovered: clear
				// their findings via a nil-finding call.
				for resourceID, f := range v.EnrichmentFindings {
					finding := f
					var ad *domain.AttentionDetail
					if got, hasAD := v.EnrichmentAttentionDetails[resourceID]; hasAD && len(got.Rows) > 0 {
						adVal := got
						ad = &adVal
					}
					m.ctrl.ApplyDetailFindingForResource(v.ResourceType, resourceID, &finding, ad)
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
			case runtime.ScreenReveal:
				// Push the reveal screen onto the controller so that ActionBack on
				// Esc pops it (not the list below). Without this, the ctrl stack
				// would be one screen behind the TUI rs stack, and Esc from reveal
				// would incorrectly pop the secrets list out of the controller.
				m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenReveal, Payload: v.Payload}})
			}
			if c := m.pushScreen(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopScreen:
			m.popRS()
		case runtime.ApplyThemeIntent:
			if c := m.applyTheme(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopSelectorIntent:
			if m.activeRS().kind == rsKindSelector {
				m.popRS()
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
			// Identity overlay state lives in the rs (not ctrl-backed). Find it
			// and update directly. If no identity rs is on the stack, noop.
			if v.Identity == nil {
				continue
			}
			for _, rs := range m.stack {
				if rs.kind == rsKindIdentity {
					rs.identityLoading = false
					rs.identityErr = ""
					rs.identityData = views.IdentityData{
						AccountID:     v.Identity.AccountID,
						AccountAlias:  v.Identity.AccountAlias,
						ARN:           v.Identity.Arn,
						RoleName:      v.Identity.RoleName,
						UserName:      v.Identity.UserName,
						SessionName:   v.Identity.SessionName,
						IsAssumedRole: v.Identity.IsAssumedRole,
					}
					break
				}
			}
		case runtime.HeaderInvalidateIntent:
			m.headerCacheKey = ""
		}
	}
	return cmds
}

// pushScreen resolves the runtime-emitted PushScreen via the screens
// builder map and pushes the resulting rendererState onto the stack. The live
// *Model is passed to the builder at invocation so it observes the
// current keymap / viewConfig / innerSize() rather than a stale
// snapshot captured at tui.New time. A missing builder (unknown
// ScreenID) surfaces as a flash so the operator sees the misconfig;
// a builder that returns a nil rs is silently dropped (the builder
// itself owns the "I refuse to render this payload" decision).
func (m *Model) pushScreen(v runtime.PushScreen) tea.Cmd {
	builder, ok := m.screens[v.ID]
	if !ok {
		id := string(v.ID)
		return func() tea.Msg {
			return messages.Flash{Text: "no screen builder: " + id, IsError: true}
		}
	}
	rs, cmd := builder(m, v.Payload)
	if rs != nil {
		m.pushRS(rs)
	}
	return cmd
}

// applyTheme parses the YAML bytes carried by ApplyThemeIntent and, on
// success, swaps the active theme, invalidates the header cache, and sets
// m.activeTheme so the next theme selector renders the "(current)"
// indicator correctly.
//
// Post-AS-784: Core's HandleThemeFileRead pre-validates the YAML via
// styles.ThemeFromYAML before emitting ApplyThemeIntent + Save task. The
// adapter parse-error branch below is therefore defensive — under normal
// flow the bytes are guaranteed to parse.
//
// With the renderer-state stack, there are no stored ResourceListModel
// instances whose style caches need invalidating — transient models are
// created fresh per render frame, so no cache walk is needed.
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
			// saveAvailabilityCache reads counts from the controller state,
			// giving authoritative values from MenuState.
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

// executeTaskCmd wraps Core.ExecuteTaskAt in a tea.Cmd. The returned event is
// delivered back into the Update loop as a tea.Msg. Adapter-only tasks fall
// back to nil (they must be handled by the caller's kind-specific branch).
//
// The session snapshot is captured SYNCHRONOUSLY here — before the goroutine
// runs — so a concurrent session.Rotate (profile/region switch) cannot cause
// the obsolete task to read the new gen/clients and wrongly pass messages.IsStale.
func (m Model) executeTaskCmd(req runtime.TaskRequest) tea.Cmd {
	ctx := m.appCtx
	// Capture the session snapshot at dispatch time (synchronous, on the Update
	// goroutine) so a profile/region switch before this cmd executes cannot
	// restamp the obsolete task with the new generation or clients.
	snap := m.core.CaptureDispatch()
	return func() tea.Msg {
		ev, err := m.core.ExecuteTaskAt(ctx, req, snap)
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
