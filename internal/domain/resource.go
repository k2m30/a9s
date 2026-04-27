// Package domain is the leaf type-declaration package for a9s.
// It carries no internal imports and no presentation library imports.
// All types declared here are stable contracts shared across layers.
package domain

// Resource represents a single AWS resource instance.
// Moved from internal/resource/resource.go in Phase 01.
// internal/resource re-exports this via a type alias.
//
// Phase 03 will migrate Status/Issues to Findings/AttentionDetails.
// Those fields are intentionally kept here until that phase.
type Resource struct {
	// ID is the primary identifier (instance ID, ARN, name).
	ID string
	// Name is the display name (from Name tag or identifier).
	Name string
	// Type is the resource short name (e.g. "ec2", "rds", "s3"). Set by
	// fetchers and by the detail view before calling a DetailProjector.
	// Used by projection.Generic to look up per-type view config, navigable
	// fields, and field aliases. Empty string = unknown type (falls back to
	// Fields-only rendering with no navigability).
	Type string
	// Status is the current state/status of the resource. Carries the top
	// phrase (with optional `(+N)` suffix) for the list view.
	// Phase 03 migrates this to Findings.
	Status string
	// Issues carries every active Wave 1 issue phrase in precedence order.
	// Empty for Healthy rows. Phase 03 migrates this to Findings.
	Issues []string
	// Fields holds all visible column values by key.
	Fields map[string]string
	// RawStruct holds the original AWS SDK typed struct for reflection-based
	// field extraction.
	RawStruct any
}
