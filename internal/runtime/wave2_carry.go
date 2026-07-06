// wave2_carry.go — C6b Wave-2 carry: when a rows-carrying write lacks the
// Wave-2 data that the rows it replaces already have, this file's helpers
// carry that data forward per row ID so a bare Wave-1 refresh (a sweep
// completion save, or the in-memory ProbeResources write a fresh Wave-1
// probe result performs) cannot strip Findings/Fields a prior enrichment
// pass wrote.
//
// See docs/design/cache-requirements.md C6b and defect D17.
package runtime

import (
	"maps"
	"slices"

	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// carryWave2ForRows returns newRows with, for every row that itself carries
// no Wave-2 Finding, the matching oldRows entry's (by ID) Wave-2-sourced
// Findings and shortName's registered enricher Fields keys merged in. A row
// present in newRows but absent from oldRows passes through unchanged (no
// prior observation to carry from). Wave-1 findings never carry — their
// absence in a fresh fetch means resolved (C6b) — so only entries where
// domain.Finding.IsWave2Sourced is true are ever copied forward.
//
// A row that ALREADY carries its own Wave-2 Finding is left untouched: the
// fresh observation is at least as current as anything carried would be, and
// only a row with NO Wave-2 Finding of its own is missing data to backfill.
//
// Used at the disk reconcile chokepoint (reconcileTypeFile's rules 3/4).
func carryWave2ForRows(oldRows, newRows []cache.Row, fieldKeys []string) []cache.Row {
	if len(oldRows) == 0 || len(newRows) == 0 {
		return newRows
	}
	oldByID := make(map[string]cache.Row, len(oldRows))
	for _, r := range oldRows {
		oldByID[r.ID] = r
	}

	out := make([]cache.Row, len(newRows))
	for i, row := range newRows {
		old, ok := oldByID[row.ID]
		if !ok {
			out[i] = row
			continue
		}
		findings, fields := carryWave2(old.Findings, old.Fields, row.Findings, row.Fields, fieldKeys)
		row.Findings = findings
		row.Fields = fields
		out[i] = row
	}
	return out
}

// carryWave2ForResources is carryWave2ForRows' counterpart for the in-memory
// session.ProbeResources write (handleAvailabilityChecked): a fresh Wave-1
// probe result (newResources) that lacks Wave-2 data replaces the previous
// in-memory rows (oldResources) for the same type, so this backfills
// Wave-2-sourced Findings and shortName's registered enricher Fields keys
// per matching resource ID before the assignment, the same way
// carryWave2ForRows protects the on-disk write.
func carryWave2ForResources(oldResources, newResources []resource.Resource, fieldKeys []string) []resource.Resource {
	if len(oldResources) == 0 || len(newResources) == 0 {
		return newResources
	}
	oldByID := make(map[string]resource.Resource, len(oldResources))
	for _, r := range oldResources {
		oldByID[r.ID] = r
	}

	out := make([]resource.Resource, len(newResources))
	for i, r := range newResources {
		old, ok := oldByID[r.ID]
		if !ok {
			out[i] = r
			continue
		}
		carried := !rowHasWave2Finding(r.Findings)
		findings, fields := carryWave2(old.Findings, old.Fields, r.Findings, r.Fields, fieldKeys)
		r.Findings = findings
		r.Fields = fields
		// The carried Wave-2 Findings above are meaningless in the detail view's
		// Attention section without their companion AttentionDetail rows (same
		// D17/C6b class as the Findings themselves) — carry the matching
		// FindingCode entries too, into a fresh map so neither side's backing
		// map can be mutated through the other (carryWave2's own no-alias
		// discipline, extended to AttentionDetails).
		if carried && len(old.AttentionDetails) > 0 {
			for _, f := range wave2FindingsOf(old.Findings) {
				ad, ok := old.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				if r.AttentionDetails == nil {
					r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, len(old.AttentionDetails))
				}
				r.AttentionDetails[f.Code] = ad
			}
		}
		out[i] = r
	}
	return out
}

// carryWave2 is the row-pair core shared by carryWave2ForRows and
// carryWave2ForResources: when newFindings has no Wave-2 Finding of its own,
// it returns newFindings with oldFindings' Wave-2-sourced entries appended,
// and newFields augmented with any of fieldKeys present in oldFields but
// missing (or empty) in newFields. Returns newFindings/newFields UNCHANGED
// (never copied) when newFindings already carries its own Wave-2 Finding —
// the caller relies on this to avoid needless allocation on the common
// already-fresh path.
//
// Every returned slice/map that IS mutated is a fresh copy — never an alias
// into oldFindings'/oldFields' (or newFindings'/newFields' own) backing
// array/map — so a later in-place mutation on either side (e.g.
// snapshotProbeResourcesForSave's element-by-element copy discipline, or a
// subsequent FieldUpdates maps.Copy) cannot corrupt the other.
func carryWave2(oldFindings []domain.Finding, oldFields map[string]string, newFindings []domain.Finding, newFields map[string]string, fieldKeys []string) ([]domain.Finding, map[string]string) {
	if rowHasWave2Finding(newFindings) {
		return newFindings, newFields
	}

	oldWave2 := wave2FindingsOf(oldFindings)
	if len(oldWave2) > 0 {
		merged := make([]domain.Finding, 0, len(newFindings)+len(oldWave2))
		merged = append(merged, newFindings...)
		merged = append(merged, oldWave2...)
		newFindings = merged
	}

	if len(fieldKeys) > 0 && len(oldFields) > 0 {
		var fields map[string]string
		for _, key := range fieldKeys {
			v, ok := oldFields[key]
			if !ok || v == "" {
				continue
			}
			if existing, already := newFields[key]; already && existing != "" {
				continue
			}
			if fields == nil {
				fields = make(map[string]string, len(newFields)+len(fieldKeys))
				maps.Copy(fields, newFields)
			}
			fields[key] = v
		}
		if fields != nil {
			newFields = fields
		}
	}

	return newFindings, newFields
}

// rowHasWave2Finding reports whether findings already contains a Wave-2
// finding — a row this fresh means "carries its own current Wave-2 data",
// not something the row-carry step needs to backfill.
func rowHasWave2Finding(findings []domain.Finding) bool {
	for _, f := range findings {
		if f.IsWave2Sourced() {
			return true
		}
	}
	return false
}

// wave2FindingsOf returns a fresh copy of every Wave-2-sourced entry in
// findings, preserving order. Never returns an alias into findings' backing
// array.
func wave2FindingsOf(findings []domain.Finding) []domain.Finding {
	if len(findings) == 0 {
		return nil
	}
	var out []domain.Finding
	for _, f := range findings {
		if f.IsWave2Sourced() {
			out = append(out, f)
		}
	}
	return out
}

// baselineWave2FieldKey is always eligible to carry alongside a carried
// Wave-2 Finding, independent of catalog registration: it is the universal
// enrichment-derived cell every list/detail render classifies a row's
// status from (see materializeResourceFields' explicit "status" exclusion —
// this key is always Wave-2/render-derived, never Wave-1-materialized), so a
// row whose type is not (yet) registered with an IssueEnricherFieldKeys
// entry — or whose catalog entry simply omits "status" — must still get its
// carried Finding's accompanying status text, or the carried glyph would
// render without the status text that explains it.
const baselineWave2FieldKey = "status"

// issueEnricherFieldKeysFor resolves shortName's registered Wave-2
// IssueEnricherFieldKeys via the catalog, always including
// baselineWave2FieldKey ("status") even when the type has no catalog
// registration or its own IssueEnricherFieldKeys entry omits it.
func issueEnricherFieldKeysFor(shortName string) []string {
	var catalogKeys []string
	if td := resource.FindResourceType(shortName); td != nil {
		catalogKeys = td.IssueEnricherFieldKeys
	}
	if slices.Contains(catalogKeys, baselineWave2FieldKey) {
		return catalogKeys
	}
	out := make([]string, 0, len(catalogKeys)+1)
	out = append(out, baselineWave2FieldKey)
	out = append(out, catalogKeys...)
	return out
}
