package demo

import (
	"sort"
	"strings"
	"sync"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// DemoPageSize is the number of items returned per page in demo mode.
const DemoPageSize = 5

var (
	mu               sync.Mutex
	demoOverflow     = map[string][]resource.Resource{}
	demoChildOverflow = map[string][]resource.Resource{}
)

// childKey builds a deterministic key from childType + sorted parent context.
func childKey(childType string, parentCtx map[string]string) string {
	keys := make([]string, 0, len(parentCtx))
	for k := range parentCtx {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{childType}
	for _, k := range keys {
		parts = append(parts, k+"="+parentCtx[k])
	}
	return strings.Join(parts, "|")
}

// GetResourcesPaginated returns the first page of demo resources for the given type.
// Returns up to DemoPageSize items with pagination metadata.
// Returns (zero, false) for unknown resource types.
func GetResourcesPaginated(resourceType string) (resource.FetchResult, bool) {
	gen, ok := demoData[resourceType]
	if !ok {
		return resource.FetchResult{}, false
	}
	all := gen()

	mu.Lock()
	defer mu.Unlock()

	if len(all) <= DemoPageSize {
		delete(demoOverflow, resourceType)
		return resource.FetchResult{
			Resources: all,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				PageSize:    len(all),
				TotalHint:   len(all),
			},
		}, true
	}

	page := all[:DemoPageSize]
	demoOverflow[resourceType] = all[DemoPageSize:]
	return resource.FetchResult{
		Resources: page,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "demo-overflow",
			PageSize:    DemoPageSize,
			TotalHint:   len(all),
		},
	}, true
}

// GetMoreResources returns the next page of demo resources after GetResourcesPaginated.
// Returns (zero, false) when no more items are available.
func GetMoreResources(resourceType string) (resource.FetchResult, bool) {
	mu.Lock()
	defer mu.Unlock()

	overflow, ok := demoOverflow[resourceType]
	if !ok || len(overflow) == 0 {
		return resource.FetchResult{}, false
	}
	delete(demoOverflow, resourceType)
	return resource.FetchResult{
		Resources: overflow,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    len(overflow),
		},
	}, true
}

// GetChildResourcesPaginated returns the first page of demo child resources.
// Returns up to DemoPageSize items with pagination metadata.
// Returns (zero, false) for unknown child types.
func GetChildResourcesPaginated(childType string, parentCtx map[string]string) (resource.FetchResult, bool) {
	gen, ok := childDemoData[childType]
	if !ok {
		return resource.FetchResult{}, false
	}
	all := gen(parentCtx)

	mu.Lock()
	defer mu.Unlock()

	key := childKey(childType, parentCtx)
	if len(all) <= DemoPageSize {
		delete(demoChildOverflow, key)
		return resource.FetchResult{
			Resources: all,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				PageSize:    len(all),
				TotalHint:   len(all),
			},
		}, true
	}

	page := all[:DemoPageSize]
	demoChildOverflow[key] = all[DemoPageSize:]
	return resource.FetchResult{
		Resources: page,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "demo-child-overflow",
			PageSize:    DemoPageSize,
			TotalHint:   len(all),
		},
	}, true
}

// GetMoreChildResources returns the next page of demo child resources.
// Returns (zero, false) when no more items are available.
func GetMoreChildResources(childType string, parentCtx map[string]string) (resource.FetchResult, bool) {
	mu.Lock()
	defer mu.Unlock()

	key := childKey(childType, parentCtx)
	overflow, ok := demoChildOverflow[key]
	if !ok || len(overflow) == 0 {
		return resource.FetchResult{}, false
	}
	delete(demoChildOverflow, key)
	return resource.FetchResult{
		Resources: overflow,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    len(overflow),
		},
	}, true
}
