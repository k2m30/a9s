// related_fetch.go provides a generic helper for fetching related resources.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/internal/resource"
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
// Callers MUST return Count=-1 (unknown) when isTruncated==true and 0 matches
// are found locally — never report a partial count as definitive.
func FetchRelatedTarget(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	if entry, ok := cache[target]; ok {
		return entry.Resources, entry.IsTruncated, nil
	}
	pf := resource.GetPaginatedFetcher(target)
	if pf == nil {
		return nil, false, nil
	}
	// Unwrap *Scope to *ServiceClients so the paginated fetcher (which still
	// type-asserts *ServiceClients per spec §3.4 — transport-only fetchers stay
	// untouched) works uniformly whether the dispatcher passed a bare
	// transport carrier or a Scope-wrapped one.
	if c := serviceClientsFromAny(clients); c != nil {
		clients = c
	}
	result, err := pf(ctx, clients, "")
	if err != nil {
		return nil, false, err
	}
	isTruncated := result.Pagination != nil && result.Pagination.IsTruncated
	return result.Resources, isTruncated, nil
}
