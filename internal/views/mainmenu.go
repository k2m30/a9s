package views

import (
	"fmt"
	"strings"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/styles"
)

// MainMenuModel represents the main menu view showing resource type list.
type MainMenuModel struct {
	Items  []resource.ResourceTypeDef
	Cursor int
	Width  int
	Height int
}

// NewMainMenu initializes a MainMenuModel with all known resource types.
func NewMainMenu() MainMenuModel {
	return MainMenuModel{
		Items:  resource.AllResourceTypes(),
		Cursor: 0,
	}
}

// View renders the main menu as a styled list with a cursor indicator.
func (m MainMenuModel) View() string {
	var b strings.Builder
	b.WriteString("\n  AWS Resources\n\n")

	for i, rt := range m.Items {
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}
		line := fmt.Sprintf("  %s%s", cursor, rt.Name)
		if i == m.Cursor {
			line = styles.TableCursorStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n  Press : for commands, ? for help\n")
	return b.String()
}

// MoveUp moves the cursor up by one position, stopping at the top.
func (m *MainMenuModel) MoveUp() {
	if m.Cursor > 0 {
		m.Cursor--
	}
}

// MoveDown moves the cursor down by one position, stopping at the bottom.
func (m *MainMenuModel) MoveDown() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	}
}

// GoTop moves the cursor to the first item.
func (m *MainMenuModel) GoTop() {
	m.Cursor = 0
}

// GoBottom moves the cursor to the last item.
func (m *MainMenuModel) GoBottom() {
	if len(m.Items) > 0 {
		m.Cursor = len(m.Items) - 1
	}
}

// SelectedItem returns the resource type definition at the current cursor position.
func (m MainMenuModel) SelectedItem() resource.ResourceTypeDef {
	if m.Cursor >= 0 && m.Cursor < len(m.Items) {
		return m.Items[m.Cursor]
	}
	return resource.ResourceTypeDef{}
}
