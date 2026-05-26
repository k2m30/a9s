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

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// applyEnrichment merges Wave-2 enrichment findings into every cached row of
// the given resource type. For each cached row it:
//
//  1. Strips any existing Wave-2 entries from r.Findings (fetchers write
//     Wave-1 Findings directly post-W1.1; nothing else needs re-derivation).
//  2. Appends the Wave-2 Finding from findings[r.ID] (when present).
//  3. Writes attentionDetails[r.ID] into r.AttentionDetails under the matching
//     FindingCode (the fold-layer re-keying from Resource.ID to FindingCode).
//
// Walks ResourceCache, LazyResourceCache, and ProbeResources. Replaces prior
// wave-2 findings in place while preserving wave-1.
func (c *Core) applyEnrichment(
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

	if entry, ok := c.session.ResourceCache[canon]; ok {
		apply(entry.Resources)
	}
	if rows, ok := c.session.LazyResourceCache[canon]; ok {
		apply(rows)
	}
	if rows, ok := c.session.ProbeResources[canon]; ok {
		apply(rows)
	}
}

// clearEnrichmentFor strips wave-2 findings from every cached row of the given
// resource type, preserving Wave-1 entries already on r.Findings. Used by
// clear-on-rerun-start logic in handleEnrichmentChecked.
func (c *Core) clearEnrichmentFor(resourceType string) {
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
			applyWave2ToRow(&rows[i], td, nil, nil)
		}
	}

	if entry, ok := c.session.ResourceCache[canon]; ok {
		clear(entry.Resources)
	}
	if rows, ok := c.session.LazyResourceCache[canon]; ok {
		clear(rows)
	}
	if rows, ok := c.session.ProbeResources[canon]; ok {
		clear(rows)
	}
}

func (c *Core) deriveFindingsForType(short string, rows []resource.Resource) {
	// W1.4a: fetchers write wave1 Findings directly; no derivation needed.
}

// applyWave2ToRow strips any existing Wave-2 entries from r.Findings, then
// appends the per-row Wave-2 Finding (if present) and writes its AttentionDetail
// under the Finding's Code. Nil findings/attentionDetails behaves as the clear
// path (strip only, no Wave-2 appended).
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
	n := 0
	for _, f := range r.Findings {
		if !strings.HasPrefix(f.Source, "wave2:") {
			r.Findings[n] = f
			n++
		}
	}
	r.Findings = r.Findings[:n]
	r.AttentionDetails = nil
	f, ok := findings[r.ID]
	if !ok || f.Phrase == "" {
		return
	}
	// AS-1395: enricher-emitted Findings already carry the canonical Code and
	// Source. Source must be "wave2:<short>" for the existing app_enrich_fold
	// readers (findingFromResource, findingsFromRows, stripWave2) to recognise
	// the entry. Tolerate enrichers that forgot to set Source by stamping the
	// canonical form here.
	if f.Source == "" {
		f.Source = "wave2:" + td.ShortName
	} else if !strings.HasPrefix(f.Source, "wave2:") {
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
