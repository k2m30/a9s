package resource

import "context"

// RelatedDef defines one related resource class for a given resource type.
type RelatedDef struct {
	TargetType  string         // target resource short name (e.g., "tg", "alarm")
	DisplayName string         // right-column row label (e.g., "Target Groups")
	Checker     RelatedChecker // async checker function (may be nil for stubs)
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
}

// ResourceCache is a read-only snapshot of already-loaded resource lists.
type ResourceCache map[string][]Resource

// RelatedChecker returns a count of related resources of a specific type.
type RelatedChecker func(ctx context.Context, clients interface{}, res Resource, cache ResourceCache) RelatedCheckResult

// RelatedDemoChecker returns hardcoded results for demo mode.
type RelatedDemoChecker func(res Resource) []RelatedCheckResult

// relatedRegistry maps resource short names to their related resource definitions.
var relatedRegistry = map[string][]RelatedDef{}

// navigableFieldRegistry maps resource short names to their navigable field definitions.
var navigableFieldRegistry = map[string][]NavigableField{}

// relatedDemoRegistry maps resource short names to their demo checker functions.
var relatedDemoRegistry = map[string]RelatedDemoChecker{}

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

// UnregisterNavigableFields removes navigable field definitions for the given short name.
// Used only in tests for cleanup.
func UnregisterNavigableFields(shortName string) {
	delete(navigableFieldRegistry, shortName)
}

// RegisterRelatedDemo stores a demo checker for the given resource short name.
// Replaces any existing entry.
func RegisterRelatedDemo(shortName string, f RelatedDemoChecker) {
	relatedDemoRegistry[shortName] = f
}

// GetRelatedDemo returns the demo checker for the given resource short name,
// or nil if none are registered.
func GetRelatedDemo(shortName string) RelatedDemoChecker {
	return relatedDemoRegistry[shortName]
}
