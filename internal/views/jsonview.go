package views

import (
	"fmt"
	"strings"
)

// JSONViewModel represents a view that displays raw JSON content.
type JSONViewModel struct {
	Title   string
	Content string
	Offset  int // scroll offset
	Width   int
	Height  int
}

// NewJSONView creates a new JSONViewModel.
func NewJSONView(title, jsonContent string) JSONViewModel {
	return JSONViewModel{
		Title:   title,
		Content: jsonContent,
		Offset:  0,
	}
}

// View renders the JSON content.
func (m JSONViewModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if m.Content == "" {
		b.WriteString("  No JSON content available.\n")
		return b.String()
	}

	lines := strings.Split(m.Content, "\n")

	// Apply scroll offset
	start := m.Offset
	if start > len(lines) {
		start = len(lines)
	}

	for i := start; i < len(lines); i++ {
		b.WriteString(fmt.Sprintf("  %s\n", lines[i]))
	}

	return b.String()
}

// ScrollUp scrolls the view up by one line.
func (m *JSONViewModel) ScrollUp() {
	if m.Offset > 0 {
		m.Offset--
	}
}

// ScrollDown scrolls the view down by one line.
func (m *JSONViewModel) ScrollDown() {
	lines := strings.Split(m.Content, "\n")
	if m.Offset < len(lines)-1 {
		m.Offset++
	}
}

// GoTop scrolls to the top of the JSON view.
func (m *JSONViewModel) GoTop() {
	m.Offset = 0
}

// GoBottom scrolls to the bottom of the JSON view.
func (m *JSONViewModel) GoBottom() {
	lines := strings.Split(m.Content, "\n")
	max := len(lines) - m.Height
	if max < 0 {
		max = 0
	}
	m.Offset = max
}
