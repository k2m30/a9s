// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package catalog

import (
	"fmt"
	"maps"
	"strings"
	"sync/atomic"
)

// registry is the installed top-level catalog. Populated exactly once by
// SetTypes (called from aws.Install at program start / TestMain). All Find /
// All / AllShortNames lookups read from this slice.
//
// The per-category catalog data lives in core/aws/ (not here) to break the
// `catalog → aws` cycle that direct fetcher references would otherwise force.
// The data slice is installed at startup via aws.Install() rather than
// computed at package-init time in this file.
var registry []ResourceTypeDef //nolint:gochecknoglobals // process-scope catalog: set once at startup

// childRegistry is the installed child-type catalog, and the only registry of
// child types there is. A nil pointer means no child catalog is installed.
//
// It is an atomic pointer to an immutable map rather than a plain map because
// SetChildTypeForTest writes it while production goroutines read it: the
// enrichment path resolves a type name on the cache-writer goroutine, and a
// test registering a child type raced it. Writers copy the map and swap the
// pointer, so a reader holds a map nobody will write to again.
var childRegistry atomic.Pointer[map[string]ResourceTypeDef] //nolint:gochecknoglobals // process-scope catalog

// installed records whether SetTypes has been called. Used to surface a
// loud panic from Find / All if a binary forgets the install (typically a
// test package whose TestMain does not call aws.Install).
var installed bool //nolint:gochecknoglobals // process-scope catalog: set once at startup

// SetTypes installs the top-level catalog. MUST be called exactly once at
// program start (main() / TestMain) BEFORE any Find / All call. Idempotent
// on identical input; panics on a second call with different data
// (defensive against accidental re-install with diverging slices in tests).
func SetTypes(types []ResourceTypeDef) {
	validateRelatedDefs(types)
	if installed {
		if !sameTypes(registry, types) {
			panic("catalog.SetTypes called twice with different data — refusing to overwrite installed catalog")
		}
		return
	}
	registry = types
	installed = true
	indexDetails(types)
}

// SetChildTypes installs the child-type catalog. Same lifecycle as SetTypes.
// Idempotent on identical input; panics on a second call with different data.
func SetChildTypes(children []ResourceTypeDef) {
	validateRelatedDefs(children)
	if existing := childRegistry.Load(); existing != nil {
		if !sameChildren(*existing, children) {
			panic("catalog.SetChildTypes called twice with different data — refusing to overwrite installed child catalog")
		}
		return
	}
	m := make(map[string]ResourceTypeDef, len(children))
	for _, c := range children {
		m[c.ShortName] = c
	}
	childRegistry.Store(&m)
	installedChildren.Store(&m)
	indexDetails(children)
}

// find returns the ResourceTypeDef for the given name (ShortName or Alias)
// among the TOP-LEVEL types, or nil.
// Case-insensitive match against ShortName and all Aliases.
//
// Unexported on purpose: a lookup that answers for half the catalog reads
// like a general one at the call site, and every caller that picked a half
// was wrong for the other — a child view file was "no such type", a child's
// Related was a declaration nothing read. Outside this package there is
// FindAny, and TopLevelOnly / ChildOnly for the three callers that mean one
// half and say so.
//
// Panics with a clear message if SetTypes has not been called — this catches
// test binaries that forget to invoke aws.Install in TestMain.
func find(name string) *ResourceTypeDef {
	requireInstalled()
	for i := range registry {
		if strings.EqualFold(registry[i].ShortName, name) {
			return &registry[i]
		}
		for _, alias := range registry[i].Aliases {
			if strings.EqualFold(alias, name) {
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

// findChild returns the child-type ResourceTypeDef for the given short name,
// or nil. Unexported for the reason find is.
//
// Panics if SetChildTypes has not been called.
func findChild(name string) *ResourceTypeDef {
	if c, ok := (*loadChildren())[name]; ok {
		return &c
	}
	return nil
}

// loadChildren returns the installed child map, panicking if none is
// installed. The returned map is never written to again — writers copy it —
// so a caller may read it without a lock.
func loadChildren() *map[string]ResourceTypeDef {
	m := childRegistry.Load()
	if m == nil {
		panic("catalog.SetChildTypes not called — programmer must invoke aws.Install() before any child-catalog accessor")
	}
	return m
}

// SetChildTypeForTest registers or replaces one child type, for a test that
// needs a type the catalog does not ship. It lives here, on the one registry,
// because a second registry in front of this one is what a production reader
// raced against.
//
// Copy-on-write: the map a reader already holds is never touched.
func SetChildTypeForTest(def ResourceTypeDef) {
	swapChildren(func(m map[string]ResourceTypeDef) { m[def.ShortName] = def })
}

// installedChildren is the map SetChildTypes installed, kept so cleanup can
// tell a test's own registration from an override of a shipped child. No
// lookup reads it: it exists only so removing an override restores what the
// catalog ships instead of deleting it for the rest of the process, which is
// what the registry this replaced did by having two maps.
var installedChildren atomic.Pointer[map[string]ResourceTypeDef] //nolint:gochecknoglobals // process-scope catalog

// CleanupChildTypeForTest removes a child type registered by
// SetChildTypeForTest, restoring the shipped definition when the test
// registration shadowed one. Same copy-on-write rule.
func CleanupChildTypeForTest(shortName string) {
	swapChildren(func(m map[string]ResourceTypeDef) {
		if base := installedChildren.Load(); base != nil {
			if shipped, ok := (*base)[shortName]; ok {
				m[shortName] = shipped
				return
			}
		}
		delete(m, shortName)
	})
}

// swapChildren applies edit to a copy of the installed child map and swaps it
// in, retrying if another writer got there first. A plain load-copy-store
// would lose one of two concurrent registrations, and a lost registration
// surfaces as "no such type" in whichever test the scheduler robbed.
//
// A nil current map means nothing is installed, and registering a child type
// is installing a child catalog as far as any reader is concerned.
func swapChildren(edit func(map[string]ResourceTypeDef)) {
	for {
		current := childRegistry.Load()
		next := map[string]ResourceTypeDef{}
		if current != nil {
			next = make(map[string]ResourceTypeDef, len(*current)+1)
			maps.Copy(next, *current)
		}
		edit(next)
		if childRegistry.CompareAndSwap(current, &next) {
			return
		}
	}
}

// FindAny returns the type a view name refers to, parent or child, or nil.
//
// A view file, a migration and a load report all name a type the same way and
// none of them cares which registry it lives in — asking Find alone answered
// "no such type" for every child view, which is how child files were stamped
// as migrated and migrated by nothing.
func FindAny(name string) *ResourceTypeDef {
	if td := find(name); td != nil {
		return td
	}
	return findChild(name)
}

// TopLevelOnly answers for the top-level types alone, and ChildOnly for the
// children alone. They exist for the three callers whose answer IS the half —
// resource.FindResourceType is the parents accessor, GetChildType the
// children one, and GetPaginatedChildFetcher reads a field only a child has —
// and their names say so at the call site, which "Find" never did.
func TopLevelOnly(name string) *ResourceTypeDef { return find(name) }

// ChildOnly is TopLevelOnly's other half. See its comment.
func ChildOnly(name string) *ResourceTypeDef { return findChild(name) }

// AllChildren returns the installed child-type catalog as a slice. The order
// is not stable — child types are stored in a map for ShortName lookup. Use
// only for replay walks where iteration order is irrelevant.
//
// Panics if SetChildTypes has not been called.
func AllChildren() []ResourceTypeDef {
	children := *loadChildren()
	out := make([]ResourceTypeDef, 0, len(children))
	for _, c := range children {
		out = append(out, c)
	}
	return out
}

// validateRelatedDefs panics if any type declares a RelatedDef with an empty
// TargetType. domain.RelatedCheckResult's fields are unexported, but Go still
// permits an empty composite literal or the bare zero value from any package;
// its zero value renders as a dimmed, non-navigable "(0)" with no evidence
// behind it — the exact failure mode a missing TargetType would produce
// undetected. Catalog registration is developer data installed once at
// process start (aws.Install), not user input, so failing fast here — before
// any resource list or detail view can render — is preferable to a live
// panic or a silently wrong badge.
func validateRelatedDefs(types []ResourceTypeDef) {
	for _, t := range types {
		for _, rd := range t.Related {
			if rd.TargetType == "" {
				panic(fmt.Sprintf(
					"catalog: %q declares a RelatedDef with empty TargetType (DisplayName=%q) — every RelatedDef must name a target",
					t.ShortName, rd.DisplayName,
				))
			}
		}
	}
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
