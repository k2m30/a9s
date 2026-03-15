package views

import (
	"fmt"
	"strings"
)

// RevealModel represents a view that displays revealed secret content.
type RevealModel struct {
	Title   string
	Content string
	Offset  int // scroll offset
	Width   int
	Height  int
}

// NewRevealView creates a new RevealModel.
func NewRevealView(title, content string) RevealModel {
	return RevealModel{
		Title:   title,
		Content: content,
		Offset:  0,
	}
}

// View renders the reveal content.
func (m RevealModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", m.Title))

	if m.Content == "" {
		b.WriteString("  No content available.\n")
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
func (m *RevealModel) ScrollUp() {
	if m.Offset > 0 {
		m.Offset--
	}
}

// ScrollDown scrolls the view down by one line.
func (m *RevealModel) ScrollDown() {
	lines := strings.Split(m.Content, "\n")
	if m.Offset < len(lines)-1 {
		m.Offset++
	}
}

// GoTop scrolls to the top of the reveal view.
func (m *RevealModel) GoTop() {
	m.Offset = 0
}

// GoBottom scrolls to the bottom of the reveal view.
func (m *RevealModel) GoBottom() {
	lines := strings.Split(m.Content, "\n")
	max := len(lines) - m.Height
	if max < 0 {
		max = 0
	}
	m.Offset = max
}
