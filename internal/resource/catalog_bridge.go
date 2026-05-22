package resource

import (
	"sync"

	"github.com/k2m30/a9s/v3/internal/catalog"
)

// bridgeOnce gates BridgeCatalogToLegacy to a single execution per process.
// The legacy registrars (e.g. RegisterDetailEnricher) panic on duplicate
// registration, so the bridge must run exactly once even when multiple
// callers want to ensure it has fired.
var bridgeOnce sync.Once //nolint:gochecknoglobals // process-scope install gate

// BridgeCatalogToLegacy populates the legacy internal/resource registries
// (relatedRegistry, paginatedFetchers, navigableFieldRegistry, etc.) from the
// installed catalog struct fields. Required during the AS-795b–p transition
// because several consumers (internal/runtime, internal/tui) still read
// internal/resource maps directly via GetRelated / GetPaginatedFetcher /
// GetPaginatedChildFetcher / GetChildType / GetFetchByIDs.
//
// MUST be called after catalog.SetTypes + catalog.SetChildTypes. Typically
// invoked by aws.Install() at program start; idempotent (sync.Once).
//
// Each Register* call only fires when the catalog has data for that field
// so non-migrated types are not clobbered with zero values.
//
// Moved out of internal/aws/install.go in AS-947 / PR #TBD so the grep gate
// `rg 'resource\.Register*' internal/aws/` is satisfied. Deletion of this
// bridge (and the underlying legacy maps) is tracked separately when every
// consumer migrates to catalog.Find / catalog.AllByWave2().
func BridgeCatalogToLegacy() {
	bridgeOnce.Do(bridgeOnceFn)
}

func bridgeOnceFn() {
	for _, rt := range catalog.All() {
		if rt.Fetcher != nil {
			RegisterPaginated(rt.ShortName, rt.Fetcher)
		}
		if len(rt.FieldKeys) > 0 {
			RegisterFieldKeys(rt.ShortName, rt.FieldKeys)
		}
		if len(rt.FieldAliases) > 0 {
			RegisterFieldAliases(rt.ShortName, rt.FieldAliases)
		}
		if len(rt.Related) > 0 {
			RegisterRelated(rt.ShortName, rt.Related)
		}
		if len(rt.Navigable) > 0 {
			RegisterDefaultNavFields(rt.ShortName, rt.Navigable)
		}
		if rt.FetchByIDs != nil {
			RegisterFetchByIDs(rt.ShortName, rt.FetchByIDs)
		}
		if rt.FilteredFetcher != nil {
			RegisterFilteredPaginated(rt.ShortName, rt.FilteredFetcher)
		}
		if rt.Reveal != nil {
			RegisterRevealFetcher(rt.ShortName, rt.Reveal)
		}
		if rt.DetailEnrich != nil {
			RegisterDetailEnricher(rt.ShortName, rt.DetailEnrich)
		}
		if len(rt.IssueEnricherFieldKeys) > 0 {
			RegisterIssueEnricherFieldKeys(rt.ShortName, rt.IssueEnricherFieldKeys)
		}
	}
	// Child types: replay catalog child-type entries onto the legacy
	// childTypes + paginatedChildRegistry maps so consumers calling
	// GetChildType / GetPaginatedChildFetcher continue to resolve migrated
	// entries.
	for _, ct := range catalog.AllChildren() {
		RegisterChildType(ct)
		if ct.ChildFetcher != nil {
			RegisterPaginatedChild(ct.ShortName, ct.ChildFetcher)
		}
		if len(ct.FieldKeys) > 0 {
			RegisterFieldKeys(ct.ShortName, ct.FieldKeys)
		}
		if ct.DetailEnrich != nil {
			RegisterDetailEnricher(ct.ShortName, ct.DetailEnrich)
		}
	}
}
