package resource

import (
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// init wires the resource-registry callbacks into the generic projector so
// that projection.Generic can access navigable-field definitions, ID
// resolvers, and field-alias normalisers without importing internal/resource
// (which would create an import cycle).
func init() {
	// NavFieldsProvider reads the active registry first; falls back to the
	// default (init-time) registry. This ensures projection.Generic always
	// sees canonical nav fields (for audit tests and projection.Generic callers)
	// while respecting explicit overrides from tests or the active session.
	projection.NavFieldsProvider = func(shortName string) []domain.NavigableField {
		if fields := GetNavigableFields(shortName); len(fields) > 0 {
			return fields
		}
		return GetDefaultNavFields(shortName)
	}
	projection.NavIDProvider = NavIDFromValue
	projection.FieldAliasProvider = ApplyFieldAliases
	projection.FieldKeysProvider = GetFieldKeys
}
