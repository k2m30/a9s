package tui

// app_enrich_fold.go — Wave-2 enrichment helpers that mutate cached row
// Findings/AttentionDetails directly (no parallel findings map).
//
// applyEnrichment is the canonical write path for Wave 2 results.
// findingFromResource and findingsFromRows rebuild detail/list view input from
// the authoritative r.Findings + r.AttentionDetails state on each cached row.

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// applyEnrichment merges Wave 2 enrichment findings into every cached row of
// the given resource type using applyWave2ToRow (mirrors runtime/helpers.go) to:
//
//  1. Strip any existing Wave-2 entries from r.Findings (fetchers write
//     Wave-1 Findings directly post-W1.1).
//  2. Append the matching domain.Finding from findings[r.ID].
//  3. Write attentionDetails[r.ID] into r.AttentionDetails under the
//     Finding's Code (the fold-layer Resource.ID → FindingCode re-key).
//
// Walks ResourceCache and LazyResourceCache in place (both legs stay
// map-mutation, Stage 3 scope). The RowStore leg (replacing the former
// ProbeResources walk) instead goes through m.core.AmendRows' copy-on-write
// fold (task #17 wave 1 stage 2) — see RowStore.Amend's doc comment for why
// in-place mutation is no longer valid for this leg. Cached rows hold their
// own Findings/AttentionDetails directly; views read from r.Findings.
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
	m.core.AmendRows(canon, func(rows []resource.Resource) []resource.Resource {
		if len(rows) == 0 {
			return rows
		}
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		for i := range out {
			applyWave2ToRow(&out[i], td, findings, attentionDetails)
		}
		return out
	})
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
	// Strip any existing wave2 entries; fetchers write wave1 Findings directly (W1.1+).
	r.Findings = stripWave2(r.Findings)
	r.AttentionDetails = nil
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

// findingsFromRows rebuilds a per-resource domain.Finding map from wave2 entries
// in the supplied resource slice (emitting domain.Finding to match the
// runtime→adapter PatchResourceList contract). Used by cache-hit navigation sites
// that populate views.ResourceListModel.findingsByID (row marker glyphs) from the
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
	// RowStore leg (task #17 wave 1 stage 2 — replaces the former
	// ForEachProbeResources in-place walk): ForEachProbeResources now hands
	// out a defensive copy (RowStore.SnapshotAll), so mutating the callback's
	// rows slice in place would be a silent no-op. AmendRows' copy-on-write
	// fold is the store's only valid mutation path.
	for _, canon := range m.core.ProbeOriginTypeNames() {
		m.core.AmendRows(canon, func(rows []resource.Resource) []resource.Resource {
			if len(rows) == 0 {
				return rows
			}
			out := make([]resource.Resource, len(rows))
			copy(out, rows)
			for i := range out {
				out[i].Findings = stripWave2(out[i].Findings)
				out[i].AttentionDetails = nil
			}
			return out
		})
	}
}

