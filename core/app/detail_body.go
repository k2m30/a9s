// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/projection"
)

// buildDetailBody constructs a DetailBody from a DetailState, mirroring the
// data that DetailModel.View() + renderFromFieldList() consume. The body is
// renderer-agnostic: scroll, width, and height remain owned by the renderer.
// The projection config and the "could not inspect this row" fact are both
// read from the controller, so no caller has to carry them.
func (c *Controller) buildDetailBody(ds *DetailState) *DetailBody {
	items := c.buildDetailFieldItems(ds)

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
			State:      domain.RelatedLoading,
			Loading:    true,
			TargetType: def.TargetType,
		})
	}
	return blocks
}

// buildDetailFieldItems runs the projector pipeline (projection.buildItems,
// core/semantics/projection/generic.go) and returns the []fieldpath.FieldItem
// that both the TUI renderer and buildDetailBody consume.
// c.viewConfig may be nil; projection.GenericWithConfig(nil) uses built-in
// defaults.
func (c *Controller) buildDetailFieldItems(ds *DetailState) []fieldpath.FieldItem {
	vc := c.viewConfig
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
	items = injectAttentionSectionDetail(items, ds, td, c.detailNotInspected(ds))
	return items
}

// detailNotInspected reports whether this detail's row is one the Wave-2
// enricher could not inspect. It reads the same session set the list's Status
// cell reads (Controller.listUninspectedIDs), so the two surfaces can never
// disagree about whether a row's posture is known.
func (c *Controller) detailNotInspected(ds *DetailState) bool {
	if ds == nil || ds.Resource.ID == "" || ds.ResourceType == "" {
		return false
	}
	return c.listUninspectedIDs(ds.ResourceType)[ds.Resource.ID]
}

// sectionsToFieldItemsDetail converts []domain.Section → []fieldpath.FieldItem,
// the single implementation (the TUI renders from these items, not its own).
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

// domainItemToFieldItemDetail maps a domain.Item back to a fieldpath.FieldItem.
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

// attentionEntry is one rendered Attention-block entry, built and sorted by
// buildAttentionEntries and rendered by injectAttentionSectionDetail, which is
// also what records the block's size on the DetailState. The FieldCursor delta
// in applyFindingToState reads that record, so no second walk of these entries
// can disagree with the layout on screen about count, order or bareness.
type attentionEntry struct {
	tier    string
	primary string
	// detailLines is the S5 operator sentence (Finding.Detail) already
	// wrapped to the panel, one rendered line per element; empty ⇒
	// Phrase-only, no extra line.
	detailLines   []string
	rows          []domain.DetailRow
	splitKeyValue bool
}

// bare reports whether this entry renders as a Phrase-only line with no
// Detail sentence and no supporting rows.
func (e attentionEntry) bare() bool {
	return len(e.detailLines) == 0 && len(e.rows) == 0
}

// defaultAttentionWrapWidth is the panel width assumed when no renderer has
// reported one (a headless caller with no terminal): the classic 80-column
// terminal, the narrowest panel in common use, so the sentence is never cut
// anywhere rather than merely unlikely to be.
const defaultAttentionWrapWidth = 80

// AttentionIndentColumns is the left margin a renderer puts in front of a
// level-1 sub-field line, and so the room the wrapped Attention sentence has
// to leave. The renderer derives its own indent from this constant, which is
// why the two cannot drift.
const AttentionIndentColumns = 5

// wrapSentence breaks s into lines of at most width columns, on spaces, and
// measures in columns rather than bytes so a sentence carrying wide runes
// wraps where it is actually painted. A word longer than the line goes on a
// line of its own rather than being split: the Attention sentence is prose,
// and a broken word reads worse than a long one. Returns nil for an empty
// sentence, which is what marks an entry bare.
func wrapSentence(s string, width int) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if width <= 0 {
		width = defaultAttentionWrapWidth
	}
	width -= AttentionIndentColumns
	if width < 1 {
		width = 1
	}
	var lines []string
	line := ""
	for word := range strings.FieldsSeq(s) {
		switch {
		case line == "":
			line = word
		case ansi.StringWidth(line)+1+ansi.StringWidth(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return append(lines, line)
}

// buildAttentionEntries converts issue-severity findings into sorted
// attentionEntry values, mirroring the entry construction + sort that
// injectAttentionSectionDetail renders. Sort: "!" (broken) before "~"
// (warning), stable otherwise — the SAME order both the renderer and the
// prepend-count calculation must observe, so they extract from this one
// function rather than deriving the order independently in two places.
func buildAttentionEntries(findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail, width int, notInspected bool) []attentionEntry {
	var entries []attentionEntry
	if notInspected {
		entries = append(entries, attentionEntry{
			tier:          "~",
			primary:       domain.NotInspectedPhrase,
			detailLines:   wrapSentence("The attention checks for this row did not answer (a cap or an API error), so its posture is unknown rather than clean.", width),
			splitKeyValue: true,
		})
	}
	for _, f := range findings {
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
		if attentionDetails != nil {
			if det, ok := attentionDetails[f.Code]; ok {
				rows = det.Rows
			}
		}
		entries = append(entries, attentionEntry{tier: tier, primary: f.Phrase, detailLines: wrapSentence(f.Detail, width), rows: rows, splitKeyValue: true})
	}
	if len(entries) == 0 {
		return entries
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].tier == "!" && entries[j].tier != "!"
	})
	return entries
}

// injectAttentionSectionDetail prepends the Attention block when the resource
// has issue-severity findings. It is the only place the block is built: the
// renderer paints the rows it emits.
func injectAttentionSectionDetail(items []fieldpath.FieldItem, ds *DetailState, td *resource.ResourceTypeDef, notInspected bool) []fieldpath.FieldItem {
	entries := buildAttentionEntries(ds.Findings, ds.AttentionDetails, ds.ViewportWidth, notInspected)
	if len(entries) == 0 {
		ds.AttentionPrepend = 0
		return items
	}
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
	// lastEntryBare tracks whether the final entry rendered no Detail sentence
	// and no AttentionDetail rows — its Phrase line is then the last line of
	// the block, and the trailing spacer below would read as a stray blank
	// Detail placeholder against it. Richer entries keep the spacer as the
	// separator from the identity fields that follow.
	lastEntryBare := false
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
		// S5 operator sentence, one item per wrapped line. The wrap lives here
		// rather than in the renderer so the body and the screen carry the same
		// rows.
		for _, dl := range e.detailLines {
			injected = append(injected, fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 1,
				Key:         dl,
				Value:       dl,
				Path:        "Attention",
				ColorTier:   entryColor,
			})
		}
		for _, row := range e.rows {
			tier := row.Tier
			if tier == "" {
				tier = e.tier
			}
			// A row with no label is a whole line rather than a labelled fact
			// — the "… +K more" row that closes a capped list is the only one
			// today. Key == Value is how this projection already asks the
			// renderer for a value-only line; leaving the key empty instead
			// would paint a bare ":" in front of it.
			key := row.Label
			if key == "" {
				key = row.Value
			}
			injected = append(injected, fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 3,
				Key:         key,
				Value:       row.Value,
				Path:        "Attention",
				ColorTier:   capTierToRowBucketDetail(tier, rowBucket),
			})
		}
		lastEntryBare = e.bare()
	}
	if !lastEntryBare {
		injected = append(injected, fieldpath.FieldItem{IsSpacer: true, Path: "Attention"})
	}
	ds.AttentionPrepend = len(injected)
	return append(injected, items...)
}

// capTierToRowBucketDetail returns the effective color tier for an Attention
// entry: "!" is permitted only on a row whose own S2 bucket is Broken, so the
// detail view never shows severity the list row did not. The glyph still
// carries severity; only the colour is capped.
func capTierToRowBucketDetail(tier string, rowBucket resource.Color) string {
	if tier == "!" && rowBucket != resource.ColorBroken {
		return "~"
	}
	return tier
}

// capitalizeFirstDetail uppercases the first rune for presentation; the
// underlying Finding.Phrase stays canonical lowercase.
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
			State:        row.State,
			Count:        row.Count,
			Loading:      row.Loading,
			Err:          row.Err != "",
			Truncated:    row.Truncated,
			FetchFilter:  row.FetchFilter,
			TargetType:   row.TargetType,
			Actionable:   isActionableDetailRow(row),
			CountDisplay: resource.FormatRelatedCount(row.State, row.Count, row.Truncated),
		})
	}
	return blocks
}
