package resource

import (
	"context"
	"strings"
)

// RelatedDef defines one related resource class for a given resource type.
type RelatedDef struct {
	TargetType       string         // target resource short name (e.g., "tg", "alarm")
	DisplayName      string         // right-column row label (e.g., "Target Groups")
	Checker          RelatedChecker // async checker function (may be nil for stubs)
	NeedsTargetCache bool           // true if checker reads target type from ResourceCache
}

// NavigableField associates a detail view field path with a target resource type.
type NavigableField struct {
	FieldPath  string // matches a path in ViewDef.Detail (e.g., "VpcId")
	TargetType string // resource short name (e.g., "vpc")
}

// RelatedCheckResult is returned by a RelatedChecker.
type RelatedCheckResult struct {
	TargetType  string   // echoed from RelatedDef.TargetType
	Count       int      // -1 = unknown; 0+ = count
	ResourceIDs []string // IDs of found related resources (empty when Count <= 0)
	Err         error    // non-nil = error
	// FetchFilter when non-nil signals navigation to use a server-side filtered fetcher instead of relatedIDSet.
	FetchFilter map[string]string
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
