package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// SelectorModel is a thin delegating renderer for profile/region/theme
// selector screens. The app.Controller is the single source of truth for
// all selector data (items, filter, cursor, activeItem, title). SelectorModel
// owns only renderer state (dimensions, key map) plus the selection callback
// that converts the chosen item into a TUI message.
//
// The controller is always non-nil: NewSelector constructs a local stub when
// callers do not supply one (e.g. isolated unit tests), mirroring the
// NewMainMenu pattern.
type SelectorModel struct {
	ctrl     *app.Controller
	onSelect func(string) tea.Msg
	width    int
	height   int
	keys     keys.Map
}

// NewSelector creates a SelectorModel. items, activeItem, and title are stored
// in the controller (via EnsureSelectorState on the appropriate screen). When
// no controller is provided (ctrl == nil) a local stub is constructed so that
// isolated unit tests work without a running TUI stack.
func NewSelector(items []string, activeItem, title string, onSelect func(string) tea.Msg, k keys.Map) SelectorModel {
	c := app.New(runtime.Bootstrap("", "", nil))
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenProfileSelector}})
	c.EnsureSelectorState(items, activeItem, title)
	return SelectorModel{
		ctrl:     c,
		onSelect: onSelect,
		keys:     k,
	}
}

// NewProfile returns a SelectorModel configured for AWS profile selection.
func NewProfile(profiles []string, activeProfile string, k keys.Map) SelectorModel {
	return NewSelector(profiles, activeProfile, "aws-profiles", func(s string) tea.Msg {
		return messages.ProfileSelected{Profile: s}
	}, k)
}

// NewRegion returns a SelectorModel configured for AWS region selection.
func NewRegion(regions []string, activeRegion string, k keys.Map) SelectorModel {
	return NewSelector(regions, activeRegion, "aws-regions", func(s string) tea.Msg {
		return messages.RegionSelected{Region: s}
	}, k)
}

// NewTheme returns a SelectorModel configured for theme selection.
func NewTheme(themeFiles []string, activeTheme string, k keys.Map) SelectorModel {
	return NewSelector(themeFiles, activeTheme, "themes", func(s string) tea.Msg {
		return messages.ThemeSelected{Theme: s}
	}, k)
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

// Init implements the view initialization pattern.
func (m SelectorModel) Init() (SelectorModel, tea.Cmd) {
	return m, nil
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

// View renders the selector list by delegating entirely to the controller
// snapshot. The controller is the single source of truth; no data is read
// from the model fields.
func (m SelectorModel) View() string {
	body := m.ctrl.Snapshot().Body.Selector
	if body == nil {
		return "No items available"
	}
	return m.RenderSelector(*body)
}

// SetSize updates dimensions.
func (m *SelectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle delegates to the controller.
func (m SelectorModel) FrameTitle() string {
	return m.ctrl.SelectorFrameTitle()
}

// CopyContent returns empty — nothing to copy from the selector.
func (m SelectorModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns HelpFromSelector.
func (m SelectorModel) GetHelpContext() HelpContext {
	return HelpFromSelector
}

// Title returns the selector's title (e.g. "aws-profiles" or "aws-regions").
func (m SelectorModel) Title() string {
	return m.ctrl.SelectorTitle()
}

// SetFilter delegates filter updates to the controller.
func (m *SelectorModel) SetFilter(text string) {
	m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: text})
}

// GetFilter returns the current filter text from the controller.
func (m *SelectorModel) GetFilter() string {
	return m.ctrl.SelectorFilter()
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
