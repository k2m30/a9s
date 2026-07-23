// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"
	"strings"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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
// injectAttentionSectionDetail: 1 section header + per issue-severity finding
// (1 phrase line + 1 Detail line when Finding.Detail != "" + len(rows)) + a
// trailing spacer, UNLESS the last issue-severity finding is "bare" (no Detail,
// no rows) — injectAttentionSectionDetail omits the spacer in that case so it
// does not sit directly against a bare Phrase line. Returns 0 when there are no
// issue findings.
//
// Single source of truth: entries are built via buildAttentionEntries (same
// helper injectAttentionSectionDetail renders from), so "last entry" here
// means the same SORTED last entry the renderer actually emits — not the
// last entry in the original findings order. Deriving lastEntryBare from the
// unsorted order previously caused a prepend-count mismatch (see
// tests/unit/app_detail_attention_cursor_test.go), shifting FieldCursor by
// the wrong delta after a mixed-severity Attention re-sort.
func attentionPrependCount(findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail) int {
	entries := buildAttentionEntries(findings, attentionDetails)
	if len(entries) == 0 {
		return 0
	}
	entryLineCount := 0
	lastEntryBare := false
	for _, e := range entries {
		entryLineCount++ // phrase line
		if e.detail != "" {
			entryLineCount++
		}
		entryLineCount += len(e.rows)
		lastEntryBare = e.bare()
	}
	spacer := 1
	if lastEntryBare {
		spacer = 0
	}
	return 1 + entryLineCount + spacer // header + entry lines (+ detail + rows) + spacer
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
	c.applyFindingToState(ds, singleFindingSlice(f), singleAttentionDetailMap(f, ad))
}

// singleFindingSlice converts a single *domain.Finding into the
// []domain.Finding slice applyFindingToState's plural contract needs, for
// callers (ApplyDetailFinding, applyDetailFindingForResource,
// ApplyDetailEnrichmentForResource) that still carry only one finding at a
// time. Returns nil (clear-only) for a nil finding or an empty Phrase —
// mirrors applyFindingToState's own former f != nil && f.Phrase != "" guard.
func singleFindingSlice(f *domain.Finding) []domain.Finding {
	if f == nil || f.Phrase == "" {
		return nil
	}
	return []domain.Finding{*f}
}

// singleAttentionDetailMap converts a single *domain.AttentionDetail paired
// with its owning finding's Code into the per-Code map applyFindingToState's
// plural contract needs. Returns nil when there is no finding, no
// AttentionDetail, or no rows to carry.
func singleAttentionDetailMap(f *domain.Finding, ad *domain.AttentionDetail) map[domain.FindingCode]domain.AttentionDetail {
	if f == nil || ad == nil || len(ad.Rows) == 0 {
		return nil
	}
	return map[domain.FindingCode]domain.AttentionDetail{f.Code: *ad}
}

// ApplyDetailFindingForResource applies a wave-2 finding to the detail screen in
// the stack whose resource matches (resourceType, resourceID), even when it is
// NOT the top screen. Enrichment results arrive while a different detail may be
// active, so the finding must reach the matching STACKED detail.
// No-op when no stacked detail matches.
func (c *Controller) ApplyDetailFindingForResource(resourceType, resourceID string, f *domain.Finding, ad *domain.AttentionDetail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.applyDetailFindingForResource(resourceType, resourceID, f, ad)
}

// applyDetailFindingForResource is the lock-free implementation of
// ApplyDetailFindingForResource. Callers must hold c.mu (write).
func (c *Controller) applyDetailFindingForResource(resourceType, resourceID string, f *domain.Finding, ad *domain.AttentionDetail) {
	c.applyDetailFindingsForResource(resourceType, resourceID, singleFindingSlice(f), singleAttentionDetailMap(f, ad))
}

// applyDetailFindingsForResource applies EVERY finding in findings (each
// looking up its own AttentionDetail in attentionDetails by its Code) to the
// detail screen(s) in the stack matching (resourceType, resourceID) — the
// plural counterpart of applyDetailFindingForResource. Used by the PatchDetail
// intent case so an already-open detail's Attention block shows every
// independently-evaluated Wave-2 finding, not just a single worst-severity
// representative. No-op when no stacked detail matches.
func (c *Controller) applyDetailFindingsForResource(resourceType, resourceID string, findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail) {
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.Resource.ID != resourceID || ds.ResourceType != resourceType {
			continue
		}
		c.applyFindingToState(ds, findings, attentionDetails)
	}
}

// ClearDetailFindingsForType clears wave-2 enrichment findings from every stacked
// detail screen whose resource type matches resourceType. Called when enrichment
// returns no findings for the entire type (nil EnrichmentFindings in PatchDetail).
func (c *Controller) ClearDetailFindingsForType(resourceType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clearDetailFindingsForType(resourceType)
}

// clearDetailFindingsForType is the lock-free implementation of
// ClearDetailFindingsForType. Callers must hold c.mu (write).
func (c *Controller) clearDetailFindingsForType(resourceType string) {
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.ResourceType != resourceType {
			continue
		}
		c.applyFindingToState(ds, nil, nil)
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
	c.applyDetailEnrichmentForResourceLocked(resourceType, resourceID, enriched, f, ad)
}

// applyDetailEnrichmentForResourceLocked is the lock-free implementation of
// ApplyDetailEnrichmentForResource, shared with foldEnrichDetailResultLocked
// (both already run under c.mu). Callers must hold c.mu (write).
func (c *Controller) applyDetailEnrichmentForResourceLocked(resourceType, resourceID string, enriched resource.Resource, f *domain.Finding, ad *domain.AttentionDetail) {
	// Universal rule 7 / S5: every issue-severity finding the enricher found
	// must reach ds.Findings, not just the caller-folded (f, ad) pair —
	// primaryWave2Finding only recognizes "wave2:"-sourced findings and folds
	// them to a single worst-severity one, so a wave1-sourced finding set an
	// on-demand DetailEnrich computes (e.g. transfer_children.go's
	// cert-expiry checks) would otherwise never reach an already-open
	// detail. applyFindingToState already re-tags every non-"wave2:" finding
	// as "wave2:controller" before appending, and sorts Broken before
	// Warning when the Attention block renders — both reused unchanged here.
	fallbackFindings := singleFindingSlice(f)
	attentionDetails := singleAttentionDetailMap(f, ad)
	if len(enriched.AttentionDetails) > 0 {
		attentionDetails = enriched.AttentionDetails
	}
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		ds := c.stack[i].State.Detail
		if ds == nil || ds.Resource.ID != resourceID || ds.ResourceType != resourceType {
			continue
		}
		findings := fallbackFindings
		if len(enriched.Findings) > 0 {
			// Some DetailEnrichers (enrichPolicy/enrichRolePolicy) pass the
			// resource's own wave1 Findings through unchanged in
			// enriched.Findings — filter those back out, or the
			// strip-then-append cycle below would re-append and retag a
			// duplicate of an already-seeded finding on every enrichment
			// cycle (e.g. a policy's orphan/unattached finding, re-opened
			// each time the detail re-enriches).
			findings = newlyReportedFindings(ds.Findings, enriched.Findings)
		}
		ds.Resource = enriched
		c.applyFindingToState(ds, findings, attentionDetails)
	}
}

// primaryWave2Finding extracts the WORST-severity wave2 Finding (and its
// companion AttentionDetail, if present) from r.Findings / r.AttentionDetails
// for wiring into applyDetailEnrichmentForResourceLocked, whose signature
// carries exactly one Finding + one AttentionDetail. Both return values are
// nil when no wave2 finding is present (detail view shows no Attention
// section).
//
// This duplicates internal/tui/app_enrich_fold.go's identical helper rather
// than sharing it: core/app cannot import internal/tui (wrong layering
// direction — the TUI adapter imports core/app, not the reverse). Both
// copies must stay in sync if this logic ever changes.
func primaryWave2Finding(r resource.Resource) (*domain.Finding, *domain.AttentionDetail) {
	var wave2 []domain.Finding
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			wave2 = append(wave2, f)
		}
	}
	if len(wave2) == 0 {
		return nil, nil
	}
	finding := domain.WorstSeverityFinding(wave2)
	var ad *domain.AttentionDetail
	if r.AttentionDetails != nil {
		if got, ok := r.AttentionDetails[finding.Code]; ok && len(got.Rows) > 0 {
			adVal := got
			ad = &adVal
		}
	}
	return &finding, ad
}

// FoldEnrichDetailResult folds a completed on-demand detail-enrichment
// result into detail state. Callers must have already applied the
// OperationID acceptance check (messages.IsStale(msg, core) against
// messages.AspectDetailOp) — this method applies unconditionally, mirroring
// every other GenStamped case in handle.go.
//
// Calls Core.HandleEnrichDetailResult for the intents/tasks (a FlashIntent on
// error, nothing on success) and, only on success, folds the enriched
// resource + its worst-severity wave2 finding into every matching stacked
// detail screen (applyDetailEnrichmentForResourceLocked). On error, returns
// without touching detail state — a half-populated EnrichedRes must never
// reach the detail merge.
//
// Also regenerates an open YAML/JSON text screen for this resource
// (regenerateTextScreenLocked) using the same neutral (uncolored)
// resourceYAMLLines/resourceJSONLines helpers navigate.go's PushYAML/PushJSON
// branches use — the web/headless lane's own single source of text-viewer
// content, independent of the TUI's syntax-colored regeneration.
//
// Callers must hold c.mu (write).
func (c *Controller) foldEnrichDetailResultLocked(msg messages.EnrichDetailResult) ([]runtime.UIIntent, []runtime.TaskRequest) {
	intents, tasks := c.core.HandleEnrichDetailResult(runtime.EnrichDetailResultEvent{
		ResourceType: msg.ResourceType,
		Err:          msg.Err,
	})
	if msg.Err != nil {
		return intents, tasks
	}
	ef, ad := primaryWave2Finding(msg.EnrichedRes)
	c.applyDetailEnrichmentForResourceLocked(msg.ResourceType, msg.ResourceID, msg.EnrichedRes, ef, ad)
	c.regenerateTextScreenLocked(msg.ResourceType, msg.ResourceID, msg.EnrichedRes)
	return intents, tasks
}

// regenerateTextScreenLocked replaces the top text screen's (YAML/JSON)
// Lines and Resource with content regenerated from the enriched resource,
// when the top screen is a text viewer whose ScreenContext matches
// (resourceType, resourceID). No-op otherwise (wrong screen, wrong resource,
// or TextState not yet initialized).
//
// Only Lines and Resource are touched — Search/SearchCursor/Wrap/ScrollY are
// left untouched, exactly like UpdateTextLines/SetTextResource, so an
// enrichment landing mid-search or mid-scroll does not reset either.
//
// Callers must hold c.mu (write).
func (c *Controller) regenerateTextScreenLocked(resourceType, resourceID string, enriched resource.Resource) {
	ts := c.topTextState()
	if ts == nil {
		return
	}
	top := c.stack[len(c.stack)-1]
	if top.Ctx.ResourceType != resourceType || top.Ctx.ResourceID != resourceID {
		return
	}
	switch top.ID {
	case runtime.ScreenYAML:
		ts.Lines = resourceYAMLLines(enriched)
	case runtime.ScreenJSON:
		ts.Lines = resourceJSONLines(enriched)
	default:
		return
	}
	ts.Resource = enriched
}

// newlyReportedFindings returns the subset of candidates whose Code is not
// already present among current's non-"wave2:"-tagged ("seed") entries — the
// pre-enrichment resource's own wave1 findings, which applyFindingToState's
// strip-then-append cycle always keeps untouched across repeated calls.
func newlyReportedFindings(current, candidates []domain.Finding) []domain.Finding {
	seedCodes := make(map[domain.FindingCode]bool, len(current))
	for _, fi := range current {
		if !strings.HasPrefix(string(fi.Source), "wave2:") {
			seedCodes[fi.Code] = true
		}
	}
	out := make([]domain.Finding, 0, len(candidates))
	for _, fi := range candidates {
		if !seedCodes[fi.Code] {
			out = append(out, fi)
		}
	}
	return out
}

// applyFindingToState merges (or clears, when findings is empty) wave-2
// enrichment findings on the given DetailState, adjusting FieldCursor for the
// change in the attention-prepend size. Strips every prior wave-2 finding
// FIRST, then appends the ENTIRE findings slice — a multi-condition resource's
// Attention block must show every independently-evaluated finding, not just
// one, so callers pass the full per-resource slice rather than looping this
// method once per finding (which would strip the previous iteration's
// just-appended entry). Callers must hold c.mu (write).
func (c *Controller) applyFindingToState(ds *DetailState, findings []domain.Finding, attentionDetails map[domain.FindingCode]domain.AttentionDetail) {
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

	for _, f := range findings {
		if f.Phrase == "" {
			continue
		}
		finding := f
		if !strings.HasPrefix(string(finding.Source), "wave2:") {
			finding.Source = "wave2:controller"
		}
		ds.Findings = append(ds.Findings, finding)
		if ad, ok := attentionDetails[finding.Code]; ok && len(ad.Rows) > 0 {
			if ds.AttentionDetails == nil {
				ds.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
			}
			ds.AttentionDetails[finding.Code] = ad
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

// ApplyDetailRelatedResultForResource merges one checker result into the RelatedRows
// of every stacked detail whose resource matches (sourceType, sourceID) — not just
// the top, since a check in flight can complete after the user has navigated to
// another detail. Applies to ALL matches rather than stopping at the first: a
// circular drill (e.g. bucket -> trail -> back to the same bucket) pushes a second
// ScreenDetail for the same (sourceType, sourceID) without popping the first, so
// stopping at the first match would merge into the buried screen while the visible
// top screen (read by Snapshot()) never sees the update. Delegates to
// mergeDetailRelatedRow so ResourceIDs are preserved (the controller-owned Enter
// navigation reads them). No-op when no stacked detail matches.
func (c *Controller) ApplyDetailRelatedResultForResource(sourceType, sourceID, displayName, targetType string, state domain.RelatedRowState, count int, loading bool, errMsg string, truncated bool, resourceIDs []string, fetchFilter map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.stack {
		if c.stack[i].ID != runtime.ScreenDetail {
			continue
		}
		if ds := c.stack[i].State.Detail; ds != nil && ds.Resource.ID == sourceID && ds.ResourceType == sourceType {
			mergeDetailRelatedRow(ds, displayName, targetType, state, count, loading, errMsg, truncated, resourceIDs, fetchFilter)
		}
	}
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
			State:       domain.RelatedLoading,
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

// SetDetailViewportHeight sets the ViewportHeight on the top detail screen's
// DetailState to the renderer-supplied usable field-viewport clip height.
// Move actions (ActionMoveUp/Down/Bottom) reconcile ScrollY against this
// stored height, so the controller is the single owner of scroll
// reconciliation and no call site needs to thread the height through
// Action.N. No-op when the top screen is not ScreenDetail.
func (c *Controller) SetDetailViewportHeight(h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ds := c.topDetailState()
	if ds == nil {
		return
	}
	ds.ViewportHeight = h
}

// ResetDetailRelatedRows unconditionally resets RelatedRows to loading state
// from the registered related defs, discarding any loaded counts. Called by
// handleRefresh so stale counts are cleared before the new checker results
// arrive — mirrors ResetRightColumn() on the TUI side.
// No-op when the top screen is not ScreenDetail.
func (c *Controller) ResetDetailRelatedRows(resourceType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resetDetailRelatedRowsLocked(resourceType)
}

// resetDetailRelatedRowsLocked is the lock-free implementation of
// ResetDetailRelatedRows, shared with handleActionRefresh's detail branch
// (core/app/actions_list.go), which already holds c.mu. Callers must hold
// c.mu (write).
func (c *Controller) resetDetailRelatedRowsLocked(resourceType string) {
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
			State:       domain.RelatedLoading,
			Loading:     true,
		})
	}
	ds.RelatedRows = rows
	// A populated related panel is visible; reflect it in the stored flag so
	// focus/filter/cursor actions (which gate on RelatedVisible) take effect.
	// A narrow terminal overrides this via SetDetailRelatedVisible(false, …).
	ds.RelatedVisible = true
}

// GetDetailResource returns the resource stored in the top detail screen's
// DetailState. Returns the zero-value Resource when the top screen is not a
// detail screen. Lets the renderer read detail state from the controller.
func (c *Controller) GetDetailResource() resource.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ds := c.topDetailState()
	if ds == nil {
		return resource.Resource{}
	}
	return ds.Resource
}

// GetDetailResourceType returns the resource type stored in the top detail
// screen's DetailState. Returns an empty string when the top screen is not a
// detail screen.
func (c *Controller) GetDetailResourceType() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ds := c.topDetailState()
	if ds == nil {
		return ""
	}
	return ds.ResourceType
}

// SelectedRelatedRow returns a copy of the related row currently under the
// cursor on the top detail screen when the related panel has focus. ok is
// false when the top screen is not a focused detail related panel or the
// cursor points past the visible rows. The TUI Enter and yank paths use this
// to source navigation/copy data (ResourceIDs, FetchFilter, DisplayName) from
// controller state rather than the renderer's right-column widget.
func (c *Controller) SelectedRelatedRow() (DetailRelatedRow, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ds := c.topDetailState()
	if ds == nil || !ds.RelatedFocus {
		return DetailRelatedRow{}, false
	}
	row := ds.focusedRelatedRow()
	if row == nil {
		return DetailRelatedRow{}, false
	}
	return *row, true
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
