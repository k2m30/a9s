// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

// helpers.go — session-state helpers on Core used by the per-handler PRs.
//
// These methods operate only on c.session fields and platform-agnostic packages
// (resource, aws/wave2.AllWave2).  They are the runtime equivalents of the
// same-named methods that still exist on tui.Model for non-migrated callers;
// both sets operate on the same *session.Session so mutations are visible to
// both.

import (
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// applyEnrichment merges Wave-2 enrichment findings into every cached row of
// the given resource type. For each cached row it:
//
//  1. Strips any existing Wave-2 entries from r.Findings (fetchers write
//     Wave-1 Findings directly post-W1.1; nothing else needs re-derivation).
//  2. Appends every Wave-2 Finding from findings[r.ID] (when present) —
//     an enricher may emit more than one independently-evaluated condition
//     per resource.
//  3. Writes attentionDetails[r.ID] into r.AttentionDetails under the
//     matching FindingCode (the fold-layer re-keying from Resource.ID to
//     FindingCode).
//
// Folds Wave-2 findings into RowStore's retained rows for canon via
// AmendRows' copy-on-write mutation (task #17 wave 1 stage 3 — the former
// ResourceCache/LazyResourceCache in-place-mutation legs are gone; a type's
// rows live in exactly one RowStore entry, so this is the only per-type-row
// destination left). The mutate-in-place bug class the dispatch-time payload
// freeze guards against is exactly what Amend exists to remove — see
// RowStore.Amend's doc comment.
// answered reports whether the enrichment result speaks for a given row ID.
// A row it does not answer for — one the enricher could not inspect, or any
// row at all when the probe failed outright — keeps the Wave-2 state it
// already has: a result replaces exactly what it answered, and an
// unanswered row is not a clean row (C1: stale-until-replaced, never
// blank-until-replaced). A nil answered folds every row, the plain
// full-result case.
func (c *Core) applyEnrichment(
	resourceType string,
	findings map[string][]domain.Finding,
	attentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail,
	answered func(id string) bool,
) {
	canon := resourceType
	var td resource.ResourceTypeDef
	if t := resource.FindResourceType(resourceType); t != nil {
		canon = t.ShortName
		td = *t
	} else {
		td = resource.ResourceTypeDef{ShortName: canon}
	}

	c.AmendRows(canon, func(rows []resource.Resource) []resource.Resource {
		if len(rows) == 0 {
			return rows
		}
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		for i := range out {
			if answered != nil && !answered(out[i].ID) {
				continue
			}
			ApplyWave2ToRow(&out[i], td, findings, attentionDetails)
		}
		return out
	})
}

// ApplyWave2ToRow strips any existing Wave-2 entries from r.Findings, then
// appends every per-row Wave-2 Finding for r.ID (an enricher may emit more
// than one independently-evaluated condition per resource; dedupe by Code
// guards against an enricher accidentally emitting the same Code twice) and
// writes each Finding's OWN AttentionDetail, looked up by (r.ID, Finding.Code)
// in attentionDetails. Nil findings/attentionDetails behaves as the clear
// path (strip only, no Wave-2 appended).
func ApplyWave2ToRow(
	r *domain.Resource,
	td resource.ResourceTypeDef,
	findings map[string][]domain.Finding,
	attentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail,
) {
	if r == nil {
		return
	}
	// Strip any existing wave2 entries; fetchers write wave1 Findings directly
	// (W1.1+). Builds a NEW backing array rather than compacting r.Findings in
	// place (r.Findings[n] = f) — applyEnrichment's row copy is shallow, so an
	// in-place compaction here would mutate the backing array still shared
	// with prior snapshots/retained rows.
	out := make([]domain.Finding, 0, len(r.Findings))
	stale := make(map[domain.FindingCode]bool, len(r.Findings))
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			stale[f.Code] = true
			continue
		}
		out = append(out, f)
	}
	r.Findings = out
	// The fold owns exactly the wave-2 slice: the outgoing findings' codes and
	// nothing else. Wave-1 supporting rows come from data only the fetcher
	// holds and nothing re-derives them, so dropping the whole map would lose
	// them for the rest of the row's life. Rebuilt into a NEW map for the same
	// reason the findings slice is: applyEnrichment's row copy is shallow.
	r.AttentionDetails = keepingCodes(r.AttentionDetails, stale)

	fs := findings[r.ID]
	if len(fs) == 0 {
		return
	}

	adByCode := attentionDetails[r.ID]
	seen := make(map[domain.FindingCode]bool, len(fs))
	for _, f := range fs {
		if f.Phrase == "" || seen[f.Code] {
			continue
		}
		seen[f.Code] = true
		// Enricher-emitted Findings already carry the canonical Code and
		// Source. Source must be "wave2:<short>" for the existing
		// app_enrich_fold readers (primaryWave2Finding, wave2FindingsByID,
		// stripWave2) to recognise the entry. Tolerate enrichers that forgot
		// to set Source by stamping the canonical form here.
		if f.Source == "" || !strings.HasPrefix(f.Source, "wave2:") {
			f.Source = "wave2:" + td.ShortName
		}
		r.Findings = append(r.Findings, f)
		// Each Finding gets its OWN AttentionDetail by its Code — a resource
		// with more than one independently-evaluated condition keeps every
		// condition's own supporting rows, none crowded out by another.
		if ad, ok := adByCode[f.Code]; ok && len(ad.Rows) > 0 {
			if r.AttentionDetails == nil {
				r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
			}
			r.AttentionDetails[f.Code] = ad
		}
	}
}

// keepingCodes copies ad without the entries named in drop, returning nil when
// nothing survives so a row with no supporting rows keeps its zero value.
func keepingCodes(
	ad map[domain.FindingCode]domain.AttentionDetail,
	drop map[domain.FindingCode]bool,
) map[domain.FindingCode]domain.AttentionDetail {
	var kept map[domain.FindingCode]domain.AttentionDetail
	for code, detail := range ad {
		if drop[code] {
			continue
		}
		if kept == nil {
			kept = make(map[domain.FindingCode]domain.AttentionDetail, len(ad))
		}
		kept[code] = detail
	}
	return kept
}
