// SPDX-License-Identifier: GPL-3.0-or-later

// Package views — RightColumnModel renders the RELATED panel in the detail view.
package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

// RightColumnModel tracks interaction state (focus, cursor, filter query,
// scroll offset) for the RELATED panel next to a detail view. It does not
// render the panel: row facts (state/count/actionability/navigation IDs)
// live in the controller-assembled app.DetailBody.Related, read directly by
// RenderDetail (detail_helpers.go) via a transient DetailModel built fresh
// each frame — see renderer.go's renderDetail(). RightColumnModel is the
// long-lived half that Bubble Tea key routing (app_stack.go) needs between
// key events: SetFocused/IsFocused gate Tab/h/l column-focus switching, and
// the filter fields track '/' typing before app_stack.go syncs the query to
// the controller (which owns the actual filtered/actionable row set and its
// cursor/scroll — core/app/detail_cursor.go).
//
// Exported so tui.rendererState can hold one without importing internal view
// model types into the forbidden files.
type RightColumnModel struct {
	cursor       int
	focused      bool
	width        int
	height       int
	scrollOffset int
	filterQuery  string
	filterActive bool
	keys         keys.Map
}

// newRightColumn constructs a RightColumnModel. defs, parentRes, and
// sourceType are accepted for call-site compatibility with the related-panel
// registration path; the widget itself holds no row facts, so they are
// unused here.
func newRightColumn(_ []resource.RelatedDef, _ resource.Resource, _ string) RightColumnModel {
	return RightColumnModel{
		keys: keys.Default(),
	}
}

// Init implements the sub-component init pattern. No async work — related
// checks are dispatched by app.go and applied to the controller directly.
func (m RightColumnModel) Init() (RightColumnModel, tea.Cmd) {
	return m, nil
}

// Update handles key navigation for the widget's own interaction state
// (focus/filter/cursor). Related-check results are applied to the
// controller (core/app), not to this widget — see
// Model.handleRelatedCheckResult in runtime_adapter_resources.go.
func (m RightColumnModel) Update(msg tea.Msg) (RightColumnModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKeyMsg(msg)
	}
	return m, nil
}

func (m RightColumnModel) updateKeyMsg(msg tea.KeyMsg) (RightColumnModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	if key.Matches(msg, m.keys.Escape) && strings.TrimSpace(m.filterQuery) != "" {
		m.filterActive = false
		m.filterQuery = ""
		m.scrollOffset = 0
		return m, nil
	}

	if m.filterActive {
		k := msg.Key()
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.filterActive = false
			m.filterQuery = ""
			m.scrollOffset = 0
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			m.filterActive = false
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.moveCursor(-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.moveCursor(1)
			return m, nil
		}
		if k.Code == tea.KeyBackspace {
			if len(m.filterQuery) > 0 {
				m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
				m.scrollOffset = 0
			}
			return m, nil
		}
		if k.Text != "" {
			m.filterQuery += k.Text
			m.scrollOffset = 0
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Search):
		m.filterActive = true
		m.filterQuery = ""
		m.scrollOffset = 0
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	}
	return m, nil
}

// SetSize sets the rendering dimensions.
func (m *RightColumnModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused sets whether this column has keyboard focus.
func (m *RightColumnModel) SetFocused(focused bool) {
	m.focused = focused
}

// IsFocused reports whether this column has keyboard focus.
func (m RightColumnModel) IsFocused() bool {
	return m.focused
}

// IsFiltering reports whether the filter input is currently active.
func (m RightColumnModel) IsFiltering() bool {
	return m.filterActive
}

// FilterQuery returns the current filter text.
func (m RightColumnModel) FilterQuery() string {
	return m.filterQuery
}

// HasFilter reports whether a non-blank filter query is set.
func (m RightColumnModel) HasFilter() bool {
	return strings.TrimSpace(m.filterQuery) != ""
}

// moveCursor adjusts the cursor by dir, floored at zero. The widget holds no
// row facts, so it has no upper bound or actionable-row skip to apply here —
// the RELATED panel's actual cursor position, scroll window, and
// skip-to-actionable behavior are owned by the controller
// (core/app/detail_cursor.go) and rendered from
// app.DetailBody.RelatedCursor/RelatedScroll.
func (m *RightColumnModel) moveCursor(dir int) {
	m.cursor += dir
	if m.cursor < 0 {
		m.cursor = 0
	}
}
