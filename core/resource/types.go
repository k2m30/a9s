// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
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

// ResolveChildContext resolves a ChildViewDef's ContextKeys against the
// selected resource, producing the parentCtx map handed to the child's
// ChildFetcher. Source expressions:
//   - "ID"            → r.ID
//   - "Name"           → r.Name
//   - "@parent.<key>" → parentCtx[<key>] (the grandparent context, for
//     multi-level drills)
//   - anything else    → r.Fields[source]
//
// This is the single resolver for ContextKeys; every navigation path that
// enters a child view (direct Enter, auto-open-single-detail) must call this
// rather than re-deriving the map inline; a hand-rolled r.Fields[source]-only
// copy silently drops "ID"/"Name" sources to "" for any ContextKeys entry
// that reads from them.
func ResolveChildContext(child ChildViewDef, r *domain.Resource, parentCtx map[string]string) map[string]string {
	ctx := make(map[string]string, len(child.ContextKeys))
	for param, source := range child.ContextKeys {
		switch {
		case source == "ID":
			ctx[param] = r.ID
		case source == "Name":
			ctx[param] = r.Name
		case strings.HasPrefix(source, "@parent."):
			parentKey := strings.TrimPrefix(source, "@parent.")
			if parentCtx != nil {
				ctx[param] = parentCtx[parentKey]
			}
		default:
			ctx[param] = r.Fields[source]
		}
	}
	return ctx
}

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

// DetailFrameTitle composes the TUI detail-view frame-border title. When
// omitID is true (types whose ID is an opaque synthetic key, e.g. a 56-digit
// CloudWatch event id — see ResourceTypeDef.TitleOmitsID) it renders
// "detail -- <Name>", falling back to "detail -- <ID>" when name is empty.
// Otherwise it renders the standard "detail -- <ID> (<Name>)" form. Pure
// string composition — callers resolve TitleOmitsID via GetChildType /
// FindResourceType and pass it in.
func DetailFrameTitle(id, name string, omitID bool) string {
	if omitID {
		if name != "" {
			return "detail -- " + name
		}
		return "detail -- " + id
	}
	if id == "" {
		return "detail"
	}
	if name != "" {
		return "detail -- " + id + " (" + name + ")"
	}
	return "detail -- " + id
}
