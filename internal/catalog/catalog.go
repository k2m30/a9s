package catalog

// registry is the installed top-level catalog. Populated exactly once by
// SetTypes (called from aws.Install at program start / TestMain). All Find /
// All / ByCategory / AllShortNames lookups read from this slice.
//
// AS-795a moved the per-category catalog data into internal/aws/ to break the
// `catalog → aws` cycle that direct fetcher references would otherwise force.
// The data slice is now installed at startup via aws.Install() instead of
// being computed at package-init time in this file.
var registry []ResourceTypeDef //nolint:gochecknoglobals // process-scope catalog: set once at startup

// childRegistry is the installed child-type catalog. Populated exactly once by
// SetChildTypes. Same lifecycle as registry.
var childRegistry map[string]ResourceTypeDef //nolint:gochecknoglobals // process-scope catalog: set once at startup

// installed records whether SetTypes has been called. Used to surface a
// loud panic from Find / All / ByCategory if a binary forgets the install
// (typically a test package whose TestMain does not call aws.Install).
var installed bool //nolint:gochecknoglobals // process-scope catalog: set once at startup

// childInstalled records whether SetChildTypes has been called. Independent
// of installed so packages that only need top-level access can skip child
// installation without tripping the panic in FindChild.
var childInstalled bool //nolint:gochecknoglobals // process-scope catalog: set once at startup

// SetTypes installs the top-level catalog. MUST be called exactly once at
// program start (main() / TestMain) BEFORE any Find / All / ByCategory call.
// Idempotent on identical input; panics on a second call with different data
// (defensive against accidental re-install with diverging slices in tests).
func SetTypes(types []ResourceTypeDef) {
	if installed {
		if !sameTypes(registry, types) {
			panic("catalog.SetTypes called twice with different data — refusing to overwrite installed catalog")
		}
		return
	}
	registry = types
	installed = true
}

// SetChildTypes installs the child-type catalog. Same lifecycle as SetTypes.
// Idempotent on identical input; panics on a second call with different data.
func SetChildTypes(children []ResourceTypeDef) {
	if childInstalled {
		if !sameChildren(childRegistry, children) {
			panic("catalog.SetChildTypes called twice with different data — refusing to overwrite installed child catalog")
		}
		return
	}
	m := make(map[string]ResourceTypeDef, len(children))
	for _, c := range children {
		m[c.ShortName] = c
	}
	childRegistry = m
	childInstalled = true
}

// Find returns the ResourceTypeDef for the given name (ShortName or Alias),
// or nil if the catalog does not have an entry for it.
// Case-insensitive match against ShortName and all Aliases.
//
// Panics with a clear message if SetTypes has not been called — this catches
// test binaries that forget to invoke aws.Install in TestMain.
func Find(name string) *ResourceTypeDef {
	requireInstalled()
	nameLower := toLower(name)
	for i := range registry {
		if toLower(registry[i].ShortName) == nameLower {
			return &registry[i]
		}
		for _, alias := range registry[i].Aliases {
			if toLower(alias) == nameLower {
				return &registry[i]
			}
		}
	}
	return nil
}

// All returns a copy of the full installed catalog slice. Safe for callers to
// store or mutate without affecting the registry.
//
// Panics if SetTypes has not been called.
func All() []ResourceTypeDef {
	requireInstalled()
	result := make([]ResourceTypeDef, len(registry))
	copy(result, registry)
	return result
}

// AllShortNames returns the ShortName of every type in the installed catalog.
//
// Panics if SetTypes has not been called.
func AllShortNames() []string {
	requireInstalled()
	names := make([]string, len(registry))
	for i, rt := range registry {
		names[i] = rt.ShortName
	}
	return names
}

// ByCategory returns all resource types with the given Category value.
// Returns nil when the catalog has no entries for that category.
//
// Panics if SetTypes has not been called.
func ByCategory(cat string) []ResourceTypeDef {
	requireInstalled()
	var result []ResourceTypeDef
	for _, rt := range registry {
		if rt.Category == cat {
			result = append(result, rt)
		}
	}
	return result
}

// FindChild returns the child-type ResourceTypeDef for the given short name,
// or nil if no child type with that name is registered.
//
// Panics if SetChildTypes has not been called.
func FindChild(name string) *ResourceTypeDef {
	if !childInstalled {
		panic("catalog.SetChildTypes not called — programmer must invoke aws.Install() before any child-catalog accessor")
	}
	if c, ok := childRegistry[name]; ok {
		return &c
	}
	return nil
}

// AllChildren returns the installed child-type catalog as a slice. The order
// is not stable — child types are stored in a map for ShortName lookup. Use
// only for replay walks (e.g. the AS-795 legacy-bridge) where iteration order
// is irrelevant.
//
// Panics if SetChildTypes has not been called.
func AllChildren() []ResourceTypeDef {
	if !childInstalled {
		panic("catalog.SetChildTypes not called — programmer must invoke aws.Install() before any child-catalog accessor")
	}
	out := make([]ResourceTypeDef, 0, len(childRegistry))
	for _, c := range childRegistry {
		out = append(out, c)
	}
	return out
}

// requireInstalled panics with a clear message if SetTypes has not yet been
// called. The message names the install hook so the failing test author knows
// where to look.
func requireInstalled() {
	if !installed {
		panic("catalog.SetTypes not called — programmer must invoke aws.Install() before any catalog accessor")
	}
}

// sameTypes reports whether two ResourceTypeDef slices represent the same
// catalog content for idempotency purposes. Compares only the identifying
// fields (ShortName + Name) — function pointers and slice fields are not
// stable under equality but ShortName uniqueness is the install invariant.
func sameTypes(a, b []ResourceTypeDef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ShortName != b[i].ShortName || a[i].Name != b[i].Name {
			return false
		}
	}
	return true
}

// sameChildren reports whether the existing childRegistry contents match the
// proposed children slice. Same shape rule as sameTypes — ShortName + Name.
func sameChildren(existing map[string]ResourceTypeDef, proposed []ResourceTypeDef) bool {
	if len(existing) != len(proposed) {
		return false
	}
	for _, c := range proposed {
		ex, ok := existing[c.ShortName]
		if !ok || ex.Name != c.Name {
			return false
		}
	}
	return true
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
