// SPDX-License-Identifier: GPL-3.0-or-later

// runtime_adapter_resources.go — Bubble Tea adapter glue for runtime.Core's
// resource-flow handlers.
//
//	handleResourcesLoaded     — wave-1 derive on msg.Resources, route through
//	                            ctrl.HandleResourcesLoadedEvent (replaces the
//	                            old updateActiveView path), then delegate
//	                            cross-view cache write + rerun probe to Core.
//	handleEnrichDetailResult  — calls Core.HandleEnrichDetailResult directly
//	                            (not Controller.Handle) so the returned
//	                            FlashIntent applies synchronously via
//	                            dispatchDetailOpResultIntents — Controller.Handle
//	                            only returns tasks to its TUI caller, discarding
//	                            the ViewState/Flash a synchronous single-Update
//	                            render needs — then applies the enriched
//	                            resource to detail state via the exported
//	                            ctrl.ApplyDetailEnrichmentForResource (the same
//	                            merge the web/headless lane's
//	                            foldEnrichDetailResultLocked calls), and
//	                            regenerates syntax-colored YAML/JSON content
//	                            when a text screen for this resource is open —
//	                            a TUI-only rendering concern with no Controller
//	                            equivalent.
//	handleRelatedCheckResult  — the same direct-Core-call +
//	                            dispatchDetailOpResultIntents + exported-merge
//	                            pattern as handleEnrichDetailResult, calling
//	                            Core.HandleRelatedCheckResult and
//	                            ctrl.ApplyDetailRelatedResultForResource.
//	dispatchDetailOpResultIntents — the shared intent-applier for the two
//	                            shims above: forwards the whole intents slice to
//	                            ctrl.ApplyIntents (the Patch{Resource,Related,
//	                            Lazy}Cache session writes either Core method can
//	                            emit — dispatchHandlerResult's applyIntent has
//	                            no case for these, and would silently drop
//	                            them), then direct-mutates m.flash for any
//	                            FlashIntent so it renders within this single
//	                            Update() call.
package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleResourcesLoaded is the adapter shim for messages.ResourcesLoaded.
// Order is preserved 1:1 with the original case body:
//
//  1. Route the message through ctrl.HandleResourcesLoadedEvent so the
//     controller list state absorbs Resources and pagination — replacing the
//     old updateActiveView path where RL.Update wrote back via
//     cacheTopLevelResourceList.
//  2. Re-apply any active checker against the freshly-loaded page (for
//     related-navigation lists with truncated ID sets).
//  3. Delegate the cross-view cache write (Branch 2 in the original body),
//     the partial-success flash, and the enrichment-rerun probe dispatch
//     to Core.HandleResourcesLoaded.
func (m Model) handleResourcesLoaded(msg messages.ResourcesLoaded) (tea.Model, tea.Cmd) {
	// Stale-gen drop: ResourcesLoaded is stamped against AspectAvailability.
	// The shim performs the check up-front because Core.HandleResourcesLoaded is
	// invoked directly (not via HandleEvent's central GenStamped gate) and the
	// pre-Core view-side derive + updateActiveView would otherwise mutate state
	// from a previous profile/region rotation.
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	// Update the controller's list state with the loaded resources.
	m.ctrl.HandleResourcesLoadedEvent(msg)
	// Re-apply the checker if the active list has one (related-navigation lists).
	rs := m.activeRS()
	if rs.kind == rsKindList {
		m.ctrl.ApplyReapplyCheckerAgainst(msg.Resources)
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

	// Auto-open-single-detail: when the active list was created with
	// autoOpenSingleDetail (e.g. related navigation from a detail field), and
	// exactly one row is visible after loading, navigate directly to detail —
	// replacing the list. This mirrors the logic previously in
	// ResourceListModel.Update(ResourcesLoadedMsg).
	if rs.kind == rsKindList && m.ctrl.GetListAutoOpenSingle() {
		snap := m.ctrl.Snapshot()
		ls := snap.Body.List
		if ls != nil && len(ls.Rows) == 1 {
			r, ok := m.ctrl.ListSelected()
			if ok {
				m.ctrl.ClearListAutoOpenSingle()
				shortName := rs.resourceType
				rCopy := r
				listType := shortName
				return m, tea.Batch(coreCmd, func() tea.Msg {
					return messages.Navigate{
						Target:         messages.TargetDetail,
						ResourceType:   listType,
						Resource:       &rCopy,
						ReplaceCurrent: true,
					}
				})
			}
		}
		// Zero rows, paginated, single target ID → load more.
		if ls != nil && len(ls.Rows) == 0 && ls.Truncated && !ls.LoadingMore {
			if targetID, ok := m.ctrl.GetListExactRelatedTargetID(); ok {
				_ = targetID
				m.ctrl.SetListLoadingMore(true)
				shortName := rs.resourceType
				token := m.ctrl.GetListPaginationCursor()
				pc := m.ctrl.GetListParentContext()
				return m, tea.Batch(coreCmd, func() tea.Msg {
					return messages.LoadMore{
						ResourceType:      shortName,
						ContinuationToken: token,
						ParentContext:     pc,
					}
				})
			}
		}
		// Zero rows, StubCreator available → synthesise stub.
		if ls != nil && len(ls.Rows) == 0 {
			shortName := rs.resourceType
			td := resource.FindResourceType(shortName)
			if td == nil {
				td = resource.GetChildType(shortName)
			}
			if td != nil && td.StubCreator != nil {
				if targetID, ok := m.ctrl.GetListExactRelatedTargetID(); ok {
					m.ctrl.ClearListAutoOpenSingle()
					stub := td.StubCreator(targetID)
					listType := shortName
					return m, tea.Batch(coreCmd, func() tea.Msg {
						return messages.Navigate{
							Target:         messages.TargetDetail,
							ResourceType:   listType,
							Resource:       &stub,
							ReplaceCurrent: true,
						}
					})
				}
			}
		}
	}

	return m, coreCmd
}

// handleEnrichDetailResult is the adapter shim for messages.EnrichDetailResult.
// Calls Core.HandleEnrichDetailResult directly (bypassing Controller.Handle,
// whose TUI-facing signature returns only tasks) so the FlashIntent on
// enrichment failure applies synchronously via dispatchDetailOpResultIntents —
// within this single Update() call, with no cmd round-trip. On success,
// applies the enriched resource to detail state via
// ctrl.ApplyDetailEnrichmentForResource (the same merge
// Controller.foldEnrichDetailResultLocked calls for the web/headless lane),
// then regenerates syntax-colored YAML/JSON content when the active screen is
// a text viewer for this resource: a TUI-only rendering concern with no
// Controller equivalent.
//
// The Err branch returns early so the detail-state merge and the
// syntax-color regeneration never fire on a half-populated EnrichedRes.
func (m Model) handleEnrichDetailResult(msg messages.EnrichDetailResult) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	intents, tasks := m.core.HandleEnrichDetailResult(runtime.EnrichDetailResultEvent{
		ResourceType: msg.ResourceType,
		Err:          msg.Err,
	})
	m.dispatchDetailOpResultIntents(intents)
	coreCmd := m.dispatchTaskRequests(tasks)
	if msg.Err != nil {
		return m, coreCmd
	}

	ef, ad := primaryWave2Finding(msg.EnrichedRes)
	m.ctrl.ApplyDetailEnrichmentForResource(msg.ResourceType, msg.ResourceID, msg.EnrichedRes, ef, ad)

	// When the active screen is a YAML or JSON text viewer for this resource,
	// regenerate the syntax-colored content lines from the enriched resource
	// and push them into the controller's TextState. This mirrors the old
	// YAMLModel.Update/JSONModel.View enrichment path.
	if m.activeRS().kind == rsKindText {
		screenID, ctx := m.ctrl.GetTextScreenContext()
		if screenID != "" && ctx.ResourceType == msg.ResourceType && ctx.ResourceID == msg.ResourceID {
			w, h := m.innerSize()
			var newLines []string
			switch screenID {
			case runtime.ScreenYAML:
				rt := resource.FindResourceType(msg.ResourceType)
				if rt == nil {
					rt = resource.GetChildType(msg.ResourceType)
				}
				if rt != nil {
					y := views.NewYAMLWithCtrl(msg.EnrichedRes, msg.ResourceType, m.keys, m.ctrl)
					y.SetSize(w, h)
					newLines = y.ContentLines()
				}
			case runtime.ScreenJSON:
				rt := resource.FindResourceType(msg.ResourceType)
				if rt == nil {
					rt = resource.GetChildType(msg.ResourceType)
				}
				if rt != nil {
					j := views.NewJSONWithCtrl(msg.EnrichedRes, msg.ResourceType, m.keys, m.ctrl)
					j.SetSize(w, h)
					newLines = j.ContentLines()
				}
			}
			if len(newLines) > 0 {
				m.ctrl.UpdateTextLines(newLines)
				m.ctrl.SetTextResource(msg.EnrichedRes)
			}
		}
	}
	return m, coreCmd
}

// handleRelatedCheckResult is the adapter shim for messages.RelatedCheckResult.
// Calls Core.HandleRelatedCheckResult directly (bypassing Controller.Handle,
// whose TUI-facing signature returns only tasks) so its PatchRelatedCache /
// PatchResourceCache / PatchLazyResourceCache session writes and any
// LazyAddError/checker-error FlashIntent apply via dispatchDetailOpResultIntents
// within this single Update() call, with no cmd round-trip. Then merges the
// row into every matching stacked detail's RelatedRows via
// ctrl.ApplyDetailRelatedResultForResource, the same exported method
// Controller.foldRelatedCheckResultLocked calls for the web/headless lane and
// the TUI's own related-navigation fetch-by-ID path
// (runtime_adapter_related.go) already calls, so the merge itself has one
// implementation regardless of which caller reaches it.
func (m Model) handleRelatedCheckResult(msg messages.RelatedCheckResult) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	intents, tasks := m.core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       msg.ResourceType,
		SourceResourceID:   msg.SourceResourceID,
		DefDisplayName:     msg.DefDisplayName,
		Result:             msg.Result,
		CachedPages:        msg.CachedPages,
		LazyAddedResources: msg.LazyAddedResources,
		LazyAddError:       msg.LazyAddError,
	})
	m.dispatchDetailOpResultIntents(intents)
	cmd := m.dispatchTaskRequests(tasks)

	errMsg := ""
	if msg.Result.Err != nil {
		errMsg = msg.Result.Err.Error()
	}
	m.ctrl.ApplyDetailRelatedResultForResource(
		msg.ResourceType,
		msg.SourceResourceID,
		msg.DefDisplayName,
		msg.Result.TargetType,
		msg.Result.EffectiveState(),
		msg.Result.Count,
		false,
		errMsg,
		msg.Result.Truncated,
		msg.Result.ResourceIDs,
		msg.Result.FetchFilter,
	)
	return m, cmd
}

// dispatchDetailOpResultIntents applies the intents returned by
// Core.HandleEnrichDetailResult / Core.HandleRelatedCheckResult. Forwards the
// whole slice to ctrl.ApplyIntents first — the PatchResourceCache /
// PatchRelatedCache / PatchLazyResourceCache session writes either Core
// method can emit (dispatchHandlerResult's applyIntent has no case for these
// three and would silently drop them, since its only callers today — the 6
// ported handlers in app_flash.go/app_session.go — never emit them) — then
// direct-mutates m.flash for any FlashIntent present so it renders within the
// same Update() call, with no cmd round-trip. Neither Core method ever
// returns a FlashTickPayload task alongside its FlashIntent (verified against
// both bodies), so there is no auto-clear tick to schedule here, matching the
// Controller.Handle web-lane fold these two callers bypass.
func (m *Model) dispatchDetailOpResultIntents(intents []runtime.UIIntent) {
	m.ctrl.ApplyIntents(intents)
	for _, in := range intents {
		if fi, ok := in.(runtime.FlashIntent); ok {
			m.flash.text = fi.Text
			m.flash.isError = fi.IsError
			m.flash.active = true
		}
	}
}
