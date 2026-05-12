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
//     m.core.Session().Clients, the session owned by core's EnrichGen and PolicyDocCache)
//     that has not yet migrated to the runtime core.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleEnrichDetail replaces the entry point previously in
// internal/tui/app_enrich.go. The signature is identical — (tea.Model,
// tea.Cmd) — so the existing app.go dispatch line is unchanged.
//
// It constructs a transient runtime.Core to invoke the migrated policy
// (HandleEnrichDetail), applies any returned UIIntents to the view stack,
// then converts the returned TaskRequests into Bubble Tea commands.
func (m Model) handleEnrichDetail(msg messages.EnrichDetail) (tea.Model, tea.Cmd) {
	core := runtime.New(m.core.Session(), resource.AllResourceTypes())
	intents, tasks := core.HandleEnrichDetail(runtime.EnrichDetailEvent{
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
			cmds = append(cmds, m.enrichDetailCmd(p))
		case runtime.ConnectPayload:
			cmds = append(cmds, m.connectAWS(p.Profile, p.Region, p.Gen))
		case runtime.FetchIdentityPayload:
			cmds = append(cmds, m.fetchIdentity())
		case runtime.LoadAvailCachePayload:
			cmds = append(cmds, m.loadAvailabilityCache())
		case runtime.DemoPrefetchCountsPayload:
			cmds = append(cmds, m.demoPrefetchCounts())
		case runtime.FlashTickPayload:
			cmds = append(cmds, flashTickCmd(p))
		case runtime.EmitNavigatePayload:
			cmds = append(cmds, emitNavigateCmd(p))
		case runtime.EmitAPIErrorPayload:
			cmds = append(cmds, emitAPIErrorCmd(p))
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
// detail enricher and emits an EnrichDetailResultMsg. It reads the
// resource type and resource directly from the typed payload — no
// Scope parsing, no side-channel resource argument.
//
// EnrichGen is captured from the session owned by core at dispatch time to
// preserve stale-result-rejection semantics: the result handler in
// app.go compares msg.Generation against m.core.Session().EnrichGen on receipt.
// PolicyDocCache and clients are adapter-owned state that has not yet
// migrated to the runtime core.
func (m Model) enrichDetailCmd(p runtime.EnrichDetailPayload) tea.Cmd {
	enricher := resource.GetDetailEnricher(p.ResourceType)

	gen := m.core.Session().EnrichGen             // session-owned, promoted via session owned by core
	policyDocs := m.core.Session().PolicyDocCache // session-owned, promoted via session owned by core
	clients := m.core.Session().Clients
	appCtx := m.appCtx
	dctx := &awsclient.DetailEnrichmentCtx{
		Clients:    clients,
		PolicyDocs: policyDocs,
	}
	resourceType := p.ResourceType
	res := p.Resource
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
