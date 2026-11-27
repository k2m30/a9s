// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"maps"
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ctEventsCheckerFor returns the RelatedChecker every type's CloudTrail
// Events pivot is wired to in its own catalog literal. The returned closure
// captures the owning type's short name, which is what CloudTrailFilterIn
// reads the lookup key and Region from, so the row defers to the same lookup
// the `t` hotkey sends.
func ctEventsCheckerFor(shortName string) domain.RelatedChecker {
	sn := shortName
	return func(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		filter := resource.CloudTrailFilterIn(res, sn, readRegion(clients))
		if filter == nil {
			return foundNone("ct-events", "filter")
		}
		return resource.DeferredRelated("ct-events", filter)
	}
}

// ctRegionUSEast1 is the CloudTrailRegion of a global service: IAM, STS,
// CloudFront and Route 53 record their events in us-east-1 whatever Region
// the operator is browsing
// (docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-concepts.html
// §Global service events).
func ctRegionUSEast1(domain.Resource) string { return "us-east-1" }

// ctRegionOfTrail is the CloudTrailRegion of a trail: a trail's own
// configuration calls are recorded in its home Region.
func ctRegionOfTrail(r domain.Resource) string { return r.Fields["home_region"] }

// ctRegionOfWebACL is the CloudTrailRegion of a web ACL: every wafv2 call
// about it is served by the endpoint of the Region its scope names, and that
// is where the call is recorded.
func ctRegionOfWebACL(r domain.Resource) string { return wafRegionOf(r.Fields["scope"]) }

// Install loads the AWS resource catalog into core/catalog. MUST be called
// exactly once at program start (main() / TestMain) before any
// catalog.Find / catalog.All call.
//
// The per-category catalog data lives in core/aws so Install can populate
// the catalog without forcing core/catalog to import core/aws (which
// would close a cycle: core/aws already depends on core/catalog through
// this file and issue_enrichment.go).
//
// Install is idempotent on identical input — calling it twice produces no
// change. Calling SetTypes a second time with different data panics, which
// catches double-install bugs in tests where one TestMain forgot to use the
// same source slice as another.
func Install() {
	catalog.SetTypes(bindTypes(allTopLevelTypes()))
	catalog.SetChildTypes(bindTypes(allChildTypes()))
	// Production consumers read the catalog via resource.Get* wrappers.
	// Those wrappers consult per-surface test-only override tables first
	// (populated via resource.Set*ForTest / AppendRelated, which panic
	// outside test binaries), so in production the catalog installed here is
	// the only source.
}

// bindTypes binds every function of a type that returns rows to the type it
// is registered under: each row it returns carries that type and has passed
// the Wave 1 door, whichever producer built it.
func bindTypes(types []catalog.ResourceTypeDef) []catalog.ResourceTypeDef {
	for i := range types {
		td := &types[i]
		short := td.ShortName
		bind := func(rows []resource.Resource) { bindRows(short, rows) }
		bindPage := func(fetch domain.PaginatedFetcher) domain.PaginatedFetcher {
			if fetch == nil {
				return nil
			}
			return func(ctx context.Context, clients any, token string) (resource.FetchResult, error) {
				res, err := fetch(ctx, clients, token)
				bind(res.Resources)
				return res, err
			}
		}
		td.Fetcher = bindPage(td.Fetcher)
		td.AvailabilityFetcher = bindPage(td.AvailabilityFetcher)
		if fetch := td.FilteredFetcher; fetch != nil {
			td.FilteredFetcher = func(ctx context.Context, clients any, filter map[string]string, token string) (resource.FetchResult, error) {
				res, err := fetch(ctx, clients, filter, token)
				bind(res.Resources)
				return res, err
			}
		}
		if fetch := td.ChildFetcher; fetch != nil {
			td.ChildFetcher = func(ctx context.Context, clients any, parent domain.ParentContext, token string) (resource.FetchResult, error) {
				res, err := fetch(ctx, clients, parent, token)
				bind(res.Resources)
				return res, err
			}
		}
		if fetch := td.FetchByIDs; fetch != nil {
			td.FetchByIDs = func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
				rows, err := fetch(ctx, clients, ids)
				bind(rows)
				return rows, err
			}
		}
		if stub := td.StubCreator; stub != nil {
			td.StubCreator = func(id string) domain.Resource {
				rows := []resource.Resource{stub(id)}
				bind(rows)
				return rows[0]
			}
		}
		if enrich := td.DetailEnrich; enrich != nil {
			td.DetailEnrich = func(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
				before := slices.Clone(res.Findings)
				out, err := enrich(ctx, clients, res)
				if !domain.DetailResultApplies(err) {
					return out, err
				}
				return rebindEnriched(short, before, out), err
			}
		}
	}
	return types
}

func bindRows(shortName string, rows []resource.Resource) {
	for i := range rows {
		rows[i].Type = shortName
		resolveDegraded(shortName, &rows[i])
		emitWave1(&rows[i])
	}
}

// rebindEnriched passes through the Wave 1 door only the findings a detail
// enricher added to a bound row. The findings the row came in with were judged
// where they were raised, by Wave 1, Wave 2 or a build that wrote the disk
// cache, and stand as they are.
func rebindEnriched(shortName string, before []domain.Finding, out resource.Resource) resource.Resource {
	resolveDegraded(shortName, &out)
	var raised []domain.Finding
	for _, f := range out.Findings {
		if !slices.Contains(before, f) {
			raised = append(raised, f)
		}
	}
	if len(raised) == 0 {
		return out
	}
	out.Type = shortName
	added := rebuiltWave1(out, raised...)
	added.AttentionDetails = maps.Clone(out.AttentionDetails)
	emitWave1(&added)
	kept := make([]domain.Finding, 0, len(out.Findings))
	for _, f := range out.Findings {
		if slices.Contains(before, f) || slices.Contains(added.Findings, f) {
			kept = append(kept, f)
		}
	}
	out = rebuiltWave1(out, kept...)
	out.AttentionDetails = added.AttentionDetails
	return out
}

// BindForTest is what the registration does to the rows a function of
// shortName returns, for a test that calls such a function directly.
func BindForTest(shortName string, rows []resource.Resource) {
	if !testing.Testing() {
		panic("BindForTest called outside a test binary")
	}
	bindRows(shortName, rows)
}

// allTopLevelTypes concatenates the per-category top-level catalog slices into
// one flat slice. The order of categories here is the order they appear in the
// main menu and in catalog.All().
func allTopLevelTypes() []catalog.ResourceTypeDef {
	all := make([]catalog.ResourceTypeDef, 0,
		len(computeTypes)+len(containersTypes)+len(networkingTypes)+
			len(databasesTypes)+len(monitoringTypes)+len(messagingTypes)+
			len(secretsTypes)+len(dnsCdnTypes)+len(securityTypes)+
			len(cicdTypes)+len(dataTypes)+len(backupTypes))
	all = append(all, computeTypes...)
	all = append(all, containersTypes...)
	all = append(all, networkingTypes...)
	all = append(all, databasesTypes...)
	all = append(all, monitoringTypes...)
	all = append(all, messagingTypes...)
	all = append(all, secretsTypes...)
	all = append(all, dnsCdnTypes...)
	all = append(all, securityTypes...)
	all = append(all, cicdTypes...)
	all = append(all, dataTypes...)
	all = append(all, backupTypes...)
	return withPolicyFindings(all)
}

// allChildTypes concatenates the per-category child-type catalog slices into
// one flat slice. The order mirrors allTopLevelTypes for consistency; each
// category appends its own `<cat>ChildTypes` slice.
func allChildTypes() []catalog.ResourceTypeDef {
	var all []catalog.ResourceTypeDef
	all = append(all, computeChildTypes...)
	all = append(all, containersChildTypes...)
	all = append(all, networkingChildTypes...)
	all = append(all, databasesChildTypes...)
	all = append(all, monitoringChildTypes...)
	all = append(all, messagingChildTypes...)
	all = append(all, securityChildTypes...)
	all = append(all, dnsCdnChildTypes...)
	all = append(all, cicdChildTypes...)
	all = append(all, dataChildTypes...)
	all = append(all, backupChildTypes...)
	return all
}
