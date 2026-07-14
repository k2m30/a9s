package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// SelectorModel is a thin delegating renderer for profile/region/theme
// selector screens. The app.Controller is the single source of truth for
// all selector data (items, filter, cursor, activeItem, title). SelectorModel
// owns only renderer state (dimensions, key map) plus the selection callback
// that converts the chosen item into a TUI message.
type SelectorModel struct {
	ctrl     *app.Controller
	onSelect func(string) tea.Msg
	width    int
	height   int
	keys     keys.Map
}

// NewSelectorWithCtrl creates a SelectorModel backed by the provided
// controller. Used by TUI screen builders (screens.go, runtime_adapter_navigate.go)
// where m.ctrl is already wired; the caller is responsible for having already
// called m.ctrl.EnsureSelectorState before construction.
func NewSelectorWithCtrl(ctrl *app.Controller, onSelect func(string) tea.Msg, k keys.Map) SelectorModel {
	return SelectorModel{
		ctrl:     ctrl,
		onSelect: onSelect,
		keys:     k,
	}
}

// Update handles navigation and selection by delegating to the controller.
func (m SelectorModel) Update(msg tea.Msg) (SelectorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
		case key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
		case key.Matches(msg, m.keys.Top):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
		case key.Matches(msg, m.keys.Bottom):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
		case key.Matches(msg, m.keys.PageUp):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: m.height})
		case key.Matches(msg, m.keys.PageDown):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: m.height})
		case key.Matches(msg, m.keys.Enter):
			selected, ok := m.ctrl.SelectorSelected()
			if !ok {
				return m, nil
			}
			onSelect := m.onSelect
			return m, func() tea.Msg {
				return onSelect(selected)
			}
		}
	}
	return m, nil
}

// SetSize updates dimensions.
func (m *SelectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// RenderSelector renders the selector list from a controller-supplied SelectorBody,
// byte-identical to the old View(). The controller owns the logical state (visible
// items, cursor, active-item); the renderer owns dimensions.
func (m *SelectorModel) RenderSelector(body app.SelectorBody) string {
	if len(body.Items) == 0 {
		return "No items available"
	}

	synthetic := NewScrollState(len(body.Items))
	synthetic.SetCursor(body.Selected)
	startRow, endRow := synthetic.VisibleWindow(m.height)

	var sb strings.Builder
	for i := startRow; i < endRow; i++ {
		if i > startRow {
			sb.WriteString("\n")
		}

		item := body.Items[i]
		label := "  " + item
		if item == body.ActiveItem {
			label += " " + styles.DimText.Render("(current)")
		}

		if i == body.Selected {
			sb.WriteString(styles.RowSelected.Width(m.width).Render(label))
		} else {
			sb.WriteString(styles.RowNormal.Render(label))
		}
	}

	return sb.String()
}
