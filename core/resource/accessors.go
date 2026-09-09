// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"context"
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
)

// DefaultPageSize is the number of resources fetched per paginated API call.
// All paginated fetchers MUST pass this as their MaxResults / MaxItems /
// MaxRecords / Limit / PageSize parameter, and all view-layer count displays
// MUST floor truncated counts to a multiple of this value (e.g. "50+", "100+").
const DefaultPageSize = 50

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher. Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
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
	if ct := TypeDef(shortName); ct != nil && len(ct.FieldKeys) > 0 {
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
	if ct := TypeDef(shortName); ct != nil && len(ct.IssueEnricherFieldKeys) > 0 {
		return ct.IssueEnricherFieldKeys
	}
	return nil
}

// GetAllFieldKeysForTest returns the union of fetcher-registered field keys
// and Wave 2 issue-enricher-registered field keys for the given short name.
// Test-only: no production caller — used by column-key coverage assertions.
func GetAllFieldKeysForTest(shortName string) []string {
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
		if ct := TypeDef(shortName); ct != nil && len(ct.FieldAliases) > 0 {
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
// fieldAliasBuiltins starts empty for every shortName: any builtin entry was
// placed there by a test's
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

// TypeDef resolves a type name the way every reader of a declaration must:
// the child registry first, so a type a test registered is a type, then the
// installed catalog, parents and children alike (catalog.FindAny). Every
// getter below and in related.go and enricher.go asks here, so none of them
// answers for half the catalog — a child type's Related and Navigable were
// declarations nothing read, because the getters that read them looked among
// the parents only. Exported because core/aws's Wave 2 lookup is one of those
// readers and had the same half-catalog bug.
func TypeDef(shortName string) *ResourceTypeDef {
	if td := GetChildType(shortName); td != nil {
		return td
	}
	return catalog.FindAny(shortName)
}

// GetChildType returns the child type definition for the given short name,
// or nil if no child type is registered. Legacy-first: test overrides via
// SetChildTypeForTest take effect; otherwise reads catalog.ChildOnly.
func GetChildType(shortName string) *ResourceTypeDef {
	if def, ok := childTypes[shortName]; ok {
		return def
	}
	if ct := catalog.ChildOnly(shortName); ct != nil {
		return ct
	}
	return nil
}

// AllChildTypes returns all registered child type definitions.
// The returned slice is in no guaranteed order.
// Combines legacy registry entries with catalog child entries; legacy wins
// on name collision so test overrides remain visible. This is the child half
// of the union TypeDef resolves a single name against, and core/aws's AllWave2
// walks it for the same reason: an enumeration over the parents alone answers
// for half the catalog.
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

// AllChildShortNamesForTest returns the ShortName of every registered child
// type. Includes both legacy registry entries and catalog child entries.
// Test-only: no production caller.
func AllChildShortNamesForTest() []string {
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
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
type PaginatedFetcher = domain.PaginatedFetcher

// PaginatedChildFetcher returns a single page of child resources.
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
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

// sanitizeFetchResult repairs a fetcher's FetchResult so it never claims a
// resumable truncation it cannot actually resume: IsTruncated=true paired
// with an empty NextToken is not a legitimate pagination state anywhere in
// this codebase's contract — every consumer (RowStore, ListState.HasPagination,
// the load-more dispatch that round-trips NextToken as the next
// continuationToken) treats "truncated" as "there is a working cursor to
// continue from". A fetcher that hits its own local item/page cap on AWS's
// terminal page (no further NextToken) must report the result as complete,
// not truncated. Downgrading is always the safe direction — a result can
// only move from truncated toward exact here, never the reverse, mirroring
// the "exact never regresses" rule the rest of the cache layer already
// enforces (RowStore, syncExactTotalToMenu) — so this can never make an
// already-correct fetcher's result wrong, only repair a broken one.
//
// The one deliberate exception is p.LowerBoundOnly: a fetcher that knows on
// its own it can never confirm an exact total (e.g. core/aws/catalog_security.go's
// "policy" AvailabilityFetcher, which never checks inline group policies) sets
// this alongside IsTruncated=true to report an honest, permanent "N+" instead
// of a wrongly-confirmed exact count. That pairing passes through untouched —
// downgrading it would recreate the exact-zero bug the flag exists to prevent.
//
// Applied at the single seam every registered fetcher's result passes
// through (GetPaginatedFetcher and its three siblings below), so a fetcher
// that gets this pairing wrong can never reach a consumer un-sanitized,
// regardless of which of the many registered fetchers produced it — the
// same "make the invalid state unrepresentable at the boundary" principle
// core/resource/related.go's smart constructors (KnownRelated/UnknownRelated/
// ErrorRelated/DeferredRelated) already apply to RelatedCheckResult.
func sanitizeFetchResult(res FetchResult, err error) (FetchResult, error) {
	p := res.Pagination
	if p == nil || !p.IsTruncated || p.NextToken != "" || p.LowerBoundOnly {
		return res, err
	}
	fixed := *p
	fixed.IsTruncated = false
	if fixed.TotalHint < 0 {
		fixed.TotalHint = len(res.Resources)
	}
	res.Pagination = &fixed
	return res, err
}

// GetPaginatedFetcher returns the paginated fetcher for the given resource short name.
// Legacy-first: the runtime map wins so SetPaginatedForTest test overrides take
// effect. Catalog is the read-only fallback. The returned function's result
// always passes through sanitizeFetchResult first.
func GetPaginatedFetcher(shortName string) PaginatedFetcher {
	fn, ok := paginatedRegistry[shortName]
	if !ok {
		if ct := TypeDef(shortName); ct != nil && ct.Fetcher != nil {
			fn = ct.Fetcher
		}
	}
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, clients any, continuationToken string) (FetchResult, error) {
		return sanitizeFetchResult(fn(ctx, clients, continuationToken))
	}
}

// CleanupPaginatedForTest removes a paginated fetcher. Used only in tests for cleanup.
func CleanupPaginatedForTest(shortName string) {
	delete(paginatedRegistry, shortName)
}

// AvailabilityFetcher is a cheap, probe-only alternative to a resource
// type's full PaginatedFetcher — same shape, used by
// Core.ProbeResourceAvailability when a type's availability/count signal is
// materially cheaper to compute than its real list content. Types with no
// registered AvailabilityFetcher fall back to their ordinary
// PaginatedFetcher, unchanged.
type AvailabilityFetcher = PaginatedFetcher

// availabilityRegistry maps resource short names to their TEST-ONLY
// availability-fetcher overrides. Deliberately separate from
// catalog.ResourceTypeDef.AvailabilityFetcher (the permanent production
// registration) — a flat single map would let CleanupAvailabilityFetcherForTest
// delete a real production registration a test never set, the same
// legacy-first split GetPaginatedFetcher/paginatedRegistry already uses for
// exactly this reason.
var availabilityRegistry = map[string]AvailabilityFetcher{}

// SetAvailabilityFetcherForTest registers f as the availability fetcher for
// shortName, for the duration of a test only.
func SetAvailabilityFetcherForTest(shortName string, f AvailabilityFetcher) {
	availabilityRegistry[shortName] = f
}

// GetAvailabilityFetcher returns the availability fetcher for shortName, or
// nil when none is registered — callers (Core.ProbeResourceAvailability)
// fall back to GetPaginatedFetcher in that case. Legacy-first: the runtime
// test-override map wins so SetAvailabilityFetcherForTest takes effect;
// catalog.ResourceTypeDef.AvailabilityFetcher (the permanent production
// registration, e.g. core/aws/catalog_security.go's "policy" entry) is
// the read-only fallback. The returned function's result always passes
// through sanitizeFetchResult first, mirroring GetPaginatedFetcher.
func GetAvailabilityFetcher(shortName string) AvailabilityFetcher {
	fn, ok := availabilityRegistry[shortName]
	if !ok {
		if ct := TypeDef(shortName); ct != nil && ct.AvailabilityFetcher != nil {
			fn = ct.AvailabilityFetcher
		}
	}
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, clients any, continuationToken string) (FetchResult, error) {
		return sanitizeFetchResult(fn(ctx, clients, continuationToken))
	}
}

// CleanupAvailabilityFetcherForTest removes a TEST-ONLY availability
// fetcher override. Used only in tests for cleanup — never touches a
// permanent catalog.ResourceTypeDef.AvailabilityFetcher registration.
func CleanupAvailabilityFetcherForTest(shortName string) {
	delete(availabilityRegistry, shortName)
}

// SetPaginatedChildForTest adds a paginated child fetcher for the given short name.
// Called from init() in each aws/*.go file for child resources that support pagination.
func SetPaginatedChildForTest(shortName string, f PaginatedChildFetcher) {
	paginatedChildRegistry[shortName] = f
}

// GetPaginatedChildFetcher returns the paginated child fetcher for the given short name.
// Legacy-first: test overrides via SetPaginatedChildForTest take effect;
// otherwise reads the catalog child-type ChildFetcher field. The returned
// function's result always passes through sanitizeFetchResult first,
// mirroring GetPaginatedFetcher.
func GetPaginatedChildFetcher(shortName string) PaginatedChildFetcher {
	fn, ok := paginatedChildRegistry[shortName]
	if !ok {
		if ct := catalog.ChildOnly(shortName); ct != nil && ct.ChildFetcher != nil {
			fn = ct.ChildFetcher
		}
	}
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, clients any, parentCtx ParentContext, continuationToken string) (FetchResult, error) {
		return sanitizeFetchResult(fn(ctx, clients, parentCtx, continuationToken))
	}
}

// CleanupPaginatedChildForTest removes a paginated child fetcher. Used only in tests for cleanup.
func CleanupPaginatedChildForTest(shortName string) {
	delete(paginatedChildRegistry, shortName)
}

// FilteredPaginatedFetcher returns a single page of resources filtered server-side.
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
type FilteredPaginatedFetcher = domain.FilteredPaginatedFetcher

var filteredPaginatedRegistry = map[string]FilteredPaginatedFetcher{}

// SetFilteredPaginatedForTest adds a filtered paginated fetcher for the given resource short name.
func SetFilteredPaginatedForTest(shortName string, f FilteredPaginatedFetcher) {
	filteredPaginatedRegistry[shortName] = f
}

// GetFilteredPaginatedFetcher returns the filtered paginated fetcher for the given short name.
// Legacy-first: test overrides via SetFilteredPaginatedForTest take effect;
// otherwise reads the catalog FilteredFetcher field. The returned function's
// result always passes through sanitizeFetchResult first, mirroring
// GetPaginatedFetcher.
func GetFilteredPaginatedFetcher(shortName string) FilteredPaginatedFetcher {
	fn, ok := filteredPaginatedRegistry[shortName]
	if !ok {
		if ct := TypeDef(shortName); ct != nil && ct.FilteredFetcher != nil {
			fn = ct.FilteredFetcher
		}
	}
	if fn == nil {
		return nil
	}
	return func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (FetchResult, error) {
		return sanitizeFetchResult(fn(ctx, clients, filter, continuationToken))
	}
}

// CleanupFilteredPaginatedForTest removes a filtered paginated fetcher. Used only in tests for cleanup.
func CleanupFilteredPaginatedForTest(shortName string) {
	delete(filteredPaginatedRegistry, shortName)
}

// RevealFetcher is the function signature for reveal value fetchers.
// Declaration lives in core/domain/contracts.go; this alias keeps
// existing consumers compiling.
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
// take effect. Catalog is the read-only fallback.
func GetRevealFetcher(shortName string) RevealFetcher {
	if fn, ok := revealRegistry[shortName]; ok {
		return fn
	}
	if ct := TypeDef(shortName); ct != nil && ct.Reveal != nil {
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
// are honored. Catalog is the read-only fallback.
func HasRevealFetcher(shortName string) bool {
	if _, ok := revealRegistry[shortName]; ok {
		return true
	}
	if ct := TypeDef(shortName); ct != nil && ct.Reveal != nil {
		return true
	}
	return false
}
