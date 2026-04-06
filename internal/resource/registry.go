package resource

import (
	"context"
	"strings"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// All paginated fetchers MUST pass this as their MaxResults / MaxItems /
// MaxRecords / Limit / PageSize parameter, and all view-layer count displays
// MUST floor truncated counts to a multiple of this value (e.g. "50+", "100+").
const DefaultPageSize = 50

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher. For example, {"bucket": "my-bucket", "prefix": "data/"}.
type ParentContext map[string]string

// fieldKeyRegistry maps resource short names to their valid Fields keys.
// Populated by RegisterFieldKeys calls in each aws/*.go init().
var fieldKeyRegistry = map[string][]string{}

// childTypes maps child type short names to their type definitions.
var childTypes = map[string]*ResourceTypeDef{}

// RegisterFieldKeys records the valid Fields keys for a resource type.
// Called from init() in each aws/*.go file alongside RegisterPaginated.
func RegisterFieldKeys(shortName string, keys []string) {
	fieldKeyRegistry[shortName] = keys
}

// GetFieldKeys returns the registered Fields keys for the given resource type,
// or nil if none are registered.
func GetFieldKeys(shortName string) []string {
	return fieldKeyRegistry[shortName]
}

// fieldAliasBuiltins holds aliases registered by init() functions in aws/*.go.
// These are permanent and never removed by UnregisterFieldAliases.
var fieldAliasBuiltins = map[string]map[string]string{}

// fieldAliasOverrides holds aliases registered outside of init() (e.g., in tests).
// UnregisterFieldAliases removes entries from this map only.
var fieldAliasOverrides = map[string]map[string]string{}

// RegisterFieldAliases records field name aliases for a resource type.
// Called from init() in aws/*.go alongside RegisterFieldKeys; registers as builtins
// (permanent). When called outside of init() — e.g., in tests — entries are stored
// as overrides that UnregisterFieldAliases can remove.
func RegisterFieldAliases(shortName string, aliases map[string]string) {
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
// Overrides (registered after init) take precedence over builtins.
func ApplyFieldAliases(shortName string, fields map[string]string) map[string]string {
	aliases := fieldAliasOverrides[shortName]
	if len(aliases) == 0 {
		aliases = fieldAliasBuiltins[shortName]
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
	for k, v := range fields {
		out[k] = v
	}
	for from, to := range aliases {
		if v, ok := fields[from]; ok && strings.TrimSpace(v) != "" {
			if _, exists := out[to]; !exists {
				out[to] = v
			}
		}
	}
	return out
}

// UnregisterFieldAliases removes field alias overrides. Used only in tests for cleanup.
// Builtin aliases registered by init() are never removed.
func UnregisterFieldAliases(shortName string) {
	delete(fieldAliasOverrides, shortName)
}

// RegisterChildType stores a child type definition in the child types registry.
// Called from init() in each aws/*.go file for sub-resource types.
func RegisterChildType(def ResourceTypeDef) {
	copy := def
	childTypes[def.ShortName] = &copy
}

// GetChildType returns the child type definition for the given short name,
// or nil if no child type is registered.
func GetChildType(shortName string) *ResourceTypeDef {
	return childTypes[shortName]
}

// UnregisterChildType removes a child type. Used only in tests for cleanup.
func UnregisterChildType(shortName string) {
	delete(childTypes, shortName)
}

// PaginatedFetcher returns a single page of resources.
type PaginatedFetcher func(ctx context.Context, clients interface{}, continuationToken string) (FetchResult, error)

// PaginatedChildFetcher returns a single page of child resources.
type PaginatedChildFetcher func(ctx context.Context, clients interface{}, parentCtx ParentContext, continuationToken string) (FetchResult, error)

// paginatedRegistry maps resource short names to their paginated fetcher functions.
var paginatedRegistry = map[string]PaginatedFetcher{}

// paginatedChildRegistry maps child type short names to their paginated child fetcher functions.
var paginatedChildRegistry = map[string]PaginatedChildFetcher{}

// RegisterPaginated adds a paginated fetcher for the given resource short name.
// Called from init() in each aws/*.go file for resources that support pagination.
func RegisterPaginated(shortName string, f PaginatedFetcher) {
	paginatedRegistry[shortName] = f
}

// GetPaginatedFetcher returns the paginated fetcher for the given resource short name,
// or nil if no paginated fetcher is registered.
func GetPaginatedFetcher(shortName string) PaginatedFetcher {
	return paginatedRegistry[shortName]
}

// UnregisterPaginated removes a paginated fetcher. Used only in tests for cleanup.
func UnregisterPaginated(shortName string) {
	delete(paginatedRegistry, shortName)
}

// RegisterPaginatedChild adds a paginated child fetcher for the given short name.
// Called from init() in each aws/*.go file for child resources that support pagination.
func RegisterPaginatedChild(shortName string, f PaginatedChildFetcher) {
	paginatedChildRegistry[shortName] = f
}

// GetPaginatedChildFetcher returns the paginated child fetcher for the given short name,
// or nil if no paginated child fetcher is registered.
func GetPaginatedChildFetcher(shortName string) PaginatedChildFetcher {
	return paginatedChildRegistry[shortName]
}

// UnregisterPaginatedChild removes a paginated child fetcher. Used only in tests for cleanup.
func UnregisterPaginatedChild(shortName string) {
	delete(paginatedChildRegistry, shortName)
}

// RevealFetcher is the function signature for reveal value fetchers.
// Each resource type that supports reveal (x key) registers a fetcher
// that takes a context, clients, and resource ID, returning the value string.
type RevealFetcher func(ctx context.Context, clients interface{}, resourceID string) (string, error)

// revealRegistry maps resource short names to their reveal fetcher functions.
var revealRegistry = map[string]RevealFetcher{}

// RegisterRevealFetcher adds a reveal fetcher for the given resource short name.
// Called from init() in each aws/*.go file for resource types that support reveal.
func RegisterRevealFetcher(shortName string, f RevealFetcher) {
	revealRegistry[shortName] = f
}

// GetRevealFetcher returns the reveal fetcher for the given resource short name,
// or nil if no reveal fetcher is registered.
func GetRevealFetcher(shortName string) RevealFetcher {
	return revealRegistry[shortName]
}

// UnregisterRevealFetcher removes a reveal fetcher. Used only in tests for cleanup.
func UnregisterRevealFetcher(shortName string) {
	delete(revealRegistry, shortName)
}

// HasRevealFetcher returns true if a reveal fetcher is registered for the given short name.
func HasRevealFetcher(shortName string) bool {
	_, ok := revealRegistry[shortName]
	return ok
}
