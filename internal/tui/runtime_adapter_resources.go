// runtime_adapter_resources.go — Bubble Tea adapter glue for runtime.Core's
// resource-flow Handle* methods ported in PR-05a-h4-b (AS-962).
//
// Three thin (~10-20 LOC) shims:
//
//	handleResourcesLoaded     — wave-1 derive on msg.Resources, route through
//	                            ctrl.HandleResourcesLoadedEvent (replaces the
//	                            old updateActiveView path), then delegate
//	                            cross-view cache write + rerun probe to Core.
//	handleRelatedCheckResult  — resolve sourceID fallback from the active
//	                            detail controller state, delegate cache writes
//	                            + error-flash surface to Core, then update the
//	                            detail's related rows via ctrl.ApplyDetailRelatedResult.
//	handleEnrichDetailResult  — stale-gen drop, delegate flash-on-error to
//	                            Core, then on success apply the enriched resource
//	                            via ctrl.ApplyDetailEnrichmentForResource.
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
//  1. Route the message through ctrl.HandleResourcesLoadedEvent so the
//     controller list state absorbs Resources and pagination — replacing the
//     old updateActiveView path where RL.Update wrote back via
//     cacheTopLevelResourceList.
//  2. Re-apply any active checker against the freshly-loaded page (for
//     related-navigation lists with approximate ID sets).
//  3. Delegate the cross-view cache write (Branch 2 in the original body),
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
				td := resource.FindResourceType(shortName)
				if td == nil {
					td = resource.GetChildType(shortName)
				}
				// Check for an enter-keyed child view on this resource type.
				if td != nil {
					for i := range td.Children {
						cv := &td.Children[i]
						if cv.Key != "enter" {
							continue
						}
						if cv.DrillCondition != nil && !cv.DrillCondition(r) {
							break
						}
						ctx := make(map[string]string)
						for k, v := range cv.ContextKeys {
							ctx[k] = r.Fields[v]
						}
						displayName := ctx[cv.DisplayNameKey]
						childType := cv.ChildType
						return m, tea.Batch(coreCmd, func() tea.Msg {
							return messages.EnterChildView{
								ChildType:     childType,
								ParentContext: ctx,
								DisplayName:   displayName,
							}
						})
					}
				}
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

// handleRelatedCheckResult is the adapter shim for
// messages.RelatedCheckResult. The stale-gen check is performed up-front
// (the orchestrator's GenStamped gate covers HandleEvent-routed messages
// but this message is dispatched through the shim, not HandleEvent).
//
// Source ID resolution: messages.SourceResourceID is authoritative when
// set; otherwise the shim falls back to the active detail controller
// state's resource ID (replacing the old m.activeView().(*views.DetailModel)
// fallback).
func (m Model) handleRelatedCheckResult(msg messages.RelatedCheckResult) (tea.Model, tea.Cmd) {
	if messages.IsStale(msg, m.core) {
		return m, nil
	}
	sourceID := msg.SourceResourceID
	if sourceID == "" && m.activeRS().kind == rsKindDetail {
		sourceID = m.ctrl.GetDetailResource().ID
	}
	// W1.4b.3: fetcher-emitted rows already carry Findings; no re-derive needed
	// on cached pages or lazy-added resources before Core writes them to cache.
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
	// Update the controller's detail related rows with this result.
	errMsg := ""
	if msg.Result.Err != nil {
		errMsg = msg.Result.Err.Error()
	}
	m.ctrl.ApplyDetailRelatedResultForResource(
		msg.ResourceType,
		sourceID,
		msg.DefDisplayName,
		msg.Result.TargetType,
		msg.Result.Count,
		false,
		errMsg,
		msg.Result.Approximate,
		msg.Result.ResourceIDs,
		msg.Result.FetchFilter,
	)
	// Keep the active detail's right-column widget rows in sync so keyboard Enter
	// has the resourceIDs to navigate: the lift renders related counts from the
	// controller, but Enter still reads rs.rightCol. Sync only when the active
	// detail IS this result's source — rightColumnModel.Update matches rows by
	// TargetType alone, so feeding another detail's widget would mispopulate it.
	if rs := m.activeRS(); rs != nil && rs.kind == rsKindDetail && m.ctrl.GetDetailResource().ID == sourceID {
		rs.rightCol, _ = rs.rightCol.Update(msg)
	}
	return m, coreCmd
}

// handleEnrichDetailResult is the adapter shim for
// messages.EnrichDetailResult. The stale-gen check stays adapter-side
// for the same reason as handleRelatedCheckResult.
//
// The Err branch returns early so the success-path detail update never
// fires on a half-populated EnrichedRes — matches the original case-body's
// early-return on err.
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
	// Apply the enriched resource to the controller's detail state. This
	// replaces the old updateActiveView path where DetailModel.Update absorbed
	// the enriched resource and rebuilt its field list.
	ef, ad := findingFromResource(msg.EnrichedRes)
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
			}
		}
	}
	return m, coreCmd
}


