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

// applyListFilters applies the relatedIDSet prefilter, text filter, and
// attention filter to base, returning the visible subset. Mirrors
// ResourceListModel.applyFilter exactly so ListSelected and buildListBody
// agree on which row is "selected".
func (c *Controller) applyListFilters(ls *ListState, typeName string, base []resource.Resource) []resource.Resource {
	// Prefer the fallback typeDef (registered via RegisterFallbackTypeDef from
	// the model constructor) over the catalog: the model's typeDef is the
	// authoritative Color classifier, matching listIssueCount/GetListIssueCount.
	// Test typeDefs frequently share a ShortName with a catalog type (e.g.
	// "ec2") but use a different Color implementation (or none, falling back
	// to colorFallback(r.Fields["status"])) — using the catalog type here
	// would silently disagree with the issue count the title/badge report.
	var td *resource.ResourceTypeDef
	if fv, ok := c.fallbackTypeDefs[typeName]; ok {
		td = &fv
	} else if catalogTD := resource.FindResourceType(typeName); catalogTD != nil {
		td = catalogTD
	}

	// RelatedIDSet prefilter: when non-nil (even if empty), only IDs in the set pass.
	if ls.RelatedIDSet != nil {
		subset := make([]resource.Resource, 0, len(ls.RelatedIDSet))
		for _, r := range base {
			if _, ok := ls.RelatedIDSet[r.ID]; ok {
				subset = append(subset, r)
			}
		}
		base = subset
	}

	// Text filter — matches r.ID, r.Name, r.Fields values, r.Findings[i].Phrase.
	result := listFilterResources(ls.Filter, base)

	// Attention filter: mirrors ResourceListModel.applyFilter §7.
	if ls.AttentionOnly && td != nil {
		findings := c.listEnrichmentFindings(typeName)
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

// listFilterResources is the pure text-filter; mirrors FilterResources in views.
func listFilterResources(query string, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0, len(resources))
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.ID), q) ||
			strings.Contains(strings.ToLower(r.Name), q) {
			result = append(result, r)
			continue
		}
		matched := false
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, r)
			continue
		}
		for _, f := range r.Findings {
			if strings.Contains(strings.ToLower(f.Phrase), q) {
				result = append(result, r)
				break
			}
		}
	}
	return result
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
	canon := typeName
	if td := resource.FindResourceType(typeName); td != nil {
		canon = td.ShortName
	}
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
		c.persistMenuAvailabilityCache(ms)
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
