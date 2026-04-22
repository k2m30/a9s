// Package resource defines the generic resource model used across all AWS resource types.
package resource

// Resource represents a single AWS resource instance displayed in a table row.
type Resource struct {
	// ID is the primary identifier (instance ID, ARN, name).
	ID string
	// Name is the display name (from Name tag or identifier).
	Name string
	// Status is the current state/status of the resource. Carries the top
	// §4 phrase (with optional `(+N)` suffix) for the list view.
	Status string
	// Issues carries every active Wave 1 issue phrase in §4 precedence order.
	// Populated by the fetcher for resources with multiple coexisting warnings.
	// Spec rule 7 (S5): "every finding individually visible" — the detail view
	// renders this as a leading Issues section so the operator sees each active
	// signal, not just the top phrase in Status. Empty for Healthy rows.
	Issues []string
	// Fields holds all visible column values by key.
	Fields map[string]string
	// RawStruct holds the original AWS SDK typed struct for reflection-based field extraction.
	RawStruct any
}
