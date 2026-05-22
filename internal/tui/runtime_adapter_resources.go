// runtime_adapter_resources.go — Bubble Tea adapter glue for runtime.Core's
// resource-flow Handle* methods ported in PR-05a-h4-b (AS-962).
//
// Three thin (~10-20 LOC) shims:
//
//	handleResourcesLoaded     — wave-1 derive on msg.Resources, route through
//	                            updateActiveView (RL.Update writes back via
//	                            cacheTopLevelResourceList), then delegate
//	                            cross-view cache write + rerun probe to Core.
//	handleRelatedCheckResult  — resolve sourceID fallback from the active
//	                            detail view, derive findings on CachedPages /
//	                            LazyAddedResources slices, delegate cache
//	                            writes + error-flash surface to Core, then
//	                            route the message through updateActiveView
//	                            so the detail view's right-column model
//	                            absorbs the result.
//	handleEnrichDetailResult  — stale-gen drop, delegate flash-on-error to
//	                            Core, then on success derive wave-1 on the
//	                            enriched resource and route to updateActiveView.
//
// Each shim's case body in app.go.Update() shrinks to a one-line dispatch
// (`return m.handle*(msg)`), bringing the three affected switch cases
// under the ≤6-line acceptance grep per spec.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleResourcesLoaded is the adapter shim for messages.ResourcesLoaded.
// Order is preserved 1:1 with the original case body:
//
//  1. Site 1 derive on msg.Resources so the view receives enriched rows.
//  2. Route the message through updateActiveView — when the active view
//     is a ResourceListModel for this type, RL.Update absorbs Resources
//     and the cacheTopLevelResourceList write-through writes a rich entry.
//  3. Re-apply any active checker against the freshly-loaded page (for
//     related-navigation lists with approximate ID sets).
//  4. Delegate the cross-view cache write (Branch 2 in the original body),
//     the partial-success flash, and the enrichment-rerun probe dispatch
//     to Core.HandleResourcesLoaded.
func (m Model) handleResourcesLoaded(msg messages.ResourcesLoaded) (tea.Model, tea.Cmd) {
	// Stale-gen drop: AS-657 stamps ResourcesLoaded against AspectAvailability.
	// The shim performs the check up-front because Core.HandleResourcesLoaded is
	// invoked directly (not via HandleEvent's central GenStamped gate) and the
	// pre-Core view-side derive + updateActiveView would otherwise mutate state
	// from a previous profile/region rotation.
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	(&m).deriveFindingsForType(msg.ResourceType, msg.Resources)
	updated, viewCmd := m.updateActiveView(msg)
	if updatedModel, ok := updated.(Model); ok {
		if rl, ok := updatedModel.activeView().(*views.ResourceListModel); ok {
			rl.ReapplyCheckerAgainst(msg.Resources)
		}
		m = updatedModel
	}
	intents, tasks := m.core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: msg.ResourceType,
		Resources:    msg.Resources,
		Pagination:   msg.Pagination,
		Append:       msg.Append,
		TypeGen:      msg.TypeGen,
		Err:          msg.Err,
	})
	coreCmd := m.dispatchCoreScreenResult(intents, tasks)
	return m, batchCmds(viewCmd, coreCmd)
}

// handleRelatedCheckResult is the adapter shim for
// messages.RelatedCheckResult. The stale-gen check is performed up-front
// (the orchestrator's GenStamped gate covers HandleEvent-routed messages
// but this message is dispatched through the shim, not HandleEvent).
//
// Source ID resolution: messages.SourceResourceID is authoritative when
// set; otherwise the shim falls back to the active detail view's
// source-resource ID (the original case-body fallback). Core receives
// the resolved value as authoritative.
func (m Model) handleRelatedCheckResult(msg messages.RelatedCheckResult) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	sourceID := msg.SourceResourceID
	if sourceID == "" {
		if d, ok := m.activeView().(*views.DetailModel); ok {
			sourceID = d.SourceResource().ID
		}
	}
	// Derive wave-1 findings on slices BEFORE Core writes them to cache.
	// Adapter owns this because deriveFindingsForType is the Model-side
	// helper used at every adapter entry point; Core has its own
	// equivalent that runs at related-navigate cache-hit time.
	for aliasName, entry := range msg.CachedPages {
		shortName := aliasName
		if td := resource.FindResourceType(aliasName); td != nil {
			shortName = td.ShortName
		}
		(&m).deriveFindingsForType(shortName, entry.Resources)
	}
	for aliasName, extra := range msg.LazyAddedResources {
		if len(extra) == 0 {
			continue
		}
		shortName := aliasName
		if td := resource.FindResourceType(aliasName); td != nil {
			shortName = td.ShortName
		}
		(&m).deriveFindingsForType(shortName, extra)
	}
	intents, tasks := m.core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       msg.ResourceType,
		SourceResourceID:   sourceID,
		DefDisplayName:     msg.DefDisplayName,
		Result:             msg.Result,
		CachedPages:        msg.CachedPages,
		LazyAddedResources: msg.LazyAddedResources,
		LazyAddError:       msg.LazyAddError,
	})
	coreCmd := m.dispatchCoreScreenResult(intents, tasks)
	updated, viewCmd := m.updateActiveView(msg)
	if updatedModel, ok := updated.(Model); ok {
		m = updatedModel
	}
	return m, batchCmds(coreCmd, viewCmd)
}

// handleEnrichDetailResult is the adapter shim for
// messages.EnrichDetailResult. The stale-gen check stays adapter-side
// for the same reason as handleRelatedCheckResult.
//
// The Err branch returns early so the success-path derive +
// updateActiveView never sees a half-populated EnrichedRes — matches the
// original case-body's early-return on err.
func (m Model) handleEnrichDetailResult(msg messages.EnrichDetailResult) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	intents, tasks := m.core.HandleEnrichDetailResult(runtime.EnrichDetailResultEvent{
		ResourceType: msg.ResourceType,
		Err:          msg.Err,
	})
	coreCmd := m.dispatchCoreScreenResult(intents, tasks)
	if msg.Err != nil {
		return m, coreCmd
	}
	(&m).deriveFindingsForResource(msg.ResourceType, &msg.EnrichedRes)
	updated, viewCmd := m.updateActiveView(msg)
	if updatedModel, ok := updated.(Model); ok {
		m = updatedModel
	}
	return m, batchCmds(coreCmd, viewCmd)
}

// batchCmds collapses up to two optional tea.Cmds into a single Cmd
// without allocating a Batch for the trivial nil / single-cmd cases.
// The h4-b shims combine viewCmd + coreCmd in different orders, so this
// helper sits next to them rather than inline.
func batchCmds(a, b tea.Cmd) tea.Cmd {
	switch {
	case a == nil && b == nil:
		return nil
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return tea.Batch(a, b)
	}
}
