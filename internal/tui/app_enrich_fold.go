// SPDX-License-Identifier: GPL-3.0-or-later

package tui

// app_enrich_fold.go — Wave-2 enrichment helpers that mutate cached row
// Findings/AttentionDetails directly (no parallel findings map).
//
// The Wave-2 fold itself lives on runtime.Core.applyEnrichment (internal/
// runtime/helpers.go); Model.applyEnrichment here is CLEAR-only, stripping
// wave2 findings from one type's cached rows ahead of a rerun.
// primaryWave2Finding, wave2FindingsByID, and wave2DetailsByID rebuild
// detail/list view input from the authoritative r.Findings +
// r.AttentionDetails state on each cached row — a row may carry several
// independently-evaluated wave2 findings. primaryWave2Finding reduces to the
// single WORST-severity one for its narrow on-demand detail-enrich consumer
// (see its own doc comment for why); wave2FindingsByID and wave2DetailsByID
// carry every one of them through unreduced into
// Controller.ApplyEnrichmentState's plural storage layer.

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// applyEnrichment strips every Wave-2 finding (and companion AttentionDetail)
// from resourceType's cached rows — a per-type Wave-2 clear, mirroring
// clearAllWave2 but scoped to one canonical type. Both production call sites
// (handleRefresh's pre-fetch cleanup) pass no finding data of their own: the
// actual Wave-2 fold that appends fresh findings back onto cached rows lives
// entirely on runtime.Core.applyEnrichment (core/runtime/helpers.go),
// reached via handleEnrichmentChecked → AmendRows once the fresh
// EnrichmentChecked result lands.
//
// Folds into RowStore's retained rows for canon via m.core.AmendRows'
// copy-on-write mutation (task #17 wave 1 stage 3 — the former
// ResourceCache/LazyResourceCache in-place-mutation legs are gone; a type's
// rows live in exactly one RowStore entry, so this is the only per-type-row
// destination left). See RowStore.Amend's doc comment for why in-place
// mutation is not valid here.
func (m *Model) applyEnrichment(resourceType string) {
	canon := resourceType
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
	}

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

// primaryWave2Finding extracts the WORST-severity wave2 Finding (and its
// companion AttentionDetail, if present) from r.Findings / r.AttentionDetails
// for wiring into detail views via Controller.ApplyDetailEnrichmentForResource,
// whose signature carries exactly one Finding + one AttentionDetail — a
// legitimate render-boundary derivation, not a compat shim: no production
// DetailEnricher (core/aws's enrichPolicy/enrichRolePolicy) emits more
// than one wave2 Finding on its returned resource today, and the multi-
// finding detail contract (every independently-evaluated condition reaching
// an already-open detail) is carried by the PatchDetail plural apply
// (handleEnrichmentChecked → ListEnrichmentPatch/PatchDetail), not this
// narrow on-demand lane. Both return values are nil when no wave2 finding is
// present (detail view shows no Attention section).
func primaryWave2Finding(r resource.Resource) (*domain.Finding, *domain.AttentionDetail) {
	var wave2 []domain.Finding
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			wave2 = append(wave2, f)
		}
	}
	if len(wave2) == 0 {
		return nil, nil
	}
	finding := domain.WorstSeverityFinding(wave2)
	var ad *domain.AttentionDetail
	if r.AttentionDetails != nil {
		if got, ok := r.AttentionDetails[finding.Code]; ok && len(got.Rows) > 0 {
			adVal := got
			ad = &adVal
		}
	}
	return &finding, ad
}

// wave2FindingsByID rebuilds a per-resource, slice-valued domain.Finding map
// from wave2 entries in the supplied resource slice (emitting the
// map[string][]domain.Finding shape the runtime→adapter PatchResourceList
// contract and Controller.ApplyEnrichmentState's plural storage layer both
// use). Used by cache-hit navigation sites that populate
// views.ResourceListModel.SetEnrichmentState from the authoritative
// r.Findings on each cached row.
//
// A row may carry more than one independently-evaluated wave2 Finding; every
// one of them is carried through unreduced — the plural store is the
// canonical, single source of truth for a resource's full finding set, and
// nothing downstream of it (row glyph, detail Attention panel, menu badge)
// should be starved of a condition another consumer needs. Returns nil when
// no wave2 findings are present.
func wave2FindingsByID(rows []resource.Resource) map[string][]domain.Finding {
	var out map[string][]domain.Finding
	for _, r := range rows {
		var wave2 []domain.Finding
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Source, "wave2:") {
				wave2 = append(wave2, f)
			}
		}
		if len(wave2) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string][]domain.Finding)
		}
		out[r.ID] = wave2
	}
	return out
}

// wave2DetailsByID mirrors wave2FindingsByID: it rebuilds a per-resource,
// per-FindingCode domain.AttentionDetail map from every row's wave2-sourced
// findings, keyed first by Resource.ID and then by Finding.Code — the same
// nested shape setWave2Finding (core/aws/issue_enrichment.go) and
// runtime.ApplyWave2ToRow (core/runtime/helpers.go) produce, and the
// shape the runtime→adapter PatchResourceList contract
// (ListEnrichmentPatch.AttentionDetails) and Controller.ApplyEnrichmentState's
// plural storage layer both require. A row with more than one
// independently-evaluated wave2 Finding contributes one entry per Code, not
// just its worst one. Only entries with at least one DetailRow are included.
// Returns nil when no row has a non-empty companion AttentionDetail.
func wave2DetailsByID(rows []resource.Resource) map[string]map[domain.FindingCode]domain.AttentionDetail {
	var out map[string]map[domain.FindingCode]domain.AttentionDetail
	for _, r := range rows {
		var wave2 []domain.Finding
		for _, f := range r.Findings {
			if strings.HasPrefix(f.Source, "wave2:") {
				wave2 = append(wave2, f)
			}
		}
		if len(wave2) == 0 {
			continue
		}
		for _, f := range wave2 {
			ad, ok := r.AttentionDetails[f.Code]
			if !ok || len(ad.Rows) == 0 {
				continue
			}
			if out == nil {
				out = make(map[string]map[domain.FindingCode]domain.AttentionDetail)
			}
			if out[r.ID] == nil {
				out[r.ID] = make(map[domain.FindingCode]domain.AttentionDetail, 1)
			}
			out[r.ID][f.Code] = ad
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
// wave2FindingsByID.
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
