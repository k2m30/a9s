package tui

// app_enrich_fold.go — PR-03a-fold helpers that replace the parallel
// m.EnrichmentFindings map approach with direct mutation of cached row
// Findings/AttentionDetails.
//
// applyEnrichment is the canonical write path for Wave 2 results.
// findingFromResource and findingsFromRows rebuild detail/list view input
// from the authoritative r.Findings + r.AttentionDetails state on each cached
// row, replacing the side maps that PR-03a-fold removed.

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/attention"
)

// applyEnrichment merges Wave 2 enrichment findings into every cached row of
// the given resource type. AS-1395 changed the contract: instead of calling
// the pre-AS-1395 DeriveFindings(r, td, enrichmentFindings) shim, this helper
// uses applyWave2ToRow (mirrors runtime/helpers.go) to:
//
//  1. Re-derive Wave-1 findings via attention.DeriveFindings(r, td).
//  2. Append the matching domain.Finding from findings[r.ID].
//  3. Write attentionDetails[r.ID] into r.AttentionDetails under the
//     Finding's Code (the fold-layer Resource.ID → FindingCode re-key).
//
// Walks ResourceCache, LazyResourceCache, and ProbeResources. Cached rows now
// hold their own Findings/AttentionDetails directly; views read from r.Findings.
func (m *Model) applyEnrichment(
	resourceType string,
	findings map[string]domain.Finding,
	attentionDetails map[string]domain.AttentionDetail,
) {
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
			applyWave2ToRow(&rows[i], td, findings, attentionDetails)
		}
	}

	if entry, ok := m.core.ResourceCache(canon); ok && entry != nil {
		apply(entry.Resources)
	}
	if rows, ok := m.core.LazyResourceCache(canon); ok {
		apply(rows)
	}
	if rows, ok := m.core.ProbeResources(canon); ok {
		apply(rows)
	}
}

// applyWave2ToRow mirrors internal/runtime/helpers.go applyWave2ToRow.  Kept as
// a sibling in this file so tui.Model.applyEnrichment does not need to import
// internal/runtime (which would create a cycle: runtime depends on internal/aws
// which depends on internal/resource, and tui depends on runtime).
func applyWave2ToRow(
	r *domain.Resource,
	td resource.ResourceTypeDef,
	findings map[string]domain.Finding,
	attentionDetails map[string]domain.AttentionDetail,
) {
	if r == nil {
		return
	}
	attention.DeriveFindings(r, td)
	f, ok := findings[r.ID]
	if !ok || f.Phrase == "" {
		return
	}
	if f.Source == "" || !strings.HasPrefix(f.Source, "wave2:") {
		f.Source = "wave2:" + td.ShortName
	}
	r.Findings = append(r.Findings, f)
	if ad, ok := attentionDetails[r.ID]; ok && len(ad.Rows) > 0 {
		if r.AttentionDetails == nil {
			r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
		}
		r.AttentionDetails[f.Code] = ad
	}
}

// findingFromResource extracts the first wave2 Finding (and its companion
// AttentionDetail, if present) from r.Findings / r.AttentionDetails for
// wiring into detail views. Both return values are nil when no wave2 finding
// is present (detail view shows no Attention section).
func findingFromResource(r resource.Resource) (*domain.Finding, *domain.AttentionDetail) {
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			finding := f
			var ad *domain.AttentionDetail
			if r.AttentionDetails != nil {
				if got, ok := r.AttentionDetails[f.Code]; ok && len(got.Rows) > 0 {
					adVal := got
					ad = &adVal
				}
			}
			return &finding, ad
		}
	}
	return nil, nil
}

// findingsFromRows rebuilds a per-resource domain.Finding map from wave2
// entries in the supplied resource slice. AS-1395 retyped this to emit
// domain.Finding (matching the runtime→adapter PatchResourceList contract).
// Used by cache-hit navigation sites that populate
// views.ResourceListModel.findingsByID (row marker glyphs) from the
// authoritative r.Findings on each cached row.
//
// Only the first wave2 finding per resource is mapped (at most one wave2
// entry per resource per type is guaranteed by applyEnrichment).
// Returns nil when no wave2 findings are present.
func findingsFromRows(rows []resource.Resource) map[string]domain.Finding {
	var out map[string]domain.Finding
	for _, r := range rows {
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Source, "wave2:") {
				if out == nil {
					out = make(map[string]domain.Finding)
				}
				out[r.ID] = f
				break // at most one wave2 finding per resource
			}
		}
	}
	return out
}


// stripWave2 returns a copy of findings with wave2 entries removed.
// Wave 1 entries (Source = "wave1") are preserved in order.
// Returns the original slice unchanged when it contains no wave2 entries
// (avoids allocation on the common happy-path: no stale wave2).
func stripWave2(findings []domain.Finding) []domain.Finding {
	if len(findings) == 0 {
		return findings
	}
	out := findings[:0:0] // empty slice, no capacity reuse to avoid alias pollution
	for _, f := range findings {
		if !strings.HasPrefix(f.Source, "wave2:") {
			out = append(out, f)
		}
	}
	return out
}

// clearAllWave2 strips wave2 findings from every row in every session-scoped
// cache. Used by main-menu Ctrl+R to ensure the next list-open doesn't
// rehydrate stale wave2 attention state via findingsFromRows.
func clearAllWave2(m *Model) {
	m.core.ForEachResourceCache(func(_ string, entry *domain.ListViewCacheEntry) {
		for i := range entry.Resources {
			entry.Resources[i].Findings = stripWave2(entry.Resources[i].Findings)
			entry.Resources[i].AttentionDetails = nil
		}
	})
	m.core.ForEachLazyResourceCache(func(_ string, rows []resource.Resource) {
		for i := range rows {
			rows[i].Findings = stripWave2(rows[i].Findings)
			rows[i].AttentionDetails = nil
		}
	})
	m.core.ForEachProbeResources(func(_ string, rows []resource.Resource) {
		for i := range rows {
			rows[i].Findings = stripWave2(rows[i].Findings)
			rows[i].AttentionDetails = nil
		}
	})
}

