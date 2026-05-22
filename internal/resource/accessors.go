package resource

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// All paginated fetchers MUST pass this as their MaxResults / MaxItems /
// MaxRecords / Limit / PageSize parameter, and all view-layer count displays
// MUST floor truncated counts to a multiple of this value (e.g. "50+", "100+").
const DefaultPageSize = 50

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher. Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type ParentContext = domain.ParentContext

// fieldKeyRegistry maps resource short names to their valid Fields keys.
// Populated by SetFieldKeysForTest calls in each aws/*.go init().
var fieldKeyRegistry = map[string][]string{}

// childTypes maps child type short names to their type definitions.
var childTypes = map[string]*ResourceTypeDef{}

// SetFieldKeysForTest records the valid Fields keys for a resource type.
// Called from init() in each aws/*.go file alongside SetPaginatedForTest.
func SetFieldKeysForTest(shortName string, keys []string) {
	fieldKeyRegistry[shortName] = keys
}

// GetFieldKeys returns the registered Fields keys for the given resource type,
// or nil if none are registered. Legacy-first: runtime map wins so test
// overrides via SetFieldKeysForTest take effect; otherwise reads the catalog
// FieldKeys field for the type (or its child type when the name is a child).
func GetFieldKeys(shortName string) []string {
	if keys, ok := fieldKeyRegistry[shortName]; ok {
		return keys
	}
	if ct := catalog.Find(shortName); ct != nil && len(ct.FieldKeys) > 0 {
		return ct.FieldKeys
	}
	if ct := catalog.FindChild(shortName); ct != nil && len(ct.FieldKeys) > 0 {
		return ct.FieldKeys
	}
	return nil
}

// issueEnricherFieldKeysRegistry stores field keys produced by Wave 2 issue
// enrichers (IssueEnricherResult.FieldUpdates) per resource short name. Keys
// declared here are additive to keys in fieldKeyRegistry (fetcher-produced).
//
// The test TestColumnKeysHaveProducers asserts every ResourceTypeDef.Columns[].Key
// appears in at least one of: fetcher keys, issue-enricher keys, or the
// documented allowlist for intentionally-blank columns.
var issueEnricherFieldKeysRegistry = map[string][]string{}

// SetIssueEnricherFieldKeysForTest declares the set of Resource.Fields keys that
// a Wave 2 issue enricher writes via IssueEnricherResult.FieldUpdates for the
// given resource short name. Multiple enrichers may target the same type; keys
// are unioned.
//
// Call from enrichment.go package init() or from each Enrich* function body
// (idempotent — duplicates are deduplicated).
func SetIssueEnricherFieldKeysForTest(shortName string, keys []string) {
	existing := issueEnricherFieldKeysRegistry[shortName]
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
	issueEnricherFieldKeysRegistry[shortName] = existing
}

// GetIssueEnricherFieldKeys returns the accumulated Wave 2 issue-enricher
// field keys for the given resource short name, or nil if none are registered.
// Legacy-first: test overrides via SetIssueEnricherFieldKeysForTest take effect;
// otherwise reads the catalog IssueEnricherFieldKeys field.
func GetIssueEnricherFieldKeys(shortName string) []string {
	if keys, ok := issueEnricherFieldKeysRegistry[shortName]; ok {
		return keys
	}
	if ct := catalog.Find(shortName); ct != nil && len(ct.IssueEnricherFieldKeys) > 0 {
		return ct.IssueEnricherFieldKeys
	}
	return nil
}

// GetAllFieldKeys returns the union of fetcher-registered field keys and
// Wave 2 issue-enricher-registered field keys for the given short name.
func GetAllFieldKeys(shortName string) []string {
	fetcher := GetFieldKeys(shortName)
	enricher := GetIssueEnricherFieldKeys(shortName)
	if len(enricher) == 0 {
		return fetcher
	}
	out := make([]string, 0, len(fetcher)+len(enricher))
	out = append(out, fetcher...)
	seen := make(map[string]bool, len(fetcher))
	for _, k := range fetcher {
		seen[k] = true
	}
	for _, k := range enricher {
		if !seen[k] {
			out = append(out, k)
			seen[k] = true
		}
	}
	return out
}

// fieldAliasBuiltins holds aliases registered by init() functions in aws/*.go.
// These are permanent and never removed by CleanupFieldAliasesForTest.
var fieldAliasBuiltins = map[string]map[string]string{}

// fieldAliasOverrides holds aliases registered outside of init() (e.g., in tests).
// CleanupFieldAliasesForTest removes entries from this map only.
var fieldAliasOverrides = map[string]map[string]string{}

// SetFieldAliasesForTest records field name aliases for a resource type.
// Called from init() in aws/*.go alongside SetFieldKeysForTest; registers as builtins
// (permanent). When called outside of init() — e.g., in tests — entries are stored
// as overrides that CleanupFieldAliasesForTest can remove.
func SetFieldAliasesForTest(shortName string, aliases map[string]string) {
	// Detect init-time registration: if init has not yet registered a builtin for this
	// short name we treat the call as a builtin. Subsequent calls (from tests) override.
	if _, hasBuiltin := fieldAliasBuiltins[shortName]; !hasBuiltin {
		fieldAliasBuiltins[shortName] = aliases
	} else {
		fieldAliasOverrides[shortName] = aliases
	}
}

// ApplyFieldAliases returns a fields map augmented with alias keys.
// For each alias (from→to), if fields[from] has a non-empty value and fields[to]
// does not exist, it's copied. Returns the original map unchanged when no copies
// are needed. Returns nil if fields is nil.
// Overrides (registered after init) take precedence over builtins; builtins
// fall back to catalog FieldAliases when the legacy map is empty.
func ApplyFieldAliases(shortName string, fields map[string]string) map[string]string {
	aliases := fieldAliasOverrides[shortName]
	if len(aliases) == 0 {
		aliases = fieldAliasBuiltins[shortName]
	}
	if len(aliases) == 0 {
		if ct := catalog.Find(shortName); ct != nil && len(ct.FieldAliases) > 0 {
			aliases = ct.FieldAliases
		}
	}
	if len(aliases) == 0 || len(fields) == 0 {
		return fields
	}
	needCopy := false
	for from, to := range aliases {
		if v, ok := fields[from]; ok && strings.TrimSpace(v) != "" {
			if _, exists := fields[to]; !exists {
				needCopy = true
				break
			}
		}
	}
	if !needCopy {
		return fields
	}
	out := make(map[string]string, len(fields)+len(aliases))
	maps.Copy(out, fields)
	for from, to := range aliases {
		if v, ok := fields[from]; ok && strings.TrimSpace(v) != "" {
			if _, exists := out[to]; !exists {
				out[to] = v
			}
		}
	}
	return out
}

// CleanupFieldAliasesForTest removes field alias overrides AND test-registered
// builtins for the given short name. Used only in tests for cleanup.
// After AS-731 deleted the catalog→legacy bridge, fieldAliasBuiltins starts
// empty for every shortName: any builtin entry was placed there by a test's
// SetFieldAliasesForTest call (the "first call becomes builtin" branch) and
// must be cleared on cleanup so subsequent reads fall through to the catalog
// FieldAliases field in ApplyFieldAliases.
func CleanupFieldAliasesForTest(shortName string) {
	delete(fieldAliasOverrides, shortName)
	delete(fieldAliasBuiltins, shortName)
}

// SetChildTypeForTest stores a child type definition in the child types registry.
// Called from init() in each aws/*.go file for sub-resource types.
func SetChildTypeForTest(def ResourceTypeDef) {
	copy := def
	childTypes[def.ShortName] = &copy
}

// GetChildType returns the child type definition for the given short name,
// or nil if no child type is registered. Legacy-first: test overrides via
// SetChildTypeForTest take effect; otherwise reads catalog.FindChild.
func GetChildType(shortName string) *ResourceTypeDef {
	if def, ok := childTypes[shortName]; ok {
		return def
	}
	if ct := catalog.FindChild(shortName); ct != nil {
		return ct
	}
	return nil
}

// AllChildTypes returns all registered child type definitions.
// The returned slice is in no guaranteed order.
// Combines legacy registry entries with catalog child entries; legacy wins
// on name collision so test overrides remain visible.
func AllChildTypes() []ResourceTypeDef {
	result := make([]ResourceTypeDef, 0, len(childTypes))
	seen := make(map[string]struct{}, len(childTypes))
	for name, def := range childTypes {
		result = append(result, *def)
		seen[name] = struct{}{}
	}
	for _, ct := range catalog.AllChildren() {
		if _, ok := seen[ct.ShortName]; ok {
			continue
		}
		result = append(result, ct)
	}
	return result
}

// AllChildShortNames returns the ShortName of every registered child type.
// Includes both legacy registry entries and catalog child entries.
func AllChildShortNames() []string {
	seen := make(map[string]struct{}, len(childTypes))
	for name := range childTypes {
		seen[name] = struct{}{}
	}
	for _, ct := range catalog.AllChildren() {
		seen[ct.ShortName] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// CleanupChildTypeForTest removes a child type. Used only in tests for cleanup.
func CleanupChildTypeForTest(shortName string) {
	delete(childTypes, shortName)
}

// PaginatedFetcher returns a single page of resources.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type PaginatedFetcher = domain.PaginatedFetcher

// PaginatedChildFetcher returns a single page of child resources.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type PaginatedChildFetcher = domain.PaginatedChildFetcher

// paginatedRegistry maps resource short names to their paginated fetcher functions.
var paginatedRegistry = map[string]PaginatedFetcher{}

// paginatedChildRegistry maps child type short names to their paginated child fetcher functions.
var paginatedChildRegistry = map[string]PaginatedChildFetcher{}

// SetPaginatedForTest adds a paginated fetcher for the given resource short name.
// Called from init() in each aws/*.go file for resources that support pagination.
func SetPaginatedForTest(shortName string, f PaginatedFetcher) {
	paginatedRegistry[shortName] = f
}

// GetPaginatedFetcher returns the paginated fetcher for the given resource short name.
// Legacy-first: the runtime map wins so SetPaginatedForTest test overrides take
// effect. Catalog is the read-only fallback during the AS-795b–m transition.
func GetPaginatedFetcher(shortName string) PaginatedFetcher {
	if fn, ok := paginatedRegistry[shortName]; ok {
		return fn
	}
	if ct := catalog.Find(shortName); ct != nil && ct.Fetcher != nil {
		return ct.Fetcher
	}
	return nil
}

// CleanupPaginatedForTest removes a paginated fetcher. Used only in tests for cleanup.
func CleanupPaginatedForTest(shortName string) {
	delete(paginatedRegistry, shortName)
}

// SetPaginatedChildForTest adds a paginated child fetcher for the given short name.
// Called from init() in each aws/*.go file for child resources that support pagination.
func SetPaginatedChildForTest(shortName string, f PaginatedChildFetcher) {
	paginatedChildRegistry[shortName] = f
}

// GetPaginatedChildFetcher returns the paginated child fetcher for the given short name.
// Legacy-first: test overrides via SetPaginatedChildForTest take effect;
// otherwise reads the catalog child-type ChildFetcher field.
func GetPaginatedChildFetcher(shortName string) PaginatedChildFetcher {
	if fn, ok := paginatedChildRegistry[shortName]; ok {
		return fn
	}
	if ct := catalog.FindChild(shortName); ct != nil && ct.ChildFetcher != nil {
		return ct.ChildFetcher
	}
	return nil
}

// CleanupPaginatedChildForTest removes a paginated child fetcher. Used only in tests for cleanup.
func CleanupPaginatedChildForTest(shortName string) {
	delete(paginatedChildRegistry, shortName)
}

// FilteredPaginatedFetcher returns a single page of resources filtered server-side.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type FilteredPaginatedFetcher = domain.FilteredPaginatedFetcher

var filteredPaginatedRegistry = map[string]FilteredPaginatedFetcher{}

// SetFilteredPaginatedForTest adds a filtered paginated fetcher for the given resource short name.
func SetFilteredPaginatedForTest(shortName string, f FilteredPaginatedFetcher) {
	filteredPaginatedRegistry[shortName] = f
}

// GetFilteredPaginatedFetcher returns the filtered paginated fetcher for the given short name.
// Legacy-first: test overrides via SetFilteredPaginatedForTest take effect;
// otherwise reads the catalog FilteredFetcher field.
func GetFilteredPaginatedFetcher(shortName string) FilteredPaginatedFetcher {
	if fn, ok := filteredPaginatedRegistry[shortName]; ok {
		return fn
	}
	if ct := catalog.Find(shortName); ct != nil && ct.FilteredFetcher != nil {
		return ct.FilteredFetcher
	}
	return nil
}

// CleanupFilteredPaginatedForTest removes a filtered paginated fetcher. Used only in tests for cleanup.
func CleanupFilteredPaginatedForTest(shortName string) {
	delete(filteredPaginatedRegistry, shortName)
}

// RevealFetcher is the function signature for reveal value fetchers.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type RevealFetcher = domain.RevealFetcher

// revealRegistry maps resource short names to their reveal fetcher functions.
var revealRegistry = map[string]RevealFetcher{}

// SetRevealFetcherForTest adds a reveal fetcher for the given resource short name.
// Called from init() in each aws/*.go file for resource types that support reveal.
func SetRevealFetcherForTest(shortName string, f RevealFetcher) {
	revealRegistry[shortName] = f
}

// GetRevealFetcher returns the reveal fetcher for the given resource short name.
// Legacy-first: runtime map wins so test overrides via SetRevealFetcherForTest
// take effect. Catalog is the read-only fallback during AS-795b–m.
func GetRevealFetcher(shortName string) RevealFetcher {
	if fn, ok := revealRegistry[shortName]; ok {
		return fn
	}
	if ct := catalog.Find(shortName); ct != nil && ct.Reveal != nil {
		return ct.Reveal
	}
	return nil
}

// CleanupRevealFetcherForTest removes a reveal fetcher. Used only in tests for cleanup.
func CleanupRevealFetcherForTest(shortName string) {
	delete(revealRegistry, shortName)
}

// HasRevealFetcher returns true if a reveal fetcher is registered for the given short name.
// Legacy-first: runtime map wins so test overrides via SetRevealFetcherForTest
// are honored. Catalog is the read-only fallback during AS-795b–m.
func HasRevealFetcher(shortName string) bool {
	if _, ok := revealRegistry[shortName]; ok {
		return true
	}
	if ct := catalog.Find(shortName); ct != nil && ct.Reveal != nil {
		return true
	}
	return false
}
