// Package domain is the leaf type-declaration package for a9s.
// It carries no internal imports and no presentation library imports.
// All types declared here are stable contracts shared across layers.
package domain

// Resource represents a single AWS resource instance.
// Moved from internal/resource/resource.go in Phase 01.
// internal/resource re-exports this via a type alias.
//
// W1.4a deleted the legacy derive shim; the Status/Issues fields are
// still consumed by a handful of read sites in internal/catalog,
// internal/semantics/{ctevent,projection}, internal/aws/catalog_compute,
// and internal/tui/views. W1.4b (AS-1428) will migrate those readers
// and the remaining fetcher Status: writes, then delete the fields.
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
	// Findings is the canonical finding list. Phase 03 populates this; views
	// read Findings[0].Phrase / Findings[0].Severity for list rendering.
	// Empty for healthy rows.
	Findings []Finding
	// AttentionDetails carries supporting structured facts per finding,
	// keyed by Finding.Code. Consumed only by the detail view's Attention
	// section. Nil/empty for rows with no findings or no extra facts.
	AttentionDetails map[FindingCode]AttentionDetail
}
