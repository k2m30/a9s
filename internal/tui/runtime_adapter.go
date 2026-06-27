// runtime_adapter.go is the Bubble Tea adapter glue for the platform-
// agnostic runtime.Core (Phase 05 PR-05a-extract / -h3). It owns:
//
//  1. handleEnrichDetail — a Model-receiver wrapper that replaces the
//     deleted internal/tui/app_enrich.go entry point. It constructs a
//     transient runtime.Core bound to the Model's session owned by core,
//     calls core.HandleEnrichDetail, and translates the returned
//     TaskRequests into tea.Cmd values. The existing app.go dispatch
//     line (return m.handleEnrichDetail(msg)) is unchanged.
//
//  2. applyIntent — the per-intent applier used by the 6 ported
//     handlers (HandleFlash / HandleClearFlash / HandleAPIError /
//     HandleClientsReady / HandleProfileSelected / HandleRegionSelected
//     adapters in app_flash.go and app_session.go) AND any future
//     handler that wires through this file. It mutates the *Model in
//     place (errorHistory, flash state, showErrorHint, …) and returns
//     a single tea.Cmd for intents that need follow-up work, such as
//     RefreshActiveListIntent.
//
//  3. runtimeTasksToCmd / enrichDetailCmd — the TaskRequest-to-tea.Cmd
//     translator. Tasks carry typed Payload values (runtime.TaskPayload
//     variants); the adapter type-switches on Payload to recover all
//     fields without parsing TaskKey.Scope or accepting side-channel
//     arguments. The closure builder stays in the adapter because it
//     returns tea.Cmd and reads adapter-owned state (m.appCtx,
//     m.core.Clients(), the session owned by core's EnrichGen and PolicyDocCache)
//     that has not yet migrated to the runtime core.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleEnrichDetail replaces the entry point previously in
// internal/tui/app_enrich.go. The signature is identical — (tea.Model,
// tea.Cmd) — so the existing app.go dispatch line is unchanged.
//
// It invokes m.core.HandleEnrichDetail (Core constructs the payload's
// DetailCtx + Generation from session state post-PR-05a-h4-b), applies
// any returned UIIntents to the view stack, then converts the returned
// TaskRequests into Bubble Tea commands. The pre-h4-b transient-Core
// shim is gone — the live Core has access to the same session pointer.
func (m Model) handleEnrichDetail(msg messages.EnrichDetail) (tea.Model, tea.Cmd) {
	intents, tasks := m.core.HandleEnrichDetail(runtime.EnrichDetailEvent{
		ResourceType: msg.ResourceType,
		Resource:     msg.Resource,
	})
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
		return m, nil
	case 1:
		return m, cmds[0]
	default:
		return m, tea.Batch(cmds...)
	}
}

// applyIntent applies a single runtime UIIntent to the adapter-owned
// Model state. Returns a tea.Cmd when the intent triggers follow-up
// work (e.g. RefreshActiveListIntent), nil otherwise.
//
// FlashIntent is applied DIRECTLY here (set flashState text/isError/active)
// rather than being dispatched back as messages.Flash the way app.go's
// multi-intent applyIntents path does. The per-handler adapters in
// app_flash.go / app_session.go pre-bump m.flash.gen before invoking the
// Core, so by the time we get here the gen is already in sync with the
// FlashTickPayload the Core returned.
//
// Unknown intent types are silently dropped for forward compatibility.
func (m *Model) applyIntent(intent runtime.UIIntent) tea.Cmd {
	switch v := intent.(type) {
	case runtime.FlashIntent:
		m.flash.text = v.Text
		m.flash.isError = v.IsError
		m.flash.active = true
	case runtime.ClearFlash:
		m.flash.active = false
	case runtime.SetErrorHintIntent:
		m.showErrorHint = v.Show
	case runtime.AppendErrorHistoryIntent:
		m.errorHistory = append(m.errorHistory, errorEntry{
			time:    v.Time,
			message: v.Message,
		})
	case runtime.ClearActiveListLoadingIntent:
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			rl.ClearLoading()
		}
	case runtime.MenuClearAvailabilityIntent:
		if menu, ok := m.stack[0].(*views.MainMenuModel); ok {
			menu.ClearAvailability()
		}
	case runtime.PopSelectorIntent:
		if _, ok := m.activeView().(*views.SelectorModel); ok {
			m.popView()
		}
	case runtime.RefreshActiveListIntent:
		if rl, ok := m.activeView().(*views.ResourceListModel); ok {
			return m.refreshResourceList(*rl)
		}
	case runtime.PushScreen:
		return m.pushScreen(v)
	case runtime.PopScreen:
		m.popView()
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
// runtime-side input (DetailCtx, Generation) from the typed payload —
// PR-05a-h4-b (AS-962) moved DetailEnrichmentCtx construction onto Core
// so the adapter no longer touches the AWS-side DetailEnrichmentCtx
// directly here. The remaining adapter-owned input is m.appCtx (the
// app-wide cancellation context); ctx still wraps a 10 s per-call
// timeout the runtime cannot express because tea.Cmd composition
// happens here.
func (m Model) enrichDetailCmd(p runtime.EnrichDetailPayload) tea.Cmd {
	enricher := resource.GetDetailEnricher(p.ResourceType)
	appCtx := m.appCtx
	dctx := p.DetailCtx
	gen := p.Generation
	res := p.Resource
	resourceType := p.ResourceType
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(appCtx, 10*time.Second)
		defer cancel()
		enriched, err := enricher(ctx, dctx, res)
		return messages.EnrichDetailResult{
			ResourceType: resourceType,
			ResourceID:   res.ID,
			EnrichedRes:  enriched,
			Err:          err,
			Generation:   gen,
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
	return func() tea.Msg {
		return messages.APIError{Err: err}
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
