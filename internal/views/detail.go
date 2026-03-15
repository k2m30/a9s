package views

import (
	"fmt"
	"sort"
	"strings"
)

// DetailModel represents a key-value detail view for a single resource.
type DetailModel struct {
	Title  string
	Data   map[string]string
	Keys   []string // ordered keys for display
	Offset int      // scroll offset
	Width  int
	Height int
}

// NewDetailModel creates a new DetailModel with sorted keys.
func NewDetailModel(title string, data map[string]string) DetailModel {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return DetailModel{
		Title:  title,
		Data:   data,
		Keys:   keys,
		Offset: 0,
	}
}

// View renders the detail view as key-value pairs.
func (m DetailModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if len(m.Keys) == 0 {
		b.WriteString("  No details available.\n")
		return b.String()
	}

	// Find the longest key for alignment
	maxKeyLen := 0
	for _, k := range m.Keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	// Apply scroll offset
	end := len(m.Keys)
	start := m.Offset
	if start > end {
		start = end
	}

	for i := start; i < end; i++ {
		k := m.Keys[i]
		v := m.Data[k]
		b.WriteString(fmt.Sprintf("  %-*s : %s\n", maxKeyLen, k, v))
	}

	return b.String()
}

// ScrollUp scrolls the view up by one line.
func (m *DetailModel) ScrollUp() {
	if m.Offset > 0 {
		m.Offset--
	}
}

// ScrollDown scrolls the view down by one line.
func (m *DetailModel) ScrollDown() {
	if m.Offset < len(m.Keys)-1 {
		m.Offset++
	}
}

// GoTop scrolls to the top of the detail view.
func (m *DetailModel) GoTop() {
	m.Offset = 0
}

// GoBottom scrolls to the bottom of the detail view.
func (m *DetailModel) GoBottom() {
	max := len(m.Keys) - m.Height
	if max < 0 {
		max = 0
	}
	m.Offset = max
}
