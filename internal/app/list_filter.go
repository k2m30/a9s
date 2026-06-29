package app

import (
	"context"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// applyListFilters applies the relatedIDSet prefilter, text filter, and
// attention filter to base, returning the visible subset. Mirrors
// ResourceListModel.applyFilter exactly so ListSelected and buildListBody
// agree on which row is "selected".
func (c *Controller) applyListFilters(ls *ListState, typeName string, base []resource.Resource) []resource.Resource {
	td := resource.FindResourceType(typeName)
	if td == nil {
		if fv, ok := c.fallbackTypeDefs[typeName]; ok {
			td = &fv
		}
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
		if resource.IsIssueSeverity(f.Severity) {
			return true
		}
	}
	return false
}

// listSortResources sorts resources by ls.SortCol/SortDir, mirroring
// sortFiltered in views/sort.go. No-op when SortCol is empty.
// vc is the per-session view config (nil = built-in defaults only); passing
// the controller's viewConfig ensures user-configured sort_key / sort_path
// columns resolve correctly — matching what buildListBody and resourcelist.go
// do (Bug 3 fix).
func listSortResources(vc *config.ViewsConfig, ls *ListState, typeName string, resources []resource.Resource) []resource.Resource {
	if ls.SortCol == "" || len(resources) == 0 {
		return resources
	}

	// Resolve columns from viewConfig first (same priority as buildListBody /
	// resolveColumns in table_render.go) so custom sort_key / sort_path columns
	// are found even when they are not in the built-in defaults.
	vd := config.GetViewDef(vc, typeName)
	if len(vd.List) == 0 {
		vd = config.GetViewDef(nil, typeName)
	}
	var col *config.ListColumn
	sortColLower := strings.ToLower(ls.SortCol)
	for i := range vd.List {
		lc := &vd.List[i]
		if lc.Key == ls.SortCol || lc.Path == ls.SortCol {
			col = lc
			break
		}
		titleUnder := strings.ToLower(strings.ReplaceAll(lc.Title, " ", "_"))
		if titleUnder == sortColLower {
			col = lc
			break
		}
	}

	sortAsc := ls.SortDir != "desc"
	out := make([]resource.Resource, len(resources))
	copy(out, resources)

	sort.SliceStable(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		// Raw struct comparison (numeric/time) when a sortPath or path is present.
		rawPath := ""
		if col != nil {
			rawPath = col.SortPath
			if rawPath == "" {
				rawPath = col.Path
			}
		}
		if rawPath != "" && a.RawStruct != nil && b.RawStruct != nil {
			if cmp, ok := listCompareRaw(a.RawStruct, b.RawStruct, rawPath); ok {
				if sortAsc {
					return cmp < 0
				}
				return cmp > 0
			}
		}

		// Display-value fallback.
		var va, vb string
		if col != nil && col.SortKey != "" {
			va = a.Fields[col.SortKey]
			vb = b.Fields[col.SortKey]
		} else {
			sortColDef := ColumnDef{Key: ls.SortCol}
			if col != nil {
				sortColDef = ColumnDef{Key: col.Key, Title: col.Title, Path: col.Path}
			}
			td := resource.FindResourceType(typeName)
			va = listExtractCellValue(sortColDef, td, a)
			vb = listExtractCellValue(sortColDef, td, b)
		}
		if fa, err := strconv.ParseFloat(va, 64); err == nil {
			if fb, err := strconv.ParseFloat(vb, 64); err == nil {
				if sortAsc {
					return fa < fb
				}
				return fa > fb
			}
		}
		if sortAsc {
			return va < vb
		}
		return va > vb
	})
	return out
}

// listCompareRaw mirrors compareRaw from views/sort.go.
func listCompareRaw(a, b any, path string) (int, bool) {
	va, errA := fieldpath.ExtractValue(a, path)
	vb, errB := fieldpath.ExtractValue(b, path)
	if errA != nil || errB != nil {
		return 0, false
	}
	// Dereference pointers.
	for va.Kind() == reflect.Pointer {
		if va.IsNil() {
			return 0, false
		}
		va = va.Elem()
	}
	for vb.Kind() == reflect.Pointer {
		if vb.IsNil() {
			return 0, false
		}
		vb = vb.Elem()
	}
	// time.Time comparison.
	if va.Type() == reflect.TypeFor[time.Time]() && vb.Type() == reflect.TypeFor[time.Time]() {
		return va.Interface().(time.Time).Compare(vb.Interface().(time.Time)), true
	}
	// Numeric comparison.
	fa, okA := listToFloat(va)
	fb, okB := listToFloat(vb)
	if okA && okB {
		if fa < fb {
			return -1, true
		}
		if fa > fb {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// listToFloat mirrors toFloat from views/sort.go.
func listToFloat(v reflect.Value) (float64, bool) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	default:
		return 0, false
	}
}

// reapplyCheckerEntry holds the per-type reapply checker + source resource for
// approximate-pivot navigations. Keyed by resource type short name.
type reapplyCheckerEntry struct {
	checker resource.RelatedChecker
	source  resource.Resource
}

// reapplyCheckerAgainst re-runs the stored checker for typeName against newPage
// and merges returned IDs into ls.RelatedIDSet. Mirrors ReapplyCheckerAgainst.
func (c *Controller) reapplyCheckerAgainst(ls *ListState, typeName string, newPage []resource.Resource) {
	if c.reapplyCheckers == nil {
		return
	}
	entry, ok := c.reapplyCheckers[typeName]
	if !ok || entry.checker == nil || len(newPage) == 0 {
		return
	}
	synth := resource.ResourceCache{
		typeName: resource.ResourceCacheEntry{Resources: newPage},
	}
	result := entry.checker(context.Background(), nil, entry.source, synth)
	if len(result.ResourceIDs) == 0 {
		return
	}
	if ls.RelatedIDSet == nil {
		ls.RelatedIDSet = make(map[string]struct{}, len(result.ResourceIDs))
	}
	for _, id := range result.ResourceIDs {
		if id != "" {
			ls.RelatedIDSet[id] = struct{}{}
		}
	}
}

// ApplyEnrichmentState stores Wave-2 enrichment results for typeName.
// Mirrors ResourceListModel.SetEnrichmentState.
func (c *Controller) ApplyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string]domain.Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyEnrichmentState(typeName, issueCount, truncated, findings)
}

// applyEnrichmentState is the lock-free implementation of ApplyEnrichmentState.
// Callers must hold c.mu (write).
func (c *Controller) applyEnrichmentState(typeName string, issueCount int, truncated bool, findings map[string]domain.Finding) {
	if c.enrichmentStore == nil {
		c.enrichmentStore = make(map[string]map[string]domain.Finding)
	}
	if c.enrichmentTruncated == nil {
		c.enrichmentTruncated = make(map[string]bool)
	}
	c.enrichmentStore[typeName] = findings
	c.enrichmentTruncated[typeName] = truncated
	_ = issueCount // retained for caller parity; issue count is recomputed in buildListBody
}

// listEnrichmentFindings returns the per-resource finding map for typeName, or nil.
func (c *Controller) listEnrichmentFindings(typeName string) map[string]domain.Finding {
	if c.enrichmentStore == nil {
		return nil
	}
	return c.enrichmentStore[typeName]
}
