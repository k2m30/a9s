package catalog

// ResourceTypes is the declarative registry of all a9s resource types.
// It is static — no init(), no Register* calls. Per-category PRs (04b–04m)
// populate this slice. Until then it is empty and all lookups fall through
// to the legacy registry in internal/resource.
var ResourceTypes = computeTypes //nolint:gochecknoglobals // static catalog: intentional package-level var

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
