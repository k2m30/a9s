// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_shared.go contains helpers shared across the per-resource
// related-resource checker files (*_related.go) in this package.
package aws

import (
	"context"
	"errors"
	"maps"
	"slices"

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
// context its ARNs are to be read against.
func relatedListIn(ctx context.Context, clients any, cache resource.ResourceCache, target, region string) ([]resource.Resource, domain.RefContext, bool, error) {
	clients = clientsIn(clients, region)
	list, truncated, err := relatedResourcesFor(ctx, clients, cache, target)
	rc := refContext(clients, cache, target)
	if elsewhere(clients) {
		rc.Targets = list
	}
	return list, rc, truncated, err
}

// clientsIn is the client set that reads region: the one place a related read
// picks another Region's clients. The session's cache holds what the
// session's Region answered and nothing of any other, so every list read
// through the returned set skips it (elsewhere).
func clientsIn(clients any, region string) any {
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		return c.InRegion(region)
	}
	return clients
}

// readRegion is the Region clients read, "" for the session's.
func readRegion(clients any) string {
	if elsewhere(clients) {
		return clients.(*ServiceClients).Region
	}
	return ""
}

// elsewhere reports whether clients read a Region other than the session's.
func elsewhere(clients any) bool {
	c, ok := clients.(*ServiceClients)
	return ok && c != nil && c.parent != nil
}

// refContextIn is refContext for references that name region: they resolve
// against that Region, never against the session's cached list.
func refContextIn(clients any, cache resource.ResourceCache, target, region string) domain.RefContext {
	clients = clientsIn(clients, region)
	if elsewhere(clients) {
		cache = nil
	}
	return refContext(clients, cache, target)
}

// FirstPage reads target's first page through clients. A page of another
// Region is read once per session through the session's per-Region store, and
// none of its rows enter the session's own list of the type.
func FirstPage(ctx context.Context, clients any, target string) (resource.FetchResult, error) {
	pf := resource.GetPaginatedFetcher(target)
	if pf == nil {
		return resource.FetchResult{}, nil
	}
	if !elsewhere(clients) {
		return pf(ctx, clients, "")
	}
	c := clients.(*ServiceClients)
	return c.RegionLists().firstPage(ctx, c.Region+"/"+target, func(ctx context.Context) (resource.FetchResult, error) {
		return pf(ctx, c, "")
	})
}

// readListIn reads target's list in region and hands it to match, which
// reports what it found there. A list that could not be read is a place not
// read, and a failed read is its failure.
func readListIn(ctx context.Context, clients any, cache resource.ResourceCache, target, region string, match func(list []resource.Resource, rc domain.RefContext, truncated bool) relatedRead) relatedRead {
	list, rc, truncated, err := relatedListIn(ctx, clients, cache, target, region)
	if list == nil {
		return unreadBy(err)
	}
	return match(list, rc, truncated)
}

// answerIn is regionalAnswer for a checker that read in one Region.
func answerIn(clients any, target, region string, r relatedRead) resource.RelatedCheckResult {
	return regionalAnswer(clients, target, map[string]relatedRead{region: r})
}

// regionalAnswer is relatedAnswer over what a checker read in each Region,
// keyed by Region ("" or the clients' own is theirs), and the one place a
// result is marked with the Region it was read in. A row drills into one
// Region: the answer is the first Region's, the clients' own first, that found
// anything, and what any other Region found or could not read makes it a
// lower bound. When no Region found anything the answer is every Region's
// read joined, drilled into the first that could not be read.
func regionalAnswer(clients any, target string, reads map[string]relatedRead) resource.RelatedCheckResult {
	own := ""
	if c, ok := clients.(*ServiceClients); ok && c != nil {
		own = c.Region
	}
	merged := map[string]relatedRead{}
	for region, r := range reads {
		if region == own {
			region = ""
		}
		merged[region] = joinReads(merged[region], r)
	}
	regions := slices.Sorted(maps.Keys(merged))
	if len(regions) == 0 {
		return relatedAnswer(target, relatedRead{})
	}
	found := slices.IndexFunc(regions, func(region string) bool { return len(merged[region].ids) > 0 })
	if found < 0 {
		all := make([]relatedRead, len(regions))
		for i, region := range regions {
			all[i] = merged[region]
		}
		pick := regions[max(0, slices.IndexFunc(regions, func(region string) bool { return merged[region].unread }))]
		return inRegion(clients, pick, relatedAnswer(target, joinReads(all...)))
	}
	pick := regions[found]
	r := merged[pick]
	for _, region := range regions {
		if other := merged[region]; region != pick {
			r.partial = r.partial || len(other.ids) > 0 || other.partial || other.unread
			r.failure = errors.Join(r.failure, other.failure)
		}
	}
	return inRegion(clients, pick, relatedAnswer(target, r))
}

// refReadsIn resolves each Region's references against that Region
// (refContextIn).
func refReadsIn(clients any, cache resource.ResourceCache, target string, byRegion map[string][]string) map[string]relatedRead {
	reads := make(map[string]relatedRead, len(byRegion))
	for region, group := range byRegion {
		ids, dropped := resolveRefs(target, group, refContextIn(clients, cache, target, region))
		reads[region] = relatedRead{ids: ids, partial: dropped}
	}
	return reads
}

// refsByRegion groups references by the Region their ARN names, "" for a bare
// name.
func refsByRegion(refs []string) map[string][]string {
	byRegion := map[string][]string{}
	for _, ref := range refs {
		region := resource.RefRegion(ref)
		byRegion[region] = append(byRegion[region], ref)
	}
	return byRegion
}

// relatedRefsByRegion is relatedRefs for references that may name several
// Regions: each ARN resolves in the Region it names, a bare name in the
// clients' own.
func relatedRefsByRegion(clients any, cache resource.ResourceCache, target string, refs []string) resource.RelatedCheckResult {
	return regionalAnswer(clients, target, refReadsIn(clients, cache, target, refsByRegion(refs)))
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
