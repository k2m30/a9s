// SPDX-License-Identifier: GPL-3.0-or-later

package tui

// app_enrich_fold.go — Wave-2 enrichment helpers that mutate cached row
// Findings/AttentionDetails directly (no parallel findings map).
//
// The Wave-2 fold itself lives on runtime.Core.applyEnrichment (internal/
// runtime/helpers.go); Model.applyEnrichment here is CLEAR-only, stripping
// wave2 findings from one type's cached rows ahead of a rerun.
// wave2FindingsByID and wave2DetailsByID rebuild list-view input from the
// authoritative r.Findings + r.AttentionDetails state on each cached row —
// a row may carry several independently-evaluated wave2 findings, and both
// helpers carry every one of them through unreduced into
// Controller.ApplyEnrichmentState's plural storage layer.
//
// The web/headless lane's on-demand detail-enrichment fold lives in core/app
// (Controller.foldEnrichDetailResultLocked, reached via Controller.Handle)
// and calls its own copy of primaryWave2Finding (core/app/detail_state.go) —
// core/app cannot import internal/tui, so the two copies cannot share code.
// The TUI's own runtime_adapter_resources.go's handleEnrichDetailResult calls
// Core.HandleEnrichDetailResult directly (not Controller.Handle) so a flash
// on enrichment error applies synchronously within one Update() call, then
// calls primaryWave2Finding below before folding into detail state via
// ctrl.ApplyDetailEnrichmentForResource.

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// applyEnrichment strips every Wave-2 finding (and companion AttentionDetail)
// from resourceType's cached rows — a per-type Wave-2 clear, mirroring
// core/runtime's Core.ClearAllWave2Findings but scoped to one canonical type.
// Its one production call site (handleRefresh's pre-fetch cleanup) passes no
// finding data of its own: the actual Wave-2 fold that appends fresh
// findings back onto cached rows lives entirely on runtime.Core.applyEnrichment
// (core/runtime/helpers.go), reached via handleEnrichmentChecked → AmendRows
// once the fresh EnrichmentChecked result lands.
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
			runtime.ApplyWave2ToRow(&out[i], resource.ResourceTypeDef{}, nil, nil)
		}
		return out
	})
}

// primaryWave2Finding extracts the WORST-severity wave2 Finding (and its
// companion AttentionDetail, if present) from r.Findings / r.AttentionDetails.
// Both return values are nil when no wave2 finding is present (detail view
// shows no Attention section).
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

// ClearAllWave2Findings (core/runtime) is the neutral, all-types counterpart
// of applyEnrichment above — used by Controller.RestartAvailabilitySweep
// (core/app/actions_list.go), the single source shared by the TUI's
// main-menu Ctrl+R path (runtime_adapter_navigate.go) and the headless/web
// menu-refresh branch.
