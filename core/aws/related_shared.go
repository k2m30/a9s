// related_shared.go contains helpers shared across the per-resource
// related-resource checker files (*_related.go) in this package.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/resource"
)

// relatedResourcesFor returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func relatedResourcesFor(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
