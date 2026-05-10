// handlers_related.go — RelatedNavigateEvent dispatch.
//
// PR-05a-h4 moves the related-navigation entry point out of internal/tui per
// the Phase 05 boundary contract (docs/refactor/05-boundary.md §"5a-extract").
//
//   HandleRelatedNavigate — resolves the navigation kind from the session
//                           cache and returns the decision plus any fetch
//                           TaskRequests the adapter should start.
//
// The view construction and all Bubble Tea specifics remain in the TUI adapter
// (internal/tui/runtime_adapter.go). The runtime owns only the pure policy:
// what kind of navigation and whether a server fetch is needed.
package runtime

import (
	"fmt"
	"maps"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/session"
)

// NavigationKind enumerates the possible outcomes of resolving a
// RelatedNavigateEvent. Mirrors the contract in
// internal/tui/navigation_resolve.go §"Related-navigation contract (#278)".
type NavigationKind int

const (
	NavigationKindUnknown        NavigationKind = iota
	NavigationKindResourceList                  // push a fresh ResourceList of TargetType
	NavigationKindFilteredList                  // push a ResourceList filtered by RelatedIDs/FetchFilter
	NavigationKindDetail                        // push a DetailView for the specific TargetID (cache hit)
	NavigationKindEnterChildView                // push a child view (e.g. s3_objects under an s3 bucket)
	NavigationKindFlash                         // emit a FlashMsg (typically error path)
)

// NavigationResult is the pure-function output of resolveRelatedNavigate.
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
	// the type named by TaskKey.Scope, using the continuation token it holds
	// in the session cache.
	KindFetchMore TaskKind = "fetch-more"
)

// HandleRelatedNavigate resolves the navigation kind using the session cache
// and returns the decision plus any fetch tasks the adapter should start.
//
// Receiver migrated from *Model to *Core per docs/refactor/05-boundary.md.
// Session fields (ResourceCache, LazyResourceCache) are accessed through
// c.session instead of the previously-embedded model fields.
//
// View construction and Bubble Tea specifics remain in the TUI adapter so this
// handler is platform-agnostic and testable without standing up Bubble Tea.
func (c *Core) HandleRelatedNavigate(ev RelatedNavigateEvent) (NavigationResult, []TaskRequest) {
	snap := relatedCacheSnapshot(c.session)
	result := resolveRelatedNavigate(ev, snap)

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
			// Client-selection (AMI exact-ID) is adapter-owned state; emit a
			// generic fetch-resources request and let the adapter specialise.
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

// relatedFetchTasks decides what fetch task (if any) is needed for a
// RelatedIDs-based filtered list. It checks ResourceCache and LazyResourceCache
// to determine coverage before emitting a task.
func relatedFetchTasks(s *session.Session, targetType string, relatedIDs []string) []TaskRequest {
	entry := s.ResourceCache[targetType]
	lazy := s.LazyResourceCache[targetType]

	// Count how many of the requested IDs are already covered.
	covered := make(map[string]struct{}, len(relatedIDs))
	if entry != nil {
		for _, r := range entry.Resources {
			covered[r.ID] = struct{}{}
		}
	}
	for _, r := range lazy {
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

	// Some IDs are missing. If the cache has more pages, ask for more.
	if entry != nil && entry.Pagination != nil && entry.Pagination.IsTruncated {
		return []TaskRequest{{
			Key:   TaskKey{Kind: KindFetchMore, Scope: targetType},
			Cache: CacheNone,
		}}
	}

	// Cache miss (no entry at all or no further pages) — fetch all resources.
	return []TaskRequest{{
		Key:   TaskKey{Kind: KindFetchResources, Scope: targetType},
		Cache: CacheNone,
	}}
}

// relatedCacheSnapshot returns a flat map[string][]resource.Resource snapshot
// of the session caches suitable for the navigation resolver. On ID collision
// ResourceCache wins over LazyResourceCache.
func relatedCacheSnapshot(s *session.Session) map[string][]resource.Resource {
	snap := make(map[string][]resource.Resource, len(s.ResourceCache)+len(s.LazyResourceCache))
	maps.Copy(snap, s.LazyResourceCache)
	for shortName, entry := range s.ResourceCache {
		if existing, ok := snap[shortName]; ok {
			known := make(map[string]struct{}, len(entry.Resources))
			for _, r := range entry.Resources {
				known[r.ID] = struct{}{}
			}
			merged := append([]resource.Resource(nil), entry.Resources...)
			for _, r := range existing {
				if _, dup := known[r.ID]; !dup {
					merged = append(merged, r)
				}
			}
			snap[shortName] = merged
		} else {
			snap[shortName] = entry.Resources
		}
	}
	return snap
}

// resolveRelatedNavigate computes the navigation kind for a RelatedNavigateEvent.
// This is the runtime-native version of the resolver; the pure contract mirrors
// internal/tui/navigation_resolve.go §"Related-navigation contract (#278)".
//
//  1. Unknown target type          → NavigationKindFlash (error surfaced to the user).
//  2. Child type                   → NavigationKindEnterChildView.
//  3. Exact target already KNOWN   → NavigationKindDetail (cache hit).
//  4. FetchFilter + registered fetcher → NavigationKindFilteredList (FetchFilter preserved).
//  5. TargetID cache miss          → NavigationKindFilteredList (FilterText=TargetID).
//  6. RelatedIDs (one or many)     → NavigationKindFilteredList (RelatedIDs preserved).
//  7. Otherwise                    → NavigationKindResourceList.
func resolveRelatedNavigate(ev RelatedNavigateEvent, cache map[string][]resource.Resource) NavigationResult {
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

	// Multiple RelatedIDs (or single miss with no FetchFilter) → filtered list.
	if len(ev.RelatedIDs) > 0 {
		return NavigationResult{
			Kind:       NavigationKindFilteredList,
			TargetType: ev.TargetType,
			RelatedIDs: ev.RelatedIDs,
		}
	}

	// Default — fresh unfiltered list.
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
