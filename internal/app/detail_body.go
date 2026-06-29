package app

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// buildDetailBody constructs a DetailBody from a DetailState, mirroring the
// data that DetailModel.View() + renderFromFieldList() consume. The body is
// renderer-agnostic: scroll, width, and height remain owned by the renderer.
// vc may be nil; when nil the built-in projection defaults are used.
func buildDetailBody(ds *DetailState, vc *config.ViewsConfig) *DetailBody {
	items := buildDetailFieldItems(ds, vc)

	// Convert []fieldpath.FieldItem → []FieldRow for the body.
	fields := fieldItemsToFieldRows(items)

	// Compute key width (mirrors computeKeyWidth in detail_fields.go).
	var topPaths []string
	for _, item := range items {
		if !item.IsHeader && !item.IsSubField {
			topPaths = append(topPaths, item.Key)
		}
	}
	keyWidth := computeDetailKeyWidth(topPaths)

	// Build RelatedBlocks. When ds.RelatedRows is nil/empty but the resource
	// type has registered related defs, synthesise loading-state blocks from
	// those defs. This mirrors what newRightColumn() does in the TUI: it creates
	// one row per RelatedDef with count=-1/loading=true so the panel renders
	// its "loading" state immediately on push, before checker results arrive.
	related := buildDetailRelatedBlocks(ds)
	if len(related) == 0 && len(ds.RelatedRows) == 0 {
		related = buildDetailRelatedLoadingBlocks(ds.ResourceType)
	}

	// Clamp FieldCursor.
	fc := ds.FieldCursor
	if len(fields) > 0 && fc >= len(fields) {
		fc = len(fields) - 1
	}
	if fc < 0 {
		fc = 0
	}

	// Clamp RelatedCursor.
	rc := ds.RelatedCursor
	if len(related) > 0 && rc >= len(related) {
		rc = len(related) - 1
	}
	if rc < 0 {
		rc = 0
	}

	// Clamp RelatedScroll.
	rs := max(ds.RelatedScroll, 0)
	if len(related) > 0 && rs >= len(related) {
		rs = len(related) - 1
	}

	// RelatedVisible: auto-show when related blocks exist (matching TUI SetSize
	// auto-show), unless the user has explicitly hidden the panel (RelatedHidden).
	// When RelatedHidden is set, use ds.RelatedVisible directly.
	var relatedVisible bool
	if ds.RelatedHidden {
		relatedVisible = ds.RelatedVisible
	} else {
		relatedVisible = ds.RelatedVisible || len(related) > 0
	}

	return &DetailBody{
		Fields:              fields,
		Related:             related,
		RelatedFocused:      ds.RelatedFocus,
		RelatedVisible:      relatedVisible,
		RelatedCursor:       rc,
		RelatedScroll:       rs,
		RelatedFilter:       ds.RelatedFilter,
		RelatedFilterActive: ds.RelatedFilterActive,
		RelatedSourceType:   ds.ResourceType,
		Search:              ds.SearchQuery,
		SearchCursor:        ds.SearchCursor,
		Wrap:                ds.Wrap,
		ScrollY:             ds.ScrollY,
		FieldCursor:         fc,
		KeyWidth:            keyWidth,
	}
}

// buildDetailRelatedLoadingBlocks constructs loading-state RelatedBlocks from
// registered related defs when the DetailState has no RelatedRows yet. Mirrors
// newRightColumn(defs, res, sourceType) which sets count=-1, loading=true.
func buildDetailRelatedLoadingBlocks(resourceType string) []RelatedBlock {
	defs := resource.GetRelated(resourceType)
	if len(defs) == 0 {
		return nil
	}
	blocks := make([]RelatedBlock, 0, len(defs))
	for _, def := range defs {
		blocks = append(blocks, RelatedBlock{
			Name:       def.DisplayName,
			Count:      -1,
			Loading:    true,
			TargetType: def.TargetType,
		})
	}
	return blocks
}

// buildDetailFieldItems runs the same projector pipeline as
// DetailModel.buildFieldList + injectAttentionSection and returns the
// []fieldpath.FieldItem that both the TUI renderer and buildDetailBody consume.
// vc may be nil; projection.GenericWithConfig(nil) uses built-in defaults.
func buildDetailFieldItems(ds *DetailState, vc *config.ViewsConfig) []fieldpath.FieldItem {
	r := ds.Resource
	if r.Type == "" {
		r.Type = ds.ResourceType
	}
	td := resource.FindResourceType(ds.ResourceType)

	// Inject findings from DetailState into the resource for the projector.
	// This mirrors what the TUI does: m.res.Findings carries the live data.
	// ds.Findings holds the unified finding set (wave-1 seeded at creation +
	// wave-2 merged by applyFindingToState), so a straight assignment is correct.
	if len(ds.Findings) > 0 {
		r.Findings = ds.Findings
	}
	if len(ds.AttentionDetails) > 0 {
		if r.AttentionDetails == nil {
			r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, len(ds.AttentionDetails))
		}
		maps.Copy(r.AttentionDetails, ds.AttentionDetails)
	}

	generic := projection.GenericWithConfig(vc)

	var proj domain.DetailProjector
	if td != nil && td.Project != nil {
		proj = td.Project
	} else {
		proj = generic
	}
	sections := proj(r)
	if len(sections) == 0 && td != nil && td.Project != nil {
		sections = generic(r)
	}
	if td != nil && td.Augment != nil {
		sections = td.Augment(r, sections)
	}
	items := sectionsToFieldItemsDetail(sections)
	items = injectAttentionSectionDetail(items, ds, td)
	return items
}

// sectionsToFieldItemsDetail converts []domain.Section → []fieldpath.FieldItem,
// mirroring sectionsToFieldItems in detail_fields.go.
func sectionsToFieldItemsDetail(sections []domain.Section) []fieldpath.FieldItem {
	if len(sections) == 0 {
		return nil
	}
	var items []fieldpath.FieldItem
	for _, sec := range sections {
		if sec.Title != "" {
			items = append(items, fieldpath.FieldItem{
				IsSection: true,
				Key:       sec.Title,
				Path:      sec.Title,
			})
		}
		for _, it := range sec.Items {
			items = append(items, domainItemToFieldItemDetail(it, sec.Title))
		}
	}
	return items
}

// domainItemToFieldItemDetail mirrors domainItemToFieldItem in detail_fields.go.
func domainItemToFieldItemDetail(it domain.Item, sectionTitle string) fieldpath.FieldItem {
	fi := fieldpath.FieldItem{
		Key:         it.Label,
		Value:       it.Value,
		Path:        it.Path,
		IsNavigable: it.Navigable,
		TargetType:  it.TargetType,
		ColorTier:   it.Tier,
		NavID:       it.NavID,
	}
	if fi.Path == "" {
		fi.Path = sectionTitle + "." + it.Label
	}
	switch it.Kind {
	case domain.ItemHeader:
		fi.IsHeader = true
		if it.Path == "" {
			fi.Path = it.Label
		}
	case domain.ItemSubfield:
		fi.IsSubField = true
		fi.IndentLevel = it.IndentLevel
		if it.Label == "" {
			fi.Key = it.Value
			if it.Path == "" {
				fi.Path = sectionTitle
			}
		}
	case domain.ItemSpacer:
		fi.IsSpacer = true
	}
	return fi
}

// injectAttentionSectionDetail mirrors injectAttentionSection in detail_fields.go,
// prepending the Attention block when the resource has issue-severity findings.
func injectAttentionSectionDetail(items []fieldpath.FieldItem, ds *DetailState, td *resource.ResourceTypeDef) []fieldpath.FieldItem {
	type entry struct {
		tier          string
		primary       string
		rows          []domain.DetailRow
		splitKeyValue bool
	}
	var entries []entry
	for _, f := range ds.Findings {
		if !f.Severity.IsIssue() {
			continue
		}
		var tier string
		switch f.Severity {
		case domain.SevBroken:
			tier = "!"
		default:
			tier = "~"
		}
		var rows []domain.DetailRow
		if ds.AttentionDetails != nil {
			if det, ok := ds.AttentionDetails[f.Code]; ok {
				rows = det.Rows
			}
		}
		entries = append(entries, entry{tier: tier, primary: f.Phrase, rows: rows, splitKeyValue: true})
	}
	if len(entries) == 0 {
		return items
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].tier == "!" && entries[j].tier != "!"
	})
	// Resolve S2 color bucket for the cap invariant.
	var rowBucket resource.Color
	if td != nil {
		rowBucket = td.ResolveColor(ds.Resource)
	} else {
		rowBucket = resource.ColorHealthy
	}
	headerTier := "~"
	for _, e := range entries {
		if e.tier == "!" {
			headerTier = "!"
			break
		}
	}
	injected := make([]fieldpath.FieldItem, 0, 1+len(entries)*2)
	injected = append(injected, fieldpath.FieldItem{
		IsSection: true,
		Key:       fmt.Sprintf("Attention (%d)", len(entries)),
		Path:      "Attention",
		ColorTier: capTierToRowBucketDetail(headerTier, rowBucket),
	})
	for _, e := range entries {
		glyph := e.tier
		if glyph != "!" && glyph != "~" {
			glyph = "~"
		}
		displayPhrase := capitalizeFirstDetail(e.primary)
		line := glyph + " " + displayPhrase
		entryColor := capTierToRowBucketDetail(e.tier, rowBucket)
		itemKey := line
		itemValue := line
		if e.splitKeyValue {
			itemKey = e.primary
			itemValue = line
		}
		injected = append(injected, fieldpath.FieldItem{
			IsSubField:  true,
			IndentLevel: 1,
			Key:         itemKey,
			Value:       itemValue,
			Path:        "Attention",
			ColorTier:   entryColor,
		})
		for _, row := range e.rows {
			tier := row.Tier
			if tier == "" {
				tier = e.tier
			}
			injected = append(injected, fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 3,
				Key:         row.Label,
				Value:       row.Value,
				Path:        "Attention",
				ColorTier:   capTierToRowBucketDetail(tier, rowBucket),
			})
		}
	}
	injected = append(injected, fieldpath.FieldItem{IsSpacer: true, Path: "Attention"})
	return append(injected, items...)
}

// capTierToRowBucketDetail mirrors capTierToRowBucket in detail_fields.go.
func capTierToRowBucketDetail(tier string, rowBucket resource.Color) string {
	if tier == "!" && rowBucket != resource.ColorBroken {
		return "~"
	}
	return tier
}

// capitalizeFirstDetail mirrors capitalizeFirst in detail_fields.go.
func capitalizeFirstDetail(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// computeDetailKeyWidth mirrors computeKeyWidth in detail_fields.go.
func computeDetailKeyWidth(keys []string) int {
	maxW := 0
	for _, k := range keys {
		if n := len(k) + 1; n > maxW { // +1 for the ":" suffix
			maxW = n
		}
	}
	return maxW
}

// fieldItemsToFieldRows converts []fieldpath.FieldItem → []FieldRow for the
// DetailBody, carrying the render-time metadata needed by RenderDetail.
func fieldItemsToFieldRows(items []fieldpath.FieldItem) []FieldRow {
	rows := make([]FieldRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, FieldRow{
			Key:         item.Key,
			Value:       item.Value,
			IsSection:   item.IsSection,
			IsHeader:    item.IsHeader,
			IsSubField:  item.IsSubField,
			IsSpacer:    item.IsSpacer,
			IsNavigable: item.IsNavigable,
			TargetType:  item.TargetType,
			NavID:       item.NavID,
			IndentLevel: item.IndentLevel,
			ColorTier:   item.ColorTier,
			Path:        item.Path,
		})
	}
	return rows
}

// buildDetailRelatedBlocks converts []DetailRelatedRow → []RelatedBlock,
// applying self-pivot-zero filtering and the current filter query.
func buildDetailRelatedBlocks(ds *DetailState) []RelatedBlock {
	query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
	var blocks []RelatedBlock
	for _, row := range ds.RelatedRows {
		if isSelfPivotZeroDetailRow(row, ds.ResourceType) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(row.DisplayName), query) {
			continue
		}
		blocks = append(blocks, RelatedBlock{
			Name:         row.DisplayName,
			Count:        row.Count,
			Loading:      row.Loading,
			Err:          row.Err != "",
			Approximate:  row.Approximate,
			FetchFilter:  row.FetchFilter,
			TargetType:   row.TargetType,
			Actionable:   isActionableDetailRow(row),
			CountDisplay: resource.FormatRelatedCount(row.Count),
		})
	}
	return blocks
}
