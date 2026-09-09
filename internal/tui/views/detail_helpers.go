// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

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

	if m.ctrl != nil {
		m.ctrl.SetDetailViewportWidth(viewportW)
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportW), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportW)
		m.viewport.SetHeight(h)
	}
}

// rightColShowing returns true when the right column should be rendered.
// The column shows when explicitly toggled on OR when auto-shown on entry.
func (m DetailModel) rightColShowing() bool {
	return m.rightColVisible || m.rightColAutoShown
}

func (m DetailModel) currentRightColWidth() int {
	return ComputeRightColWidth(m.width, m.rightColWidth)
}

// DefaultRightColWidth is the related panel's base width, which every detail
// model starts at.
const DefaultRightColWidth = 32

// DetailContentWidth returns the width the field list is laid out in: the
// whole inner width, less the related panel and its separator when the panel
// is showing and the terminal is wide enough for it.
//
// It is the one statement of that split. The renderer sizes its left viewport
// with it, and the terminal hands the same number to the controller on a
// resize, so the body's layout decisions (the key width, the attention
// wrapping) are made for the pane the operator is looking at.
func DetailContentWidth(innerWidth, baseRightColWidth int, relatedVisible bool) int {
	if !relatedVisible || innerWidth < layout.MinInnerContentWidth {
		return innerWidth
	}
	return innerWidth - ComputeRightColWidth(innerWidth, baseRightColWidth) - 1
}

// ComputeRightColWidth returns the right-column width for a given terminal
// inner width and base right-column width. Used by the renderer stack to size
// the right column without a DetailModel instance (e.g., after Ctrl+R resets
// the column on the rendererState directly).
func ComputeRightColWidth(innerWidth, baseWidth int) int {
	// Keep right panel readable at medium widths while preserving left detail space.
	if innerWidth <= 0 {
		return baseWidth
	}
	if innerWidth >= 100 {
		return baseWidth
	}
	w := max(24, innerWidth/3)
	maxAllowed := max(16, innerWidth-40) // keep at least 40 cols for left pane
	if w > maxAllowed {
		w = maxAllowed
	}
	return w
}

// NeedsRelatedCheck returns true when the right column was auto-shown
// and checkers have not yet been dispatched. The root model checks this
// after pushing the detail view to emit RelatedCheckStartedMsg.
func (m DetailModel) NeedsRelatedCheck() bool {
	return m.rightColAutoShown
}

// RenderDetail produces the same string that View() would produce, reading
// field rows, attention, related, scroll, wrap, and cursor data from body
// rather than from m.fieldList / m.rightCol live model state. Width, height,
// and viewport dimensions remain renderer-owned and are read from m.
//
// The related panel is rendered from body.Related + body.RelatedCursor +
// body.RelatedScroll + body.RelatedFocused via renderDetailRelatedFromBody,
// a thin adapter over renderRelatedPanel — the panel's one render
// implementation, reading row facts from the controller-assembled
// app.RelatedBlock slice verbatim. RightColumnModel (rightcolumn.go) holds
// only the widget's key-routing/focus/filter interaction state; it carries
// no row facts and has nothing to drift against.
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

	// Apply search highlights when a query is active. Uses the same approach
	// as RenderText: construct a local SearchModel from body state, call Apply
	// to inject ANSI colour spans, and scroll the viewport to the current match.
	scrollY := body.ScrollY
	if body.Search != "" {
		plain := ansi.Strip(leftRaw)
		var sm SearchModel
		sm.active = true
		sm.SetQuery(body.Search)
		sm.SetContent(plain)
		n := len(sm.matches)
		if n > 0 {
			// Apply modulo so the controller's unbounded cursor wraps correctly.
			sm.currentIdx = ((body.SearchCursor % n) + n) % n
		}
		var matchLine int
		leftRaw, matchLine = sm.Apply(leftRaw)
		if matchLine >= 0 {
			scrollY = matchLine
		}
	}

	m.viewport.SoftWrap = body.Wrap
	m.viewport.SetContent(leftRaw)
	m.viewport.GotoTop()
	m.viewport.SetYOffset(scrollY)

	// Use body.RelatedVisible as the gate — it mirrors the TUI's rightColShowing()
	// (auto-show when defs exist + wide terminal, or explicit user toggle).
	// Width guard matches the TUI's MinInnerContentWidth check.
	if body.RelatedVisible && m.width >= layout.MinInnerContentWidth {
		rightW := m.currentRightColWidth()
		leftW := DetailContentWidth(m.width, m.rightColWidth, true)
		// Size the viewport to the left-panel width so its View() clips content
		// to leftW, not to m.width. Without this, transient detail paths (e.g.
		// renderDetail in renderer.go) that create a fresh viewport at rs.width
		// produce leftContent lines wider than leftW, causing the combined line
		// (left + sep + right) to exceed m.width.
		m.viewport.SetWidth(leftW)
		sep := styles.ColSepDim.Render("│")
		if body.RelatedFocused {
			sep = styles.ColSepAccent.Render("│")
		}
		leftContent := m.viewport.View()
		rightContent := renderDetailRelatedFromBody(body, rightW, m.height)
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		maxLines := max(len(leftLines), len(rightLines))
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
			leftVisible := text.Width(left)
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

// renderDetailRelatedFromBody renders the RELATED right panel from body data.
// w is the panel width; h is the panel height (same as viewport height).
func renderDetailRelatedFromBody(body app.DetailBody, w, h int) string {
	return renderRelatedPanel(body.Related, body.RelatedFilterActive, body.RelatedCursor, body.RelatedScroll, body.RelatedFocused, w, h)
}

// renderRelatedPanel is the single pure renderer for the RELATED right
// panel, called by RenderDetail via renderDetailRelatedFromBody with the
// controller-assembled app.RelatedBlock slice. It is the one place this
// rendering logic lives, so there is no second lane it can drift against.
//
// rows is the visible, filtered, ordered list of related-panel entries.
// cursor is an index into rows (-1 when no row is selected/highlighted).
// scroll is the first visible row index. focused gates the scroll-to-cursor
// adjustment and the cursor-row highlight. w/h are the panel's rendering
// dimensions (h includes the header line).
func renderRelatedPanel(rows []app.RelatedBlock, filterActive bool, cursor, scroll int, focused bool, w, h int) string {
	if w <= 0 {
		return ""
	}

	lines := make([]string, 0, h)

	header := "RELATED"
	padLeft := max((w-text.Width(header))/2, 0)
	centeredHeader := strings.Repeat(" ", padLeft) + header
	lines = append(lines, styles.DimText.Render(centeredHeader))

	switch {
	case len(rows) == 0:
		if filterActive {
			lines = append(lines, styles.DimText.Render("  No matches"))
		} else {
			lines = append(lines, styles.DimText.Render("  No related types registered"))
		}
	default:
		usableHeight := max(h-1, 1) // after header

		start := scroll
		// Keep the focused cursor row visible. Idempotent with respect to a
		// caller that has already applied the same clamp (e.g. RightColumnModel's
		// ensureScrollVisible): re-running these two branches on an already-valid
		// start is a no-op, since start already satisfies
		// start <= cursor < start+usableHeight.
		if focused {
			if cursor < start {
				start = cursor
			} else if cursor >= start+usableHeight {
				start = cursor - usableHeight + 1
			}
		}
		if start < 0 {
			start = 0
		}
		// Upper clamp: when the row set has shrunk (e.g. a related filter)
		// while a larger scroll offset persists from before the shrink,
		// start must not run past the last full window — otherwise
		// rows[start:end] below can panic (end computed as
		// min(start+usableHeight, len(rows)) can fall below start) or, even
		// when it doesn't panic, needlessly render a partial trailing window
		// instead of the fullest valid one.
		if maxStart := max(len(rows)-usableHeight, 0); start > maxStart {
			start = maxStart
		}
		end := min(start+usableHeight, len(rows))

		for i, blk := range rows[start:end] {
			idx := start + i // index into rows (matches cursor)
			var rowText string
			var rowStyle lipgloss.Style

			switch {
			case blk.Loading:
				rowText = "  " + blk.Name
				rowStyle = styles.DimText
			case blk.Err:
				rowText = "  " + blk.Name + "  —" // em dash
				rowStyle = styles.DimText
			default:
				rowText = "  " + blk.Name
				if blk.CountDisplay != "" {
					rowText += " " + blk.CountDisplay
				}
				if blk.Actionable {
					rowStyle = styles.RowNormal
				} else {
					rowStyle = styles.DimText
				}
			}

			if focused && cursor == idx {
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
// renderFromFieldList but reading state from body (FieldCursor) and
// m (viewport width for cursor-row padding, ready flag).
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
	// / m.ready / m.viewport) produces byte-identical output.
	tmp := *m
	tmp.fieldList = items
	tmp.fieldCursor = body.FieldCursor
	// rightCol focus drives leftFocused in renderFromFieldList; set a consistent state.
	tmp.rightCol.SetFocused(!leftFocused)
	// A body the controller built carries the key width it laid the rows out
	// against. A body assembled by hand carries none, and the same rule is
	// applied here to the width this model was sized to.
	keyW := body.KeyWidth
	if keyW <= 0 {
		keyW = app.DetailKeyWidth(body.Fields, m.width)
	}
	return tmp.renderFromFieldList(keyW)
}
