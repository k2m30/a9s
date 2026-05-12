package views

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// View renders detail content via viewport.
// When the right column is showing and width >= 60, renders left and right columns side by side
// with a │ separator.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	if m.rightColShowing() && m.width >= layout.MinInnerContentWidth {
		// Keep RELATED visible at medium widths by using side-by-side layout.
		rightW := m.currentRightColWidth()
		sep := styles.ColSepDim.Render("│")
		if m.rightCol.IsFocused() {
			sep = styles.ColSepAccent.Render("│")
		}
		leftContent := m.viewport.View()
		rightContent := m.rightCol.View()
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		// Normalise to same number of lines.
		maxLines := max(len(leftLines), len(rightLines))
		leftW := m.width - rightW - 1 // -1 for separator character
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
			// Pad left column to its fixed width so right column aligns correctly.
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
		if m.ready { // resize case — first paint is handled via Init/first Update
			m.pendingRelatedDispatch = true
		}
	} else if w < layout.MinInnerContentWidth && wasShowing {
		m.rightColAutoShown = false
		m.rightColVisible = false
		m.rightCol.SetFocused(false)
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
func (m DetailModel) BottomHints() []layout.KeyHint {
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

// SetEnrichmentFinding sets (or clears) the enrichment finding for this resource.
// A nil value clears any existing finding (recovery case). Setting a new value
// invalidates the field list and triggers a viewport re-render so the Attention
// section appears or disappears immediately.
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
func (m *DetailModel) SetEnrichmentFinding(f *resource.EnrichmentFinding) {
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

	m.enrichmentFinding = f
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
