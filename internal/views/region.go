package views

import (
	"fmt"
	"strings"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/styles"
)

// RegionSelectModel represents the region selector view.
type RegionSelectModel struct {
	Regions      []awsclient.AWSRegion
	Cursor       int
	ActiveRegion string
	Width, Height int
}

// NewRegionSelect creates a new RegionSelectModel with the given regions and active region.
func NewRegionSelect(regions []awsclient.AWSRegion, activeRegion string) RegionSelectModel {
	cursor := 0
	for i, r := range regions {
		if r.Code == activeRegion {
			cursor = i
			break
		}
	}
	return RegionSelectModel{
		Regions:      regions,
		Cursor:       cursor,
		ActiveRegion: activeRegion,
	}
}

// View renders regions with code + display name. Active region marked with "*".
func (m RegionSelectModel) View() string {
	var b strings.Builder
	b.WriteString("\n  Select AWS Region\n\n")

	for i, r := range m.Regions {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}
		active := "  "
		if r.Code == m.ActiveRegion {
			active = "* "
		}
		line := fmt.Sprintf("  %s%s%-18s %s", cursor, active, r.Code, r.DisplayName)
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
func (m *RegionSelectModel) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// MoveDown moves the cursor down by one position, stopping at the bottom.
func (m *RegionSelectModel) MoveDown() {
	if m.Cursor < len(m.Regions)-1 {
		m.Cursor++
	}
}

// GoTop moves the cursor to the first region.
func (m *RegionSelectModel) GoTop() {
	m.Cursor = 0
}

// GoBottom moves the cursor to the last region.
func (m *RegionSelectModel) GoBottom() {
	if len(m.Regions) > 0 {
		m.Cursor = len(m.Regions) - 1
	}
}

// SelectedRegion returns the region at the current cursor position.
func (m RegionSelectModel) SelectedRegion() awsclient.AWSRegion {
	if m.Cursor >= 0 && m.Cursor < len(m.Regions) {
		return m.Regions[m.Cursor]
	}
	return awsclient.AWSRegion{}
}
