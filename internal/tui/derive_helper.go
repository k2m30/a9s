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

// deriveFindingsForType re-derives wave1 findings across rows in-place for the
// given resource type, preserving any existing wave2 entries populated by a
// prior applyEnrichment call. Safe to call on empty or nil inputs.
//
// The short parameter may be an alias (e.g. "rds") or the canonical ShortName
// (e.g. "dbi"). FindResourceType resolves aliases to their canonical type def.
//
// After PR-03a-fold, this helper calls DeriveWave1Only (not DeriveFindings) so
// that wave2 findings already on r.Findings are preserved across subsequent
// derive calls at non-EnrichmentChecked entry points.
//
// Used by Sites 1–5 (slice-shaped entry points):
//   - Site 1: ResourcesLoadedMsg handler in app.go
//   - Site 2: AvailabilityCheckedMsg → ProbeResources in app_handlers_availability.go
//   - Site 3: EnrichmentCheckedMsg Wave-2 bridge in app_handlers_availability.go (replaced by applyEnrichment)
//   - Site 4: RelatedCheckResultMsg CachedPages in app.go
//   - Site 5: RelatedCheckResultMsg LazyAddedResources in app.go
func (m *Model) deriveFindingsForType(short string, rows []resource.Resource) {
	if len(rows) == 0 {
		return
	}
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(short); t != nil {
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: short}
	}
	for i := range rows {
		attention.DeriveWave1Only(&rows[i], td)
	}
}

// deriveFindingsForResource re-derives wave1 findings on a single resource
// in-place, preserving any existing wave2 entries populated by a prior
// applyEnrichment call.
//
// After PR-03a-fold, this helper calls DeriveWave1Only so wave2 findings
// already on r.Findings are preserved.
//
// Used by Sites 6–7 (single-resource entry points):
//   - Site 6: child-view fetcher path in app_handlers_navigate.go
//   - Site 7: EnrichDetailResultMsg in app.go (enriched detail resource)
func (m *Model) deriveFindingsForResource(short string, r *resource.Resource) {
	if r == nil {
		return
	}
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(short); t != nil {
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: short}
	}
	attention.DeriveWave1Only(r, td)
}
