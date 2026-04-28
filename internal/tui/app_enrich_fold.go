package tui

// app_enrich_fold.go — PR-03a-fold helpers that replace the parallel
// m.EnrichmentFindings map approach with direct mutation of cached row
// Findings/AttentionDetails.
//
// applyEnrichment is the canonical write path for Wave 2 results: it calls
// DeriveFindings with the full findings map on every cached row of the given
// resource type, so r.Findings and r.AttentionDetails are authoritative and
// views need not consult a side map.
//
// clearEnrichmentFor re-derives wave1 only on cached rows of the given type,
// effectively stripping any wave2 entries. Used by Ctrl+R and by the
// clear-on-rerun-start logic in handleEnrichmentChecked.
//
// findingsFromRows rebuilds a per-resource EnrichmentFinding map from wave2
// entries in the given resource slice. Used by cache-hit navigation paths to
// populate views.ResourceListModel.findingsByID for row marker glyphs without
// the now-deleted m.EnrichmentFindings parallel map.

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
)

// applyEnrichment merges Wave 2 enrichment findings into every cached row of
// the given resource type by calling DeriveFindings with the provided findings
// map. Replaces prior wave2 findings in place while preserving wave1. Walks
// ResourceCache, LazyResourceCache, and ProbeResources for the canonical short
// name.
//
// Replaces the prior re-derive loops that followed the
// m.EnrichmentFindings[type] = msg.Findings write. Cached rows now hold their
// own Findings/AttentionDetails directly; views read from r.Findings.
func (m *Model) applyEnrichment(resourceType string, findings map[string]resource.EnrichmentFinding) {
	canon := resourceType
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	apply := func(rows []resource.Resource) {
		for i := range rows {
			attention.DeriveFindings(&rows[i], td, findings)
		}
	}

	if entry, ok := m.ResourceCache[canon]; ok {
		apply(entry.Resources)
	}
	if rows, ok := m.LazyResourceCache[canon]; ok {
		apply(rows)
	}
	if rows, ok := m.ProbeResources[canon]; ok {
		apply(rows)
	}
}

// clearEnrichmentFor strips wave2 findings from every cached row of the given
// resource type by re-deriving with nil enrichment. Wave1 findings (from
// r.Status / r.Issues) are preserved. Used by clear-on-rerun-start logic so
// stale wave2 markers disappear immediately without waiting for the rerun to
// complete.
func (m *Model) clearEnrichmentFor(resourceType string) {
	canon := resourceType
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	clear := func(rows []resource.Resource) {
		for i := range rows {
			attention.DeriveFindings(&rows[i], td, nil)
		}
	}

	if entry, ok := m.ResourceCache[canon]; ok {
		clear(entry.Resources)
	}
	if rows, ok := m.LazyResourceCache[canon]; ok {
		clear(rows)
	}
	if rows, ok := m.ProbeResources[canon]; ok {
		clear(rows)
	}
}

// findingFromResource extracts the first wave2 entry from r.Findings and
// returns it as a *resource.EnrichmentFinding for wiring into detail views.
// Returns nil when no wave2 finding is present (detail view shows no Attention
// section). This replaces the m.EnrichmentFindings[resType][r.ID] lookup that
// was deleted in PR-03a-fold; cached rows are now the authoritative source.
func findingFromResource(r resource.Resource) *resource.EnrichmentFinding {
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			ef := resource.EnrichmentFinding{
				Severity: glyphFromDomainSeverity(f.Severity),
				Summary:  f.Phrase,
			}
			// Preserve any Rows (AttentionDetail body) if the wave2 entry carried them.
			// AttentionDetails is keyed by FindingCode; look up by f.Code.
			if r.AttentionDetails != nil {
				if ad, ok := r.AttentionDetails[f.Code]; ok {
					ef.Rows = make([]resource.FindingRow, 0, len(ad.Rows))
					for _, row := range ad.Rows {
						ef.Rows = append(ef.Rows, resource.FindingRow{
							Label: row.Label,
							Value: row.Value,
							Tier:  row.Tier,
						})
					}
				}
			}
			return &ef
		}
	}
	return nil
}

// findingsFromRows rebuilds a per-resource resource.EnrichmentFinding map from
// wave2 entries in the supplied resource slice. This replaces the read of the
// now-deleted m.EnrichmentFindings[canon] at cache-hit navigation sites so that
// views.ResourceListModel.findingsByID (used for row marker glyphs) is correctly
// populated from the authoritative r.Findings on each cached row.
//
// Only the first wave2 finding per resource is mapped (at most one wave2 entry
// per resource per type is guaranteed by DeriveFindings).
// Returns nil when no wave2 findings are present (avoids allocating an empty map).
func findingsFromRows(rows []resource.Resource) map[string]resource.EnrichmentFinding {
	var out map[string]resource.EnrichmentFinding
	for _, r := range rows {
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Source, "wave2:") {
				if out == nil {
					out = make(map[string]resource.EnrichmentFinding)
				}
				out[r.ID] = resource.EnrichmentFinding{
					Severity: glyphFromDomainSeverity(f.Severity),
					Summary:  f.Phrase,
				}
				break // at most one wave2 finding per resource
			}
		}
	}
	return out
}

// glyphFromDomainSeverity converts a domain.Severity constant to the
// resource.EnrichmentFinding.Severity glyph string used by row marker rendering.
//
//	SevBroken → "!"   (failed / degraded)
//	SevWarn   → "~"   (scheduled maintenance / informational)
//	other     → ""    (no glyph rendered)
func glyphFromDomainSeverity(s domain.Severity) string {
	switch s {
	case domain.SevBroken:
		return "!"
	case domain.SevWarn:
		return "~"
	default:
		return ""
	}
}

