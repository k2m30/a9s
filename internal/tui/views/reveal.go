// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/domain"
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
//
// What is PAINTED crosses the text boundary; what is COPIED does not. A secret
// is the one value on any screen the operator takes away to use verbatim, so
// the clipboard hands back the bytes AWS stored (handleCopy reads the raw
// value). Painting those same bytes would let a secret's own escape sequence
// drive the terminal, so the screen shows the inert form — and says so, since
// the two now differ and a reader comparing screen to clipboard deserves to
// know which one they are looking at.
func (m RevealModel) displayValue() string {
	value := domain.Sanitize(m.value)
	out := revealFormat(value)
	if value != m.value {
		out = styles.FlashError.Render("Control characters not shown here; copy (c) yields the value as stored.") + "\n\n" + out
	}
	return out
}

// revealFormat pretty-prints and colorises a JSON secret, and returns anything
// else unchanged.
func revealFormat(value string) string {
	s := strings.TrimSpace(value)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return value
	}
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return value
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return value
	}
	return colorizeJSON(string(pretty))
}
