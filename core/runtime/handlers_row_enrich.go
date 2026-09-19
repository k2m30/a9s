// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package runtime

import (
	"maps"
	"slices"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// handleRowEnriched folds one row's on-demand Wave-2 answer (KindEnrichRow)
// onto that row alone. The sweep's answer for every other row stands: the
// session's uninspected set loses this id (or renames its check to the call
// that refused on demand), the row store folds this id, and the list and any
// open detail are patched for this id only.
func (c *Core) handleRowEnriched(msg messages.RowEnriched) ([]UIIntent, []TaskRequest) {
	canon := resource.CanonicalShortName(msg.ResourceType)
	td := resource.FindResourceType(canon)
	if td == nil {
		return nil, nil
	}
	// A refresh since the dispatch (the list's, or the menu's) asked for
	// every row again; an answer from before it is not the one asked for.
	if msg.Gen != c.session.EnrichmentGen || msg.TypeGen != c.session.EnrichmentTypeGenGet(canon) {
		return nil, nil
	}
	// The same rule as the sweep's: a probe that came back with an error and
	// nothing else answered for nobody. The row keeps its mark and what it
	// renders, and the operator hears why (a marked, stale answer beats
	// a confidently wrong clean one).
	if msg.Err != nil && len(msg.Findings) == 0 && len(msg.FieldUpdates) == 0 && !msg.Uninspected {
		_, region := c.session.CurrentPair()
		return []UIIntent{FlashIntent{Text: failureLine("enrich "+canon, msg.Err, region), IsError: true}}, nil
	}

	set := c.session.EnrichmentTruncatedIDs[canon]
	if msg.Uninspected {
		// Both surfaces read the mark from the session at their next build.
		if set == nil {
			set = make(map[string]string)
			c.session.EnrichmentTruncatedIDs[canon] = set
		}
		set[msg.ResourceID] = msg.Check
	} else {
		delete(set, msg.ResourceID)
		answered := c.session.EnrichmentRowAnswered[canon]
		if answered == nil {
			answered = make(map[string]struct{})
			c.session.EnrichmentRowAnswered[canon] = answered
		}
		answered[msg.ResourceID] = struct{}{}
	}
	// A refused check can still have read a field (a count up to the page
	// that was refused); the field lands, the findings do not.
	c.AmendRows(canon, func(rows []resource.Resource) []resource.Resource {
		out := make([]resource.Resource, len(rows))
		copy(out, rows)
		for i := range out {
			if out[i].ID != msg.ResourceID {
				continue
			}
			if !msg.Uninspected {
				ApplyWave2ToRow(&out[i], *td, msg.Findings, msg.AttentionDetails)
			}
			if updates := msg.FieldUpdates[msg.ResourceID]; len(updates) > 0 {
				fields := make(map[string]string, len(out[i].Fields)+len(updates))
				maps.Copy(fields, out[i].Fields)
				maps.Copy(fields, updates)
				out[i].Fields = fields
			}
		}
		return out
	})

	rows, _ := c.ProbeResources(canon)
	unified := unifiedIssueCount(rows, *td)
	truncated := issueLowerBound(canon, c.pageTruncated(canon), c.session.EnrichmentCutGet(canon), unified > 0 || slices.ContainsFunc(rows, func(r resource.Resource) bool { return rowHasWave2Finding(r.Findings) }), len(c.session.EnrichmentTruncatedIDs[canon]))
	// Saved the way the sweep's answer is, with the set as it now stands.
	var tasks []TaskRequest
	if save := c.snapshotRowStoreForSave(map[string]bool{canon: true}); save != nil {
		tasks = append(tasks, TaskRequest{Key: TaskKey{Kind: TaskKindSaveCache}, Payload: save})
	}
	rowIDs := []string{}
	if !msg.Uninspected {
		rowIDs = append(rowIDs, msg.ResourceID)
	}
	intents := []UIIntent{
		PatchMenu{ResourceType: canon, Issues: unified, Truncated: truncated},
		PatchResourceList{
			ResourceType: canon,
			Issues:       &IssueBadgePatch{Count: unified, Truncated: truncated},
			Enrichment: &ListEnrichmentPatch{
				Findings:         msg.Findings,
				AttentionDetails: msg.AttentionDetails,
				FieldUpdates:     msg.FieldUpdates,
				TruncatedIDs:     set,
				RowIDs:           rowIDs,
			},
		},
	}
	detail := PatchDetail{ResourceType: canon, ResourceID: msg.ResourceID, FieldUpdates: msg.FieldUpdates}
	if !msg.Uninspected {
		detail.EnrichmentFindings = map[string][]domain.Finding{msg.ResourceID: msg.Findings[msg.ResourceID]}
		detail.EnrichmentAttentionDetails = msg.AttentionDetails
	}
	return append(intents, detail), tasks
}

// keepRowAnswers folds the rows a KindEnrichRow answered for into a sweep
// result that reports them at its cap: the sweep was dispatched before the
// answer landed and never looked at those rows, so they keep the answer —
// out of the uninspected set, and carrying the findings the row store holds
// for them, so the fold and the list store see them as answered. A sweep
// that did look at such a row (it is absent from TruncatedIDs, or refused
// by a named call) supersedes the answer.
func (c *Core) keepRowAnswers(msg messages.EnrichmentChecked) messages.EnrichmentChecked {
	answered := c.session.EnrichmentRowAnswered[msg.ResourceType]
	if len(answered) == 0 {
		return msg
	}
	rows, _ := c.ProbeResources(msg.ResourceType)
	byID := make(map[string]resource.Resource, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	truncated := maps.Clone(msg.TruncatedIDs)
	findings := maps.Clone(msg.Findings)
	details := maps.Clone(msg.AttentionDetails)
	for id := range answered {
		if truncated[id] != awsclient.CheckCap {
			continue
		}
		delete(truncated, id)
		row, ok := byID[id]
		if !ok {
			continue
		}
		var wave2 []domain.Finding
		for _, f := range row.Findings {
			if f.IsWave2Sourced() {
				wave2 = append(wave2, f)
			}
		}
		if len(wave2) == 0 {
			continue
		}
		if findings == nil {
			findings = make(map[string][]domain.Finding)
		}
		findings[id] = wave2
		if len(row.AttentionDetails) > 0 {
			if details == nil {
				details = make(map[string]map[domain.FindingCode]domain.AttentionDetail)
			}
			details[id] = maps.Clone(row.AttentionDetails)
		}
	}
	msg.TruncatedIDs, msg.Findings, msg.AttentionDetails = truncated, findings, details
	return msg
}
