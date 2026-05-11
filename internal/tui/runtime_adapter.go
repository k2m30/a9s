// runtime_adapter.go is the Bubble Tea adapter glue for the platform-
// agnostic runtime.Core (Phase 05 PR-05a-extract). It owns:
//
//  1. handleEnrichDetail — a Model-receiver wrapper that replaces the
//     deleted internal/tui/app_enrich.go entry point. It constructs a
//     transient runtime.Core bound to the Model's embedded Session,
//     calls core.HandleEnrichDetail, and translates the returned
//     TaskRequests into tea.Cmd values. The existing app.go dispatch
//     line (return m.handleEnrichDetail(msg)) is unchanged.
//
//  2. applyIntent — the stack walker that translates runtime.UIIntent
//     into renderer-specific view mutations. Today's intent set is empty
//     for the only migrated handler; the switch exists so per-handler
//     PRs can add cases without touching app.go.
//
//  3. runtimeTasksToCmd / enrichDetailCmd — the TaskRequest-to-tea.Cmd
//     translator. Tasks carry typed Payload values (runtime.TaskPayload
//     variants); the adapter type-switches on Payload to recover all
//     fields without parsing TaskKey.Scope or accepting side-channel
//     arguments. The closure builder stays in the adapter because it
//     returns tea.Cmd and reads adapter-owned state (m.appCtx,
//     m.Session.Clients, the embedded session's EnrichGen and PolicyDocCache)
//     that has not yet migrated to the runtime core.
package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// handleEnrichDetail replaces the entry point previously in
// internal/tui/app_enrich.go. The signature is identical — (tea.Model,
// tea.Cmd) — so the existing app.go dispatch line is unchanged.
//
// It constructs a transient runtime.Core to invoke the migrated policy
// (HandleEnrichDetail), applies any returned UIIntents to the view stack,
// then converts the returned TaskRequests into Bubble Tea commands.
func (m Model) handleEnrichDetail(msg messages.EnrichDetailMsg) (tea.Model, tea.Cmd) {
	core := runtime.New(m.Session, resource.AllResourceTypes())
	intents, tasks := core.HandleEnrichDetail(runtime.EnrichDetailEvent{
		ResourceType: msg.ResourceType,
		Resource:     msg.Resource,
	})
	for _, in := range intents {
		m.applyIntent(in)
	}
	return m, m.runtimeTasksToCmd(tasks)
}

// applyIntent walks the view stack applying a runtime UIIntent.
// Per-handler PRs add cases as their corresponding handler migrates.
// Unknown intent types are silently dropped for forward compatibility.
func (m *Model) applyIntent(_ runtime.UIIntent) {
	// No intent variants are emitted by migrated handlers yet; the
	// switch is a placeholder that PR-05a-h1..h8 successors extend.
}

// runtimeTasksToCmd translates a slice of runtime.TaskRequest values
// into a single Bubble Tea command. Each task carries a typed Payload
// (a runtime.TaskPayload variant) whose concrete type tells the adapter
// which closure builder to use. Unknown payload types are dropped for
// forward-compat with newer runtime builds.
//
// Per-handler PRs (probes, related, fetchers, …) add their own
// type-switch cases here as they migrate. The signature accepts only
// the runtime's own TaskRequest slice — no side-channel parameters.
func (m Model) runtimeTasksToCmd(tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch p := t.Payload.(type) {
		case runtime.EnrichDetailPayload:
			cmds = append(cmds, m.enrichDetailCmd(p))
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
// The runtime guarantees that an enricher is registered for
// p.ResourceType (it would not have emitted this TaskRequest otherwise).
// The adapter does not re-check this invariant. If somehow violated, the
// nil-deref panic on enricher(...) is the correct surfacing — silent
// no-op would mask a runtime/registry bug.
//
// EnrichGen is captured from the embedded Session at dispatch time to
// preserve stale-result-rejection semantics: the result handler in
// app.go compares msg.Generation against m.Session.EnrichGen on receipt.
// PolicyDocCache and clients are adapter-owned state that has not yet
// migrated to the runtime core.
func (m Model) enrichDetailCmd(p runtime.EnrichDetailPayload) tea.Cmd {
	enricher := resource.GetDetailEnricher(p.ResourceType)
	gen := m.Session.EnrichGen             // session-owned, promoted via embedded *Session
	policyDocs := m.Session.PolicyDocCache // session-owned, promoted via embedded *Session
	clients := m.Session.Clients
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
		return messages.EnrichDetailResultMsg{
			ResourceType: resourceType,
			ResourceID:   res.ID,
			EnrichedRes:  enriched,
			Err:          err,
			Generation:   gen,
		}
	}
}
