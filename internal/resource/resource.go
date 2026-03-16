// Package resource defines the generic resource model used across all AWS resource types.
package resource

// Resource represents a single AWS resource instance displayed in a table row.
type Resource struct {
	// ID is the primary identifier (instance ID, ARN, name).
	ID string
	// Name is the display name (from Name tag or identifier).
	Name string
	// Status is the current state/status of the resource.
	Status string
	// Fields holds all visible column values by key.
	Fields map[string]string
	// RawJSON is the raw JSON representation for JSON view.
	RawJSON string
	// DetailData holds all attributes for the describe view.
	DetailData map[string]string
	// RawStruct holds the original AWS SDK typed struct for reflection-based field extraction.
	RawStruct interface{}
}
