// SPDX-License-Identifier: GPL-3.0-or-later

// runtime_adapter_related.go — Bubble Tea adapter glue for runtime.Core's
// HandleRelatedNavigate entry point, plus relatedCheckCmd, the TUI's own
// per-def related-check fan-out.
//
// handleRelatedNavigate replaces the deleted entry point from
// internal/tui/app_handlers_related_navigate.go. It constructs a transient
// runtime.Core, calls core.HandleRelatedNavigate, then applies the navigation
// decision to the view stack and translates TaskRequests into tea.Cmd values.
// The existing app.go dispatch line (return m.handleRelatedNavigate(msg)) is
// unchanged.
//
// handleRelatedNavigateChild stays here as a TUI-only helper because it
// dispatches a messages.EnterChildView — a Bubble Tea message type.
//
// relatedCheckCmd fans out one goroutine per RelatedDef registered for a
// DetailOperation's resource type (capped by runtime.MaxConcurrentProbes),
// each calling runtime.RunRelatedDef — the exact per-def logic
// core/runtime/executor.go's KindRelatedCheck case also calls, so the two
// lanes cannot diverge on timeout, panic recovery, NeedsTargetCache
// prefetch, or lazy-add behavior. Dispatched from dispatchTaskRequests
// (app_dispatch.go)'s KindRelatedCheck case, so every TUI call site that
// returns a KindRelatedCheck TaskRequest (detail/YAML/JSON open, Ctrl+R
// refresh, ActionBack reveal, related-panel resolve-in-place) reaches this
// same fan-out.
//
// Exact-ID drills now route through the runtime's KindFetchByIDDetail task for
// any type with a registered FetchByIDs helper (ami, kms, policy, ebs-snap).
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleRelatedNavigate invokes runtime.Core's HandleRelatedNavigate, then builds
// the view and starts any required fetch based on the returned NavigationResult
// and TaskRequests.
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigate) (tea.Model, tea.Cmd) {
	ev := runtime.RelatedNavigateEvent{
		TargetType:     msg.TargetType,
		SourceResource: msg.SourceResource,
		SourceType:     msg.SourceType,
		TargetID:       msg.TargetID,
		RelatedIDs:     msg.RelatedIDs,
		FetchFilter:    msg.FetchFilter,
		Truncated:      msg.Truncated,
		Checker:        msg.Checker,
	}
	result, tasks := m.core.HandleRelatedNavigate(ev)

	switch result.Kind {
	case runtime.NavigationKindFlash:
		return m, func() tea.Msg {
			return messages.Flash{
				Text:    result.FlashMessage,
				IsError: result.FlashIsError,
			}
		}

	case runtime.NavigationKindEnterChildView:
		return m.handleRelatedNavigateChild(msg)

	case runtime.NavigationKindFilteredList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			// Fetcher-only type (registered paginated fetcher but no ResourceTypeDef).
			if len(result.RelatedIDs) > 0 {
				if lazyRows, hasLazy := m.core.LazyResourceCache(msg.TargetType); hasLazy {
					idSet := make(map[string]bool, len(result.RelatedIDs))
					for _, id := range result.RelatedIDs {
						idSet[id] = true
					}
					var filtered []resource.Resource
					for _, r := range lazyRows {
						if idSet[r.ID] {
							filtered = append(filtered, r)
						}
					}
					// Partial coverage: fall through to fetch so missing IDs are retrieved.
					if len(filtered) < len(result.RelatedIDs) {
						fetchCmd := m.fetchResources(msg.TargetType, m.core.AvailabilityGen())
						return m, fetchCmd
					}
				}
			}
			return m, func() tea.Msg {
				return messages.Flash{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		// FetchFilter path: use server-side filtered fetcher.
		if len(result.FetchFilter) > 0 {
			m.ctrl.PushChildListScreen(rt.ShortName)
			rl := views.NewResourceList(*rt, m.viewConfig, m.keys, m.ctrl)
			rl.SetTitleSuffix(runtime.RelatedTitleSuffix(msg.SourceResource))
			rl.SetFetchFilter(result.FetchFilter)
			rl.SetEscPops(true)
			rl.SetSize(m.innerSize())
			_, initCmd := rl.Init()
			rs := newListRS(rt.ShortName)
			w, h := m.innerSize()
			rs.width, rs.height = w, h
			m.pushRS(rs)
			// C6 replay: seed instantly from the session filtered-rows cache; the
			// fetch below stays as the ⟳ verify-refresh.
			m.ctrl.SeedFilteredListFromCache(msg.TargetType, result.FetchFilter)
			return m, tea.Batch(initCmd, m.fetchResourcesFiltered(msg.TargetType, result.FetchFilter, m.core.AvailabilityGen()))
		}

		// TargetID-based filtered list (cache miss).
		if result.TargetID != "" {
			// If the runtime decided this is a by-ID detail drill and clients are
			// ready, run that fetch and navigate straight to detail — no list view.
			if m.core.Clients() != nil {
				for _, t := range tasks {
					if t.Key.Kind == runtime.KindFetchByIDDetail {
						return m, relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
					}
				}
			}
			// No by-ID fetcher or clients not yet ready: fall back to a filtered
			// list that auto-opens the single match.
			m.flash = flashState{
				text:    fmt.Sprintf("Resource %s not in cache; loading %s list", result.TargetID, msg.TargetType),
				isError: false,
				active:  true,
			}
			initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
				pendingFilter:        result.TargetID,
				relatedIDs:           []string{result.TargetID},
				autoOpenSingleDetail: true,
				reapplyChecker:       msg.Checker,
			})
			fetchCmd := relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
			return m, tea.Batch(initCmd, fetchCmd)
		}

		// Exact-ID or truncated filtered list. Build the lean shell — a related
		// list that renders from the controller snapshot — and run the runtime's
		// coverage-aware tasks verbatim (nil on a full cache hit, fetch-more on a
		// partial page, fetch-all on a miss). Exact IDs seed their rows from cache
		// inside newRelatedList so a hit needs no fetch; a truncated "(0+)"/"(N+)"
		// carries the reapply-checker and fetches the population. A lone exact miss
		// auto-opens its detail once the fetch lands.
		opts := relatedListOpts{relatedIDs: result.RelatedIDs}
		if result.Truncated {
			opts.reapplyChecker = msg.Checker
		} else if len(result.RelatedIDs) == 1 {
			opts.pendingFilter = result.RelatedIDs[0]
			opts.autoOpenSingleDetail = true
		}
		initCmd := m.newRelatedList(*rt, msg.SourceResource, opts)
		fetchCmd := relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
		return m, tea.Batch(initCmd, fetchCmd)

	case runtime.NavigationKindDetail:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.Flash{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		targetID := result.TargetID
		if targetID == "" && len(result.RelatedIDs) == 1 {
			targetID = result.RelatedIDs[0]
		}
		resolveDetailResource := func(rows []resource.Resource) (resource.Resource, bool) {
			for _, r := range rows {
				if r.ID == targetID {
					return r, true
				}
			}
			return resource.Resource{}, false
		}
		var detailRes resource.Resource
		var detailFound bool
		if entry, ok := m.core.AnyOriginResourceCache(msg.TargetType); ok && entry != nil {
			detailRes, detailFound = resolveDetailResource(entry.Resources)
		}
		if !detailFound {
			if lazyRows, ok := m.core.LazyResourceCache(msg.TargetType); ok {
				detailRes, detailFound = resolveDetailResource(lazyRows)
			}
		}
		if detailFound {
			r := detailRes
			// Push ScreenDetail onto the controller stack and seed DetailState.
			m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
			m.ctrl.EnsureDetailState(r, msg.TargetType)
			m.ctrl.InitDetailRelatedRows(msg.TargetType)
			detail := views.NewDetailWithCtrl(r, msg.TargetType, m.viewConfig, m.keys, m.ctrl)
			detail.SetNavProvider(resource.GetNavigableFields)
			detail.SetSize(m.innerSize())
			detailRS := newDetailRS(msg.TargetType)
			wD, hD := m.innerSize()
			detailRS.width, detailRS.height = wD, hD
			defs := resource.GetRelated(msg.TargetType)
			if len(defs) > 0 {
				detailRS.rightCol = views.NewRightColumn(defs, r, msg.TargetType)
				detailRS.rightColAutoShown = true
				detailRS.rightColVisible = true
			}
			m.pushRS(detailRS)

			// BeginDetailWorkload begins the op and returns its complete
			// workload (enrich + related) in one call — cache-replay
			// suppression (D6: no re-fan-out over cached data) is decided
			// INSIDE it (Controller.replayRelatedCache, merging directly into
			// the same DetailState this screen renders from), replacing the
			// hand-rolled RelatedCacheReplay/ApplyDetailRelatedResultForResource
			// duplicate that used to live here.
			_, tasks := m.ctrl.BeginDetailWorkload(msg.TargetType, r, false, false)

			needsRelated := detail.NeedsRelatedCheck()
			if needsRelated {
				var cmds []tea.Cmd
				if len(tasks) > 0 {
					cmds = append(cmds, m.dispatchTaskRequests(tasks))
				}
				if len(cmds) == 0 {
					return m, nil
				}
				return m, tea.Batch(cmds...)
			}
			// needsRelated is false (narrow terminal or no registered related
			// defs): the right column never shows, so drop the related half —
			// dispatch only enrich — then fall through to the cache-miss
			// fallback below, exactly mirroring the pre-builder shape.
			var cmds []tea.Cmd
			if enrichOnly := dropTaskKind(tasks, runtime.KindRelatedCheck); len(enrichOnly) > 0 {
				cmds = append(cmds, m.dispatchTaskRequests(enrichOnly))
			}
			// Cache miss: the runtime resolved NavigationKindDetail from its own
			// snapshot, but the target isn't in the adapter's caches (e.g. a
			// lazily-added related target such as an IAM role named by a
			// CloudTrail event). Fall back to a by-ID fetch that navigates
			// straight to the detail rather than silently no-op'ing.
			if targetID != "" && resource.GetFetchByIDs(msg.TargetType) != nil {
				cmds = append(cmds, m.fetchByIDDetail(msg.TargetType, targetID))
			}
			if len(cmds) == 0 {
				return m, nil
			}
			return m, tea.Batch(cmds...)
		}

	case runtime.NavigationKindResourceList:
		// Owner decision #38 (2026-07-06): a related row with no TargetID,
		// RelatedIDs, or FetchFilter to narrow by (e.g. the transient "(?)"
		// no-filter unknown state) carries no filtering information at all —
		// there is nothing "related" left to scope the list by. Dispatch the
		// SAME messages.Navigate a menu entry would produce so the pushed
		// list is a plain, unfiltered top-level list (no RelatedTitleSuffix,
		// no forced EscPops), not a related/contextual list.
		return m.handleNavigate(messages.Navigate{
			Target:       messages.TargetResourceList,
			ResourceType: msg.TargetType,
		})
	}

	return m, nil
}

// handleRelatedNavigateChild handles navigation to a child resource type from
// the related panel. It dispatches an EnterChildViewMsg so that the existing
// child-view machinery handles the push and fetch.
func (m Model) handleRelatedNavigateChild(msg messages.RelatedNavigate) (tea.Model, tea.Cmd) {
	childDef := resource.GetChildType(msg.TargetType)
	if childDef == nil {
		return m, func() tea.Msg {
			return messages.Flash{
				Text:    fmt.Sprintf("unknown child type: %s", msg.TargetType),
				IsError: true,
			}
		}
	}

	var parentCtx map[string]string
	if childDef.RelatedContextFromIDs != nil {
		parentCtx = childDef.RelatedContextFromIDs(msg.RelatedIDs)
	}
	if parentCtx == nil {
		parentCtx = map[string]string{}
	}

	displayName := msg.TargetType
	if childDef.Name != "" {
		displayName = childDef.Name
	}

	return m, func() tea.Msg {
		return messages.EnterChildView{
			ChildType:     msg.TargetType,
			ParentContext: parentCtx,
			DisplayName:   displayName,
		}
	}
}

// relatedNavigateTasksToCmd translates TaskRequests from HandleRelatedNavigate
// into Bubble Tea commands. KindFetchFiltered is resolved here — the only
// case that needs targetType/result, unavailable to the shared dispatcher —
// every other kind (KindFetchResources, KindFetchMore, KindFetchByIDDetail,
// and any future addition) delegates to m.dispatchTaskRequests, the single
// switch every screen's adapter shares.
func relatedNavigateTasksToCmd(m Model, targetType string, result runtime.NavigationResult, tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	rest := make([]runtime.TaskRequest, 0, len(tasks))
	for _, t := range tasks {
		if t.Key.Kind == runtime.KindFetchFiltered {
			// The related handler does not set a fetchFilteredPayload on the
			// task — the filter lives in result.FetchFilter. ExecuteTask
			// would fail with "missing fetchFilteredPayload", so this one
			// case stays adapter-local instead of going through the shared
			// dispatcher.
			cmds = append(cmds, m.fetchResourcesFiltered(targetType, result.FetchFilter, m.core.AvailabilityGen()))
			continue
		}
		rest = append(rest, t)
	}
	if tc := m.dispatchTaskRequests(rest); tc != nil {
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

// relatedCheckCmd fans out one goroutine per RelatedDef registered for
// op.ResourceType, capped by runtime.MaxConcurrentProbes. Each goroutine
// calls runtime.RunRelatedDef — the exact per-def logic
// core/runtime/executor.go's KindRelatedCheck case also calls — so a
// checker's timeout, panic recovery, NeedsTargetCache prefetch, and
// lazy-add behavior can never diverge between the TUI's progressive
// per-def rendering and the headless/web bounded-concurrent fan-out.
func (m Model) relatedCheckCmd(op runtime.DetailOperation) tea.Cmd {
	defs := resource.GetRelated(op.ResourceType)
	if len(defs) == 0 {
		return nil
	}

	cacheSnap := m.buildResourceCacheSnapshot()
	keys := m.core.FetchOriginCacheKeys()
	mainCacheKeys := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		mainCacheKeys[k] = struct{}{}
	}

	sem := make(chan struct{}, runtime.MaxConcurrentProbes)
	cmds := make([]tea.Cmd, 0, len(defs))
	for _, def := range defs {
		cmds = append(cmds, func() tea.Msg {
			sem <- struct{}{}
			defer func() { <-sem }()
			return runtime.RunRelatedDef(m.appCtx, op, cacheSnap, mainCacheKeys, def)
		})
	}
	return tea.Batch(cmds...)
}
