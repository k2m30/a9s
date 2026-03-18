package views

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// RevealModel displays a secret value with a persistent red header warning.
type RevealModel struct {
	secretName string
	value      string
	viewport   viewport.Model
	ready      bool
	width      int
	height     int
	keys       keys.Map
}

// NewReveal creates a RevealModel.
func NewReveal(secretName, value string, k keys.Map) RevealModel {
	return RevealModel{
		secretName: secretName,
		value:      value,
		keys:       k,
	}
}

// Init implements tea.Model.
func (m RevealModel) Init() (RevealModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; esc pops.
func (m RevealModel) Update(msg tea.Msg) (RevealModel, tea.Cmd) {
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders the secret value.
func (m RevealModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.viewport.View()
}

// SetSize initializes or resizes viewport.
func (m *RevealModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(w)
		m.viewport.SetHeight(h)
	}
	m.viewport.SetContent(m.value)
}

// FrameTitle returns the secret name.
func (m RevealModel) FrameTitle() string {
	return m.secretName
}

// CopyContent returns the secret value for clipboard copy.
func (m RevealModel) CopyContent() (string, string) {
	return m.value, "Secret copied to clipboard"
}

// GetHelpContext returns HelpFromReveal.
func (m RevealModel) GetHelpContext() HelpContext {
	return HelpFromReveal
}

// SecretValue returns the raw secret value for clipboard copy.
func (m RevealModel) SecretValue() string {
	return m.value
}

// HeaderWarning returns the persistent red warning for the header right side.
func (m RevealModel) HeaderWarning() string {
	return styles.FlashError.Render("Secret visible — press esc to close")
}
