package aws

import "github.com/k2m30/a9s/v3/internal/catalog"

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

// allChildTypes returns the child-type catalog slice. Empty in AS-795a — the
// per-category PRs populate this as they migrate child fetchers off the
// internal/resource.childTypes map. Returning an empty slice now keeps the
// install contract uniform with allTopLevelTypes and lets catalog.FindChild
// panic loudly when called without an install.
func allChildTypes() []catalog.ResourceTypeDef {
	return nil
}
