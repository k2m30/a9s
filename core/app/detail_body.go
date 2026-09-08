// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/projection"
)

// detailLayout is what one body build observed about the layout the operator
// is looking at: the identity of the item under the cursor and the size of
// the Attention block that cursor was indexed against. A build returns it
// rather than writing it, so the one caller that renders — snapshot() — is
// the one that records it, and the several callers that build a body for
// something else (a copy, a navigation lookup) cannot overwrite the record
// for a layout nobody saw.
type detailLayout struct {
	CursorKey string
	Prepend   int
}

// buildDetailBody constructs a DetailBody from a DetailState, mirroring the
// data that DetailModel.View() + renderFromFieldList() consume. The body is
// renderer-agnostic: scroll, width, and height remain owned by the renderer.
// The projection config and the "could not inspect this row" fact are both
// read from the controller, so no caller has to carry them.
//
// The returned detailLayout describes THIS build; see DetailState.cursorLayout
// for who stores it.
func (c *Controller) buildDetailBody(ds *DetailState) (*DetailBody, detailLayout) {
	built := c.buildDetailFieldItems(ds)
	items := built.items

	// Convert []fieldpath.FieldItem → []FieldRow for the body.
	fields := fieldItemsToFieldRows(items)

	// Build RelatedBlocks. When ds.RelatedRows is nil/empty but the resource
	// type has registered related defs, synthesise loading-state blocks from
	// those defs. This mirrors what newRightColumn() does in the TUI: it creates
	// one row per RelatedDef with count=-1/loading=true so the panel renders
	// its "loading" state immediately on push, before checker results arrive.
	related := buildDetailRelatedBlocks(ds)
	if len(related) == 0 && len(ds.RelatedRows) == 0 {
		related = buildDetailRelatedLoadingBlocks(ds.ResourceType)
	}

	fc := selectableRowAtOrAbove(items, ds.FieldCursor)

	// Name the row the cursor is on. This build is the layout the operator
	// sees, so it is the only honest place to take that identity from.
	layout := detailLayout{Prepend: built.prepend}
	if fc < len(built.keys) {
		layout.CursorKey = built.keys[fc]
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
	}, layout
}

// selectableRowAtOrAbove returns the index at or above i of the first row a
// cursor can rest on, clamping i into the list first. A spacer is a blank line
// and a section header is a label, so neither is a place to leave a cursor:
// the Attention block ends in a spacer, and on a resource whose projection
// yields no content rows that spacer is the last row. The jump to the bottom
// and the body's clamp both land through here, so the row the move chooses and
// the row the screen shows cannot be different rows.
func selectableRowAtOrAbove(items []fieldpath.FieldItem, i int) int {
	if i >= len(items) {
		i = len(items) - 1
	}
	for i > 0 && (items[i].IsSection || items[i].IsSpacer) {
		i--
	}
	if i < 0 {
		return 0
	}
	return i
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

// detailItems is one build of the detail field list: the items the renderer
// paints, one stable identity per item, and the number of items the Attention
// block contributed at the front. keys is index-aligned with items — an
// Attention row's identity is not the text painted on it (two findings can
// wrap to the same sentence), so it cannot be recovered from the item later.
type detailItems struct {
	items   []fieldpath.FieldItem
	keys    []string
	prepend int
}

// buildDetailFieldItems runs the projector pipeline (projection.buildItems,
// core/semantics/projection/generic.go) and returns the []fieldpath.FieldItem
// that both the TUI renderer and buildDetailBody consume, alongside the
// identities the cursor is relocated by. Reads ds; writes nothing to it.
// c.viewConfig may be nil; projection.GenericWithConfig(nil) uses built-in
// defaults.
func (c *Controller) buildDetailFieldItems(ds *DetailState) detailItems {
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
	content := sectionsToFieldItemsDetail(sections)
	attention, keys := buildAttentionSectionDetail(ds, td, c.detailNotInspected(ds))
	built := detailItems{
		items:   append(attention, content...),
		keys:    keys,
		prepend: len(attention),
	}
	// A content row is identified by its path and its label, which together
	// survive the rebuild a finding or an enrichment triggers while a bare
	// index does not. One projection can repeat a pair — two targets of one
	// CloudTrail event, two identical lines of a raw YAML document — so a
	// repeat is numbered in projection order, which is as stable as the
	// projection itself.
	seenContent := make(map[string]int, len(content))
	for _, it := range content {
		key := it.Path + "\x1f" + it.Key
		if n := seenContent[key]; n > 0 {
			key = fmt.Sprintf("%s#%d", key, n)
		}
		seenContent[it.Path+"\x1f"+it.Key]++
		built.keys = append(built.keys, key)
	}
	return built
}

// relocateDetailCursor returns the index the cursor takes in a freshly built
// layout: the row carrying the same identity, wherever the rebuild put it.
// Both cursor cases go through here — an Attention entry that sorted down
// under a more severe finding, and a content field the block pushed along —
// so neither can be relocated by a rule the other does not use. When the row
// is gone (its finding cleared), the cursor keeps its distance below the
// Attention block instead.
func relocateDetailCursor(was detailLayout, cursor int, built detailItems) int {
	if was.CursorKey != "" {
		if i := slices.Index(built.keys, was.CursorKey); i >= 0 {
			return i
		}
	}
	if adjusted := cursor - was.Prepend + built.prepend; adjusted >= 0 && adjusted < len(built.items) {
		return adjusted
	}
	return cursor
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
// buildAttentionEntries and rendered by buildAttentionSectionDetail, which is
// also what reports the block's size. The cursor relocation in
// applyFindingToState reads that report, so no second walk of these entries
// can disagree with the layout on screen about count, order or bareness.
type attentionEntry struct {
	tier string
	// code is the finding this entry came from, and the stem of every one of
	// its rows' identities. The prose is not usable for that: two findings
	// can carry the same Detail sentence word for word. A resource can report
	// two findings under one code, so a repeat carries the ordinal of the
	// order it was REPORTED in — the sort that follows moves entries, and a
	// number that meant "where you are in the block" would be exchanged
	// between the two the moment one outranked the other.
	code    string
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
// buildAttentionSectionDetail renders. Sort: "!" (broken) before "~"
// (warning), stable otherwise — the SAME order both the renderer and the
// prepend-count calculation must observe, so they extract from this one
// function rather than deriving the order independently in two places.
func buildAttentionEntries(findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail, width int, notInspected bool) []attentionEntry {
	var entries []attentionEntry
	if notInspected {
		entries = append(entries, attentionEntry{
			tier:          "~",
			code:          "not-inspected",
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
		entries = append(entries, attentionEntry{tier: tier, code: string(f.Code), primary: f.Phrase, detailLines: wrapSentence(f.Detail, width), rows: rows, splitKeyValue: true})
	}
	if len(entries) == 0 {
		return entries
	}
	seen := make(map[string]int, len(entries))
	for i := range entries {
		code := entries[i].code
		if n := seen[code]; n > 0 {
			entries[i].code = fmt.Sprintf("%s#%d", code, n)
		}
		seen[code]++
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].tier == "!" && entries[j].tier != "!"
	})
	return entries
}

// buildAttentionSectionDetail builds the Attention block for a resource with
// issue-severity findings, and one identity per row it emits. It is the only
// place the block is built: the renderer paints the rows it returns. Returns
// nil, nil when there is nothing to attend to.
func buildAttentionSectionDetail(ds *DetailState, td *resource.ResourceTypeDef, notInspected bool) ([]fieldpath.FieldItem, []string) {
	entries := buildAttentionEntries(ds.Findings, ds.AttentionDetails, ds.ViewportWidth, notInspected)
	if len(entries) == 0 {
		return nil, nil
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
	keys := make([]string, 0, cap(injected))
	emit := func(item fieldpath.FieldItem, key string) {
		injected = append(injected, item)
		keys = append(keys, key)
	}
	// The header's line counts the findings, so its text changes whenever the
	// count does; its identity does not, and a cursor parked on it stays on it.
	emit(fieldpath.FieldItem{
		IsSection: true,
		Key:       fmt.Sprintf("Attention (%d)", len(entries)),
		Path:      "Attention",
		ColorTier: capTierToRowBucketDetail(headerTier, rowBucket),
	}, "attention:header")
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
		stem := "attention:" + e.code
		emit(fieldpath.FieldItem{
			IsSubField:  true,
			IndentLevel: 1,
			Key:         itemKey,
			Value:       itemValue,
			Path:        "Attention",
			ColorTier:   entryColor,
		}, stem)
		// S5 operator sentence, one item per wrapped line. The wrap lives here
		// rather than in the renderer so the body and the screen carry the same
		// rows.
		for i, dl := range e.detailLines {
			emit(fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 1,
				Key:         dl,
				Value:       dl,
				Path:        "Attention",
				ColorTier:   entryColor,
			}, fmt.Sprintf("%s:sentence:%d", stem, i))
		}
		for i, row := range e.rows {
			tier := row.Tier
			if tier == "" {
				tier = e.tier
			}
			// A row with no label is a whole line rather than a labelled fact
			// — the "… +K more" row that closes a capped list is the only one
			// today. The empty label is passed through: how a row without one
			// is painted is the renderer's to decide.
			emit(fieldpath.FieldItem{
				IsSubField:  true,
				IndentLevel: 3,
				Key:         row.Label,
				Value:       row.Value,
				Path:        "Attention",
				ColorTier:   capTierToRowBucketDetail(tier, rowBucket),
			}, fmt.Sprintf("%s:row:%d", stem, i))
		}
		lastEntryBare = e.bare()
	}
	if !lastEntryBare {
		emit(fieldpath.FieldItem{IsSpacer: true, Path: "Attention"}, "attention:spacer")
	}
	return injected, keys
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
