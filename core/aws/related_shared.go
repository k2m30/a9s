// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_shared.go contains helpers shared across the per-resource
// related-resource checker files (*_related.go) in this package.
package aws

import (
	"context"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// relatedResourcesFor returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func relatedResourcesFor(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	return FetchRelatedTarget(ctx, clients, cache, target)
}

// relatedListIn is relatedResourcesFor for a target that lives in region
// rather than in the session's own: the list as read there, and the reference
// context its ARNs are to be read against. The session's cache holds what the
// session's region answered and nothing of any other, so a read elsewhere goes
// to the fetcher through that region's clients.
func relatedListIn(ctx context.Context, clients any, cache resource.ResourceCache, target, region string) ([]resource.Resource, domain.RefContext, bool, error) {
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		if elsewhere := c.InRegion(region); elsewhere != c {
			list, truncated, err := relatedResourcesFor(ctx, elsewhere, nil, target)
			rc := refContext(elsewhere, nil, target)
			rc.Targets = list
			return list, rc, truncated, err
		}
	}
	list, truncated, err := relatedResourcesFor(ctx, clients, cache, target)
	return list, refContext(clients, cache, target), truncated, err
}

// inRegion marks a result with the region it was read in, when that is not
// the region the session's own clients answer for. A drill into the row reads
// the same region, or it lists another one and finds none of what the count
// names. InRegion decides what counts as elsewhere, so a checker and its drill
// cannot disagree about it.
func inRegion(clients any, region string, r resource.RelatedCheckResult) resource.RelatedCheckResult {
	if c, ok := clients.(*ServiceClients); ok && c != nil && c.InRegion(region) != c {
		return r.WithRegion(region)
	}
	return r
}

// arnRegionOf returns the region an ARN of service names. A bare name, another
// service's ARN and a global ARN name none, which reads as the session's own.
func arnRegionOf(ref, service string) string {
	if _, ok := ARNForService(ref, service); !ok {
		return ""
	}
	return resource.RefRegion(ref)
}
