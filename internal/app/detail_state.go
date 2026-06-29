package app

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

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
	// Mirror m.rightColVisible: true only when the user explicitly toggled ON.
	// hidden=true means the user acted; visible=true means they turned it on.
	if hidden {
		ds.RelatedUserVisible = visible
	}
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
