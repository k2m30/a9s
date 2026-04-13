package views

import (
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// RevealModel displays a secret value with a persistent red header warning.
type RevealModel struct {
	secretName string
	value      string
	viewport   viewport.Model
	ready      bool
	wrap       bool
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

// Update handles wrap toggle (w) and delegates scroll to viewport.
func (m RevealModel) Update(msg tea.Msg) (RevealModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.ToggleWrap) {
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.viewport.SetContent(m.displayValue())
			return m, nil
		}
	}

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
	m.viewport.SetContent(m.displayValue())
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

// displayValue returns a formatted version of the secret for display.
// JSON values are pretty-printed with indentation; non-JSON values are returned as-is.
// Uses JSON (not YAML) because secret keys often contain colons, which are
// visually ambiguous in YAML's key: value syntax.
func (m RevealModel) displayValue() string {
	s := strings.TrimSpace(m.value)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return m.value
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return m.value
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return m.value
	}
	return colorizeJSON(string(pretty))
}

// HeaderWarning returns the persistent red warning for the header right side.
func (m RevealModel) HeaderWarning() string {
	return styles.FlashError.Render("Secret visible — press esc to close")
}
