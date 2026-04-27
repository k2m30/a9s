package resource

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// Color classifies a resource's health for display, filtering, and badges.
type Color uint8

const (
	ColorHealthy Color = iota // green  — nominal
	ColorWarning              // yellow — transitioning / degrading
	ColorBroken               // red    — stopped / failed / impaired
	ColorDim                  // grey   — terminated / inactive
)

// IsIssue reports whether this color contributes to attention filtering and issue badges.
func (c Color) IsIssue() bool { return c == ColorWarning || c == ColorBroken }

// fallbackColor classifies a resource status string when no per-type Color func
// is set. Covers the common AWS vocabulary so ad-hoc test ResourceTypeDef
// instances (which omit Color) behave sensibly without requiring every test to
// set up a full registered type.
func fallbackColor(status string) Color {
	switch status {
	case "running", "available", "active", "ACTIVE", "AVAILABLE", "RUNNING",
		"in-service", "healthy":
		return ColorHealthy
	case "stopped", "failed", "error", "impaired", "FAILED", "ERROR",
		"STOPPED":
		return ColorBroken
	case "terminated", "TERMINATED", "shutting-down", "deleted", "DELETED",
		"deregistered", "inactive", "INACTIVE":
		// "inactive" is a steady-state (e.g. ASG scaled to 0, disabled rule);
		// dim, not broken. Aligns with StandardLifecycleColor.
		return ColorDim
	}
	// Suffix patterns for compound statuses (e.g. "create_failed", "update_in_progress").
	lower := strings.ToLower(status)
	switch {
	case strings.HasSuffix(lower, "_failed") || strings.HasSuffix(lower, "-failed"):
		return ColorBroken
	case strings.HasSuffix(lower, "_in_progress") || strings.HasSuffix(lower, "_progress") ||
		strings.HasSuffix(lower, "-in-progress") || status == "pending" ||
		status == "creating" || status == "modifying" || status == "updating" ||
		status == "initializing":
		return ColorWarning
	}
	return ColorHealthy
}

// ResolveColor classifies r using d.Color, defaulting to a generic status-based
// color when d.Color is nil. All registered types have non-nil Color (invariant #7);
// the fallback exists only for ad-hoc ResourceTypeDef test doubles.
func (d ResourceTypeDef) ResolveColor(r Resource) Color {
	if d.Color == nil {
		return fallbackColor(r.Status)
	}
	return d.Color(r)
}

// Column defines a column in a resource table view.
// Declaration lives in internal/domain/contracts.go; this alias keeps
// existing consumers compiling. Deleted in PR-04n.
type Column = domain.Column

// ChildViewDef describes a child view that can be drilled into from a parent
// resource list. Declaration lives in internal/domain/contracts.go; this alias
// keeps existing consumers compiling. Deleted in PR-04n.
type ChildViewDef = domain.ChildViewDef

// ResourceTypeDef defines a category of AWS resources the app can browse.
type ResourceTypeDef struct {
	// Name is the display name (e.g., "EC2 Instances").
	Name string
	// ShortName is the colon-command alias (e.g., "ec2").
	ShortName string
	// ListTitle overrides ShortName for list view frame titles (e.g., "alarms" instead of "alarm").
	// When empty, ShortName is used.
	ListTitle string
	// Aliases are alternative command names for this resource type.
	Aliases []string
	// Category groups resource types in the main menu (e.g., "COMPUTE", "NETWORKING").
	Category string
	// Columns are the table columns for list view.
	Columns []Column
	// Children defines child views that can be drilled into from this resource
	// type's list view. Each entry maps a key press to a child type navigation.
	Children []ChildViewDef
	// CopyField overrides which field CopyContent copies. When non-empty,
	// the resource list copies Fields[CopyField] instead of the default ID.
	CopyField string
	// StubCreator optionally creates a minimal stub Resource for the given ID when
	// the target resource is not yet in the resource cache. Used by ResourceListModel
	// to auto-navigate to a detail view when the filtered list is empty but a specific
	// target ID is known (e.g., AMI navigation from EC2 detail before AMIs are loaded).
	// When nil, no stub navigation occurs — the list just shows empty with a spinner.
	StubCreator func(id string) Resource
	// RelatedContextFromIDs extracts the ParentContext for a child-view navigation
	// triggered from the related panel. Called when this type is the target of a
	// RelatedNavigateMsg and the type is a registered child type (isChild=true).
	// The first non-empty ID in relatedIDs is typically the encoded parent+child key.
	// When nil, an empty parent context is used (bucket/prefix shown as empty).
	RelatedContextFromIDs func(relatedIDs []string) map[string]string
	// CloudTrailKey specifies how to build the CloudTrail LookupEvents filter.
	// Format: "LookupAttr:ValueSource" where:
	//   LookupAttr  = "ResourceName" or "Username"
	//   ValueSource = "ID" (res.ID), "Name" (res.Name), or "Fields.xxx" (res.Fields["xxx"])
	// Empty string means no CloudTrail support (t key suppressed).
	CloudTrailKey string
	// IdentityKey optionally names the column key used to position the
	// enrichment-finding row marker. When empty, the row-marker resolver uses
	// a cascade: match "name" key → path contains "Name"/"Identifier" →
	// title equal to "Name" or the type Name → column index 0.
	// Set this only when the cascade would pick the wrong column.
	IdentityKey string

	// Color classifies the row's health. REQUIRED for all registered types.
	// Reads the resource's structural fields directly.
	Color func(Resource) Color

	// ExcludeFromIssueBadge, when true, still colors rows and honors ctrl+z,
	// but excludes the type from the main-menu badge count. Used for event-severity
	// types (ct-events) where severity is event-level, not resource-health.
	ExcludeFromIssueBadge bool

	// CellDecorators optionally transforms cell values per column key before render.
	// Key = column key; value = decorator func receiving the full resource and the
	// already-extracted cell string, returning the replacement. nil map = no decorators.
	CellDecorators map[string]func(r Resource, value string) string

	// Project is an optional custom DetailProjector for this resource type.
	// When nil, projection.Generic is used as the fallback projector.
	// Set on types whose detail view requires specialised rendering that the
	// generic field-path projector cannot produce (e.g. ct-events).
	Project domain.DetailProjector

	// Augment is an optional post-projector hook that injects additional sections
	// after the main projector has run (e.g. EC2 status checks). When nil, no
	// augmentation is applied. Pure function.
	Augment domain.Augmenter
}

// resourceTypes holds all registered resource type definitions in menu display order.
// Built once at package init from category-specific functions to preserve deterministic ordering.
var resourceTypes = buildResourceTypes()

func buildResourceTypes() []ResourceTypeDef {
	var all []ResourceTypeDef
	all = append(all, computeResourceTypes()...)
	all = append(all, containersResourceTypes()...)
	all = append(all, networkingResourceTypes()...)
	all = append(all, databasesResourceTypes()...)
	all = append(all, monitoringResourceTypes()...)
	all = append(all, messagingResourceTypes()...)
	all = append(all, secretsResourceTypes()...)
	all = append(all, dnsCdnResourceTypes()...)
	all = append(all, securityResourceTypes()...)
	all = append(all, cicdResourceTypes()...)
	all = append(all, dataResourceTypes()...)
	all = append(all, backupResourceTypes()...)
	return all
}

// AllResourceTypes returns the definitions for all supported resource types.
func AllResourceTypes() []ResourceTypeDef {
	result := make([]ResourceTypeDef, len(resourceTypes))
	copy(result, resourceTypes)
	return result
}

// AllShortNames returns the ShortName of every registered resource type.
func AllShortNames() []string {
	names := make([]string, len(resourceTypes))
	for i, rt := range resourceTypes {
		names[i] = rt.ShortName
	}
	return names
}

// FindResourceType looks up a resource type by its ShortName or any of its Aliases.
// Returns nil if no match is found.
func FindResourceType(name string) *ResourceTypeDef {
	for i := range resourceTypes {
		if strings.EqualFold(resourceTypes[i].ShortName, name) {
			return &resourceTypes[i]
		}
		for _, alias := range resourceTypes[i].Aliases {
			if strings.EqualFold(alias, name) {
				return &resourceTypes[i]
			}
		}
	}
	return nil
}
