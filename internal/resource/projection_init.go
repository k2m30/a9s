package resource

import (
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// init wires the resource-registry callbacks into the generic projector so
// that projection.Generic can access navigable-field definitions, ID
// resolvers, and field-alias normalisers without importing internal/resource
// (which would create an import cycle).
func init() {
	// NavFieldsProvider reads from the immutable default registry only.
	// Tests that need to override nav fields use GenericWithConfigAndNavProvider
	// to inject a custom provider; they do NOT mutate the global default.
	// This eliminates the active-registry test-pollution surface that caused
	// Ubuntu CI failures (see PR #301 review).
	projection.NavFieldsProvider = GetDefaultNavFields
	projection.NavIDProvider = NavIDFromValue
	projection.FieldAliasProvider = ApplyFieldAliases
	projection.FieldKeysProvider = GetFieldKeys
}
