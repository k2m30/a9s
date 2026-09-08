// SPDX-License-Identifier: GPL-3.0-or-later

// runtime_adapter.go is the Bubble Tea adapter glue for the platform-
// agnostic runtime.Core. It owns:
//
//  1. applyIntent — the per-intent applier used by the 6 ported
//     handlers (HandleFlash / HandleClearFlash / HandleAPIError /
//     HandleClientsReady / HandleProfileSelected / HandleRegionSelected
//     adapters in app_flash.go and app_session.go) AND any future
//     handler that wires through this file. It mutates the *Model in
//     place (flash state, showErrorHint, …), forwards to the controller
//     where the controller is the source of truth (FlashIntent),
//     and returns a single tea.Cmd for intents that need follow-up work,
//     such as RefreshActiveListIntent.
//
//  2. runtimeTasksToCmd / enrichDetailCmd — the TaskRequest-to-tea.Cmd
//     translator. Tasks carry typed Payload values (runtime.TaskPayload
//     variants); the adapter type-switches on Payload to recover all
//     fields without parsing TaskKey.Scope or accepting side-channel
//     arguments. enrichDetailCmd stays in the adapter because it returns
//     tea.Cmd and wraps a 10 s per-call timeout Core.ExecuteTask's
//     KindEnrichDetail path does not apply.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// applyIntent applies a single runtime UIIntent to the adapter-owned
// Model state. Returns a tea.Cmd when the intent triggers follow-up
// work (e.g. RefreshActiveListIntent), nil otherwise.
//
// FlashIntent's renderer half is applied DIRECTLY here (set flashState
// text/isError/active) rather than being dispatched back as messages.Flash the
// way app.go's multi-intent applyIntents path does. The per-handler adapters in
// app_flash.go / app_session.go pre-bump m.flash.gen before invoking the
// Core, so by the time we get here the gen is already in sync with the
// FlashTickPayload the Core returned.
//
// PushScreen / PopScreen / PopSelectorIntent forward to m.ctrl.ApplyIntents
// first, then apply only the rendererState half — the same controller-first
// ordering app_dispatch.go's applyIntents (plural) uses. Before this both
// paths independently re-derived the controller-side gate (rs.kind checks
// mirroring the controller's Screen.ID checks) instead of asking the
// controller directly, which is the divergence risk goal 4 closes: the
// controller's screen stack is the single source of truth for depth/identity,
// and the renderer stack must be a strict mirror (see StackInSync in
// app_stack_invariant.go).
//
// FlashIntent also forwards to m.ctrl.ApplyIntents (single-intent slice, not
// the caller's whole batch) so the controller records the session error-log
// entry — see that case for why a single-intent forward is safe here where a
// blanket forward of the whole intents slice would not be.
//
// Unknown intent types are silently dropped for forward compatibility.
func (m *Model) applyIntent(intent runtime.UIIntent) tea.Cmd {
	switch v := intent.(type) {
	case runtime.FlashIntent:
		// Controller first: it is the single source of truth for the session
		// error log, and it makes that entry when it applies an error flash.
		// The two bulk-forward sites (app_dispatch.go's applyIntents,
		// runtime_adapter_resources.go's dispatchDetailOpResultIntents)
		// therefore withhold FlashIntent from their forward and re-emit it
		// through handleFlash, which lands back here — one application per
		// flash, one entry.
		m.ctrl.ApplyIntents([]runtime.UIIntent{v})
		if v.LogOnly {
			break
		}
		m.flash.text = v.Text
		m.flash.isError = v.IsError
		m.flash.active = true
	case runtime.ClearFlash:
		m.flash.active = false
	case runtime.SetErrorHintIntent:
		m.showErrorHint = v.Show
	case runtime.ClearActiveListLoadingIntent:
		if m.activeRS().kind == rsKindList {
			m.ctrl.ClearListLoading()
			// Per cache contract C4: mirror the headless applyIntents case (intents.go) —
			// a fetch failure over cached content must set the list's error
			// marker too, or the TUI never renders it (renderer parity).
			m.ctrl.SetListFetchError(v.Err)
		}
	case runtime.MenuClearAvailabilityIntent:
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
	case runtime.ClearIdentityIntent:
		m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.ClearIdentityIntent{}})
		// The controller forward above clears ctrl.identityResult, but the
		// identity overlay's DISPLAYED state lives on the renderer's own
		// rendererState (identityData/identityLoading/identityErr — see
		// newIdentityRS's doc comment), populated by the 'i' key handler and
		// by SetIdentityIntent's loop over m.stack (app_dispatch.go). Without
		// this, a rotation that reveals an already-pushed identity rs (e.g.
		// selector-on-top popped by PopSelectorIntent) would keep showing the
		// previous pair's ARN. Reset every identity rs to a truthful state;
		// loading=true is only truthful when a post-connect refetch will
		// actually follow — HandleClientsReady's no-cache branch skips
		// TaskKindFetchIdentity entirely (core/runtime/handlers.go, "Identity
		// fetch is skipped in this mode (synthetic creds)"), so no-cache
		// rotation zeroes the data and leaves loading=false (a blank identity
		// screen; pressing i again re-fetches) instead of spinning forever.
		willRefetch := !m.core.NoCache()
		for _, s := range m.stack {
			if s.kind == rsKindIdentity {
				s.identityData = views.IdentityData{}
				s.identityErr = ""
				s.identityLoading = willRefetch
			}
		}
	case runtime.PopSelectorIntent:
		// Controller-first (goal 4): forward so the controller applies its own
		// type-checked gate (pop only when top.ID is a selector screen —
		// core/app/intents.go) BEFORE the renderer decides whether to drop
		// its own rendererState. Previously this case gated on rs.kind==
		// rsKindSelector alone and popped via popRS() (which re-derives its own
		// ActionBack-driven controller pop) — two independently-maintained
		// conditionals that only agreed because the stacks were assumed already
		// in sync. Forwarding first makes the controller state authoritative;
		// popRSOnly then removes only the renderer half so ActionBack is not
		// invoked a second time.
		m.ctrl.ApplyIntents([]runtime.UIIntent{v})
		if m.activeRS().kind == rsKindSelector {
			m.popRSOnly()
		}
	case runtime.RefreshActiveListIntent:
		if m.activeRS().kind == rsKindList {
			return m.refreshActiveList()
		}
	case runtime.PushScreen:
		// Controller-first (goal 4): forward the intent so the controller's
		// Screen{ID, Ctx} push (core/app/intents.go) lands before the
		// renderer constructs its rendererState half. pushScreen only calls
		// m.pushRS — it never touches m.ctrl — so this cannot double-push.
		m.ctrl.ApplyIntents([]runtime.UIIntent{v})
		return m.pushScreen(v)
	case runtime.PopScreen:
		// Controller-first (goal 4), mirroring app_dispatch.go's applyIntents
		// PushScreen/PopScreen handling: forward to the controller, then
		// popRSOnly to remove only the renderer half — popRS() would re-derive
		// an ActionBack call and pop the controller stack a second time.
		m.ctrl.ApplyIntents([]runtime.UIIntent{v})
		m.popRSOnly()
	case runtime.ApplyThemeIntent:
		return m.applyTheme(v)
	}
	return nil
}

// runtimeTasksToCmd translates a slice of runtime.TaskRequest values
// into a single Bubble Tea command. Each task carries a typed Payload
// (a runtime.TaskPayload variant) whose concrete type tells the adapter
// which closure builder to use. Unknown payload types are dropped for
// forward-compat with newer runtime builds.
func (m Model) runtimeTasksToCmd(tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch p := t.Payload.(type) {
		case runtime.EnrichDetailPayload:
			// Keep adapter-local: the adapter wraps a 10 s per-call timeout that
			// Core.ExecuteTask's KindEnrichDetail path does not apply.
			cmds = append(cmds, m.enrichDetailCmd(p))

		case runtime.ConnectPayload:
			// ExecuteTask handles TaskKindConnect.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.FetchIdentityPayload:
			// ExecuteTask handles TaskKindFetchIdentity.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.LoadAvailCachePayload:
			// ExecuteTask handles TaskKindLoadAvailCache.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.DemoPrefetchCountsPayload:
			// ExecuteTask handles TaskKindDemoPrefetchCounts.
			cmds = append(cmds, m.executeTaskCmd(t))

		case runtime.FlashTickPayload:
			// ErrAdapterOnlyTask — timer is a renderer concern; keep adapter-local.
			cmds = append(cmds, flashTickCmd(p))

		case runtime.EmitNavigatePayload:
			// ErrAdapterOnlyTask — navigation directive; keep adapter-local.
			cmds = append(cmds, emitNavigateCmd(p))

		case runtime.EmitAPIErrorPayload:
			// ErrAdapterOnlyTask — re-dispatches into the render loop; keep adapter-local.
			cmds = append(cmds, emitAPIErrorCmd(p))

		case runtime.FetchChildResourcesPayload:
			// ExecuteTask handles TaskKindFetchChildResources.
			if cmd := m.executeTaskCmd(t); cmd != nil {
				cmds = append(cmds, cmd)
			}

		case runtime.ReadThemePayload:
			// ErrAdapterOnlyTask — theme file read produces a TUI-private message; keep adapter-local.
			cmds = append(cmds, readThemeFileCmd(p))

		case runtime.SaveThemeConfigPayload:
			// ErrAdapterOnlyTask — persists a theme choice with no data event; keep adapter-local.
			cmds = append(cmds, saveThemeConfigCmd(p))
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

// enrichDetailCmd builds the Bubble Tea command that runs the on-demand
// detail enricher and emits an EnrichDetailResultMsg. It reads every
// runtime-side input (DetailCtx, the operation) from the typed payload —
// DetailEnrichmentCtx construction lives on Core; the only adapter-owned
// input here is m.appCtx (the app-wide cancellation context), wrapped in a
// 10 s per-call timeout the runtime cannot express because tea.Cmd
// composition happens here.
func (m Model) enrichDetailCmd(p runtime.EnrichDetailPayload) tea.Cmd {
	enricher := resource.GetDetailEnricher(p.Op.ResourceType)
	appCtx := m.appCtx
	dctx := p.DetailCtx
	op := p.Op
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()
		enriched, err := enricher(ctx, dctx, op.Resource)
		return messages.EnrichDetailResult{
			ResourceType: op.ResourceType,
			ResourceID:   op.Resource.ID,
			EnrichedRes:  enriched,
			Err:          err,
			OperationID:  op.ID,
		}
	}
}

// flashTickCmd schedules the auto-clear ClearFlashMsg dispatch using the
// gen and duration carried by FlashTickPayload. The Core method that
// emitted this payload already echoed back the gen the adapter pre-bumped
// before invocation; equality on receipt rejects ticks superseded by a
// newer flash.
func flashTickCmd(p runtime.FlashTickPayload) tea.Cmd {
	gen, dur := p.Gen, p.Duration
	return tea.Tick(dur, func(_ time.Time) tea.Msg {
		return messages.ClearFlash{Gen: gen}
	})
}

// emitNavigateCmd dispatches the one-shot NavigateMsg carried by
// EmitNavigatePayload. Used by HandleClientsReady for the -c CLI flag's
// initial-list navigation on first successful connect. Translates the
// runtime-owned NavigateTarget to the adapter-owned messages.ViewTarget.
func emitNavigateCmd(p runtime.EmitNavigatePayload) tea.Cmd {
	var target messages.ViewTarget
	switch p.Target {
	case runtime.NavigateTargetResourceList:
		target = messages.TargetResourceList
	case runtime.NavigateTargetCosts:
		target = messages.TargetCosts
	default:
		target = messages.TargetMainMenu
	}
	rt := p.ResourceType
	return func() tea.Msg {
		return messages.Navigate{Target: target, ResourceType: rt}
	}
}

// emitAPIErrorCmd dispatches the APIErrorMsg carried by EmitAPIErrorPayload.
// Used by HandleClientsReady's "wrong concrete type on Clients" branch to
// route the error through HandleAPIError's classification flow.
func emitAPIErrorCmd(p runtime.EmitAPIErrorPayload) tea.Cmd {
	err := p.Err
	gen := p.Gen
	return func() tea.Msg {
		return messages.APIError{Err: err, Gen: gen}
	}
}

// dispatchHandlerResult is the shared adapter-side glue used by every
// ≤12-line handler in app_flash.go / app_session.go: it applies the
// returned UIIntents (in order, via applyIntent) and translates the
// returned TaskRequests into tea.Cmd values, batching everything into a
// single tea.Cmd.
func (m *Model) dispatchHandlerResult(intents []runtime.UIIntent, tasks []runtime.TaskRequest) tea.Cmd {
	var cmds []tea.Cmd
	for _, in := range intents {
		if c := m.applyIntent(in); c != nil {
			cmds = append(cmds, c)
		}
	}
	if tc := m.runtimeTasksToCmd(tasks); tc != nil {
		cmds = append(cmds, tc)
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
