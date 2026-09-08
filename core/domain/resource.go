// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package domain is the leaf type-declaration package for a9s.
// It carries no internal imports and no presentation library imports.
// All types declared here are stable contracts shared across layers.
package domain

import "maps"

// Resource represents a single AWS resource instance.
// core/resource re-exports this via a type alias.
//
// Canonical model — see `docs/historical/refactor/03-finding-model.md`.
type Resource struct {
	// ID is the primary identifier (instance ID, ARN, name).
	ID string
	// Name is the display name (from Name tag or identifier).
	Name string
	// Type is the resource short name (e.g. "ec2", "rds", "s3"). Set by
	// fetchers and by the detail view before calling a DetailProjector.
	// Used by projection.GenericWithConfig to look up per-type view config,
	// navigable fields, and field aliases. Empty string = unknown type
	// (falls back to Fields-only rendering with no navigability).
	Type string
	// Fields holds all visible column values by key.
	Fields map[string]string
	// RawStruct holds the original AWS SDK typed struct for reflection-based
	// field extraction.
	RawStruct any
	// Findings is the canonical finding list. Fetchers and enrichers populate
	// it; views read Findings[0].Phrase / Findings[0].Severity for list
	// rendering.
	// Empty for healthy rows.
	Findings []Finding
	// AttentionDetails carries supporting structured facts per finding,
	// keyed by Finding.Code. Consumed only by the detail view's Attention
	// section. Nil/empty for rows with no findings or no extra facts.
	AttentionDetails map[FindingCode]AttentionDetail
}

// Clone returns a deep copy of r's reference-typed fields — Fields,
// Findings, and AttentionDetails (including each AttentionDetail's own Rows
// slice) — so a caller that retains the clone across an ownership boundary
// (e.g. core/session.RowStore, which hands rows across a two-mutex boundary
// to a controller's own per-screen state) can never leak an in-place
// mutation (r.Fields[k] = ..., r.Findings = append(...),
// r.AttentionDetails[code] = ...) back into whatever r was cloned from, or
// vice versa. This is the single place that decides how deep a Resource
// copy goes — a field added to Resource above is a decision made here, in
// the same file, rather than in a copier several packages away that has no
// reason to know the field exists.
//
// RawStruct is deliberately left aliased: it is the original AWS SDK typed
// struct, assigned exactly once by a fetcher/enricher constructing a fresh
// Resource value (every known write site reassigns the field on a
// freshly-built value — core/aws/detail_enrich_engine.go's enrichDetail,
// e.g. — never mutates a field of the pointed-to SDK struct on a Resource
// that has already been handed across a boundary), and every consumer of it
// (core/fieldpath reflection, json.Marshal, related-checker type assertions)
// only reads through it. Aliasing a read-only allocation is safe; cloning it
// would just be an unreachable-benefit allocation on every row.
func (r Resource) Clone() Resource {
	if r.Fields != nil {
		fields := make(map[string]string, len(r.Fields))
		maps.Copy(fields, r.Fields)
		r.Fields = fields
	}
	if r.Findings != nil {
		findings := make([]Finding, len(r.Findings))
		copy(findings, r.Findings)
		r.Findings = findings
	}
	if r.AttentionDetails != nil {
		details := make(map[FindingCode]AttentionDetail, len(r.AttentionDetails))
		for code, ad := range r.AttentionDetails {
			if ad.Rows != nil {
				rows := make([]DetailRow, len(ad.Rows))
				copy(rows, ad.Rows)
				ad.Rows = rows
			}
			details[code] = ad
		}
		r.AttentionDetails = details
	}
	return r
}
