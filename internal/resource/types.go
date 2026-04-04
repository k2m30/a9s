package resource

import "strings"

// Column defines a column in a resource table view.
type Column struct {
	// Key is the field key used to extract the value from Resource.Fields.
	Key string
	// Title is the column header display text.
	Title string
	// Width is the fixed column width; 0 means flexible.
	Width int
	// Sortable indicates whether this column supports sorting.
	Sortable bool
}

// ChildViewDef describes a child view that can be drilled into from a parent
// resource list. Used to make child-view navigation data-driven instead of
// hardcoded per resource type.
type ChildViewDef struct {
	// ChildType is the registered child type short name (e.g., "s3_objects").
	ChildType string
	// Key is the trigger key name (e.g., "enter", "e", "L").
	Key string
	// ContextKeys maps child-fetcher parameter names to source expressions:
	//   "ID"         → parent resource's ID
	//   "Name"       → parent resource's Name
	//   "@parent.x"  → value from the parent view's ParentContext["x"]
	//   anything else → parent resource's Fields[key]
	ContextKeys map[string]string
	// DisplayNameKey is the context key whose value becomes the child view's
	// display name (frame title). For example "bucket" shows the bucket name.
	DisplayNameKey string
	// DrillCondition is an optional predicate. When non-nil, the child view
	// is only entered if the predicate returns true for the selected resource.
	// A nil DrillCondition means always drill.
	DrillCondition func(Resource) bool
	// DrillBlockMessage is the flash text shown when DrillCondition returns false.
	// Empty means silent skip (no flash).
	DrillBlockMessage string
}

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
