package resource

import (
	"context"
	"fmt"
	"strings"
)

// RelatedDef defines one related resource class for a given resource type.
type RelatedDef struct {
	TargetType       string         // target resource short name (e.g., "tg", "alarm")
	DisplayName      string         // right-column row label (e.g., "Target Groups")
	// TODO(no-middle-state): a registered RelatedDef must have a real Checker.
	// Treat nil as a structural bug, not as a supported "stub" state.
	Checker          RelatedChecker // async checker function
	NeedsTargetCache bool           // true if checker reads target type from ResourceCache
}

// NavigableField associates a detail view field path with a target resource type.
type NavigableField struct {
	FieldPath  string // matches a path in ViewDef.Detail (e.g., "VpcId")
	TargetType string // resource short name (e.g., "vpc")
}

// RelatedCheckResult is returned by a RelatedChecker and carries all state
// needed by the right-column panel to display a row and navigate on Enter.
//
// Semantics (FR-008 / FR-014):
//
//   - Count == -1: unknown — the checker could not determine a count (wrong
//     RawStruct type, nil clients, API error, or a stubbed checker). The UI
//     renders "?" for the row.
//   - Count == 0: definitively zero related resources of this type. The UI
//     dims the row.
//   - Count >= 1: confirmed N related resources. The UI highlights the row.
//   - Approximate == true: Count was derived from a truncated cache page; more
//     matches may exist beyond the cached window. The UI renders "N+" (or
//     "0+"). Only valid on reverse-scan checkers (NeedsTargetCache: true);
//     forward checkers MUST leave this false. Invariant: Approximate == true ⇒
//     Count >= 0.
//   - FetchFilter non-nil: navigation drill-in should use a server-side
//     filtered paginated fetcher rather than a relatedIDSet jump.
type RelatedCheckResult struct {
	TargetType  string   // echoed from RelatedDef.TargetType
	Count       int      // -1 = unknown; 0+ = count
	ResourceIDs []string // IDs of found related resources (empty when Count <= 0)
	Err         error    // non-nil = error
	// FetchFilter when non-nil signals navigation to use a server-side filtered fetcher instead of relatedIDSet.
	FetchFilter map[string]string
	// Approximate is true when Count was derived from a truncated reverse-scan cache entry.
	// Pairs only with Count >= 0; UI renders "N+" / "0+". Forward checkers MUST leave this false.
	Approximate bool
}

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state. IsTruncated=true means the snapshot is a partial page;
// related checkers should return Count=-1 (unknown) when 0 local matches found.
type ResourceCacheEntry struct {
	Resources   []Resource
	IsTruncated bool
	Pagination  *PaginationMeta // full pagination from cold-miss fetch; nil when derived from snapshot
}

// ResourceCache is a read-only snapshot of already-loaded resource lists,
// keyed by resource short name. Each entry carries truncation state so that
// related checkers can distinguish "0 matches in complete list" (Count=0)
// from "0 matches in partial list" (Count=-1).
type ResourceCache map[string]ResourceCacheEntry

// RelatedChecker returns a count of related resources of a specific type.
type RelatedChecker func(ctx context.Context, clients any, res Resource, cache ResourceCache) RelatedCheckResult

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
// not for production error returns. The drill-in path can additionally cross-
// check that returned IDs exist in the target-type's cache (out of scope here).
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

// relatedRegistry maps resource short names to their related resource definitions.
var relatedRegistry = map[string][]RelatedDef{}

// navigableFieldRegistry maps resource short names to their navigable field definitions.
var navigableFieldRegistry = map[string][]NavigableField{}

// RegisterRelated stores related definitions for the given resource short name.
// Replaces any existing entry.
func RegisterRelated(shortName string, defs []RelatedDef) {
	relatedRegistry[shortName] = defs
}

// GetRelated returns the related definitions for the given resource short name,
// or nil if none are registered.
func GetRelated(shortName string) []RelatedDef {
	return relatedRegistry[shortName]
}

// UnregisterRelated removes related definitions for the given short name.
// Used only in tests for cleanup.
func UnregisterRelated(shortName string) {
	delete(relatedRegistry, shortName)
}

// RegisterNavigableFields stores navigable field definitions for the given resource short name.
// Replaces any existing entry.
func RegisterNavigableFields(shortName string, fields []NavigableField) {
	navigableFieldRegistry[shortName] = fields
}

// GetNavigableFields returns the navigable field definitions for the given resource short name,
// or nil if none are registered.
func GetNavigableFields(shortName string) []NavigableField {
	return navigableFieldRegistry[shortName]
}

// IsFieldNavigable returns the NavigableField for the given field path, or nil if not registered.
func IsFieldNavigable(shortName, fieldPath string) *NavigableField {
	for _, f := range navigableFieldRegistry[shortName] {
		if f.FieldPath == fieldPath {
			return &f
		}
	}
	return nil
}

// UnregisterNavigableFields removes navigable field definitions for the given short name.
// Used only in tests for cleanup.
func UnregisterNavigableFields(shortName string) {
	delete(navigableFieldRegistry, shortName)
}

// AppendRelated adds a single RelatedDef to the existing registration for shortName.
// If the target type is already present, it is a no-op (prevents duplicates).
// If no registration exists yet, it creates a new one.
func AppendRelated(shortName string, def RelatedDef) {
	existing := relatedRegistry[shortName]
	for _, d := range existing {
		if d.TargetType == def.TargetType {
			return // already registered, skip duplicate
		}
	}
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
