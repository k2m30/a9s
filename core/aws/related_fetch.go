// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_fetch.go provides a generic helper for fetching related resources.
package aws

import (
	"context"
	"slices"

	"github.com/k2m30/a9s/v3/core/resource"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// It aliases resource.DefaultPageSize, the single source of truth.
const DefaultPageSize = resource.DefaultPageSize

// FetchRelatedTarget returns the resource list for the given target type.
// It checks the ResourceCache first (returning cached data immediately), then
// falls back to calling the registered paginated fetcher for the first page only.
//
// Returns (resources, isTruncated, error):
//   - cache hit: returns cached resources and IsTruncated state, no AWS call.
//   - cache entry marked FieldsOnly (disk-restored rows, no RawStruct): a miss.
//   - cache miss + registered fetcher: fetches first page only, returns IsTruncated from pagination.
//   - cache miss + no fetcher: returns nil, false, nil (graceful no-op).
//
// A nil list means nothing was read: callers MUST answer UnknownRelated, since
// a count from a list that was never fetched is a guess. A list that WAS read
// but came back truncated is a different answer — whatever matched in it is
// real, and zero matches is a real zero so far, so callers carry isTruncated
// through and the row renders "(N+)".
func FetchRelatedTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := cache[target]; ok && !entry.FieldsOnly {
		// A present entry is a complete answer even when it holds nothing:
		// the executor seeds it straight from a fetcher that found zero
		// resources, and nil there would be indistinguishable from "no entry".
		resources := entry.Resources
		if resources == nil {
			resources = []resource.Resource{}
		}
		return resources, entry.IsTruncated || anyDegraded(resources), nil
	}
	pf := resource.GetPaginatedFetcher(target)
	if pf == nil {
		return nil, false, nil
	}
	result, err := pf(ctx, clients, "")
	if err != nil && len(result.Resources) == 0 {
		return nil, false, err
	}
	isTruncated := FetchIsPartial(result, err)
	resources := result.Resources
	if resources == nil {
		resources = []resource.Resource{}
	}
	return resources, isTruncated, nil
}

// FetchIsPartial reports whether a fetcher's answer is a subset of the list
// it was asked for. A page that ended with a continuation token, an error
// returned beside rows (a per-item failure the fetcher kept the rest of), and
// a row whose details could not be read each leave a match possibly unseen,
// so a count over the rows is a lower bound.
func FetchIsPartial(result resource.FetchResult, err error) bool {
	return err != nil || (result.Pagination != nil && result.Pagination.IsTruncated) || anyDegraded(result.Resources)
}

// anyDegraded reports whether list holds a row whose details could not be
// read: such a row may be the one that matches, so a scan over list is a
// lower bound.
func anyDegraded(list []resource.Resource) bool {
	return slices.ContainsFunc(list, func(r resource.Resource) bool { return r.Fields[DegradedFindingField] != "" })
}

// TargetClientMissing is resource.ClientMissing for the target type.
func TargetClientMissing(clients any, target string) error {
	return resource.ClientMissing(resource.TypeDef(target), clients)
}
