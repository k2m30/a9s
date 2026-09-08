// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// relatedIDSubset applies the related-drill prefilter: when RelatedIDSet is
// non-nil (empty for a "(0+)" drill), only rows the drill's own check named
// pass. The one owner of that rule — the rendered rows and the title's count
// both read it, so a drill can never title the type's whole population over
// the ten rows it is showing.
func relatedIDSubset(ls *ListState, base []resource.Resource) []resource.Resource {
	if ls.RelatedIDSet == nil {
		return base
	}
	subset := make([]resource.Resource, 0, len(ls.RelatedIDSet))
	for _, r := range base {
		if _, ok := ls.RelatedIDSet[r.ID]; ok {
			subset = append(subset, r)
		}
	}
	return subset
}

func (c *Controller) applyListFilters(ls *ListState, typeName string, base []resource.Resource) []resource.Resource {
	td := c.typeDefForLocked(typeName)
	return applyListFiltersWith(ls, td, resolveListColumnsForBuild(c.viewConfig, typeName, td),
		c.listEnrichmentFindings(typeName), base)
}

// applyListFiltersWith is applyListFilters with its controller reads hoisted
// into arguments, so an off-lock build (listBodyBuild.run) and the locked
// callers run the identical filter chain rather than two spellings of it.
func applyListFiltersWith(ls *ListState, td *resource.ResourceTypeDef, columns []ColumnDef, findings map[string][]domain.Finding, base []resource.Resource) []resource.Resource {
	// RelatedIDSet prefilter: when non-nil (even if empty), only IDs in the set pass.
	base = relatedIDSubset(ls, base)

	// Text filter. The columns are resolved the way the list resolves them, so
	// the filter compares the strings the rows on screen are made of.
	result := listFilterResources(ls.Filter, columns, td, base)

	// Attention filter: a row is kept when it carries an issue finding of its
	// own, or — having no findings at all — when the type's classifier calls
	// it an issue or the Wave-2 store holds one for it.
	if ls.AttentionOnly && td != nil {
		kept := make([]resource.Resource, 0, len(result))
		for _, r := range result {
			if listHasIssueFinding(r) {
				kept = append(kept, r)
				continue
			}
			if len(r.Findings) == 0 {
				if td.ResolveColor(r).IsIssue() {
					kept = append(kept, r)
					continue
				}
				if _, hasFinding := findings[r.ID]; hasFinding {
					kept = append(kept, r)
				}
			}
		}
		result = kept
	}

	return result
}

// listFilterResources is the pure text-filter, and the only one. It matches
// the row's identity, every cell the row renders, and every finding phrase.
//
// Cells rather than raw Fields values: the operator types what the screen
// showed them. A humanized column renders "task failed to start" and the AWS
// constant behind it appears nowhere, so the constant matches nothing and the
// words do — and a Fields entry no column shows cannot pull in a row for a
// reason the operator can neither see nor guess. columns and td are what the
// list itself resolved, so the two cannot disagree about what a cell says.
func listFilterResources(query string, columns []ColumnDef, td *resource.ResourceTypeDef, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if listRowMatches(q, columns, td, r) {
			result = append(result, r)
		}
	}
	return result
}

// listRowMatches reports whether one row answers the (already lowercased)
// query on any of the three things an operator can read off it.
func listRowMatches(q string, columns []ColumnDef, td *resource.ResourceTypeDef, r resource.Resource) bool {
	if strings.Contains(strings.ToLower(r.ID), q) || strings.Contains(strings.ToLower(r.Name), q) {
		return true
	}
	for _, col := range columns {
		if strings.Contains(strings.ToLower(ExtractCellValue(col, td, r)), q) {
			return true
		}
	}
	for _, f := range r.Findings {
		if strings.Contains(strings.ToLower(f.Phrase), q) {
			return true
		}
	}
	return false
}

// listHasIssueFinding mirrors hasIssueFinding in views.
func listHasIssueFinding(r resource.Resource) bool {
	for _, f := range r.Findings {
		if f.Severity.IsIssue() {
			return true
		}
	}
	return false
}

// listSortResources sorts resources by ls.SortCol/SortDir. No-op when SortCol
// is empty.
//
// columns is the resolved set the list is rendering, so the comparator reads
// the same Path, SortKey and identity election the cells were
// extracted with. A set resolved separately here would sort by a column the
// list does not show, and would compare every row that only the identity
// column can distinguish — a warm-cache replay, a degraded fetch — as equal.
func listSortResources(columns []ColumnDef, td *resource.ResourceTypeDef, ls *ListState, resources []resource.Resource) []resource.Resource {
	if ls.SortCol == "" || len(resources) == 0 {
		return resources
	}

	col := ColumnDef{Key: ls.SortCol}
	if i := SortColIndex(columns, ls.SortCol); i >= 0 {
		col = columns[i]
	}

	sortAsc := ls.SortDir != "desc"
	out := make([]resource.Resource, len(resources))
	copy(out, resources)

	sort.SliceStable(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		// An explicit sort key wins over everything: it is the column's own
		// statement that its displayed text is not what it sorts by.
		if col.SortKey != "" {
			return sortStrings(a.Fields[col.SortKey], b.Fields[col.SortKey], sortAsc)
		}

		// The cell, on every frame. A live row still carries its SDK struct and
		// a replayed one does not, so a comparator that read the struct where
		// it was there answered from a representation the cached frame cannot
		// reach — and the list re-ordered under the operator the moment the
		// fetch landed. sortStrings reads a number as a number, so the orders
		// a struct comparison used to add are the ones the cache cannot
		// express anyway.
		return sortStrings(ExtractCellValue(col, td, a), ExtractCellValue(col, td, b), sortAsc)
	})
	return out
}

// sortStrings orders two cell values, numerically when both parse as numbers
// (a byte count stored as text sorts 900 before 2048, not after it) and
// lexically otherwise.
func sortStrings(va, vb string, asc bool) bool {
	if fa, err := strconv.ParseFloat(va, 64); err == nil {
		if fb, err := strconv.ParseFloat(vb, 64); err == nil {
			if asc {
				return fa < fb
			}
			return fa > fb
		}
	}
	if asc {
		return va < vb
	}
	return va > vb
}

// reapplyCheckerAgainst re-runs THIS list screen's own reapply checker against
// newPage and merges returned IDs into ls.RelatedIDSet. The checker lives on the
// ListState (set by patchListReapplyChecker), so a screen with no checker — a
// normal, non-related list — is a no-op and can never be filtered by a stale
// checker left over from a popped related list. typeName only keys the synthetic
// cache the checker scans.
func (c *Controller) reapplyCheckerAgainst(ls *ListState, typeName string, newPage []resource.Resource) {
	if ls == nil || ls.reapplyChecker == nil || len(newPage) == 0 {
		return
	}
	synth := resource.ResourceCache{
		typeName: resource.ResourceCacheEntry{Resources: newPage},
	}
	result := ls.reapplyChecker(context.Background(), nil, ls.reapplySource, synth)
	if len(result.ResourceIDs()) == 0 {
		return
	}
	if ls.RelatedIDSet == nil {
		ls.RelatedIDSet = make(map[string]struct{}, len(result.ResourceIDs()))
	}
	for _, id := range result.ResourceIDs() {
		if id != "" {
			ls.RelatedIDSet[id] = struct{}{}
		}
	}
	ls.rowsVersion++
}

// ApplyEnrichmentState stores Wave-2 enrichment results for typeName.
// Mirrors ResourceListModel.SetEnrichmentState. issueCount is always treated
// as an authoritative Wave-2 result — every caller of this exported entry
// point (ResourceListModel, tests) passes a real observed count, never a
// defensive placeholder. findings/details carry every independently-
// evaluated Wave-2 condition per resource (never a single worst-severity
// representative).
func (c *Controller) ApplyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string][]domain.Finding, details map[string]map[domain.FindingCode]domain.AttentionDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyEnrichmentState(typeName, issueCount, truncated, findings, details, true)
}

// applyEnrichmentState is the lock-free implementation of ApplyEnrichmentState.
// authoritative marks whether issueCount is a confirmed Wave-2 result (true)
// or a defensive fallback standing in for "no result carried" (false — used
// by intents.go's PatchResourceList case when v.Issues is nil, so a
// fabricated 0/false does not falsely mark the type's issue badge Known).
// Callers must hold c.mu (write).
func (c *Controller) applyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string][]domain.Finding, details map[string]map[domain.FindingCode]domain.AttentionDetail, authoritative bool) {
	if c.enrichmentStore == nil {
		c.enrichmentStore = make(map[string]map[string][]domain.Finding)
	}
	if c.enrichmentDetails == nil {
		c.enrichmentDetails = make(map[string]map[string]map[domain.FindingCode]domain.AttentionDetail)
	}
	if c.enrichmentTruncated == nil {
		c.enrichmentTruncated = make(map[string]bool)
	}
	c.enrichmentStore[typeName] = findings
	c.enrichmentDetails[typeName] = details
	c.enrichmentTruncated[typeName] = truncated
	c.enrichmentGen++

	// Menu issue-badge sync: the in-list Wave-2 enrichment lane is the ONLY
	// in-session source of the menu issue badge for renderers that run no
	// background availability sweep (the web/headless lane) — syncing here
	// mirrors syncExactTotalToMenu's issue-count half via the shared
	// syncMenuIssueCount chokepoint, so a TUI-only sweep-driven badge doesn't
	// mask this lane's inability to update it any other way.
	canon := resource.CanonicalShortName(typeName)
	if ms := c.rootMenuState(); ms != nil {
		// authoritative propagates the caller's own authority over issueCount:
		// when true (issueCount IS the confirmed Wave-2 result for canon), even
		// a genuine zero must flip IssueKnown — otherwise a clean type stays
		// "unknown" forever.
		c.syncMenuIssueCount(ms, canon, issueCount, truncated, authoritative)

		// Persist, mirroring syncExactTotalToMenu's disk-write half (the
		// badge must survive a restart). Best-effort, same as the
		// sweep lane — AmendRows (called by applyRowFindings right after this
		// in the PatchResourceList intent path) only mutates the in-memory
		// RowStore and never reaches store.SaveType, so without this call the
		// badge this function just raised would be lost on the next launch
		// even though it is visible for the rest of the session.
		c.persistMenuAvailabilityCache()
	}
}

// listEnrichmentFindings returns the per-resource, slice-valued Wave-2
// finding map for typeName (every independently-evaluated condition, not
// just a single worst-severity representative), or nil.
func (c *Controller) listEnrichmentFindings(typeName string) map[string][]domain.Finding {
	if c.enrichmentStore == nil {
		return nil
	}
	return c.enrichmentStore[typeName]
}

// listEnrichmentDetails returns the per-resource, per-FindingCode nested
// AttentionDetail map for typeName, or nil.
func (c *Controller) listEnrichmentDetails(typeName string) map[string]map[domain.FindingCode]domain.AttentionDetail {
	if c.enrichmentDetails == nil {
		return nil
	}
	return c.enrichmentDetails[typeName]
}
