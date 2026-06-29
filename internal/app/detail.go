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
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/semantics/projection"
)

// detailPageSize is the default scroll jump for PageUp/PageDown on a detail screen
// when the renderer does not supply a viewport size via Action.N.
const detailPageSize = 10

// detailPageSizeFor returns the page size for a PageUp/PageDown action on a
// detail screen.
func detailPageSizeFor(a Action) int {
	if a.N > 0 {
		return a.N
	}
	return detailPageSize
}

// topDetailState returns the DetailState of the top-of-stack screen when the
// top screen is ScreenDetail, nil otherwise.
func (c *Controller) topDetailState() *DetailState {
	if len(c.stack) == 0 {
		return nil
	}
	top := c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenDetail {
		return nil
	}
	return c.stack[len(c.stack)-1].State.Detail
}

// ensureDetailState initialises the top detail screen's DetailState. It is a
// set-once operation: if DetailState is already non-nil the call is a no-op.
// Callers must hold c.mu (write).
func (c *Controller) ensureDetailState(res resource.Resource, resourceType string) {
	if len(c.stack) == 0 {
		return
	}
	top := &c.stack[len(c.stack)-1]
	if top.ID != runtime.ScreenDetail {
		return
	}
	if top.State.Detail == nil {
		ds := &DetailState{
			Resource:     res,
			ResourceType: resourceType,
			// Seed Findings with the resource's own (wave-1, fetcher-emitted)
			// findings so the Attention section shows them — mirrors the legacy
			// detail, whose injectAttentionSection read m.res.Findings. Wave-2
			// enrichment findings are merged in later by applyFindingToState,
			// which strips only prior wave-2 entries and preserves these.
			Findings: append([]domain.Finding(nil), res.Findings...),
		}
		if len(res.AttentionDetails) > 0 {
			ds.AttentionDetails = maps.Clone(res.AttentionDetails)
		}
		top.State.Detail = ds
	}
}

// EnsureDetailState is the exported surface that TUI builders call immediately
// after pushing a Detail screen so that Snapshot().Body.Detail is non-nil from
// the first render. Delegates to ensureDetailState.
func (c *Controller) EnsureDetailState(res resource.Resource, resourceType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ensureDetailState(res, resourceType)
}

// attentionPrependCount returns the number of items that injectAttentionSectionDetail
// would prepend for the given findings and attentionDetails. Mirrors the layout in
// injectAttentionSectionDetail: 1 section header + 1 entry per issue finding +
// len(rows) per entry + 1 spacer. Returns 0 when there are no issue findings.
func attentionPrependCount(findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail) int {
	issueCount := 0
	rowCount := 0
	for _, fi := range findings {
		if !fi.Severity.IsIssue() {
			continue
		}
		issueCount++
		if attentionDetails != nil {
			if det, ok := attentionDetails[fi.Code]; ok {
				rowCount += len(det.Rows)
			}
		}
	}
	if issueCount == 0 {
		return 0
	}
	return 1 + issueCount + rowCount + 1 // header + entries + detail rows + spacer
}

// ApplyDetailFinding merges a wave-2 enrichment finding (and its optional
// AttentionDetail rows) into the top detail screen's DetailState. Strips any
// prior wave-2 finding for the same resource before appending the new one, so
// repeated calls replace rather than accumulate. A nil finding clears wave-2
// data. No-op when the top screen is not ScreenDetail.
//
// Cursor stability: mirrors DetailModel.SetEnrichmentFinding — computes the old
// and new attention-prepend sizes and adjusts FieldCursor by the delta so that
// the cursor continues to point at the same logical field after the Attention
// block is injected or removed.
func (c *Controller) ApplyDetailFinding(f *domain.Finding, ad *domain.AttentionDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	c.applyFindingToState(ds, f, ad)
}

// ApplyDetailFindingForResource applies a wave-2 finding to the detail screen in
// the stack whose resource matches (resourceType, resourceID), even when it is
// NOT the top screen. Enrichment results arrive while a different detail may be
// active, so the finding must reach the matching STACKED detail.
// No-op when no stacked detail matches.
func (c *Controller) ApplyDetailFindingForResource(resourceType, resourceID string, f *domain.Finding, ad *domain.AttentionDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.Resource.ID != resourceID || ds.ResourceType != resourceType {
			continue
		}
		c.applyFindingToState(ds, f, ad)
	}
}

// ApplyDetailEnrichmentForResource applies a completed detail-enrichment result
// to the detail screen(s) whose resource matches (resourceType, resourceID),
// even when stacked beneath the active screen. It replaces ds.Resource with the
// enriched resource — detail enrichers (e.g. IAM policy/role-policy) put the
// fetched document into RawStruct, so without this the field projection and
// subsequent YAML/JSON opens keep showing the pre-enrichment resource — and then
// applies the wave-2 finding. No-op when no stacked detail matches.
func (c *Controller) ApplyDetailEnrichmentForResource(resourceType, resourceID string, enriched resource.Resource, f *domain.Finding, ad *domain.AttentionDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.Resource.ID != resourceID || ds.ResourceType != resourceType {
			continue
		}
		ds.Resource = enriched
		c.applyFindingToState(ds, f, ad)
	}
}

// applyFindingToState merges (or clears, when f is nil) a wave-2 enrichment
// finding on the given DetailState, adjusting FieldCursor for the change in the
// attention-prepend size. Callers must hold c.mu (write).
func (c *Controller) applyFindingToState(ds *DetailState, f *domain.Finding, ad *domain.AttentionDetail) {
	// Capture old prepend size before stripping, so the cursor delta can be computed.
	oldPrepend := attentionPrependCount(ds.Findings, ds.AttentionDetails)

	// Strip prior wave-2 findings (same strip semantics as DetailModel.SetEnrichmentFinding).
	if len(ds.Findings) > 0 {
		kept := ds.Findings[:0:0]
		for _, fi := range ds.Findings {
			if strings.HasPrefix(string(fi.Source), "wave2:") {
				if ds.AttentionDetails != nil {
					delete(ds.AttentionDetails, fi.Code)
				}
				continue
			}
			kept = append(kept, fi)
		}
		ds.Findings = kept
	}

	if f != nil && f.Phrase != "" {
		finding := *f
		if !strings.HasPrefix(string(finding.Source), "wave2:") {
			finding.Source = "wave2:controller"
		}
		ds.Findings = append(ds.Findings, finding)
		if ad != nil && len(ad.Rows) > 0 {
			if ds.AttentionDetails == nil {
				ds.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
			}
			ds.AttentionDetails[finding.Code] = *ad
		}
	}

	// Adjust FieldCursor by the change in attention-prepend size so the cursor
	// continues to point at the same logical field (mirrors SetEnrichmentFinding's
	// snapshot/relocate sequence).
	//
	// The TUI's SetEnrichmentFinding only relocates the cursor when haveSnapshot=true,
	// which requires the pre-injection fieldList to be non-empty and the cursor to
	// point at a non-Attention item. This means:
	//   1. If cursor was inside the old attention block (< oldPrepend): reset to 0.
	//   2. If cursor was in content (>= oldPrepend): shift by delta, but only when
	//      content items actually exist after injection — mirrors haveSnapshot=false
	//      for resources with no content fields (empty resource).
	newPrepend := attentionPrependCount(ds.Findings, ds.AttentionDetails)
	delta := newPrepend - oldPrepend
	if delta != 0 {
		if ds.FieldCursor < oldPrepend {
			// Cursor was inside the old attention block — land on new section header.
			ds.FieldCursor = 0
		} else {
			// Cursor was pointing at a content item; shift it to track the same item
			// in the new layout. Skip if no content exists beyond the attention block
			// (empty resource case), matching SetEnrichmentFinding's haveSnapshot=false.
			adjusted := ds.FieldCursor - oldPrepend + newPrepend
			newTotalItems := len(buildDetailFieldItems(ds, c.viewConfig))
			if adjusted < newTotalItems {
				ds.FieldCursor = adjusted
			}
			// else: only attention items, no content — cursor stays at 0.
		}
	}
}

// ApplyDetailRelated replaces the RelatedRows slice on the top detail screen's
// DetailState. Used for bulk updates (e.g. cache-hit replay). No-op when the
// top screen is not ScreenDetail.
func (c *Controller) ApplyDetailRelated(rows []DetailRelatedRow) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	ds.RelatedRows = rows
}

// ApplyDetailRelatedResult merges one checker result into the top detail
// screen's DetailState.RelatedRows, matching by DefDisplayName (identical to
// rightColumnModel.Update semantics). If no row with that DisplayName exists,
// the result is appended. No-op when the top screen is not ScreenDetail.
func (c *Controller) ApplyDetailRelatedResult(displayName, targetType string, count int, loading bool, errMsg string, approximate bool, fetchFilter map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	// Find existing row by DisplayName.
	for i := range ds.RelatedRows {
		if ds.RelatedRows[i].DisplayName == displayName {
			ds.RelatedRows[i].Count = count
			ds.RelatedRows[i].Loading = loading
			ds.RelatedRows[i].Err = errMsg
			ds.RelatedRows[i].Approximate = approximate
			ds.RelatedRows[i].FetchFilter = fetchFilter
			return
		}
	}
	// Not found — append new row.
	ds.RelatedRows = append(ds.RelatedRows, DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: displayName,
		Count:       count,
		Loading:     loading,
		Err:         errMsg,
		Approximate: approximate,
		FetchFilter: fetchFilter,
	})
}

// InitDetailRelatedRows initialises the RelatedRows slice from registered
// related defs for the resource type, setting all rows to loading state.
// This mirrors what newRightColumn() does in the TUI on SetSize. Must be
// called after EnsureDetailState when the type has related defs, so that
// Snapshot().Body.Detail.Related shows loading rows immediately.
// No-op when the top screen is not ScreenDetail or rows are already set.
func (c *Controller) InitDetailRelatedRows(resourceType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initDetailRelatedRows(resourceType)
}

// initDetailRelatedRows is the lock-free implementation of InitDetailRelatedRows.
// Callers must hold c.mu (write).
func (c *Controller) initDetailRelatedRows(resourceType string) {
	ds := c.topDetailState()
	if ds == nil || len(ds.RelatedRows) > 0 {
		return
	}
	defs := resource.GetRelated(resourceType)
	if len(defs) == 0 {
		return
	}
	rows := make([]DetailRelatedRow, 0, len(defs))
	for _, def := range defs {
		rows = append(rows, DetailRelatedRow{
			TargetType:  def.TargetType,
			DisplayName: def.DisplayName,
			Count:       -1,
			Loading:     true,
		})
	}
	ds.RelatedRows = rows
	// A populated related panel is visible; reflect it in the stored flag so
	// focus/filter/cursor actions (which gate on RelatedVisible) take effect.
	// A narrow terminal overrides this via SetDetailRelatedVisible(false, …).
	ds.RelatedVisible = true
}

// SetDetailRelatedVisible sets the RelatedVisible and RelatedHidden flags on
// the top detail screen directly. Used by the TUI when it has already computed
// the desired state from local flags (e.g. auto-show vs explicit toggle) and
// needs the controller to reflect that state. Setting hidden=true suppresses
// the auto-show logic in buildDetailBody for the lifetime of the screen.
// No-op when the top screen is not ScreenDetail.
func (c *Controller) SetDetailRelatedVisible(visible, hidden bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	ds.RelatedVisible = visible
	ds.RelatedHidden = hidden
	if !visible {
		ds.RelatedFocus = false
	}
}

// ResetDetailRelatedRows unconditionally resets RelatedRows to loading state
// from the registered related defs, discarding any loaded counts. Called by
// handleRefresh so stale counts are cleared before the new checker results
// arrive — mirrors ResetRightColumn() on the TUI side.
// No-op when the top screen is not ScreenDetail.
func (c *Controller) ResetDetailRelatedRows(resourceType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	defs := resource.GetRelated(resourceType)
	if len(defs) == 0 {
		ds.RelatedRows = nil
		return
	}
	rows := make([]DetailRelatedRow, 0, len(defs))
	for _, def := range defs {
		rows = append(rows, DetailRelatedRow{
			TargetType:  def.TargetType,
			DisplayName: def.DisplayName,
			Count:       -1,
			Loading:     true,
		})
	}
	ds.RelatedRows = rows
	// A populated related panel is visible; reflect it in the stored flag so
	// focus/filter/cursor actions (which gate on RelatedVisible) take effect.
	// A narrow terminal overrides this via SetDetailRelatedVisible(false, …).
	ds.RelatedVisible = true
}

// DetailFrameTitle returns the frame-border title for the top detail screen.
// Returns an empty string when the top screen is not a detail screen.
func (c *Controller) DetailFrameTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.detailFrameTitleLocked()
}

// detailFrameTitleLocked computes the detail frame title. The caller MUST hold
// c.mu — snapshot() calls this while Apply already holds the write lock, so
// taking the lock here would deadlock (RWMutex is not reentrant).
func (c *Controller) detailFrameTitleLocked() string {
	ds := c.topDetailState()
	if ds == nil {
		return ""
	}
	if ds.Resource.Name != "" {
		return ds.Resource.Name
	}
	return ds.Resource.ID
}

// applyDetailActions handles detail-screen-specific action kinds within
// applyLocked. Returns (snapshot, tasks, handled). If handled is false the
// caller should continue to the next action group.
func (c *Controller) applyDetailActions(a Action) (ViewState, []runtime.TaskRequest, bool) {
	ds := c.topDetailState()
	if ds == nil {
		return ViewState{}, nil, false
	}

	switch a.Kind {
	case ActionMoveUp:
		// Field cursor moves up in the left column; scroll up when right-focused.
		if !ds.RelatedFocus {
			if ds.FieldCursor > 0 {
				ds.FieldCursor--
				// Skip section headers and spacers — mirrors the TUI legacy path.
				items := buildDetailFieldItems(ds, c.viewConfig)
				for ds.FieldCursor > 0 && ds.FieldCursor < len(items) &&
					(items[ds.FieldCursor].IsSection || items[ds.FieldCursor].IsSpacer) {
					ds.FieldCursor--
				}
			}
		} else {
			if ds.RelatedCursor > 0 {
				ds.RelatedCursor--
			}
		}
		return c.snapshot(), nil, true

	case ActionMoveDown:
		if !ds.RelatedFocus {
			items := buildDetailFieldItems(ds, c.viewConfig)
			fieldCount := len(items)
			if ds.FieldCursor < fieldCount-1 {
				ds.FieldCursor++
				// Skip section headers and spacers — mirrors the TUI legacy path.
				for ds.FieldCursor < fieldCount-1 &&
					(items[ds.FieldCursor].IsSection || items[ds.FieldCursor].IsSpacer) {
					ds.FieldCursor++
				}
			}
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			if ds.RelatedCursor < relatedCount-1 {
				ds.RelatedCursor++
			}
		}
		return c.snapshot(), nil, true

	case ActionMoveTop:
		if !ds.RelatedFocus {
			ds.FieldCursor = 0
			ds.ScrollY = 0
		} else {
			ds.RelatedCursor = 0
			ds.RelatedScroll = 0
		}
		return c.snapshot(), nil, true

	case ActionMoveBottom:
		if !ds.RelatedFocus {
			fieldCount := c.detailFieldCount(ds)
			if fieldCount > 0 {
				ds.FieldCursor = fieldCount - 1
			}
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			if relatedCount > 0 {
				ds.RelatedCursor = relatedCount - 1
			}
		}
		return c.snapshot(), nil, true

	case ActionPageUp:
		if !ds.RelatedFocus {
			ds.ScrollY -= detailPageSizeFor(a)
			if ds.ScrollY < 0 {
				ds.ScrollY = 0
			}
		} else {
			ds.RelatedScroll -= detailPageSizeFor(a)
			if ds.RelatedScroll < 0 {
				ds.RelatedScroll = 0
			}
		}
		return c.snapshot(), nil, true

	case ActionPageDown:
		if !ds.RelatedFocus {
			ds.ScrollY += detailPageSizeFor(a)
		} else {
			relatedCount := c.detailRelatedVisibleCount(ds)
			ds.RelatedScroll += detailPageSizeFor(a)
			if ds.RelatedScroll >= relatedCount {
				ds.RelatedScroll = max(relatedCount-1, 0)
			}
		}
		return c.snapshot(), nil, true

	case ActionToggleWrap:
		ds.Wrap = !ds.Wrap
		return c.snapshot(), nil, true

	case ActionSearch:
		ds.SearchQuery = a.Arg
		ds.SearchCursor = 0
		return c.snapshot(), nil, true

	case ActionSearchNext:
		if ds.SearchQuery != "" {
			ds.SearchCursor++
		}
		return c.snapshot(), nil, true

	case ActionSearchPrev:
		if ds.SearchQuery != "" && ds.SearchCursor > 0 {
			ds.SearchCursor--
		}
		return c.snapshot(), nil, true

	case ActionSearchClear:
		ds.SearchQuery = ""
		ds.SearchCursor = 0
		return c.snapshot(), nil, true

	case ActionToggleRelated:
		ds.RelatedVisible = !ds.RelatedVisible
		// Once the user toggles, RelatedHidden gates auto-show so the panel
		// stays hidden across resizes when the user has turned it off.
		ds.RelatedHidden = true
		if !ds.RelatedVisible {
			ds.RelatedFocus = false
		}
		return c.snapshot(), nil, true

	case ActionToggleFocus:
		// Tab: toggle focus between left (field) column and right (related) column.
		// Only effective when the related panel is visible and has actionable rows.
		if ds.RelatedVisible {
			ds.RelatedFocus = !ds.RelatedFocus
		}
		return c.snapshot(), nil, true

	case ActionSetFilter:
		// Related-panel filter. The renderer only emits ActionSetFilter for the
		// related panel while it is focused, so we trust that intent rather than
		// re-checking ds.RelatedFocus (which can lag the renderer's focus state).
		if ds.RelatedVisible {
			ds.RelatedFilter = a.Arg
			ds.RelatedFilterActive = a.Arg != ""
			ds.RelatedCursor = 0
			ds.RelatedScroll = 0
		}
		return c.snapshot(), nil, true
	}

	return ViewState{}, nil, false
}

// detailFieldCount returns the number of field items for the given DetailState's
// resource by running the projector pipeline. Used by cursor clamping in
// applyDetailActions without building a full body.
func (c *Controller) detailFieldCount(ds *DetailState) int {
	return len(buildDetailFieldItems(ds, c.viewConfig))
}

// detailRelatedVisibleCount returns the number of visible related rows after
// applying the current filter.
func (c *Controller) detailRelatedVisibleCount(ds *DetailState) int {
	query := strings.TrimSpace(strings.ToLower(ds.RelatedFilter))
	count := 0
	for _, row := range ds.RelatedRows {
		if isSelfPivotZeroDetailRow(row, ds.ResourceType) {
			continue
		}
		if query == "" || strings.Contains(strings.ToLower(row.DisplayName), query) {
			count++
		}
	}
	return count
}

// isSelfPivotZeroDetailRow mirrors rightColumnModel.isSelfPivotZeroRow for
// DetailRelatedRow values.
func isSelfPivotZeroDetailRow(row DetailRelatedRow, sourceType string) bool {
	return !row.Loading &&
		row.Err == "" &&
		row.Count == 0 &&
		sourceType != "" &&
		row.TargetType == sourceType
}

// isActionableDetailRow delegates to the single shared predicate
// resource.IsRelatedActionable so the actionability rule is defined once.
func isActionableDetailRow(row DetailRelatedRow) bool {
	return resource.IsRelatedActionable(row.Count, row.Approximate, len(row.FetchFilter) > 0, row.Loading, row.Err != "")
}

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
			Name:        row.DisplayName,
			Count:       row.Count,
			Loading:     row.Loading,
			Err:         row.Err != "",
			Approximate: row.Approximate,
			FetchFilter: row.FetchFilter,
			TargetType:   row.TargetType,
			Actionable:   resource.IsRelatedActionable(row.Count, row.Approximate, len(row.FetchFilter) > 0, row.Loading, row.Err != ""),
			CountDisplay: resource.FormatRelatedCount(row.Count),
		})
	}
	return blocks
}
