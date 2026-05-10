package catalog

import "github.com/k2m30/a9s/v3/internal/domain"

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
