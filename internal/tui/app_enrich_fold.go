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
// Folds Wave-2 findings into RowStore's retained rows for canon via
// m.core.AmendRows' copy-on-write mutation (task #17 wave 1 stage 3 — the
// former ResourceCache/LazyResourceCache in-place-mutation legs are gone; a
// type's rows live in exactly one RowStore entry, so this is the only
// per-type-row destination left). See RowStore.Amend's doc comment for why
// in-place mutation is not valid here. Cached rows hold their own
// Findings/AttentionDetails directly; views read from r.Findings.
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

// clearAllWave2 strips wave2 findings from every row RowStore retains for
// every type this session has touched. Used by main-menu Ctrl+R to ensure
// the next list-open doesn't rehydrate stale wave2 attention state via
// findingsFromRows.
//
// Every retained type's rows live in exactly one RowStore entry (task #17
// wave 1 stage 3 — the former ResourceCache/LazyResourceCache/ProbeResources
// three-leg walk collapses to one), so the type-name set is the union of the
// full (ResourceCacheKeys) and Partial (lazy, via ForEachLazyResourceCache)
// entries, and ProbeOriginTypeNames' Origin=Probe/Disk entries — every one
// of those is also reachable through the full/Partial split, so gathering
// via all three sources and deduping is defensive against any entry this
// enumeration might otherwise miss. AmendRows' copy-on-write fold is the
// store's only valid mutation path — ForEach{Resource,LazyResource}Cache now
// hand out a defensive copy (RowStore.Snapshot*), so mutating the callback's
// rows/entry slice in place would be a silent no-op.
func clearAllWave2(m *Model) {
	canons := make(map[string]struct{})
	for _, c := range m.core.ResourceCacheKeys() {
		canons[c] = struct{}{}
	}
	m.core.ForEachLazyResourceCache(func(rt string, _ []resource.Resource) {
		canons[rt] = struct{}{}
	})
	for _, c := range m.core.ProbeOriginTypeNames() {
		canons[c] = struct{}{}
	}
	for canon := range canons {
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

