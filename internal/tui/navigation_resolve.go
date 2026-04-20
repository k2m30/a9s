// navigation_resolve.go contains the pure resolver helper for related-navigation
// dispatching. handleRelatedNavigate (in app_related.go) uses this resolver to
// determine what view should be pushed; the resolver itself has no side effects
// and is independently testable.
//
// Related-navigation contract (#278):
//
//  1. Unknown target type          → KindFlash (error surfaced to the user).
//  2. Child type                   → KindEnterChildView.
//  3. Exact target already KNOWN   → KindDetail. "Known" means the row is in
//     the cache. Covers TargetID cache hit and single RelatedIDs cache hit.
//     This precedes FetchFilter deliberately — drilling into a row we already
//     have wastes no API call and preserves user context.
//  4. FetchFilter with a registered FilteredPaginatedFetcher for the target
//     type → KindFilteredList with FetchFilter preserved. Checkers may set
//     FetchFilter on types without a registered fetcher as a hint; the
//     runtime ignores the filter in that case rather than failing.
//  5. TargetID cache miss          → KindFilteredList with FilterText set to
//     TargetID (the list filters by the ID string).
//  6. RelatedIDs (one or many)     → KindFilteredList with RelatedIDs preserved.
//  7. Otherwise                    → KindResourceList (fresh unfiltered list).

package tui

import (
	"fmt"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// NavigationKind enumerates the possible outcomes of resolving a RelatedNavigateMsg.
type NavigationKind int

const (
	KindUnknown        NavigationKind = iota
	KindResourceList                  // push a fresh ResourceList of TargetType
	KindFilteredList                  // push a ResourceList filtered by RelatedIDs/FetchFilter
	KindDetail                        // push a DetailView for the specific TargetID (cache hit)
	KindEnterChildView                // push a child view (e.g. s3_objects under an s3 bucket)
	KindFlash                         // emit a FlashMsg (typically error path)
)

// NavigationResult is the pure-function output of resolveRelatedNavigate.
// All fields except Kind are conditionally populated based on Kind:
//   - KindResourceList: TargetType set
//   - KindFilteredList: TargetType, RelatedIDs, FetchFilter, FilterText set
//   - KindDetail: TargetType, TargetID set
//   - KindEnterChildView: TargetType (child short name), ChildContext set
//   - KindFlash: FlashMessage, FlashIsError set
type NavigationResult struct {
	Kind         NavigationKind
	TargetType   string
	TargetID     string
	RelatedIDs   []string
	FetchFilter  map[string]string
	FilterText   string
	ChildContext map[string]string
	FlashMessage string
	FlashIsError bool
}

// ResolveRelatedNavigate computes what view should be pushed for a related-navigation
// message, without mutating any state. handleRelatedNavigate uses this resolver to
// drive its actual view-stack push.
//
// Ordering follows the related-navigation contract documented at the top of
// this file. Exact drill-ins (cache-known TargetID or single RelatedID) take
// precedence over FetchFilter so we never issue a filtered fetch when the
// target row is already visible.
func ResolveRelatedNavigate(msg messages.RelatedNavigateMsg, cache map[string][]resource.Resource) NavigationResult {
	_, isChild, found := resource.ResolveNavigationTarget(msg.TargetType)
	if !found {
		return NavigationResult{
			Kind:         KindFlash,
			FlashMessage: fmt.Sprintf("unknown resource type: %s", msg.TargetType),
			FlashIsError: true,
		}
	}

	// Child type → enter child view (e.g. s3_objects under an s3 bucket).
	if isChild {
		return NavigationResult{
			Kind:       KindEnterChildView,
			TargetType: msg.TargetType,
			RelatedIDs: msg.RelatedIDs,
		}
	}

	// Exact drill-in, TargetID cache hit → KindDetail.
	// This precedes FetchFilter: when we already have the row cached, drilling
	// in preserves user context and saves an API call.
	if msg.TargetID != "" && cacheHit(cache, msg.TargetType, msg.TargetID) {
		return NavigationResult{
			Kind:       KindDetail,
			TargetType: msg.TargetType,
			TargetID:   msg.TargetID,
		}
	}

	// Exact drill-in, single RelatedID cache hit → KindDetail.
	if len(msg.RelatedIDs) == 1 && cacheHit(cache, msg.TargetType, msg.RelatedIDs[0]) {
		return NavigationResult{
			Kind:       KindDetail,
			TargetType: msg.TargetType,
			RelatedIDs: msg.RelatedIDs,
		}
	}

	// FetchFilter path: only honor when a filtered paginated fetcher is
	// registered for the target type. Otherwise fall through — checker may
	// have populated FetchFilter as a hint but the runtime cannot dispatch
	// without a registered fetcher.
	if len(msg.FetchFilter) > 0 && resource.GetFilteredPaginatedFetcher(msg.TargetType) != nil {
		return NavigationResult{
			Kind:        KindFilteredList,
			TargetType:  msg.TargetType,
			FetchFilter: msg.FetchFilter,
		}
	}

	// TargetID cache miss → KindFilteredList filtering by the ID string.
	if msg.TargetID != "" {
		return NavigationResult{
			Kind:       KindFilteredList,
			TargetType: msg.TargetType,
			TargetID:   msg.TargetID,
			FilterText: msg.TargetID,
		}
	}

	// Multiple RelatedIDs (or single miss with no FetchFilter) → filtered list.
	if len(msg.RelatedIDs) > 0 {
		return NavigationResult{
			Kind:       KindFilteredList,
			TargetType: msg.TargetType,
			RelatedIDs: msg.RelatedIDs,
		}
	}

	// Default — fresh unfiltered list.
	return NavigationResult{
		Kind:       KindResourceList,
		TargetType: msg.TargetType,
	}
}

// cacheHit reports whether a resource with the given ID exists in the cache for targetType.
func cacheHit(cache map[string][]resource.Resource, targetType, id string) bool {
	for _, r := range cache[targetType] {
		if r.ID == id {
			return true
		}
	}
	return false
}
