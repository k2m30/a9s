package aws

import (
	"context"
	"sync"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ctEventsCheckerFor returns the RelatedChecker used for the CloudTrail Events
// pivot on every top-level resource type. Migrated types embed this in their
// catalog struct literal; zzz_ct_events_all_related.go falls back to
// AppendRelated for non-migrated types. The returned closure captures the
// owning resource type's short name so BuildCloudTrailFilter routes the
// LookupEvents call against the right ResourceName/Fields key.
func ctEventsCheckerFor(shortName string) domain.RelatedChecker {
	sn := shortName
	return func(_ context.Context, _ any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		filter := resource.BuildCloudTrailFilter(res, sn)
		if filter == nil {
			return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
		}
		return resource.RelatedCheckResult{
			TargetType:  "ct-events",
			Count:       -1,
			FetchFilter: filter,
		}
	}
}

// Install loads the AWS resource catalog into internal/catalog. MUST be called
// exactly once at program start (main() / TestMain) before any
// catalog.Find / catalog.All / catalog.ByCategory call.
//
// Replaces the package-init-time `var ResourceTypes = allTypes()` pattern in
// internal/catalog. By relocating the per-category catalog data into
// internal/aws (AS-795a), Install can populate the catalog without forcing
// internal/catalog to import internal/aws (which would close a cycle since
// internal/aws already depends on internal/catalog through this file and
// issue_enrichment.go).
//
// Install is idempotent on identical input — calling it twice produces no
// change. Calling SetTypes a second time with different data panics, which
// catches double-install bugs in tests where one TestMain forgot to use the
// same source slice as another.
//
// Per-category PRs AS-795b–m will populate the Fetcher / Wave2 / Related /
// Navigable / FieldKeys / FieldAliases / FetchByIDs / FilteredFetcher /
// ChildFetcher / IssueEnricherFieldKeys fields on each entry. AS-795a installs
// identity / columns / color / augment only — the init() bodies in
// internal/aws/*.go continue to populate internal/resource registries until
// each category PR migrates them.
func Install() {
	catalog.SetTypes(allTopLevelTypes())
	catalog.SetChildTypes(allChildTypes())
	bridgeOnce.Do(bridgeCatalogToLegacy)
}

// bridgeOnce gates bridgeCatalogToLegacy to a single execution per process.
// The legacy internal/resource maps include at least one panic-on-duplicate
// registrar (RegisterDetailEnricher) so the bridge must run exactly once even
// though catalog.SetTypes accepts idempotent calls. sync.Once is preferred
// over a plain bool to guard against the race between concurrent Install()
// callers landing both into bridgeCatalogToLegacy (per CodeRabbit on PR #397).
var bridgeOnce sync.Once //nolint:gochecknoglobals // process-scope install gate

// bridgeCatalogToLegacy populates the internal/resource legacy maps from the
// catalog struct fields. Required during the AS-795b–m transition because
// several consumers still read internal/resource maps directly (not through
// the catalog-aware accessors). Each Register* call only fires when the
// catalog has data for that field, so non-migrated types' init() registrations
// are not clobbered with zero values.
//
// AS-795n deletes both this bridge and the matching internal/resource maps
// once consumers migrate to catalog.Find / catalog.AllByWave2().
func bridgeCatalogToLegacy() {
	for _, rt := range catalog.All() {
		if rt.Fetcher != nil {
			resource.RegisterPaginated(rt.ShortName, rt.Fetcher)
		}
		if len(rt.FieldKeys) > 0 {
			resource.RegisterFieldKeys(rt.ShortName, rt.FieldKeys)
		}
		if len(rt.FieldAliases) > 0 {
			resource.RegisterFieldAliases(rt.ShortName, rt.FieldAliases)
		}
		if len(rt.Related) > 0 {
			resource.RegisterRelated(rt.ShortName, rt.Related)
		}
		if len(rt.Navigable) > 0 {
			resource.RegisterDefaultNavFields(rt.ShortName, rt.Navigable)
		}
		if rt.FetchByIDs != nil {
			resource.RegisterFetchByIDs(rt.ShortName, rt.FetchByIDs)
		}
		if rt.FilteredFetcher != nil {
			resource.RegisterFilteredPaginated(rt.ShortName, rt.FilteredFetcher)
		}
		if rt.Reveal != nil {
			resource.RegisterRevealFetcher(rt.ShortName, rt.Reveal)
		}
		if rt.DetailEnrich != nil {
			resource.RegisterDetailEnricher(rt.ShortName, rt.DetailEnrich)
		}
	}
	// Child types: replay catalog child-type entries onto the legacy
	// resource.childTypes + paginatedChildRegistry maps so consumers calling
	// resource.GetChildType / resource.GetPaginatedChildFetcher continue to
	// resolve migrated entries. Child loop introduced by AS-808 / PR #395
	// (containers, ecr_images); AS-815 / PR #397 adds DetailEnrich support
	// here so the role_policies detail enricher resolves through the bridge.
	for _, ct := range allChildTypes() {
		resource.RegisterChildType(ct)
		if ct.ChildFetcher != nil {
			resource.RegisterPaginatedChild(ct.ShortName, ct.ChildFetcher)
		}
		if len(ct.FieldKeys) > 0 {
			resource.RegisterFieldKeys(ct.ShortName, ct.FieldKeys)
		}
		if ct.DetailEnrich != nil {
			resource.RegisterDetailEnricher(ct.ShortName, ct.DetailEnrich)
		}
	}
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
	return all
}

// allChildTypes concatenates the per-category child-type catalog slices into
// one flat slice. The order mirrors allTopLevelTypes for consistency. Sibling
// category PRs (AS-795b/d–m) extend this by appending their own
// `<cat>ChildTypes` slice — using append-of-slices keeps the per-PR diff
// localized to one new `all = append(all, <cat>ChildTypes...)` line.
//
// First populated in AS-808 / PR #395 round-2 with containersChildTypes
// (ecr_images); AS-815 / PR #397 adds securityChildTypes
// (iam_group_members, role_policies).
func allChildTypes() []catalog.ResourceTypeDef {
	var all []catalog.ResourceTypeDef
	all = append(all, containersChildTypes...)
	all = append(all, securityChildTypes...)
	return all
}
