// navigation_resolve.go contains the pure resolver helper for related-navigation
// dispatching. handleRelatedNavigate (in app_related.go) uses this resolver to
// determine what view should be pushed; the resolver itself has no side effects
// and is independently testable.

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

	// FetchFilter path: use server-side filtered fetcher.
	if len(msg.FetchFilter) > 0 {
		return NavigationResult{
			Kind:        KindFilteredList,
			TargetType:  msg.TargetType,
			FetchFilter: msg.FetchFilter,
		}
	}

	// TargetID path: exact resource navigation.
	if msg.TargetID != "" {
		if cacheHit(cache, msg.TargetType, msg.TargetID) {
			return NavigationResult{
				Kind:       KindDetail,
				TargetType: msg.TargetType,
				TargetID:   msg.TargetID,
			}
		}
		return NavigationResult{
			Kind:       KindFilteredList,
			TargetType: msg.TargetType,
			TargetID:   msg.TargetID,
			FilterText: msg.TargetID,
		}
	}

	// RelatedIDs path: single ID with cache hit → detail directly.
	if len(msg.RelatedIDs) == 1 {
		if cacheHit(cache, msg.TargetType, msg.RelatedIDs[0]) {
			return NavigationResult{
				Kind:       KindDetail,
				TargetType: msg.TargetType,
				RelatedIDs: msg.RelatedIDs,
			}
		}
	}

	// Multiple RelatedIDs (or single miss) → filtered list.
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
