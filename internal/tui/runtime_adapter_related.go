// runtime_adapter_related.go — Bubble Tea adapter glue for the runtime
// HandleRelatedNavigate entry point (Phase 05 PR-05a-h4).
//
// handleRelatedNavigate replaces the deleted entry point from
// internal/tui/app_handlers_related_navigate.go. It constructs a transient
// runtime.Core, calls core.HandleRelatedNavigate, then applies the navigation
// decision to the view stack and translates TaskRequests into tea.Cmd values.
// The existing app.go dispatch line (return m.handleRelatedNavigate(msg)) is
// unchanged.
//
// handleRelatedNavigateChild stays here as a TUI-only helper because it
// dispatches a messages.EnterChildViewMsg — a Bubble Tea message type.
//
// Decision-locus follow-up (PR-05b): a few branches in this adapter still walk
// the session cache directly to drive view construction (AMI exact-ID drill,
// lazy-cache full-coverage shortcut, RelatedIDs partial-coverage pre-populated
// list). The runtime emits the same task decisions in HandleRelatedNavigate /
// relatedFetchTasks; when PR-05b lands the typed cmd/event split it will carry
// enough payload (continuation tokens, lazy-cache slices, client selection)
// for the adapter to be purely mechanical. Until then the adapter mirrors the
// runtime's policy and trusts the emitted []TaskRequest rather than overriding
// it.
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleRelatedNavigate replaces the entry point previously in
// internal/tui/app_handlers_related_navigate.go. The signature is identical
// so the existing app.go dispatch line is unchanged.
//
// It constructs a transient runtime.Core to invoke the migrated policy
// (HandleRelatedNavigate), then builds the view and starts any required fetch
// based on the returned NavigationResult and TaskRequests.
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	core := runtime.New(m.Session, resource.AllResourceTypes())
	ev := runtime.RelatedNavigateEvent{
		TargetType:     msg.TargetType,
		SourceResource: msg.SourceResource,
		SourceType:     msg.SourceType,
		TargetID:       msg.TargetID,
		RelatedIDs:     msg.RelatedIDs,
		FetchFilter:    msg.FetchFilter,
		Checker:        msg.Checker,
	}
	result, tasks := core.HandleRelatedNavigate(ev)

	switch result.Kind {
	case runtime.NavigationKindFlash:
		return m, func() tea.Msg {
			return messages.FlashMsg{
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
				if lazyRows, hasLazy := m.LazyResourceCache[msg.TargetType]; hasLazy {
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
						fetchCmd := m.fetchResources(msg.TargetType)
						return m, fetchCmd
					}
				}
			}
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		// FetchFilter path: use server-side filtered fetcher.
		if len(result.FetchFilter) > 0 {
			rl := views.NewResourceList(*rt, m.viewConfig, m.keys)
			rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
			rl.SetFetchFilter(result.FetchFilter)
			rl.SetEscPops(true)
			rl.SetSize(m.innerSize())
			rl, initCmd := rl.Init()
			m.pushView(&rl)
			return m, tea.Batch(initCmd, m.fetchResourcesFiltered(msg.TargetType, result.FetchFilter))
		}

		// TargetID-based filtered list (cache miss).
		if result.TargetID != "" {
			// Exact AMI navigation should fetch by image ID instead of falling
			// back to the owned-AMI list, which misses public and third-party images.
			if msg.TargetType == "ami" && m.clients != nil {
				cmd := m.fetchAMIDetail(result.TargetID)
				return m, cmd
			}
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

		// RelatedIDs-based filtered list (multi or single cache miss).
		if len(result.RelatedIDs) > 0 {
			if entry, ok := m.ResourceCache[msg.TargetType]; ok {
				idSet := make(map[string]bool, len(result.RelatedIDs))
				for _, id := range result.RelatedIDs {
					idSet[id] = true
				}
				var filtered []resource.Resource
				for _, r := range entry.Resources {
					if idSet[r.ID] {
						filtered = append(filtered, r)
					}
				}
				// Augment with lazy-cached resources. Prefer ResourceCache on ID collision.
				if lazyRows, hasLazy := m.LazyResourceCache[msg.TargetType]; hasLazy {
					found := make(map[string]struct{}, len(filtered))
					for _, r := range filtered {
						found[r.ID] = struct{}{}
					}
					for _, r := range lazyRows {
						if idSet[r.ID] {
							if _, dup := found[r.ID]; !dup {
								found[r.ID] = struct{}{}
								filtered = append(filtered, r)
							}
						}
					}
				}
				// If some IDs are missing and cache may have more pages, fetch the rest.
				// Pre-populate with already-cached filtered rows so they remain visible when
				// subsequent pages arrive via Append:true ResourcesLoadedMsg.
				if len(filtered) < len(result.RelatedIDs) && entry.Pagination != nil && entry.Pagination.IsTruncated {
					rl := views.NewResourceListFromCache(
						*rt, m.viewConfig, m.keys,
						filtered, entry.Pagination,
						"",
						entry.SortColIdx, entry.SortAsc,
						0, 0,
						false,
					)
					rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
					rl.SetRelatedIDFilter(result.RelatedIDs)
					if msg.Checker != nil {
						rl.SetReapplyChecker(msg.Checker, msg.SourceResource)
					}
					rl.SetEscPops(true)
					rl.SetSize(m.innerSize())
					m.pushView(&rl)
					fetchCmd := relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
					return m, fetchCmd
				}
				// All RelatedIDs matched — strip IsTruncated so the view doesn't show
				// a misleading "load more" footer for a fully-resolved exact-ID filter.
				paginationForView := entry.Pagination
				if paginationForView != nil && paginationForView.IsTruncated {
					clone := *paginationForView
					clone.IsTruncated = false
					clone.NextToken = ""
					paginationForView = &clone
				}
				rl := views.NewResourceListFromCache(
					*rt, m.viewConfig, m.keys,
					filtered, paginationForView,
					"",
					entry.SortColIdx, entry.SortAsc,
					0, 0,
					false,
				)
				rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
				rl.SetRelatedIDFilter(result.RelatedIDs)
				if msg.Checker != nil {
					rl.SetReapplyChecker(msg.Checker, msg.SourceResource)
				}
				rl.SetEscPops(true)
				rl.SetSize(m.innerSize())
				m.pushView(&rl)
				return m, nil
			}
			// ResourceCache miss: check LazyResourceCache before triggering a fetch.
			if lazyRows, hasLazy := m.LazyResourceCache[msg.TargetType]; hasLazy {
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
				if len(filtered) > 0 && len(filtered) == len(result.RelatedIDs) {
					rl := views.NewResourceListFromCache(
						*rt, m.viewConfig, m.keys,
						filtered, nil,
						"",
						0, true,
						0, 0,
						false,
					)
					rl.SetTitleSuffix(relatedTitleSuffix(msg.SourceResource))
					rl.SetRelatedIDFilter(result.RelatedIDs)
					if msg.Checker != nil {
						rl.SetReapplyChecker(msg.Checker, msg.SourceResource)
					}
					rl.SetEscPops(true)
					rl.SetSize(m.innerSize())
					m.pushView(&rl)
					return m, nil
				}
			}
			// Full cache miss: fetch and preserve exact-ID filtering.
			var opts relatedListOpts
			if len(result.RelatedIDs) == 1 {
				opts = relatedListOpts{
					pendingFilter:        result.RelatedIDs[0],
					relatedIDs:           result.RelatedIDs,
					autoOpenSingleDetail: true,
					reapplyChecker:       msg.Checker,
				}
			} else {
				opts = relatedListOpts{relatedIDs: result.RelatedIDs, reapplyChecker: msg.Checker}
			}
			initCmd := m.newRelatedList(*rt, msg.SourceResource, opts)
			fetchCmd := relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
			return m, tea.Batch(initCmd, fetchCmd)
		}

	case runtime.NavigationKindDetail:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
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
		if entry, ok := m.ResourceCache[msg.TargetType]; ok {
			detailRes, detailFound = resolveDetailResource(entry.Resources)
		}
		if !detailFound {
			if lazyRows, ok := m.LazyResourceCache[msg.TargetType]; ok {
				detailRes, detailFound = resolveDetailResource(lazyRows)
			}
		}
		if detailFound {
			r := detailRes
			if enterChild := enterChildForResource(rt, r); enterChild != nil {
				ctx := buildChildContextForResource(*enterChild, r)
				displayName := ctx[enterChild.DisplayNameKey]
				childType := enterChild.ChildType
				return m, func() tea.Msg {
					return messages.EnterChildViewMsg{
						ChildType:     childType,
						ParentContext: ctx,
						DisplayName:   displayName,
					}
				}
			}
			detail := views.NewDetail(r, msg.TargetType, m.viewConfig, m.keys)
			detail.SetNavProvider(resource.GetNavigableFields)
			detail.SetSize(m.innerSize())
			m.pushView(&detail)
			if detail.NeedsRelatedCheck() {
				ck := session.RelatedCacheKey(msg.TargetType, r.ID)
				if cached, ok := m.RelatedCache.Get(ck); ok && len(cached) > 0 {
					detail.ApplyRelatedResults(session.RelatedCacheReplay(msg.TargetType, cached))
					return m, nil
				}
				srcRes := r
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   msg.TargetType,
						SourceResource: srcRes,
					}
				}
			}
			return m, nil
		}

	case runtime.NavigationKindResourceList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			reapplyChecker: msg.Checker,
		})
		fetchCmd := relatedNavigateTasksToCmd(m, msg.TargetType, result, tasks)
		return m, tea.Batch(initCmd, fetchCmd)
	}

	return m, nil
}

// handleRelatedNavigateChild handles navigation to a child resource type from
// the related panel. It dispatches an EnterChildViewMsg so that the existing
// child-view machinery handles the push and fetch.
func (m Model) handleRelatedNavigateChild(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	childDef := resource.GetChildType(msg.TargetType)
	if childDef == nil {
		return m, func() tea.Msg {
			return messages.FlashMsg{
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
		return messages.EnterChildViewMsg{
			ChildType:     msg.TargetType,
			ParentContext: parentCtx,
			DisplayName:   displayName,
		}
	}
}

// relatedNavigateTasksToCmd translates TaskRequests from HandleRelatedNavigate
// into Bubble Tea commands. Unknown TaskKind values are dropped for
// forward-compatibility.
func relatedNavigateTasksToCmd(m Model, targetType string, result runtime.NavigationResult, tasks []runtime.TaskRequest) tea.Cmd {
	if len(tasks) == 0 {
		return nil
	}
	var cmds []tea.Cmd
	for _, t := range tasks {
		switch t.Key.Kind {
		case runtime.KindFetchResources:
			cmds = append(cmds, m.fetchResources(targetType))
		case runtime.KindFetchFiltered:
			cmds = append(cmds, m.fetchResourcesFiltered(targetType, result.FetchFilter))
		case runtime.KindFetchMore:
			if entry, ok := m.ResourceCache[targetType]; ok && entry.Pagination != nil {
				cmds = append(cmds, m.fetchMoreResources(messages.LoadMoreMsg{
					ResourceType:      targetType,
					ContinuationToken: entry.Pagination.NextToken,
				}))
			}
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
