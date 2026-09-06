// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_fetch.go provides a generic helper for fetching related resources.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/resource"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// Re-exported from the resource package so existing aws.DefaultPageSize call
// sites keep working without churn. The single source of truth is
// resource.DefaultPageSize.
const DefaultPageSize = resource.DefaultPageSize

// FetchRelatedTarget returns the resource list for the given target type.
// It checks the ResourceCache first (returning cached data immediately), then
// falls back to calling the registered paginated fetcher for the first page only.
//
// Returns (resources, isTruncated, error):
//   - cache hit: returns cached resources and IsTruncated state, no AWS call.
//   - cache miss + registered fetcher: fetches first page only, returns IsTruncated from pagination.
//   - cache miss + no fetcher: returns nil, false, nil (graceful no-op).
//
// A nil list means nothing was read: callers MUST answer UnknownRelated, since
// a count from a list that was never fetched is a guess. A list that WAS read
// but came back truncated is a different answer — whatever matched in it is
// real, and zero matches is a real zero so far, so callers carry isTruncated
// through and the row renders "(N+)".
func FetchRelatedTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := cache[target]; ok {
		// A present entry is a complete answer even when it holds nothing:
		// the executor seeds it straight from a fetcher that found zero
		// resources, and nil there would be indistinguishable from "no entry".
		resources := entry.Resources
		if resources == nil {
			resources = []resource.Resource{}
		}
		return resources, entry.IsTruncated, nil
	}
	pf := resource.GetPaginatedFetcher(target)
	if pf == nil {
		return nil, false, nil
	}
	result, err := pf(ctx, clients, "")
	if err != nil && len(result.Resources) == 0 {
		return nil, false, err
	}
	// Rows returned beside an error are a per-item partial failure (the
	// fetcher already kept the degraded rows): a proven subset, answered as
	// truncated rather than thrown away.
	isTruncated := err != nil || (result.Pagination != nil && result.Pagination.IsTruncated)
	resources := result.Resources
	if resources == nil {
		resources = []resource.Resource{}
	}
	return resources, isTruncated, nil
}
