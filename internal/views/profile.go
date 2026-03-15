package views

import (
	"fmt"
	"strings"

	"github.com/k2m30/a9s/internal/styles"
)

// ProfileSelectModel represents the profile selector view.
type ProfileSelectModel struct {
	Profiles      []string
	Cursor        int
	ActiveProfile string
	Width, Height int
}

// NewProfileSelect creates a new ProfileSelectModel with the given profiles and active profile.
func NewProfileSelect(profiles []string, activeProfile string) ProfileSelectModel {
	cursor := 0
	for i, p := range profiles {
		if p == activeProfile {
			cursor = i
			break
		}
	}
	return ProfileSelectModel{
		Profiles:      profiles,
		Cursor:        cursor,
		ActiveProfile: activeProfile,
	}
}

// View renders profiles as a list with cursor. The active profile gets a "* " prefix.
func (m ProfileSelectModel) View() string {
	var b strings.Builder
	b.WriteString("\n  Select AWS Profile\n\n")

	for i, p := range m.Profiles {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}
		active := "  "
		if p == m.ActiveProfile {
			active = "* "
		}
		line := fmt.Sprintf("  %s%s%s", cursor, active, p)
		if i == m.Cursor {
			line = styles.TableCursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n  Enter: select | Esc: cancel\n")
	return b.String()
}

// MoveUp moves the cursor up by one position, stopping at the top.
func (m *ProfileSelectModel) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// MoveDown moves the cursor down by one position, stopping at the bottom.
func (m *ProfileSelectModel) MoveDown() {
	if m.Cursor < len(m.Profiles)-1 {
		m.Cursor++
	}
}

// GoTop moves the cursor to the first profile.
func (m *ProfileSelectModel) GoTop() {
	m.Cursor = 0
}

// GoBottom moves the cursor to the last profile.
func (m *ProfileSelectModel) GoBottom() {
	if len(m.Profiles) > 0 {
		m.Cursor = len(m.Profiles) - 1
	}
}

// SelectedProfile returns the profile at the current cursor position.
func (m ProfileSelectModel) SelectedProfile() string {
	if m.Cursor >= 0 && m.Cursor < len(m.Profiles) {
		return m.Profiles[m.Cursor]
	}
	return ""
}
