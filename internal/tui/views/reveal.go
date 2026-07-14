package views

import (
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
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
