// app_dispatch.go — TUI-side runtime intent + task dispatchers (applyIntents,
// pushScreen, applyTheme, tasksToCmd, dispatchTaskRequests, coreUpdate).
//
// applyIntents forwards every intent to the headless controller
// (m.ctrl.ApplyIntents) first — see internal/app/intents.go for the
// controller-side cases, including the cache-cross-write intents
// (PatchResourceCache, PatchRelatedCache, PatchLazyResourceCache) and
// PatchDetail (detail-view enrichment), which write session/screen-stack
// state owned by Core/Controller via the typed accessors in
// internal/runtime/accessors.go. The local switch in applyIntents below only
// covers renderer-side effects with no controller equivalent, or the
// rendererState half of an intent already applied controller-side by the
// forward.
package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// applyIntents walks the []runtime.UIIntent slice returned by
// m.core.HandleEvent (or any direct Handle* method call) and applies
// each intent to the TUI view tree + session state. Returns any
// follow-up tea.Cmds the intents themselves require (flash re-emit,
// screen-builder closures, theme-apply errors).
//
// The ENTIRE slice is forwarded to m.ctrl.ApplyIntents in one call, first —
// the controller (internal/app/intents.go) is the single source of truth for
// every intent it knows about (menu/list/enrichment patches, stack ops,
// identity, flash, error-log, and the cache-cross-write intents
// PatchResourceCache/PatchRelatedCache/PatchLazyResourceCache, which write
// through the same *runtime.Core the TUI holds as m.core). Intents the
// controller does not model (HeaderInvalidateIntent, ApplyThemeIntent) are
// documented no-ops there; RefreshActiveListIntent is handled separately via
// refreshTasksForIntents at the Handle call site.
//
// The local switch below runs AFTER the forward and handles ONLY
// renderer-side effects that have no controller equivalent, or the
// rendererState half of an intent whose controller half the forward pass
// already applied. See each case comment for which half is being applied
// here to avoid double-application.
func (m *Model) applyIntents(intents []runtime.UIIntent) []tea.Cmd {
	m.ctrl.ApplyIntents(intents)

	var cmds []tea.Cmd
	for _, intent := range intents {
		switch v := intent.(type) {
		case runtime.FlashIntent:
			// Re-emit as messages.Flash so the flash routes through
			// HandleFlash and picks up the auto-clear tick + history
			// entry. The controller half (c.flash, used by the web renderer's
			// snapshot) was already set by the forward above. The h3
			// direct-mutate path is in runtime_adapter.go's applyIntent
			// (singular) used by dispatchHandlerResult only.
			text, isErr := v.Text, v.IsError
			cmds = append(cmds, func() tea.Msg {
				return messages.Flash{Text: text, IsError: isErr}
			})
		case runtime.ClearFlash:
			// Controller half (c.flash = Flash{}) already applied by the forward.
			m.flash.active = false
		case runtime.PushScreen:
			// Controller half (the Screen{ID, Ctx} push) already applied by the
			// forward above. Only the rendererState half + the per-screen-kind
			// "ensure initial state" seeding remain local:
			//
			// ScreenChildList/ScreenResourceList: the forward's generic PushScreen
			// case sets State.List = nil; EnsureListState seeds it (mirrors what
			// PushChildListScreen used to do inline) so topListState() inside
			// NewChildResourceList resolves to a fresh, initialised ListState
			// rather than nil.
			// ScreenProfileSelector: the builder in screens.go already calls
			// m.ctrl.EnsureSelectorState after the forward's push, so no extra
			// seeding is needed here.
			// ScreenReveal: the reveal payload is carried entirely by the rs
			// (newRevealRS), not controller SelectorState — no seeding needed.
			switch v.ID {
			case runtime.ScreenChildList, runtime.ScreenResourceList:
				m.ctrl.EnsureListState()
			}
			if c := m.pushScreen(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopScreen:
			// Controller half (c.stack pop) already applied by the forward above.
			// Only the rendererState half remains — popRSOnly must NOT re-invoke
			// ActionBack (that would pop the controller stack a second time).
			m.popRSOnly()
		case runtime.ApplyThemeIntent:
			if c := m.applyTheme(v); c != nil {
				cmds = append(cmds, c)
			}
		case runtime.PopSelectorIntent:
			// Controller half (popping a selector screen off c.stack) already
			// applied by the forward above. Only the rendererState half remains.
			if m.activeRS().kind == rsKindSelector {
				m.popRSOnly()
			}
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
// Core's HandleThemeFileRead pre-validates the YAML via
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
// availability/enrich probe + save-cache tasks that the singular
// dispatcher does not route.
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
			// Must route through the shared executor (not an adapter-local
			// save) so req.Payload's *SaveCachePayload reaches the
			// executor's row/finding persistence — an adapter-local save
			// would silently drop per-type rows, keeping only counts.
			cmd := m.executeTaskCmd(req)
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

		case runtime.TaskKindEmitNavigate:
			// ErrAdapterOnlyTask — navigation directive; keep adapter-local.
			// DEF-14/D11: handleAvailabilityCacheLoaded emits this once its
			// ProbeResources seed has landed, so this must reach the same
			// emitNavigateCmd translator runtimeTasksToCmd uses for the
			// NoCache path's direct emission.
			if p, ok := req.Payload.(runtime.EmitNavigatePayload); ok {
				cmds = append(cmds, emitNavigateCmd(p))
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

// dispatchTaskRequests is the single shared task->tea.Cmd translation switch
// every screen's adapter routes through, so a task kind's translation rule
// lives in exactly one place — a future kind added here works from every
// caller, not just the one that happened to need it first. Consolidates the
// former per-screen switches in runtime_adapter_related.go
// (relatedNavigateTasksToCmd) and app_costs.go (handleCostsKeyMsg), which
// independently reimplemented the same KindFetchByIDDetail navigation
// special case and generic executeTaskCmd passthrough.
//
// KindFetchFiltered is deliberately absent: HandleRelatedNavigate never
// attaches a payload to that task — the filter clause travels out-of-band on
// its own NavigationResult, not on the TaskRequest — so it cannot be
// resolved from tasks alone. relatedNavigateTasksToCmd, the only producer of
// that kind, resolves it locally before delegating the rest here.
func (m Model) dispatchTaskRequests(tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch t.Key.Kind {
		case runtime.KindFetchByIDDetail:
			// ExecuteTask returns ResourcesLoaded for this kind, but the
			// caller must also navigate to the detail view — an
			// adapter-only side effect ExecuteTask cannot perform, so this
			// stays a dedicated case rather than falling through to
			// executeTaskCmd.
			payload, ok := t.Payload.(runtime.FetchByIDDetailPayload)
			if !ok {
				continue
			}
			cmds = append(cmds, m.fetchByIDDetail(payload.TargetType, payload.ID))

		case runtime.KindFetchMore:
			// FetchMorePayload is set by the runtime — ExecuteTask can
			// execute it. A missing or wrong-typed payload is a runtime
			// bug; drop the task.
			if _, ok := t.Payload.(runtime.FetchMorePayload); !ok {
				continue
			}
			cmds = append(cmds, m.executeTaskCmd(t))

		default:
			// Every other kind (KindFetchResources, KindFetchCosts, and any
			// future addition with no adapter-only side effect) is a plain
			// ExecuteTask passthrough.
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

// coreUpdate dispatches a tea.Msg through m.core.HandleEvent, applies
// the returned UIIntents to the view tree, and converts TaskRequests
// to tea.Cmds. Used by the Update() switch for messages routed
// entirely through the orchestrator (availability/enrichment events,
// identity loaded/error).
func (m Model) coreUpdate(msg messages.Event) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleEvent(msg)
	cmds := m.applyIntents(intents)
	if tc := m.tasksToCmd(tasks); tc != nil {
		cmds = append(cmds, tc)
	}
	return m, tea.Batch(cmds...)
}
