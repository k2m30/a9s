package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// ProfileModel is a tea.Model for the AWS profile selector.
type ProfileModel struct {
	allProfiles      []string
	filteredProfiles []string
	filterText       string
	activeProfile    string
	cursor           int
	width            int
	height           int
	keys             keys.Map
}

// NewProfile returns a ProfileModel populated with profile names.
func NewProfile(profiles []string, activeProfile string, k keys.Map) ProfileModel {
	return ProfileModel{
		allProfiles:      profiles,
		filteredProfiles: profiles,
		activeProfile:    activeProfile,
		keys:             k,
	}
}

// Init implements tea.Model.
func (m ProfileModel) Init() (ProfileModel, tea.Cmd) {
	return m, nil
}

// Update handles navigation and selection.
func (m ProfileModel) Update(msg tea.Msg) (ProfileModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredProfiles)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if len(m.filteredProfiles) > 0 && m.cursor < len(m.filteredProfiles) {
				selected := m.filteredProfiles[m.cursor]
				return m, func() tea.Msg {
					return messages.ProfileSelectedMsg{Profile: selected}
				}
			}
		}
	}
	return m, nil
}

// View renders the profile list.
func (m ProfileModel) View() string {
	if len(m.filteredProfiles) == 0 {
		return "No profiles available"
	}

	var sb strings.Builder
	for i, p := range m.filteredProfiles {
		if i > 0 {
			sb.WriteString("\n")
		}

		label := "  " + p
		if p == m.activeProfile {
			label += " " + styles.DimText.Render("(current)")
		}

		if i == m.cursor {
			sb.WriteString(styles.RowSelected.Width(m.width).Render(label))
		} else {
			sb.WriteString(styles.RowNormal.Render(label))
		}
	}

	return sb.String()
}

// SetSize updates dimensions.
func (m *ProfileModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns e.g. "aws-profiles(6)".
func (m ProfileModel) FrameTitle() string {
	total := len(m.allProfiles)
	filtered := len(m.filteredProfiles)
	if m.filterText != "" && filtered != total {
		return "aws-profiles(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return "aws-profiles(" + itoa(total) + ")"
}

// CopyContent returns empty — nothing to copy from the profile selector.
func (m ProfileModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns HelpFromSelector.
func (m ProfileModel) GetHelpContext() HelpContext {
	return HelpFromSelector
}

// SetFilter applies a filter to profiles; cursor resets to 0.
func (m *ProfileModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.cursor = 0
}

// GetFilter returns the current filter text.
func (m *ProfileModel) GetFilter() string {
	return m.filterText
}



// applyFilter filters allProfiles into filteredProfiles.
func (m *ProfileModel) applyFilter() {
	if m.filterText == "" {
		m.filteredProfiles = m.allProfiles
		return
	}
	q := strings.ToLower(m.filterText)
	result := make([]string, 0)
	for _, p := range m.allProfiles {
		if strings.Contains(strings.ToLower(p), q) {
			result = append(result, p)
		}
	}
	m.filteredProfiles = result
}
