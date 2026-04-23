package resource

import (
	"context"
	"fmt"
	"strings"
)

// RelatedDef defines one related resource class for a given resource type.
type RelatedDef struct {
	TargetType  string // target resource short name (e.g., "tg", "alarm")
	DisplayName string // right-column row label (e.g., "Target Groups")
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

// relatedRegistry maps resource short names to their related resource definitions.
var relatedRegistry = map[string][]RelatedDef{}

// navigableFieldRegistry maps resource short names to their navigable field definitions.
var navigableFieldRegistry = map[string][]NavigableField{}

// RegisterRelated stores related definitions for the given resource short
// name. Panics at init-time if any RelatedDef has a nil Checker or empty
// TargetType — a nil Checker is a structural bug, not a supported stub state.
// Replaces any existing entry.
func RegisterRelated(shortName string, defs []RelatedDef) {
	for _, d := range defs {
		if d.Checker == nil {
			panic(fmt.Sprintf("RegisterRelated(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, d.TargetType))
		}
		if d.TargetType == "" {
			panic(fmt.Sprintf("RegisterRelated(%q): empty TargetType — every RelatedDef must name a target", shortName))
		}
	}
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
// If no registration exists yet, it creates a new one. Panics at init-time if
// def.Checker is nil or def.TargetType is empty — a nil Checker is a
// structural bug, not a supported stub state.
func AppendRelated(shortName string, def RelatedDef) {
	if def.Checker == nil {
		panic(fmt.Sprintf("AppendRelated(%q): nil Checker for target %q — every RelatedDef must have a real checker", shortName, def.TargetType))
	}
	if def.TargetType == "" {
		panic(fmt.Sprintf("AppendRelated(%q): empty TargetType — every RelatedDef must name a target", shortName))
	}
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
