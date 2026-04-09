package resource

// ResolveNavigationTarget looks up a navigation target by name, checking both
// top-level resource type and child type registries.
// Returns (display, isChild, found).
// isChild is true when the match comes from the child type registry.
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
	return "", false, false
}
