package catalog

import (
	"fmt"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// ResourceTypes is the declarative registry of all a9s resource types.
// It is static — no init(), no Register* calls. Per-category PRs (04b–04m)
// populate this slice. Until then it is empty and all lookups fall through
// to the legacy registry in internal/resource.
var ResourceTypes = allTypes() //nolint:gochecknoglobals // static catalog: intentional package-level var

// allTypes concatenates per-category slices into the full catalog.
// Per-category PRs (04b–04m) add their slice here; PR-04n removes this func.
func allTypes() []ResourceTypeDef {
	var all []ResourceTypeDef
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

// Find returns the ResourceTypeDef for the given name (ShortName or Alias),
// or nil if the catalog does not have an entry for it yet.
// Case-insensitive match against ShortName and all Aliases.
func Find(name string) *ResourceTypeDef {
	nameLower := toLower(name)
	for i := range ResourceTypes {
		if toLower(ResourceTypes[i].ShortName) == nameLower {
			return &ResourceTypes[i]
		}
		for _, alias := range ResourceTypes[i].Aliases {
			if toLower(alias) == nameLower {
				return &ResourceTypes[i]
			}
		}
	}
	return nil
}

// All returns a copy of the full ResourceTypes slice.
func All() []ResourceTypeDef {
	result := make([]ResourceTypeDef, len(ResourceTypes))
	copy(result, ResourceTypes)
	return result
}

// AllShortNames returns the ShortName of every type in the catalog.
func AllShortNames() []string {
	names := make([]string, len(ResourceTypes))
	for i, rt := range ResourceTypes {
		names[i] = rt.ShortName
	}
	return names
}

// ByCategory returns all resource types with the given Category value.
// Returns nil when the catalog has no entries for that category.
func ByCategory(cat string) []ResourceTypeDef {
	var result []ResourceTypeDef
	for _, rt := range ResourceTypes {
		if rt.Category == cat {
			result = append(result, rt)
		}
	}
	return result
}

// RegisterProject sets the Project (DetailProjector) for the catalog entry with
// the given shortName. Called from init() in packages whose projector cannot be
// imported from internal/catalog without creating an import cycle. No-op when
// the shortName is not in the catalog.
func RegisterProject(shortName string, p domain.DetailProjector) {
	nameLower := toLower(shortName)
	for i := range ResourceTypes {
		if toLower(ResourceTypes[i].ShortName) == nameLower {
			ResourceTypes[i].Project = p
			return
		}
	}
}

// RegisterAugment sets the Augment hook for the catalog entry with the given
// shortName. Same cycle-breaking pattern as RegisterProject. No-op when the
// shortName is not in the catalog.
func RegisterAugment(shortName string, aug domain.Augmenter) {
	nameLower := toLower(shortName)
	for i := range ResourceTypes {
		if toLower(ResourceTypes[i].ShortName) == nameLower {
			ResourceTypes[i].Augment = aug
			return
		}
	}
}

// findIndex returns the index of the catalog row with ShortName matching name
// (case-insensitive). Returns -1 when no row matches. Used by the Register*
// mutators below.
func findIndex(name string) int {
	nameLower := toLower(name)
	for i := range ResourceTypes {
		if toLower(ResourceTypes[i].ShortName) == nameLower {
			return i
		}
	}
	return -1
}

// RegisterFetcher sets the Wave 1 paginated fetcher on the named catalog row.
// Panics if the row already has a non-nil Fetcher (duplicate registration).
// No-op when the row is missing — the legacy registry fallback continues to
// own un-migrated types.
func RegisterFetcher(shortName string, fn domain.PaginatedFetcher) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	if ResourceTypes[i].Fetcher != nil {
		panic(fmt.Sprintf("catalog.RegisterFetcher(%q): duplicate registration — row already has a non-nil Fetcher", shortName))
	}
	ResourceTypes[i].Fetcher = fn
}

// RegisterWave2 sets the Wave 2 enricher on the named catalog row. The value
// is a concrete aws.IssueEnricher stored as any to avoid an import cycle.
// Panics if the row already has a non-nil Wave2. No-op when the row is
// missing.
func RegisterWave2(shortName string, enr any) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	if ResourceTypes[i].Wave2 != nil {
		panic(fmt.Sprintf("catalog.RegisterWave2(%q): duplicate registration — row already has a non-nil Wave2", shortName))
	}
	ResourceTypes[i].Wave2 = enr
}

// RegisterRelated sets the related-resource defs on the named catalog row.
// Panics if the row already has non-empty Related. No-op when the row is
// missing.
func RegisterRelated(shortName string, defs []domain.RelatedDef) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	if len(ResourceTypes[i].Related) > 0 {
		panic(fmt.Sprintf("catalog.RegisterRelated(%q): duplicate registration — row already has non-empty Related", shortName))
	}
	ResourceTypes[i].Related = defs
}

// RegisterNavigable sets the navigable-field defs on the named catalog row.
// Panics if the row already has non-empty Navigable. No-op when the row is
// missing.
func RegisterNavigable(shortName string, fields []domain.NavigableField) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	if len(ResourceTypes[i].Navigable) > 0 {
		panic(fmt.Sprintf("catalog.RegisterNavigable(%q): duplicate registration — row already has non-empty Navigable", shortName))
	}
	ResourceTypes[i].Navigable = fields
}

// RegisterFieldKeys sets the fetcher-produced FieldKeys on the named catalog
// row. Panics on duplicate (non-empty existing slice). No-op when the row is
// missing.
func RegisterFieldKeys(shortName string, keys []string) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	if len(ResourceTypes[i].FieldKeys) > 0 {
		panic(fmt.Sprintf("catalog.RegisterFieldKeys(%q): duplicate registration — row already has non-empty FieldKeys", shortName))
	}
	ResourceTypes[i].FieldKeys = keys
}

// RegisterIssueEnricherFieldKeys appends Wave 2 field keys on the named
// catalog row. Idempotent — duplicates are deduplicated. Multiple enrichers
// may target the same type and union their keys here. Original order is
// preserved; new (non-duplicate) keys are appended in input order. No-op
// when the row is missing.
func RegisterIssueEnricherFieldKeys(shortName string, keys []string) {
	i := findIndex(shortName)
	if i < 0 {
		return
	}
	existing := ResourceTypes[i].IssueEnricherFieldKeys
	seen := make(map[string]bool, len(existing))
	for _, k := range existing {
		seen[k] = true
	}
	for _, k := range keys {
		if !seen[k] {
			existing = append(existing, k)
			seen[k] = true
		}
	}
	ResourceTypes[i].IssueEnricherFieldKeys = existing
}

// childTypes holds catalog-registered child-view definitions keyed by
// ShortName. Child views are NOT in the top-level ResourceTypes slice.
// FindChild is the authoritative accessor.
var childTypes = map[string]ResourceTypeDef{} //nolint:gochecknoglobals // catalog child registry

// RegisterChildView registers a child-view ResourceTypeDef into the catalog's
// child registry, keyed by ShortName. Panics on duplicate registration for
// the same ShortName.
func RegisterChildView(child ResourceTypeDef) {
	key := toLower(child.ShortName)
	if _, exists := childTypes[key]; exists {
		panic(fmt.Sprintf("catalog.RegisterChildView(%q): duplicate registration", child.ShortName))
	}
	childTypes[key] = child
}

// FindChild returns the child-view ResourceTypeDef registered under the given
// ShortName, or nil if not in the catalog's child registry. Case-insensitive.
func FindChild(shortName string) *ResourceTypeDef {
	key := toLower(shortName)
	if c, ok := childTypes[key]; ok {
		return &c
	}
	return nil
}

// AddForTest appends a synthetic ResourceTypeDef to the top-level
// ResourceTypes slice. Test-only scaffolding for mutator tests; pair with
// RemoveForTest in t.Cleanup. No-op for empty ShortName.
func AddForTest(def ResourceTypeDef) {
	if def.ShortName == "" {
		return
	}
	ResourceTypes = append(ResourceTypes, def)
}

// RemoveForTest removes the catalog row whose ShortName matches shortName.
// Pair with AddForTest. Also clears any matching entry in childTypes.
// No-op when the shortName is not present.
func RemoveForTest(shortName string) {
	nameLower := toLower(shortName)
	for i := range ResourceTypes {
		if toLower(ResourceTypes[i].ShortName) == nameLower {
			ResourceTypes = append(ResourceTypes[:i], ResourceTypes[i+1:]...)
			break
		}
	}
	delete(childTypes, nameLower)
}

// toLower is a minimal ASCII lower-case helper that avoids importing strings
// or unicode for this hot path.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
