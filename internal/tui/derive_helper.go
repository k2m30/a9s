package tui

// derive_helper.go — shared DeriveFindings call helpers for all 7 PR-03a-shim
// entry points (docs/refactor/03-finding-model.md lines 141-147).
//
// The grep exit criterion (rg 'attention\.DeriveFindings\b' internal/tui/)
// resolves to exactly two call sites in this file — one for slice paths
// (deriveFindingsForType) and one for single-resource paths
// (deriveFindingsForResource). Every entry-point handler calls one of these
// two helpers, so every resource that reaches view state has Findings populated.

import (
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
)

// deriveFindingsForType derives findings across rows in-place for the given
// resource type. Pulls the type def via the registry and the per-type enrichment
// map from the model. Safe to call on empty or nil inputs.
//
// The short parameter may be an alias (e.g. "rds") or the canonical ShortName
// (e.g. "dbi"). FindResourceType resolves aliases to their canonical type def,
// and the enrichment lookup always uses the canonical ShortName so Wave-2
// findings stored under the canonical key are visible regardless of which alias
// the caller passes.
//
// Used by Sites 1–5 (slice-shaped entry points):
//   - Site 1: ResourcesLoadedMsg handler in app.go
//   - Site 2: AvailabilityCheckedMsg → ProbeResources in app_handlers_availability.go
//   - Site 3: EnrichmentCheckedMsg Wave-2 bridge in app_handlers_availability.go
//   - Site 4: RelatedCheckResultMsg CachedPages in app.go
//   - Site 5: RelatedCheckResultMsg LazyAddedResources in app.go
func (m *Model) deriveFindingsForType(short string, rows []resource.Resource) {
	if len(rows) == 0 {
		return
	}
	var (
		td    resource.ResourceTypeDef
		canon = short
	)
	if t := resource.FindResourceType(short); t != nil {
		td = *t
		canon = t.ShortName
	} else {
		td = resource.ResourceTypeDef{ShortName: short}
	}
	enrich := m.EnrichmentFindings[canon]
	for i := range rows {
		attention.DeriveFindings(&rows[i], td, enrich)
	}
}

// deriveFindingsForResource derives findings on a single resource in-place.
// Pulls the type def and enrichment map from the model.
//
// The short parameter may be an alias or the canonical ShortName; the enrichment
// lookup always uses the resolved canonical ShortName (same alias-safe pattern as
// deriveFindingsForType).
//
// Used by Sites 6–7 (single-resource entry points):
//   - Site 6: child-view fetcher path in app_handlers_navigate.go
//   - Site 7: EnrichDetailResultMsg in app.go (enriched detail resource)
func (m *Model) deriveFindingsForResource(short string, r *resource.Resource) {
	if r == nil {
		return
	}
	var (
		td    resource.ResourceTypeDef
		canon = short
	)
	if t := resource.FindResourceType(short); t != nil {
		td = *t
		canon = t.ShortName
	} else {
		td = resource.ResourceTypeDef{ShortName: short}
	}
	attention.DeriveFindings(r, td, m.EnrichmentFindings[canon])
}
