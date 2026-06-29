package views

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// View renders detail content via viewport.
// When the right column is showing and width >= 60, renders left and right columns side by side
// with a │ separator.
//
// When ctrl is set (controller-backed TUI path), delegates to
// RenderDetail(ctrl.Snapshot().Body.Detail) so the headless and TUI renderers
// share one code path (mirrors YAMLModel.View()).
//
// When ctrl is nil (parity tests, isolated callers), builds a DetailBody from
// the live model state and delegates to RenderDetail — this ensures View() and
// RenderDetail(body) produce byte-identical output for the same logical state.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	if m.ctrl != nil {
		body := m.ctrl.Snapshot().Body.Detail
		if body == nil {
			return "Initializing..."
		}
		return m.RenderDetail(*body)
	}
	// Build a body from live model state and delegate to RenderDetail so View()
	// and RenderDetail(body) are guaranteed byte-identical (parity invariant).
	body := m.buildLiveBody()
	return m.RenderDetail(body)
}

// buildLiveBody constructs a DetailBody from the live DetailModel state, used
// by View() when ctrl is nil. This is the inverse of buildDetailBody: it reads
// from m.fieldList / m.rightCol / m.search / m.wrap / etc. to produce the same
// body that the controller would snapshot for this exact visual state.
func (m DetailModel) buildLiveBody() app.DetailBody {
	// Build field rows from live fieldList.
	fields := make([]app.FieldRow, len(m.fieldList))
	for i, item := range m.fieldList {
		fields[i] = app.FieldRow{
			Key:         item.Key,
			Value:       item.Value,
			IsSection:   item.IsSection,
			IsHeader:    item.IsHeader,
			IsSubField:  item.IsSubField,
			IsSpacer:    item.IsSpacer,
			IsNavigable: item.IsNavigable,
			IndentLevel: item.IndentLevel,
			ColorTier:   item.ColorTier,
			Path:        item.Path,
		}
	}

	// Compute key width from live fieldList.
	var topKeys []string
	for _, item := range m.fieldList {
		if !item.IsHeader && !item.IsSubField {
			topKeys = append(topKeys, item.Key)
		}
	}
	keyWidth := 0
	for _, k := range topKeys {
		if n := len(k) + 1; n > keyWidth {
			keyWidth = n
		}
	}

	// Build related blocks from live rightCol rows — mirrors buildDetailRelatedBlocks.
	var related []app.RelatedBlock
	relatedQuery := strings.TrimSpace(strings.ToLower(m.rightCol.filterQuery))
	if m.rightColShowing() {
		for _, row := range m.rightCol.rows {
			if m.rightCol.isSelfPivotZeroRow(row) {
				continue
			}
			// Mirror buildDetailRelatedBlocks: drop rows that don't match the filter.
			if relatedQuery != "" && !strings.Contains(strings.ToLower(row.displayName), relatedQuery) {
				continue
			}
			related = append(related, app.RelatedBlock{
				Name:        row.displayName,
				Count:       row.count,
				Loading:     row.loading,
				Err:         row.err != nil,
				Approximate: row.approximate,
				FetchFilter: row.fetchFilter,
				TargetType:  row.targetType,
			})
		}
	}

	// Compute scroll offset from rightCol state.
	relatedScroll := 0
	relatedCursor := 0
	if m.rightColShowing() {
		relatedScroll = m.rightCol.scrollOffset
		relatedCursor = m.rightCol.cursor
		// Convert absolute cursor index to index-in-Related slice (visible filtered slice).
		// body.Related contains only non-self-pivot-zero rows in definition order.
		// The cursor in rightCol is an index into m.rightCol.rows (raw); we need
		// the index into body.Related (filtered). Map via name match.
		if len(related) > 0 {
			cursorRow := m.rightCol.SelectedRow()
			if cursorRow != nil {
				for i, blk := range related {
					if blk.Name == cursorRow.displayName {
						relatedCursor = i
						break
					}
				}
			}
		}
	}

	return app.DetailBody{
		Fields:              fields,
		Related:             related,
		RelatedFocused:      m.rightCol.IsFocused(),
		RelatedVisible:      m.rightColShowing(),
		RelatedCursor:       relatedCursor,
		RelatedScroll:       relatedScroll,
		RelatedFilter:       m.rightCol.filterQuery,
		RelatedFilterActive: m.rightCol.filterActive,
		RelatedSourceType:   m.resourceType,
		Search:              m.search.Query(),
		Wrap:                m.wrap,
		ScrollY:             m.viewport.YOffset(),
		FieldCursor:         m.fieldCursor,
		KeyWidth:            keyWidth,
	}
}

// SetSize initializes or resizes the viewport. Must be called before View().
// On first call, if width >= 60 and related defs are registered, the right
// column is auto-shown (rightColAutoShown = true). The first explicit toggle
// hides the auto-shown column. A subsequent toggle re-shows it.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	wasShowing := m.rightColShowing()

	// Auto-show right column when wide enough and related defs exist:
	// - on first SetSize call, and
	// - on later resizes only if user hasn't explicitly toggled visibility.
	if w >= layout.MinInnerContentWidth && len(resource.GetRelated(m.resourceType)) > 0 &&
		(!m.ready || (!m.rightColShowing() && !m.rightColUserToggled)) {
		m.rightColAutoShown = true
		m.rightCol = newRightColumn(resource.GetRelated(m.resourceType), m.res, m.resourceType)
		m.rightCol.keys = m.keys
		if m.ctrl != nil {
			// Sync the auto-show to the controller so Tab/cursor/filter actions
			// (which gate on ds.RelatedVisible) take effect. hidden=false preserves
			// the auto-show contract; an explicit hide sets hidden=true elsewhere.
			m.ctrl.SetDetailRelatedVisible(true, false)
		}
		if m.ready { // resize case — first paint is handled via Init/first Update
			m.pendingRelatedDispatch = true
		}
	} else if w < layout.MinInnerContentWidth && wasShowing {
		m.rightColAutoShown = false
		m.rightColVisible = false
		m.rightCol.SetFocused(false)
		if m.ctrl != nil {
			m.ctrl.SetDetailRelatedVisible(false, false)
		}
	}

	viewportW := w
	if m.rightColShowing() && w >= layout.MinInnerContentWidth {
		rightW := m.currentRightColWidth()
		viewportW = w - rightW - 1 // -1 for separator character
		m.rightCol.SetSize(rightW, h)
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportW), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportW)
		m.viewport.SetHeight(h)
	}
	m.refreshViewportContent()
}

// rightColShowing returns true when the right column should be rendered.
// The column shows when explicitly toggled on OR when auto-shown on entry.
func (m DetailModel) rightColShowing() bool {
	return m.rightColVisible || m.rightColAutoShown
}

// recalcViewportWidth adjusts the viewport width based on the right column visibility.
func (m *DetailModel) recalcViewportWidth() {
	if m.rightColShowing() && m.width >= layout.MinInnerContentWidth {
		leftW := m.width - m.currentRightColWidth() - 1 // -1 for separator
		if m.ready {
			m.viewport.SetWidth(leftW)
		}
	} else if m.ready {
		m.viewport.SetWidth(m.width)
	}
	m.refreshViewportContent()
}

func (m DetailModel) currentRightColWidth() int {
	// Keep right panel readable at medium widths while preserving left detail space.
	if m.width <= 0 {
		return m.rightColWidth
	}
	if m.width >= 100 {
		return m.rightColWidth
	}
	w := max(24, m.width/3)
	maxAllowed := max(16, m.width-40) // keep at least 40 cols for left pane
	if w > maxAllowed {
		w = maxAllowed
	}
	return w
}

// syncViewportToCursor adjusts the viewport scroll to keep fieldCursor visible.
func (m *DetailModel) syncViewportToCursor() {
	if !m.ready {
		return
	}
	yOffset := m.viewport.YOffset()
	visibleLines := m.viewport.Height()
	if m.fieldCursor < yOffset {
		m.viewport.SetYOffset(m.fieldCursor)
	} else if m.fieldCursor >= yOffset+visibleLines {
		m.viewport.SetYOffset(m.fieldCursor - visibleLines + 1)
	}
}

// FieldCursor returns the current field cursor index for testing.
func (m DetailModel) FieldCursor() int {
	return m.fieldCursor
}

// IsControllerBacked reports whether this DetailModel was constructed with a
// controller (NewDetailWithCtrl). popView uses this to decide whether to sync
// the controller stack on pop.
func (m DetailModel) IsControllerBacked() bool {
	return m.ctrl != nil
}

// refreshViewportContent re-renders content and applies search highlights.
func (m *DetailModel) refreshViewportContent() {
	if m.fieldList == nil {
		m.buildFieldList()
	}
	content := m.renderContent()
	if m.search.IsActive() && m.search.Query() != "" {
		plain := ansi.Strip(content)
		m.search.SetContent(plain)
		var matchLine int
		content, matchLine = m.search.Apply(content)
		if matchLine >= 0 {
			m.viewport.GotoTop()
			m.viewport.SetYOffset(matchLine)
		}
	}
	m.viewport.SetContent(content)
}

// FrameTitle returns the resource identifier.
func (m DetailModel) FrameTitle() string {
	if m.res.Name != "" {
		return m.res.Name
	}
	return m.res.ID
}

// BottomHints implements Hintable for DetailModel.
// When the model is controller-backed, delegates to the controller's footer
// hints so the TUI and web renderers share a single source of truth.
// Non-controller-backed callers (tests, isolated views) use the live model state.
func (m DetailModel) BottomHints() []layout.KeyHint {
	if m.ctrl != nil {
		ctrlHints := m.ctrl.Snapshot().Footer
		hints := make([]layout.KeyHint, len(ctrlHints))
		for i, kh := range ctrlHints {
			hints[i] = layout.KeyHint{Key: kh.Key, Desc: kh.Help}
		}
		return hints
	}

	var hints []layout.KeyHint

	// Right column focused state
	if m.rightColShowing() && m.rightCol.IsFocused() {
		name := m.rightCol.SelectedTypeName()
		if name != "" {
			// Resolve display name
			displayName := name
			if rt := resource.FindResourceType(name); rt != nil {
				displayName = rt.Name
			} else if ct := resource.GetChildType(name); ct != nil {
				displayName = ct.Name
			}
			hints = append(hints, layout.KeyHint{Key: "enter", Desc: displayName})
		}
		hints = append(hints, layout.KeyHint{Key: "tab", Desc: "Fields"})
		hints = append(hints, layout.KeyHint{Key: "y", Desc: "YAML"})
		hints = append(hints, layout.KeyHint{Key: "J", Desc: "JSON"})
		if resource.BuildCloudTrailFilter(m.res, m.resourceType) != nil {
			hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
		}
		hints = append(hints, layout.KeyHint{Key: "ctrl+r", Desc: "Refresh"})
		return hints
	}

	// Left column — check navigable field under cursor
	if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
		item := m.fieldList[m.fieldCursor]
		if item.IsNavigable && item.TargetType != "" {
			displayName := item.TargetType
			if rt := resource.FindResourceType(item.TargetType); rt != nil {
				displayName = rt.Name
			} else if ct := resource.GetChildType(item.TargetType); ct != nil {
				displayName = ct.Name
			}
			hints = append(hints, layout.KeyHint{Key: "enter", Desc: displayName})
		}
	}

	hints = append(hints, layout.KeyHint{Key: "y", Desc: "YAML"})
	hints = append(hints, layout.KeyHint{Key: "J", Desc: "JSON"})
	if resource.BuildCloudTrailFilter(m.res, m.resourceType) != nil {
		hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
	}

	// Related panel
	if related := resource.GetRelated(m.resourceType); len(related) > 0 {
		hints = append(hints, layout.KeyHint{Key: "r", Desc: "Related"})
		if m.rightColVisible {
			hints = append(hints, layout.KeyHint{Key: "tab", Desc: "Cols"})
		}
	}

	hints = append(hints, layout.KeyHint{Key: "ctrl+r", Desc: "Refresh"})
	hints = append(hints, layout.KeyHint{Key: "w", Desc: "Wrap"})

	return hints
}

// CopyContent returns column-aware clipboard content for the active selection.
func (m DetailModel) CopyContent() (string, string) {
	if m.rightCol.IsFocused() {
		name := m.rightCol.SelectedTypeName()
		if name == "" {
			return "", ""
		}
		return name, "Copied: " + name
	}
	if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
		item := m.fieldList[m.fieldCursor]
		content := item.Value
		if content == "" {
			content = item.Key
		}
		if content == "" {
			return "", ""
		}
		return content, "Copied: " + content
	}
	content := m.RawYAML()
	if content == "" {
		return "", ""
	}
	return content, "Copied detail to clipboard"
}

// GetHelpContext returns HelpFromDetail.
func (m DetailModel) GetHelpContext() HelpContext {
	return HelpFromDetail
}

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m DetailModel) IsSearchActive() bool {
	return m.search.IsActive() || m.rightCol.IsFiltering()
}

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m DetailModel) IsSearchInputMode() bool {
	return m.search.IsInputMode() || m.rightCol.IsFiltering()
}

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m DetailModel) SearchInfo() string {
	if m.rightCol.IsFiltering() {
		return "/" + m.rightCol.FilterQuery()
	}
	if !m.search.IsActive() {
		return ""
	}
	if m.search.IsInputMode() {
		q := m.search.Query()
		return "/" + q
	}
	return m.search.MatchInfo()
}

// ResourceID returns the resource ID for clipboard copy.
func (m DetailModel) ResourceID() string {
	return m.res.ID
}

// ResourceType returns the resource type short name.
func (m DetailModel) ResourceType() string {
	return m.resourceType
}

// SourceResource returns the resource being viewed.
func (m DetailModel) SourceResource() resource.Resource {
	return m.res
}

// SetEnrichmentFinding sets (or clears) the wave-2 enrichment finding for this
// resource. A nil value clears any prior wave-2 entry (recovery case). Setting
// a new value invalidates the field list and triggers a viewport re-render so
// the Attention section appears or disappears immediately.
//
// Phase 03 W1.4b.3: the wave-2 entry now lives directly on m.res.Findings
// (tagged with Source="wave2:…") instead of a separate DetailModel field, so
// the renderer reads a single source of truth (m.res.Findings + AttentionDetails).
// Subsequent calls strip any existing wave-2 entry before appending the new
// one, preserving the prior "replace, don't accumulate" semantics. Wave-1
// entries on m.res.Findings are preserved.
//
// Cursor stability: the rebuild prepends / removes the Attention section
// (detail_fields.go:injectAttentionSection), which would otherwise leave
// m.fieldCursor pointing at a different logical item. The sequence is:
//  1. Snapshot the cursor's FieldItem identity (Key + Path).
//  2. Rebuild fieldList eagerly (NOT via refreshViewportContent — render
//     would otherwise happen against the old cursor value, producing an
//     off-by-N highlight for one frame).
//  3. Relocate cursor to the snapshot's new index.
//  4. syncViewportToCursor to scroll the viewport to the new position.
//  5. refreshViewportContent renders with the already-correct cursor.
//
// Attention-internal rows (Path=="Attention") are deliberately not pinned —
// if the selection was on an Attention entry that the rebuild removes, fall
// back to cursor=0 rather than hunting for a gone row. Regression guards:
// views.TestDetail_SetEnrichmentFinding_PreservesCursorIdentity and
// views.TestDetail_SetEnrichmentFinding_RenderedSelectionFollowsCursor.
func (m *DetailModel) SetEnrichmentFinding(f *domain.Finding, ad *domain.AttentionDetail) {
	var beforeKey, beforePath string
	haveSnapshot := false
	if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
		prev := m.fieldList[m.fieldCursor]
		if prev.Path != "Attention" {
			beforeKey = prev.Key
			beforePath = prev.Path
			haveSnapshot = true
		}
	}

	// Strip any prior wave-2 entries from m.res.Findings + their companion
	// AttentionDetails so repeated SetEnrichmentFinding calls replace rather
	// than accumulate. Wave-1 entries (Source != "wave2:…") are preserved.
	if len(m.res.Findings) > 0 {
		kept := m.res.Findings[:0:0]
		for _, fi := range m.res.Findings {
			if strings.HasPrefix(fi.Source, "wave2:") {
				if m.res.AttentionDetails != nil {
					delete(m.res.AttentionDetails, fi.Code)
				}
				continue
			}
			kept = append(kept, fi)
		}
		m.res.Findings = kept
	}

	// Append the new wave-2 finding (and AttentionDetail) when present.
	// Source is stamped so subsequent strip cycles can recognise it, even if
	// the caller passed an unstamped Finding (e.g. legacy test fixtures).
	if f != nil && f.Phrase != "" {
		finding := *f
		if !strings.HasPrefix(finding.Source, "wave2:") {
			finding.Source = "wave2:tui"
		}
		m.res.Findings = append(m.res.Findings, finding)
		if ad != nil && len(ad.Rows) > 0 {
			if m.res.AttentionDetails == nil {
				m.res.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
			}
			m.res.AttentionDetails[finding.Code] = *ad
		}
	}

	m.fieldList = nil
	m.buildFieldList() // eager rebuild BEFORE render — see sequence in docstring

	if haveSnapshot {
		located := false
		for i, item := range m.fieldList {
			if item.Key == beforeKey && item.Path == beforePath {
				m.fieldCursor = i
				located = true
				break
			}
		}
		if !located && m.fieldCursor >= len(m.fieldList) {
			m.fieldCursor = 0
		}
	} else if m.fieldCursor >= len(m.fieldList) {
		m.fieldCursor = 0
	}

	m.syncViewportToCursor()
	m.refreshViewportContent()
}

// NeedsRelatedCheck returns true when the right column was auto-shown
// and checkers have not yet been dispatched. The root model checks this
// after pushing the detail view to emit RelatedCheckStartedMsg.
func (m DetailModel) NeedsRelatedCheck() bool {
	return m.rightColAutoShown
}

// ApplyRelatedResults injects cached related check result messages into the
// right column, avoiding re-dispatch of async checkers. Called by root model
// on detail re-entry.
//
// Callers pass the full RelatedCheckResultMsg values (including the per-row
// DefDisplayName disambiguator) so rows with multiple defs sharing a
// TargetType — e.g. the four ct-events self-pivot rows ("by AccessKeyId" /
// "by Username" / "by EventName" / "by SharedEventId") — resolve to the
// correct row on replay instead of leaving all four stuck in the loading
// state.
func (m *DetailModel) ApplyRelatedResults(msgs []messages.RelatedCheckResult) {
	for _, msg := range msgs {
		m.rightCol, _ = m.rightCol.Update(msg)
		if m.ctrl != nil {
			// Mirror the live RelatedCheckResult handler: cached results must also
			// land in the controller's DetailState.RelatedRows so the body render
			// (counts) reflects them immediately on re-entry.
			errMsg := ""
			if msg.Result.Err != nil {
				errMsg = msg.Result.Err.Error()
			}
			m.ctrl.ApplyDetailRelatedResult(
				msg.DefDisplayName,
				msg.Result.TargetType,
				msg.Result.Count,
				false,
				errMsg,
				msg.Result.Approximate,
				msg.Result.FetchFilter,
			)
		}
	}
}

// ResetRightColumn resets the right column to its initial loading state,
// discarding any loaded counts. Called by handleRefresh before re-dispatching
// async checks so stale counts are not shown during reload.
func (m *DetailModel) ResetRightColumn() {
	if !m.rightColShowing() {
		return
	}
	defs := resource.GetRelated(m.resourceType)
	m.rightCol = newRightColumn(defs, m.res, m.resourceType)
	m.rightCol.keys = m.keys
	m.rightCol.SetSize(m.currentRightColWidth(), m.height)
}

// ConsumesEscapeLocally reports whether Escape should be handled inside the
// detail view instead of by the root view-stack pop logic.
func (m DetailModel) ConsumesEscapeLocally() bool {
	return m.rightCol.IsFocused() || m.rightCol.IsFiltering()
}

// RenderDetail produces the same string that View() would produce, reading
// field rows, attention, related, scroll, wrap, and cursor data from body
// rather than from m.fieldList / m.rightCol live model state. Width, height,
// and viewport dimensions remain renderer-owned and are read from m.
//
// The related panel is rendered from body.Related + body.RelatedCursor +
// body.RelatedScroll + body.RelatedFocused via renderDetailRelatedFromBody,
// which replicates rightColumnModel.View() byte-for-byte from body data.
// The panel visibility gate uses body.RelatedVisible (set by buildDetailBody
// when the type has registered defs or ds.RelatedVisible is true), matching
// the TUI's rightColShowing() auto-show behaviour.
func (m *DetailModel) RenderDetail(body app.DetailBody) string {
	if !m.ready {
		return "Initializing..."
	}

	// Render left column raw content from body fields, then route it through
	// the viewport — exactly as View() does via m.viewport.View(). This gives
	// height-padding (blank lines to fill viewport height) and width-clipping,
	// and ensures the cursor-row background highlight is embedded at the correct
	// scroll-offset position (Bug 1 fix).
	leftRaw := renderDetailFieldsFromBody(m, body)
	m.viewport.SoftWrap = body.Wrap
	m.viewport.SetContent(leftRaw)
	m.viewport.GotoTop()
	m.viewport.SetYOffset(body.ScrollY)

	// Use body.RelatedVisible as the gate — it mirrors the TUI's rightColShowing()
	// (auto-show when defs exist + wide terminal, or explicit user toggle).
	// Width guard matches the TUI's MinInnerContentWidth check.
	if body.RelatedVisible && m.width >= layout.MinInnerContentWidth {
		rightW := m.currentRightColWidth()
		sep := styles.ColSepDim.Render("│")
		if body.RelatedFocused {
			sep = styles.ColSepAccent.Render("│")
		}
		leftContent := m.viewport.View()
		rightContent := renderDetailRelatedFromBody(body, rightW, m.height)
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		maxLines := max(len(leftLines), len(rightLines))
		leftW := m.width - rightW - 1
		var sb strings.Builder
		for i := range maxLines {
			if i > 0 {
				sb.WriteString("\n")
			}
			left := ""
			if i < len(leftLines) {
				left = leftLines[i]
			}
			right := ""
			if i < len(rightLines) {
				right = ansi.Truncate(rightLines[i], rightW, "")
			}
			padded := left
			leftVisible := lipgloss.Width(left)
			if leftVisible < leftW {
				padded = left + strings.Repeat(" ", leftW-leftVisible)
			}
			sb.WriteString(padded)
			sb.WriteString(sep)
			sb.WriteString(right)
		}
		return sb.String()
	}
	return m.viewport.View()
}

// renderDetailRelatedFromBody renders the RELATED right panel from body data,
// replicating rightColumnModel.View() byte-for-byte without a live model.
// w is the panel width; h is the panel height (same as viewport height).
func renderDetailRelatedFromBody(body app.DetailBody, w, h int) string {
	if w <= 0 {
		return ""
	}

	lines := make([]string, 0, h)

	// Header: "RELATED" centered — mirrors rightColumnModel.View() header block.
	header := "RELATED"
	padLeft := max((w-lipgloss.Width(header))/2, 0)
	centeredHeader := strings.Repeat(" ", padLeft) + header
	lines = append(lines, styles.DimText.Render(centeredHeader))

	switch {
	case len(body.Related) == 0:
		// Mirror rightColumnModel.View(): an active filter with no surviving
		// rows shows "No matches"; otherwise the panel is genuinely empty.
		if body.RelatedFilterActive {
			lines = append(lines, styles.DimText.Render("  No matches"))
		} else {
			lines = append(lines, styles.DimText.Render("  No related types registered"))
		}
	default:
		usableHeight := max(h-1, 1) // after header

		start := body.RelatedScroll
		// Keep the focused cursor row visible. Scroll-to-cursor is renderer-side
		// (it depends on the panel height the controller doesn't own) — mirrors the
		// menu's adjustScroll.
		if body.RelatedFocused {
			if body.RelatedCursor < start {
				start = body.RelatedCursor
			} else if body.RelatedCursor >= start+usableHeight {
				start = body.RelatedCursor - usableHeight + 1
			}
		}
		if start < 0 {
			start = 0
		}
		end := min(start+usableHeight, len(body.Related))

		for i, blk := range body.Related[start:end] {
			idx := start + i // index into body.Related (matches rightCol cursor logic)
			var rowText string
			var rowStyle lipgloss.Style

			switch {
			case blk.Loading:
				rowText = "  " + blk.Name
				rowStyle = styles.DimText
			case blk.Err:
				rowText = "  " + blk.Name + "  —" // em dash
				rowStyle = styles.DimText
			case blk.Count == -1 && len(blk.FetchFilter) > 0:
				rowText = "  " + blk.Name
				rowStyle = styles.RowNormal
			case blk.Count == -1:
				rowText = "  " + blk.Name
				rowStyle = styles.DimText
			case blk.Count == 0 && blk.Approximate:
				rowText = "  " + blk.Name + " (0)"
				rowStyle = styles.RowNormal
			case blk.Count == 0:
				rowText = "  " + blk.Name + " (0)"
				rowStyle = styles.DimText
			default:
				rowText = "  " + blk.Name + " (" + itoa(blk.Count) + ")"
				rowStyle = styles.RowNormal
			}

			if body.RelatedFocused && body.RelatedCursor == idx {
				lines = append(lines, styles.RowSelected.Width(w).Render(rowText))
			} else {
				lines = append(lines, rowStyle.Render(rowText))
			}
		}
	}

	// Pad remaining height with empty strings.
	for len(lines) < h {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderDetailFieldsFromBody renders the field list from body.Fields, mirroring
// renderFromFieldList but reading state from body (FieldCursor, KeyWidth) and
// m (viewport width for cursor-row padding, ready flag, plainMode).
func renderDetailFieldsFromBody(m *DetailModel, body app.DetailBody) string {
	if len(body.Fields) == 0 {
		return styles.DimText.Render("  No detail data available")
	}

	leftFocused := !body.RelatedFocused

	// Convert body.Fields → []fieldpath.FieldItem so we can reuse
	// renderFromFieldList's exact logic via a temporary model.
	items := make([]fieldpath.FieldItem, len(body.Fields))
	for i, f := range body.Fields {
		items[i] = fieldpath.FieldItem{
			Key:         f.Key,
			Value:       f.Value,
			IsSection:   f.IsSection,
			IsHeader:    f.IsHeader,
			IsSubField:  f.IsSubField,
			IsSpacer:    f.IsSpacer,
			IsNavigable: f.IsNavigable,
			IndentLevel: f.IndentLevel,
			ColorTier:   f.ColorTier,
			Path:        f.Path,
		}
	}

	// Build a temporary model snapshot with fieldList and fieldCursor from body
	// so renderFromFieldList (which reads m.fieldList / m.fieldCursor / m.rightCol
	// / m.ready / m.plainMode / m.viewport) produces byte-identical output.
	tmp := *m
	tmp.fieldList = items
	tmp.fieldCursor = body.FieldCursor
	// rightCol focus drives leftFocused in renderFromFieldList; set a consistent state.
	tmp.rightCol.SetFocused(!leftFocused)
	return tmp.renderFromFieldList()
}


// ensure the viewport import is used (it is used by SetSize / other methods —
// this blank assignment guards against an "imported and not used" error if the
// compiler's unused-import analysis sees only the new imports through the file).
var _ = viewport.New
var _ = text.PadOrTrunc
