package resource

// ResolveNavigationTarget looks up a navigation target by name, checking both
// top-level resource type and child type registries.
// Returns (display, isChild, found).
// isChild is true when the match comes from the child type registry.
//
// A type is considered "found" if any of the following is true:
//   - It is registered as a child type (isChild=true).
//   - It has a full ResourceTypeDef in the type registry.
//   - It has a registered paginated fetcher (e.g. a dynamic test type). In this
//     case display is empty and isChild=false; handleRelatedNavigate falls back to
//     a raw fetch without rendering a typed list view.
func ResolveNavigationTarget(name string) (display string, isChild bool, found bool) {
	// Check child type registry first — child types take precedence over any
	// top-level type with the same short name (e.g. s3_objects is registered both
	// ways for fixture-discovery purposes, but navigates as a child type).
	if ct := GetChildType(name); ct != nil {
		return ct.Name, true, true
	}
	if rt := FindResourceType(name); rt != nil {
		return rt.Name, false, true
	}
	// Fetcher-only types (no visual ResourceTypeDef): treat as navigable so the
	// lazy-cache partial-coverage path in handleRelatedNavigate can still fall
	// through to a full fetch. The caller must handle rt==nil gracefully.
	if GetPaginatedFetcher(name) != nil {
		return "", false, true
	}
	return "", false, false
}
