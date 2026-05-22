package resource

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// RelatedDef defines one related resource class for a given resource type.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type RelatedDef = domain.RelatedDef

// NavigableField associates a detail view field path with a target resource type.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type NavigableField = domain.NavigableField

// NavIDFromValue returns the bare resource ID suitable for target lookup,
// given a raw field value. When the value is an AWS ARN and the target
// resource type indexes on a bare name/UUID (not the ARN), this extracts
// the correct lookup key so Enter navigation lands on the matching row.
//
// When no extractor is registered for the target type, or the value is
// already bare (no "/" or ":" segment to strip), the value is returned
// unchanged — the caller should fall back to the raw value.
//
// Registered extractors cover the target types where AWS consistently
// emits ARNs in describe-response fields but a9s indexes on the bare
// id/name. For target types whose IDs ARE ARNs (sns, for example), no
// extractor is registered — navigation works directly.
func NavIDFromValue(targetType, value string) string {
	if value == "" {
		return ""
	}
	if f, ok := navIDExtractors[targetType]; ok {
		if extracted := f(value); extracted != "" {
			return extracted
		}
	}
	return value
}

// navIDExtractors maps target resource types to extractors that derive
// the bare lookup ID from a raw field value (typically an ARN).
var navIDExtractors = map[string]func(string) string{
	"kms":      arnLastSlashSegment,
	"role":     arnLastSlashSegment,
	"ecs":      arnLastSlashSegment,
	"logs":     arnLastColonSegment,
	"s3":       s3BucketFromARN,
	"iam-user": arnLastSlashSegment,
}

// arnLastSlashSegment returns the substring after the last "/".
// Example: "arn:aws:kms:us-east-1:123:key/UUID" → "UUID".
// Returns "" if the input has no "/" or if "/" is the final character.
func arnLastSlashSegment(s string) string {
	i := strings.LastIndex(s, "/")
	if i < 0 || i == len(s)-1 {
		return ""
	}
	return s[i+1:]
}

// arnLastColonSegment returns the substring after the last ":".
// Example: "arn:aws:logs:us-east-1:123:log-group:/aws/lambda/fn" → "/aws/lambda/fn".
// Returns "" if the input has no ":" or if ":" is the final character.
func arnLastColonSegment(s string) string {
	i := strings.LastIndex(s, ":")
	if i < 0 || i == len(s)-1 {
		return ""
	}
	return s[i+1:]
}

// s3BucketFromARN extracts the bucket name from an S3 bucket ARN.
// Example: "arn:aws:s3:::my-bucket" → "my-bucket". "arn:aws:s3:::" → "".
// Input without the "arn:aws:s3:::" prefix is returned unchanged so a bare
// bucket name (the common case) passes through.
func s3BucketFromARN(s string) string {
	const prefix = "arn:aws:s3:::"
	if rest, ok := strings.CutPrefix(s, prefix); ok {
		return rest
	}
	return s
}

// RelatedCheckResult is returned by a RelatedChecker and carries all state
// needed by the right-column panel to display a row and navigate on Enter.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
//
// Semantics (FR-008 / FR-014):
//
//   - Count == -1: unknown — the checker could not determine a count.
//   - Count == 0: definitively zero related resources of this type.
//   - Count >= 1: confirmed N related resources.
//   - Approximate == true: Count was derived from a truncated cache page.
//   - FetchFilter non-nil: navigation should use a server-side filtered fetcher.
type RelatedCheckResult = domain.RelatedCheckResult

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state. Declaration lives in internal/domain/contracts.go; this
// alias keeps existing consumers compiling. Deleted in PR-04n.
type ResourceCacheEntry = domain.ResourceCacheEntry

// ResourceCache is a read-only snapshot of already-loaded resource lists,
// keyed by resource short name. Declaration lives in internal/domain/contracts.go;
// this alias keeps existing consumers compiling. Deleted in PR-04n.
type ResourceCache = domain.ResourceCache

// RelatedChecker returns a count of related resources of a specific type.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type RelatedChecker = domain.RelatedChecker

// ValidateRelatedResult sanity-checks that a checker's result is internally
// consistent with its declared TargetType. Catches bugs where a checker
// scans the wrong cache (e.g., returning ecs-task IDs as TargetType "ecs").
//
// Returns the first violation as an error, or nil if the result is consistent.
// Currently checks:
//   - TargetType is non-empty
//   - When Count > 0, ResourceIDs is non-empty
//   - When Count is -1, no IDs are populated
//   - When Approximate is true, Count must be >= 0 (never paired with -1)
//
// This is intended for test invariants and optional debug-mode runtime checks,
// not for production error returns.
//
// For cross-checking that returned IDs match the target type's canonical
// Resource.ID, use ValidateRelatedResultAgainstCache.
func ValidateRelatedResult(r RelatedCheckResult) error {
	if r.TargetType == "" {
		return fmt.Errorf("RelatedCheckResult: empty TargetType")
	}
	if r.Count > 0 && len(r.ResourceIDs) == 0 {
		return fmt.Errorf("RelatedCheckResult[%s]: Count=%d but no ResourceIDs", r.TargetType, r.Count)
	}
	if r.Count == -1 && len(r.ResourceIDs) > 0 {
		return fmt.Errorf("RelatedCheckResult[%s]: Count=-1 but %d ResourceIDs present", r.TargetType, len(r.ResourceIDs))
	}
	if r.Approximate && r.Count < 0 {
		return fmt.Errorf("RelatedCheckResult[%s]: Approximate=true paired with Count=%d (must be >=0)", r.TargetType, r.Count)
	}
	return nil
}

// ValidateRelatedResultAgainstCache enforces the canonical-target-identity
// contract (#279): every ResourceID returned by a checker for a given
// TargetType MUST match the canonical Resource.ID that the TargetType's
// fetcher emits. We prove this by cross-checking the returned IDs against the
// target-type's cache entry.
//
// The check is deliberately opportunistic: it only runs when the cache has
// a non-truncated entry for the target type. A truncated cache could miss a
// legitimate ID, so we skip the check rather than produce false positives. If
// the target type has no cache entry at all, the check also skips (nothing to
// compare against). Shape invariants from ValidateRelatedResult are enforced
// regardless.
//
// This is the hard contract that catches bugs where a checker returns an ARN,
// name, or adjacent ID kind instead of the target type's canonical Resource.ID
// — the class of drill-in regressions called out in the architecture audit.
func ValidateRelatedResultAgainstCache(r RelatedCheckResult, cache ResourceCache) error {
	if err := ValidateRelatedResult(r); err != nil {
		return err
	}
	if len(r.ResourceIDs) == 0 {
		return nil
	}
	entry, ok := cache[r.TargetType]
	if !ok {
		return nil
	}
	if entry.IsTruncated {
		return nil
	}
	known := make(map[string]struct{}, len(entry.Resources))
	for _, res := range entry.Resources {
		known[res.ID] = struct{}{}
	}
	for _, id := range r.ResourceIDs {
		if _, seen := known[id]; !seen {
			return fmt.Errorf(
				"RelatedCheckResult[%s]: ResourceID %q is not a canonical Resource.ID for target type %q "+
					"(not found in target-type cache of %d resources); "+
					"checker likely returned an ARN/name/adjacent-ID kind instead of the target's canonical ID",
				r.TargetType, id, r.TargetType, len(entry.Resources),
			)
		}
	}
	return nil
}

// ApproximateZero returns a RelatedCheckResult representing "the checker scanned
// a truncated cache, found no matches in what was visible, but additional matches
// may exist beyond the cached window." Renders in the UI as "0+". This is the
// honest answer for reverse-scan checkers when `truncated && len(ids)==0`.
//
// Prefer this over `{Count: -1}` which means "unknown" and renders as a dead-
// ended dim row.
func ApproximateZero(targetType string) RelatedCheckResult {
	return RelatedCheckResult{
		TargetType:  targetType,
		Count:       0,
		Approximate: true,
	}
}

// UnknownRelated returns a RelatedCheckResult representing "the checker
// could not determine the count because a prerequisite lookup failed". The
// most common case is a two-hop checker (snapshot → source DB instance →
// cluster) where the SOURCE was not found in a truncated intermediate cache,
// so the hop to the TARGET was never attempted. Renders as "?".
//
// Distinct from ApproximateZero: ApproximateZero says "we scanned the target
// cache and found 0 matches (more may exist)". UnknownRelated says "we could
// not perform the scan at all". Distinct from the raw Count:-1 anti-pattern
// because this is a deliberate, audited unknown state (the count-minus-one
// guard test accepts this helper as an approved site).
func UnknownRelated(targetType string) RelatedCheckResult {
	return RelatedCheckResult{TargetType: targetType, Count: -1}
}

// NoopChecker is a stub RelatedChecker suitable for tests that exercise
// registry wiring (RegisterRelated / AppendRelated / GetRelated) without
// exercising real related-resource logic. Production code MUST NOT use it:
// RegisterRelated panics if any RelatedDef is registered with a nil Checker,
// but production tests using this explicit stub satisfy the guard while
// remaining free of test-specific behavior.
func NoopChecker(_ context.Context, _ any, _ Resource, _ ResourceCache) RelatedCheckResult {
	return RelatedCheckResult{}
}

// relatedRegistryMu guards relatedRegistry and relatedRegistryPrevious. All
// reads and writes to those two maps must hold this mutex.
var relatedRegistryMu sync.RWMutex

// relatedRegistry maps resource short names to their related resource definitions.
var relatedRegistry = map[string][]RelatedDef{}

// relatedRegistryPrevious is a stack (per short name) of registration snapshots
// saved before each RegisterRelated / AppendRelated call. UnregisterRelated pops
// the top entry to restore the previous state. Using a stack (instead of a single
// slot) prevents nested Register calls — typical when production init() registers
// once and a test then re-registers — from losing the original production
// registration past the second Unregister.
//
// A nil entry on the stack means "no previous registration existed" and Unregister
// should delete the active entry rather than restore.
var relatedRegistryPrevious = map[string][][]RelatedDef{}

// navigableFieldMu guards navigableFieldRegistry and navigableFieldPrevious.
// All reads and writes to those two maps must hold this mutex.
var navigableFieldMu sync.RWMutex

// navigableFieldRegistry maps resource short names to their active navigable
// field definitions. This is the mutable "session" registry: it starts empty
// and is populated only by explicit RegisterNavigableFields calls (from tests
// or from BootstrapActiveNavFields at app startup). This keeps unit tests that
// do not call RegisterNavigableFields isolated from production init-time defaults.
var navigableFieldRegistry = map[string][]NavigableField{}

// navigableFieldPrevious is a stack (per short name) of registration snapshots
// saved before each RegisterNavigableFields call. UnregisterNavigableFields pops
// the top entry to restore the previous state. Using a stack (instead of a single
// slot) prevents nested Register calls from losing the original default-registered
// state past the second Unregister.
var navigableFieldPrevious = map[string][][]NavigableField{}

// defaultNavFieldMu guards defaultNavFieldRegistry. Writes happen only during
// package init; reads can happen from any goroutine after startup. The mutex
// provides defense-in-depth for test binaries that may call
// RegisterDefaultNavFields from multiple goroutines in parallel.
var defaultNavFieldMu sync.RWMutex

// defaultNavFieldRegistry is an immutable-by-convention registry populated at
// init time by aws/*.go packages via RegisterDefaultNavFields. It is never
// modified after package initialisation. NavFieldsProvider (used by
// projection.Generic) reads from this registry. DetailModel reads from the
// mutable navigableFieldRegistry so that tests can construct models without
// any nav field registrations.
var defaultNavFieldRegistry = map[string][]NavigableField{}

// RegisterRelated stores related definitions for the given resource short
// name. Panics at init-time if any RelatedDef has a nil Checker or empty
// TargetType — a nil Checker is a structural bug, not a supported stub state.
//
// The current value for shortName (which may be nil) is pushed onto a per-key
// stack in relatedRegistryPrevious so that subsequent UnregisterRelated calls
// restore the previous registration instead of destroying it. This is critical
// for tests: production init() registers production defs once, and tests that
// override-then-cleanup must not nuke the production registration for the rest
// of the test process (AS-67).
func RegisterRelated(shortName string, defs []RelatedDef) {
	for _, d := range defs {
		if d.Checker == nil {
			panic(fmt.Sprintf("RegisterRelated(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, d.TargetType))
		}
		if d.TargetType == "" {
			panic(fmt.Sprintf("RegisterRelated(%q): empty TargetType — every RelatedDef must name a target", shortName))
		}
	}
	relatedRegistryMu.Lock()
	defer relatedRegistryMu.Unlock()
	existing := relatedRegistry[shortName] // nil when not yet set
	relatedRegistryPrevious[shortName] = append(relatedRegistryPrevious[shortName], existing)
	relatedRegistry[shortName] = defs
}

// GetRelated returns the related definitions for the given resource short name.
// Legacy-first: reads the mutable runtime map so RegisterRelated / AppendRelated
// overrides (test helpers, zzz_ct_events_all_related.go) take precedence over
// the catalog source slice. The catalog acts as the read-only fallback for the
// AS-795b–m transition window when a type's init() body is gone but the
// runtime map was not populated by either a sibling init() or aws.Install's
// bridgeCatalogToLegacy pass. AS-731 deletes both the legacy map and this
// fallback once every consumer reads catalog directly.
func GetRelated(shortName string) []RelatedDef {
	relatedRegistryMu.RLock()
	if defs, ok := relatedRegistry[shortName]; ok {
		relatedRegistryMu.RUnlock()
		return defs
	}
	relatedRegistryMu.RUnlock()
	if ct := catalog.Find(shortName); ct != nil && len(ct.Related) > 0 {
		return ct.Related
	}
	return nil
}

// UnregisterRelated restores the previous registration for the given short name
// (or deletes the entry entirely if no previous registration existed). Used only
// in tests for cleanup.
//
// Pops the most recently pushed snapshot from the per-key stack in
// relatedRegistryPrevious. If the popped snapshot is nil (the key had no entry
// before the most recent Register/Append call), the active-registry entry is
// deleted entirely. If the stack is empty (Unregister called without a matching
// Register/Append), the entry is deleted as a safe fallback — preserving the
// historical destructive semantics for test-only types like `test_append`,
// `srcType`, and `resizeTestType` that were never registered before the test.
func UnregisterRelated(shortName string) {
	relatedRegistryMu.Lock()
	defer relatedRegistryMu.Unlock()
	stack := relatedRegistryPrevious[shortName]
	if len(stack) == 0 {
		delete(relatedRegistry, shortName)
		return
	}
	prev := stack[len(stack)-1]
	relatedRegistryPrevious[shortName] = stack[:len(stack)-1]
	if prev == nil {
		delete(relatedRegistry, shortName)
	} else {
		relatedRegistry[shortName] = prev
	}
}

// FetchByIDsFunc fetches specific resource instances by ID, bypassing any
// filter the top-level paginated fetcher applies.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type FetchByIDsFunc = domain.FetchByIDsFunc

// fetchByIDsRegistry maps target resource short name to its FetchByIDs helper.
var fetchByIDsRegistry = map[string]FetchByIDsFunc{}

// RegisterFetchByIDs stores the FetchByIDs helper for the given target short
// name. Replaces any existing entry. Safe to call from an init() alongside
// RegisterPaginated.
func RegisterFetchByIDs(shortName string, fn FetchByIDsFunc) {
	fetchByIDsRegistry[shortName] = fn
}

// GetFetchByIDs returns the FetchByIDs helper for the target short name.
// Catalog-backed: falls through to the legacy map (catalog does not carry
// FetchByIDs separately in PR-04a; per-category PRs wire this). Legacy-first:
// test overrides via RegisterFetchByIDs take effect; otherwise reads the
// catalog FetchByIDs field.
func GetFetchByIDs(shortName string) FetchByIDsFunc {
	if fn, ok := fetchByIDsRegistry[shortName]; ok {
		return fn
	}
	if ct := catalog.Find(shortName); ct != nil && ct.FetchByIDs != nil {
		return ct.FetchByIDs
	}
	return nil
}

// UnregisterFetchByIDs removes the FetchByIDs helper for the given short
// name. Parity with UnregisterRelated — used only in tests for cleanup,
// never from production code.
func UnregisterFetchByIDs(shortName string) {
	delete(fetchByIDsRegistry, shortName)
}

// RegisterNavigableFields stores navigable field definitions for the given
// resource short name. Replaces any existing entry.
//
// The current value for shortName (which may be nil) is pushed onto a per-key
// stack in navigableFieldPrevious so that nested Register calls can all be
// rolled back in order by successive UnregisterNavigableFields calls.
//
// Contract: every Register MUST be paired with an Unregister, otherwise the
// per-key snapshot stack grows unbounded for the lifetime of the process. In
// practice every test that registers also unregisters via t.Cleanup; production
// callers register once at init and never unregister.
func RegisterNavigableFields(shortName string, fields []NavigableField) {
	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	existing := navigableFieldRegistry[shortName] // nil when not yet set
	navigableFieldPrevious[shortName] = append(navigableFieldPrevious[shortName], existing)
	navigableFieldRegistry[shortName] = fields
}

// GetNavigableFields returns the navigable field definitions for the given
// resource short name from the active registry. If the active registry has no
// entry for shortName, it falls back to the default (init-time) registry,
// then to the catalog. Returns nil only when no entry exists anywhere.
//
// Catalog-backed: catalog is checked after the active and default registries;
// per-category PRs (04b+) populate catalog entries. Fallback to legacy removed
// in PR-04n.
func GetNavigableFields(shortName string) []NavigableField {
	navigableFieldMu.RLock()
	if fields := navigableFieldRegistry[shortName]; len(fields) > 0 {
		navigableFieldMu.RUnlock()
		return fields
	}
	if fields := defaultNavFieldRegistry[shortName]; len(fields) > 0 {
		navigableFieldMu.RUnlock()
		return fields
	}
	navigableFieldMu.RUnlock()
	// Catalog fallback — active during 04b–04m for migrated types.
	if ct := catalog.Find(shortName); ct != nil && len(ct.Navigable) > 0 {
		return ct.Navigable
	}
	return nil
}

// GetActiveNavigableFields returns the navigable field definitions for the
// given resource short name from the active registry ONLY. Unlike
// GetNavigableFields, this function does NOT fall back to the default registry.
// Returns nil when no explicit RegisterNavigableFields call has been made for
// shortName.
//
// Used by DetailModel.buildFieldList so that navigable affordances in the
// detail view require an explicit registration (from tests or from
// BootstrapActiveNavFields at app startup). This prevents init-time default
// entries from being visible in test models that deliberately omit nav fields.
func GetActiveNavigableFields(shortName string) []NavigableField {
	navigableFieldMu.RLock()
	defer navigableFieldMu.RUnlock()
	return navigableFieldRegistry[shortName]
}

// IsFieldNavigable returns the NavigableField for the given field path, or nil if not registered.
func IsFieldNavigable(shortName, fieldPath string) *NavigableField {
	for _, f := range GetNavigableFields(shortName) {
		if f.FieldPath == fieldPath {
			return &f
		}
	}
	return nil
}

// UnregisterNavigableFields removes the navigable field registration for the
// given short name. Used only in tests for cleanup.
//
// Pops the most recently pushed snapshot from the per-key stack in
// navigableFieldPrevious. If the popped snapshot is nil (the key had no entry
// before the most recent Register call), the active-registry entry is deleted
// entirely. If the stack is empty (Unregister called without a matching
// Register), the entry is deleted as a safe fallback.
func UnregisterNavigableFields(shortName string) {
	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	stack := navigableFieldPrevious[shortName]
	if len(stack) == 0 {
		delete(navigableFieldRegistry, shortName)
		return
	}
	prev := stack[len(stack)-1]
	navigableFieldPrevious[shortName] = stack[:len(stack)-1]
	if prev == nil {
		delete(navigableFieldRegistry, shortName)
	} else {
		navigableFieldRegistry[shortName] = prev
	}
}

// RegisterDefaultNavFields stores the canonical (production) navigable field
// definitions for a resource type into the immutable-by-convention default
// registry. Called from aws/*.go init() functions instead of
// RegisterNavigableFields so that the mutable active registry (read by
// DetailModel) stays empty until BootstrapActiveNavFields is invoked at app
// startup. Tests that construct DetailModels directly never see init-time nav
// fields unless they explicitly call RegisterNavigableFields.
func RegisterDefaultNavFields(shortName string, fields []NavigableField) {
	defaultNavFieldMu.Lock()
	defer defaultNavFieldMu.Unlock()
	defaultNavFieldRegistry[shortName] = fields
}

// GetDefaultNavFields returns the default (init-time) navigable field definitions
// for the given resource short name. Returns nil if none were registered at init.
// Used by NavFieldsProvider so that projection.Generic always sees the canonical
// nav fields regardless of the active-registry state.
func GetDefaultNavFields(shortName string) []NavigableField {
	defaultNavFieldMu.RLock()
	if fields, ok := defaultNavFieldRegistry[shortName]; ok && len(fields) > 0 {
		defaultNavFieldMu.RUnlock()
		return fields
	}
	defaultNavFieldMu.RUnlock()
	if ct := catalog.Find(shortName); ct != nil && len(ct.Navigable) > 0 {
		return ct.Navigable
	}
	return nil
}

// BootstrapActiveNavFields copies all entries from the default nav field
// registry into the active registry. Called once at app startup (from
// cmd/a9s/main.go) so that DetailModel navigability works in production.
// Must be called after all init() functions have run (i.e. inside main()).
// Noop in test binaries that never call this function.
//
// Concurrency: NOT safe to call concurrently with RegisterDefaultNavFields.
// Bootstrap snapshots the default registry under one lock, releases it, and
// then takes the active registry's lock — there is a small window where a
// concurrent RegisterDefaultNavFields would not be reflected in the snapshot.
// In production this is fine because Bootstrap runs after init() in the
// single-threaded main goroutine, before any concurrent activity begins.
func BootstrapActiveNavFields() {
	defaultNavFieldMu.RLock()
	snapshot := make(map[string][]NavigableField, len(defaultNavFieldRegistry))
	maps.Copy(snapshot, defaultNavFieldRegistry)
	defaultNavFieldMu.RUnlock()

	navigableFieldMu.Lock()
	defer navigableFieldMu.Unlock()
	for k, v := range snapshot {
		// Only populate entries that have not already been explicitly set via
		// RegisterNavigableFields. This preserves test-supplied overrides when
		// BootstrapActiveNavFields is called inside tui.New (e.g. by golden
		// scenario helpers that register custom nav fields before constructing
		// the TUI model).
		if _, exists := navigableFieldRegistry[k]; !exists {
			navigableFieldRegistry[k] = v
		}
	}
}

// AppendRelated adds a single RelatedDef to the existing registration for shortName.
// If the target type is already present, it is a no-op (prevents duplicates).
// If no registration exists yet, it creates a new one. Panics at init-time if
// def.Checker is nil or def.TargetType is empty — a nil Checker is a
// structural bug, not a supported stub state.
//
// Like RegisterRelated, the pre-append value is pushed onto the per-key
// snapshot stack so that a subsequent UnregisterRelated restores the previous
// state (or deletes the entry, if no prior value existed). A duplicate-target
// no-op does NOT push a snapshot — Unregister has nothing to undo.
func AppendRelated(shortName string, def RelatedDef) {
	if def.Checker == nil {
		panic(fmt.Sprintf("AppendRelated(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, def.TargetType))
	}
	if def.TargetType == "" {
		panic(fmt.Sprintf("AppendRelated(%q): empty TargetType — every RelatedDef must name a target", shortName))
	}
	relatedRegistryMu.Lock()
	defer relatedRegistryMu.Unlock()
	existing := relatedRegistry[shortName]
	for _, d := range existing {
		if d.TargetType == def.TargetType {
			return // already registered, skip duplicate
		}
	}
	relatedRegistryPrevious[shortName] = append(relatedRegistryPrevious[shortName], existing)
	relatedRegistry[shortName] = append(existing, def)
}

// BuildCloudTrailFilter returns the CloudTrail LookupEvents filter for a resource.
// The filter is determined by the resource type's CloudTrailKey field, not by heuristics.
// Returns nil when the resource type has no CloudTrail support (empty CloudTrailKey).
func BuildCloudTrailFilter(res Resource, resourceType string) map[string]string {
	rt := FindResourceType(resourceType)
	if rt == nil || rt.CloudTrailKey == "" {
		return nil
	}
	return buildFilterFromKey(res, rt.CloudTrailKey)
}

func buildFilterFromKey(res Resource, ctKey string) map[string]string {
	parts := strings.SplitN(ctKey, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	attr, source := parts[0], parts[1]

	var val string
	switch source {
	case "ID":
		val = res.ID
	case "Name":
		val = res.Name
	default:
		if key, ok := strings.CutPrefix(source, "Fields."); ok {
			val = res.Fields[key]
		}
	}
	if val == "" {
		return nil
	}
	return map[string]string{attr: val}
}
