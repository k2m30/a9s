package aws

import (
	"context"

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
	// Bridge catalog → legacy internal/resource maps. The bridge body itself
	// lives in internal/resource (BridgeCatalogToLegacy) so internal/aws has
	// no resource.Register* call sites — the AS-947 grep gate (Wave 2.5)
	// enforces zero such calls outside the resource package.
	resource.BridgeCatalogToLegacy()
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
// (ecr_images); AS-812 / PR #402 adds messagingChildTypes
// (eb_rule_targets); AS-815 / PR #397 adds securityChildTypes
// (iam_group_members, role_policies); AS-816 / PR #400 adds cicdChildTypes
// (cb_builds, cb_build_logs, pipeline_stages). AS-947 / PR #TBD adds the
// remaining per-category child slices (compute, containers, monitoring,
// data, backup, databases, dns-cdn, networking, messaging) so the init()
// bodies in internal/aws/*.go can be deleted in the same PR.
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
