package resource

import (
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
)

// Color classifies a resource's health for display, filtering, and badges.
// Type alias of domain.Color — zero-churn backward compat for TUI consumers.
type Color = domain.Color

const (
	ColorHealthy = domain.ColorHealthy // green  — nominal
	ColorWarning = domain.ColorWarning // yellow — transitioning / degrading
	ColorBroken  = domain.ColorBroken  // red    — stopped / failed / impaired
	ColorDim     = domain.ColorDim     // grey   — terminated / inactive
)

// ResourceTypeDef defines a category of AWS resources the app can browse.
// Type alias of catalog.ResourceTypeDef — zero-churn backward compat for TUI consumers.
type ResourceTypeDef = catalog.ResourceTypeDef

// Column defines a column in a resource table view.
// Type alias of domain.Column — zero-churn backward compat for TUI consumers.
type Column = domain.Column

// ChildViewDef describes a child view that can be drilled into from a parent
// resource list. Type alias of domain.ChildViewDef — zero-churn backward compat.
type ChildViewDef = domain.ChildViewDef

// AllResourceTypes returns the definitions for all supported resource types.
// Pure catalog passthrough.
func AllResourceTypes() []ResourceTypeDef {
	return catalog.All()
}

// AllShortNames returns the ShortName of every registered resource type.
func AllShortNames() []string {
	return catalog.AllShortNames()
}

// FindResourceType looks up a resource type by its ShortName or any of its Aliases.
func FindResourceType(name string) *ResourceTypeDef {
	return catalog.Find(name)
}
