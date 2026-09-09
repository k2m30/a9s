// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package resource

import (
	"github.com/k2m30/a9s/v3/core/semantics/projection"
)

// WireProjection wires the resource-registry callbacks into the generic
// projectors so that projection.GenericWithConfig and
// projection.GenericWithConfigAndNavProvider can access navigable-field
// definitions, ID resolvers, and field-alias normalisers without importing
// core/resource (which would create an import cycle).
//
// `core/resource/` deliberately contains zero `func init()`.
// WireProjection is
// explicitly called from cmd/a9s/main.go and from every TestMain that needs
// a wired projector. Idempotent: callers may invoke it multiple times safely.
func WireProjection() {
	// NavFieldsProvider reads from the immutable default registry only.
	// Tests that need to override nav fields use GenericWithConfigAndNavProvider
	// to inject a custom provider; they do NOT mutate the global default.
	// A mutable global default would let one test's registration leak into
	// the next.
	projection.NavFieldsProvider = GetDefaultNavFields
	projection.NavIDProvider = NavIDFromValue
	projection.FieldAliasProvider = ApplyFieldAliases
	projection.FieldKeysProvider = GetFieldKeys
}
