// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// handlers_related.go — RelatedNavigateEvent dispatch (platform-agnostic; no
// Bubble Tea imports).
//
//	HandleRelatedNavigate — resolves the navigation kind from the session
//	                        cache and returns the decision plus any fetch
//	                        TaskRequests the adapter should start.
//
// The view construction and all Bubble Tea specifics remain in the TUI adapter
// (internal/tui/runtime_adapter.go). The runtime owns only the pure policy:
// what kind of navigation and whether a server fetch is needed.
package runtime

import (
	"fmt"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// NavigationKind enumerates the possible outcomes of resolving a
// RelatedNavigateEvent. The pure contract is documented in
// §"Related-navigation contract (#278)" on ResolveRelatedNavigate below.
type NavigationKind int

const (
	NavigationKindUnknown        NavigationKind = iota
	NavigationKindResourceList                  // push a fresh ResourceList of TargetType
	NavigationKindFilteredList                  // push a ResourceList filtered by RelatedIDs/FetchFilter
	NavigationKindDetail                        // push a DetailView for the specific TargetID (cache hit)
	NavigationKindEnterChildView                // push a child view (e.g. s3_objects under an s3 bucket)
	NavigationKindFlash                         // emit a FlashMsg (typically error path)
)

// NavigationResult is the pure-function output of ResolveRelatedNavigate.
// Fields are conditionally populated depending on Kind.
type NavigationResult struct {
	Kind         NavigationKind
	TargetType   string
	TargetID     string
	RelatedIDs   []string
	FetchFilter  map[string]string
	FilterText   string
	FlashMessage string
	FlashIsError bool
	// Truncated marks a reverse scan in progress ("(0+)"/"(N+)"): RelatedIDs is a
	// lower bound the reapply-checker extends as later pages load, so navigation
	// seeds the list with them and fetches the population. "(0+)" is not special.
	Truncated bool
}

// RelatedNavigateEvent is the runtime-side event for related-resource navigation.
// Adapters translate from their native message type before calling
// HandleRelatedNavigate.
type RelatedNavigateEvent struct {
	TargetType     string
	SourceResource resource.Resource
	SourceType     string
	TargetID       string
	RelatedIDs     []string
	FetchFilter    map[string]string
	Checker        resource.RelatedChecker
	// Truncated is the source row's truncation flag, routing "(0+)"/"(N+)" to the
	// reverse-scan scoped path.
	Truncated bool
}

// TaskKind constants for fetch operations emitted by HandleRelatedNavigate.
// Adapters type-switch on these in their TaskRequest-to-Cmd translators.
const (
	// KindFetchResources asks the adapter to fetch all resources of the type
	// named by TaskKey.Scope.
	KindFetchResources TaskKind = "fetch-resources"

	// KindFetchFiltered asks the adapter to run a server-side filtered fetch
	// for the type named by TaskKey.Scope.
	KindFetchFiltered TaskKind = "fetch-filtered"

	// KindFetchMore asks the adapter to fetch the next page of resources for
	// the type named by TaskKey.Scope. The continuation token rides on the
	// TaskRequest as a FetchMorePayload so the runtime is the single decision-
	// maker and the adapter is a pure mechanical translator.
	KindFetchMore TaskKind = "fetch-more"

	// KindFetchByIDDetail asks the adapter to fetch a single resource by exact
	// ID via its registered FetchByIDs helper and navigate straight to its
	// detail view. This replaces the former ami-only adapter shortcut and fires
	// on any cache-miss exact-ID drill for types that have a registered
	// FetchByIDs (currently: ami, kms, policy, ebs-snap). When the target type
	// is in the owned cache, navigation resolves to NavigationKindDetail upstream
	// and this task is never emitted.
	KindFetchByIDDetail TaskKind = "fetch-by-id-detail"
)

// FetchMorePayload carries the continuation token the adapter must use when
// the runtime requests a KindFetchMore fetch. The runtime captures the token
// from the session cache at dispatch time so the adapter no longer reaches
// back into session state to re-derive it.
//
// ParentContext and FetchFilter are non-nil when the list being paginated is
// a child list (ParentContext) or a filtered list (FetchFilter). The executor
// routes to the appropriate child/filtered fetcher when either is set.
type FetchMorePayload struct {
	ContinuationToken string
	ParentContext     map[string]string
	FetchFilter       map[string]string
}

func (FetchMorePayload) isTaskPayload() {}

// FetchByIDDetailPayload carries the target type and exact ID for a
// KindFetchByIDDetail task. The runtime populates these at dispatch time so the
// adapter is a pure pass-through with no session-cache reads.
type FetchByIDDetailPayload struct {
	TargetType string
	ID         string
}

func (FetchByIDDetailPayload) isTaskPayload() {}

// HandleRelatedNavigate resolves the navigation kind using the session cache
// and returns the decision plus any fetch tasks the adapter should start.
//
// View construction and Bubble Tea specifics remain in the TUI adapter so this
// handler is platform-agnostic and testable without standing up Bubble Tea.
func (c *Core) HandleRelatedNavigate(ev RelatedNavigateEvent) (NavigationResult, []TaskRequest) {
	snap := relatedCacheSnapshot(c.session)
	result := ResolveRelatedNavigate(ev, snap)

	switch result.Kind {
	case NavigationKindFlash, NavigationKindEnterChildView, NavigationKindDetail:
		// No server fetch required; the adapter serves these from cached state
		// or its own message dispatch.
		return result, nil

	case NavigationKindFilteredList:
		if len(result.FetchFilter) > 0 {
			return result, []TaskRequest{{
				Key:   TaskKey{Kind: KindFetchFiltered, Scope: ev.TargetType},
				Cache: CacheNone,
			}}
		}
		if result.TargetID != "" {
			if resource.GetFetchByIDs(ev.TargetType) != nil {
				return result, []TaskRequest{{
					Key:     TaskKey{Kind: KindFetchByIDDetail, Scope: ev.TargetType},
					Cache:   CacheNone,
					Payload: FetchByIDDetailPayload{TargetType: ev.TargetType, ID: result.TargetID},
				}}
			}
			return result, []TaskRequest{{
				Key:   TaskKey{Kind: KindFetchResources, Scope: ev.TargetType},
				Cache: CacheNone,
			}}
		}
		if result.Truncated {
			// Reverse scan in progress ("(0+)"/"(N+)"): fetch the whole target
			// population so the reapply-checker scopes each page. Distinct from an
			// exact result (below), which found all its targets and fetches by ID.
			return result, []TaskRequest{{
				Key:   TaskKey{Kind: KindFetchResources, Scope: ev.TargetType},
				Cache: CacheNone,
			}}
		}
		if len(result.RelatedIDs) > 0 {
			tasks := relatedFetchTasks(c.session, ev.TargetType, result.RelatedIDs)
			return result, tasks
		}
		return result, nil

	case NavigationKindResourceList:
		return result, []TaskRequest{{
			Key:   TaskKey{Kind: KindFetchResources, Scope: ev.TargetType},
			Cache: CacheNone,
		}}
	}

	return result, nil
}

// RelatedCachedResource returns the cached resource of targetType with the given
// id, using the same snapshot ResolveRelatedNavigate consults. The controller
// uses it to seed a detail for a NavigationKindDetail (cache-hit) related-
// navigate without a server fetch — mirroring the TUI adapter, which serves
// Detail from cached state. NavigationKindDetail is only ever returned on a
// cache hit, so this lookup succeeds for that path.
func (c *Core) RelatedCachedResource(targetType, id string) (resource.Resource, bool) {
	snap := relatedCacheSnapshot(c.session)
	for _, r := range snap[targetType] {
		if r.ID == id {
			return r, true
		}
	}
	return resource.Resource{}, false
}

// relatedFetchTasks decides what fetch task (if any) is needed for a
// RelatedIDs-based filtered list. Reads RowStore directly (task #17 wave 1
// stage 3 — the former ResourceCache/LazyResourceCache maps are gone; a
// type's rows live in exactly one RowStore entry, full or Partial alike).
func relatedFetchTasks(s *session.Session, targetType string, relatedIDs []string) []TaskRequest {
	tr := s.RowStore.Snapshot(targetType)

	// Count how many of the requested IDs are already covered.
	covered := make(map[string]struct{}, len(relatedIDs))
	for _, r := range tr.Rows {
		covered[r.ID] = struct{}{}
	}
	missing := 0
	for _, id := range relatedIDs {
		if _, ok := covered[id]; !ok {
			missing++
		}
	}

	if missing == 0 {
		// All IDs are in cache — no fetch needed.
		return nil
	}

	// Some IDs are missing. If the cache has more pages, ask for more and
	// carry the continuation token as a structured payload
	// so the adapter is a pure pass-through. A Partial (lazy-only) entry
	// never carries a continuation token of its own (ObservePartial leaves
	// Pagination untouched), so this branch only fires for a full entry's
	// own pagination state, matching the former ResourceCache-only check.
	if tr.Gen != 0 && !tr.Partial && tr.Pagination != nil && tr.Pagination.IsTruncated {
		return []TaskRequest{{
			Key:     TaskKey{Kind: KindFetchMore, Scope: targetType},
			Cache:   CacheNone,
			Payload: FetchMorePayload{ContinuationToken: tr.Pagination.NextToken},
		}}
	}

	// Cache miss (no entry at all or no further pages) — fetch all resources.
	return []TaskRequest{{
		Key:   TaskKey{Kind: KindFetchResources, Scope: targetType},
		Cache: CacheNone,
	}}
}

// relatedCacheSnapshot returns a flat map[string][]resource.Resource snapshot
// suitable for the navigation resolver, reading directly from RowStore (task
// #17 wave 1 stage 3). A type's rows now live in exactly one RowStore entry
// (full or Partial), so there is no merge-precedence to apply — the former
// two-map "ResourceCache wins over LazyResourceCache on ID collision" rule
// is now vacuous (the store itself is the single source for both roles).
func relatedCacheSnapshot(s *session.Session) map[string][]resource.Resource {
	all := s.RowStore.SnapshotAll(true)
	snap := make(map[string][]resource.Resource, len(all))
	for shortName, tr := range all {
		snap[shortName] = tr.Rows
	}
	return snap
}

// ResolveRelatedNavigate computes the navigation kind for a RelatedNavigateEvent
// against a flat resource-cache snapshot. It is the SSOT for
// related-navigation resolution. Exported so tests/unit can drive it without
// reaching into runtime internals.
//
// Related-navigation contract (#278):
//
//  1. Unknown target type          → NavigationKindFlash (error surfaced to the user).
//  2. Child type                   → NavigationKindEnterChildView.
//  3. Exact target already KNOWN   → NavigationKindDetail (cache hit).
//  4. FetchFilter + registered fetcher → NavigationKindFilteredList (FetchFilter preserved).
//  5. TargetID cache miss          → NavigationKindFilteredList (FilterText=TargetID).
//  6. RelatedIDs (one or many)     → NavigationKindFilteredList (RelatedIDs preserved).
//  7. Otherwise                    → NavigationKindResourceList.
func ResolveRelatedNavigate(ev RelatedNavigateEvent, cache map[string][]resource.Resource) NavigationResult {
	_, isChild, found := resource.ResolveNavigationTarget(ev.TargetType)
	if !found {
		return NavigationResult{
			Kind:         NavigationKindFlash,
			FlashMessage: fmt.Sprintf("unknown resource type: %s", ev.TargetType),
			FlashIsError: true,
		}
	}

	if isChild {
		return NavigationResult{
			Kind:       NavigationKindEnterChildView,
			TargetType: ev.TargetType,
			RelatedIDs: ev.RelatedIDs,
		}
	}

	// Truncated reverse scan ("(0+)"/"(N+)") always opens the scan list, never a
	// single detail — even a "(1+)" whose one found id is already cached, and even
	// when the caller also set TargetID. Later pages may hold more matches, so the
	// population fetch + reapply must run. Checked before the TargetID and
	// single-RelatedID cache-hit fast paths, which would otherwise short-circuit a
	// truncated single-match pivot to one detail.
	if ev.Truncated {
		return NavigationResult{
			Kind:       NavigationKindFilteredList,
			TargetType: ev.TargetType,
			RelatedIDs: ev.RelatedIDs,
			Truncated:  true,
		}
	}

	// Exact drill-in, TargetID cache hit → NavigationKindDetail.
	if ev.TargetID != "" && relatedCacheHit(cache, ev.TargetType, ev.TargetID) {
		return NavigationResult{
			Kind:       NavigationKindDetail,
			TargetType: ev.TargetType,
			TargetID:   ev.TargetID,
		}
	}

	// Single RelatedID cache hit → NavigationKindDetail.
	if len(ev.RelatedIDs) == 1 && relatedCacheHit(cache, ev.TargetType, ev.RelatedIDs[0]) {
		return NavigationResult{
			Kind:       NavigationKindDetail,
			TargetType: ev.TargetType,
			RelatedIDs: ev.RelatedIDs,
		}
	}

	// FetchFilter path — only when a filtered paginated fetcher is registered.
	if len(ev.FetchFilter) > 0 && resource.GetFilteredPaginatedFetcher(ev.TargetType) != nil {
		return NavigationResult{
			Kind:        NavigationKindFilteredList,
			TargetType:  ev.TargetType,
			FetchFilter: ev.FetchFilter,
		}
	}

	// TargetID cache miss → filtered list by the ID string.
	if ev.TargetID != "" {
		return NavigationResult{
			Kind:       NavigationKindFilteredList,
			TargetType: ev.TargetType,
			TargetID:   ev.TargetID,
			FilterText: ev.TargetID,
		}
	}

	// Exact (non-truncated) scan result with found IDs → a filtered list scoped
	// to them. The truncated lower bound ("(0+)"/"(N+)") returned earlier.
	if len(ev.RelatedIDs) > 0 {
		return NavigationResult{
			Kind:       NavigationKindFilteredList,
			TargetType: ev.TargetType,
			RelatedIDs: ev.RelatedIDs,
		}
	}

	// No scope at all (no IDs, no filter, no target, not truncated) — a defensive
	// fallback that the real Enter flow never produces (a non-truncated 0 row is a
	// dead end, not a navigation). Open the plain target list.
	return NavigationResult{
		Kind:       NavigationKindResourceList,
		TargetType: ev.TargetType,
	}
}

// relatedCacheHit reports whether a resource with the given ID exists in the
// snapshot cache for targetType.
func relatedCacheHit(cache map[string][]resource.Resource, targetType, id string) bool {
	for _, r := range cache[targetType] {
		if r.ID == id {
			return true
		}
	}
	return false
}
