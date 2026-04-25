// app_handlers_related_navigate.go — RelatedNavigateMsg dispatch.
//
// Split from app_related.go (which now holds RelatedCheckStartedMsg dispatch
// and the cache-snapshot helpers) to keep both files under the 500-line
// file-size budget.
//
//   handleRelatedNavigate       — main switch over NavigationResult.Kind
//                                 (KindFlash / KindEnterChildView / KindFilteredList
//                                  / KindDetail / KindResourceList).
//   handleRelatedNavigateChild  — KindEnterChildView fan-out: resolves the
//                                 ChildViewDef and emits EnterChildViewMsg.
package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// handleRelatedNavigate pushes a resource list view for the related target type,
// or a detail view if a single related ID is found in the cache.
// Cache hits skip the fetch and render filtered results immediately.
// Cache misses dispatch a fetch command alongside the list view init.
func (m Model) handleRelatedNavigate(msg messages.RelatedNavigateMsg) (tea.Model, tea.Cmd) {
	result := ResolveRelatedNavigate(msg, m.snapshotCache())

	switch result.Kind {
	case KindFlash:
		return m, func() tea.Msg {
			return messages.FlashMsg{
				Text:    result.FlashMessage,
				IsError: result.FlashIsError,
			}
		}

	case KindEnterChildView:
		return m.handleRelatedNavigateChild(msg)

	case KindFilteredList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			// Fetcher-only type (registered paginated fetcher but no ResourceTypeDef —
			// e.g. a dynamically-registered test type). When the lazy cache has partial
			// coverage, fall through to a full fetch so the missing IDs are retrieved.
			// This preserves the never-use-fast-path-for-partial-coverage invariant even
			// when the type has no visual definition.
			if len(result.RelatedIDs) > 0 {
				if lazyRows, hasLazy := m.lazyResourceCache[msg.TargetType]; hasLazy {
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
					// Full coverage: no need to fetch — but we have no view to render, so flash.
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
			// Exact AMI navigation should fetch by image ID instead of
			// falling back to the owned-AMI list, which misses public and third-party images.
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
			return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
		}

		// RelatedIDs-based filtered list (multi or single cache miss).
		if len(result.RelatedIDs) > 0 {
			if entry, ok := m.resourceCache[msg.TargetType]; ok {
				idSet := make(map[string]bool, len(result.RelatedIDs))
				for _, id := range result.RelatedIDs {
					idSet[id] = true
				}
				var filtered []resource.Resource
				for _, r := range entry.resources {
					if idSet[r.ID] {
						filtered = append(filtered, r)
					}
				}
				// Augment with lazy-cached resources for this type. These are
				// out-of-scope entries (AWS-managed KMS keys, public AMIs, shared
				// snapshots) that the top-level fetcher filtered out. Prefer
				// resourceCache on ID collision (already covered above).
				if lazyRows, hasLazy := m.lazyResourceCache[msg.TargetType]; hasLazy {
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
				// Pre-populate the list with already-cached filtered rows so they remain
				// visible when subsequent pages arrive via Append:true ResourcesLoadedMsg.
				if len(filtered) < len(result.RelatedIDs) && entry.pagination != nil && entry.pagination.IsTruncated {
					rl := views.NewResourceListFromCache(
						*rt, m.viewConfig, m.keys,
						filtered, entry.pagination,
						"",
						entry.sortColIdx, entry.sortAsc,
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
					fetchCmd := m.fetchMoreResources(messages.LoadMoreMsg{
						ResourceType:      msg.TargetType,
						ContinuationToken: entry.pagination.NextToken,
					})
					return m, fetchCmd
				}
				// All RelatedIDs matched — no more pages can contribute to this
				// filter, so strip IsTruncated on the pagination we hand to the
				// view. Otherwise the list inherits the upstream cache's
				// truncation flag and renders a misleading "m: load more"
				// footer for a fully-resolved exact-ID filter.
				paginationForView := entry.pagination
				if paginationForView != nil && paginationForView.IsTruncated {
					clone := *paginationForView
					clone.IsTruncated = false
					clone.NextToken = ""
					paginationForView = &clone
				}
				rl := views.NewResourceListFromCache(
					*rt, m.viewConfig, m.keys,
					filtered, paginationForView,
					"", // no text filter needed, already filtered by ID
					entry.sortColIdx, entry.sortAsc,
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
			// resourceCache miss: check lazyResourceCache before triggering a fetch.
			// This handles the case where all requested IDs were pulled via FetchByIDs
			// (e.g. AWS-managed KMS key drill — never in resourceCache).
			if lazyRows, hasLazy := m.lazyResourceCache[msg.TargetType]; hasLazy {
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
					// All requested IDs are covered by the lazy cache — render immediately.
					// Partial coverage (some IDs missing) falls through to the full-fetch
					// path below so those IDs are retrieved from AWS.
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
			return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
		}

	case KindDetail:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}

		// Find resource in cache. Search both resourceCache (primary, scope-filtered
		// top-level entries) and lazyResourceCache (out-of-scope entries pulled via
		// FetchByIDs, e.g. AWS-managed KMS keys, public AMIs).
		targetID := result.TargetID
		if targetID == "" && len(result.RelatedIDs) == 1 {
			targetID = result.RelatedIDs[0]
		}
		// resolveDetailResource searches a slice for targetID and returns the
		// resource if found.
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
		if entry, ok := m.resourceCache[msg.TargetType]; ok {
			detailRes, detailFound = resolveDetailResource(entry.resources)
		}
		if !detailFound {
			if lazyRows, ok := m.lazyResourceCache[msg.TargetType]; ok {
				detailRes, detailFound = resolveDetailResource(lazyRows)
			}
		}
		if detailFound {
			r := detailRes
			// Mirror manual Enter on this row: if the target type
			// registers a child under Key="enter" and its DrillCondition
			// (if any) admits the row, jump straight into the child
			// view. Otherwise push the generic detail. This keeps
			// single-result auto-drill consistent with what Enter does
			// in the target list — a pivot that narrows to one row
			// must not strand the operator on bucket metadata when
			// Enter would have opened bucket contents.
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
			detail.SetSize(m.innerSize())
			m.pushView(&detail)
			if detail.NeedsRelatedCheck() {
				ck := relatedCacheKey(msg.TargetType, r.ID)
				if cached, ok := m.relatedCache.get(ck); ok && len(cached) > 0 {
					detail.ApplyRelatedResults(relatedCacheReplay(msg.TargetType, cached))
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

	case KindResourceList:
		rt := resource.FindResourceType(msg.TargetType)
		if rt == nil {
			return m, func() tea.Msg {
				return messages.FlashMsg{
					Text:    fmt.Sprintf("unknown resource type: %s", msg.TargetType),
					IsError: true,
				}
			}
		}
		// Approximate-zero (0+) path: zero known IDs but the reverse-scan
		// cache was truncated. Navigate with the checker so each loaded page
		// re-applies the predicate and matches accumulate.
		initCmd := m.newRelatedList(*rt, msg.SourceResource, relatedListOpts{
			reapplyChecker: msg.Checker,
		})
		return m, tea.Batch(initCmd, m.fetchResources(msg.TargetType))
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

	// Extract parent context from related IDs using the child type's extractor.
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
