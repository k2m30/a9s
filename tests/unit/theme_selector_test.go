package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// T037 — NewTheme FrameTitle and (current) indicator
// ===========================================================================

func TestNewTheme_FrameTitleAndCurrentIndicator(t *testing.T) {
	k := keys.Default()
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml"}

	m := views.NewTheme(themeFiles, "dracula.yaml", k)
	m.SetSize(80, 24)

	// FrameTitle must contain "themes" and the item count.
	title := m.FrameTitle()
	if !strings.Contains(title, "themes") {
		t.Errorf("FrameTitle() = %q: expected to contain %q", title, "themes")
	}
	if title != "themes(2)" {
		t.Errorf("FrameTitle() = %q, want %q", title, "themes(2)")
	}

	// View must contain "dracula.yaml" and "(current)" for the active item.
	view := m.View()
	plain := stripANSI(view)
	if !strings.Contains(plain, "dracula.yaml") {
		t.Errorf("View() does not contain %q; got:\n%s", "dracula.yaml", plain)
	}
	if !strings.Contains(plain, "(current)") {
		t.Errorf("View() does not contain %q for active item; got:\n%s", "(current)", plain)
	}

	// Verify (current) appears on the dracula.yaml line specifically.
	found := false
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "dracula.yaml") && strings.Contains(line, "(current)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("dracula.yaml line should have (current) marker; got:\n%s", plain)
	}

	// Non-active items must NOT have (current).
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "tokyo-night.yaml") && strings.Contains(line, "(current)") {
			t.Errorf("tokyo-night.yaml (non-active) should not have (current) marker; got line: %q", line)
		}
	}
}

// ===========================================================================
// T038 — NewTheme Enter returns ThemeSelectedMsg
// ===========================================================================

func TestNewTheme_SelectionReturnsThemeSelectedMsg(t *testing.T) {
	k := keys.Default()
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml"}

	// Cursor starts at index 0 (tokyo-night.yaml). Move down to dracula.yaml.
	m := views.NewTheme(themeFiles, "tokyo-night.yaml", k)
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

	_, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on dracula.yaml should produce a command, got nil")
	}

	msg := cmd()
	tsm, ok := msg.(messages.ThemeSelectedMsg)
	if !ok {
		t.Fatalf("expected ThemeSelectedMsg, got %T: %v", msg, msg)
	}
	if tsm.Theme != "dracula.yaml" {
		t.Errorf("ThemeSelectedMsg.Theme = %q, want %q", tsm.Theme, "dracula.yaml")
	}
}

// ===========================================================================
// T060 — NewTheme marks the correct theme as current, not others
// ===========================================================================

func TestNewTheme_MarksCorrectThemeAsCurrent(t *testing.T) {
	k := keys.Default()
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml", "nord.yaml"}

	m := views.NewTheme(themeFiles, "tokyo-night.yaml", k)
	m.SetSize(80, 24)

	plain := stripANSI(m.View())

	// "tokyo-night.yaml" must have (current).
	activeFound := false
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "tokyo-night.yaml") && strings.Contains(line, "(current)") {
			activeFound = true
			break
		}
	}
	if !activeFound {
		t.Errorf("tokyo-night.yaml (active) should have (current) marker; got:\n%s", plain)
	}

	// Non-active items must NOT have (current).
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "dracula.yaml") && strings.Contains(line, "(current)") {
			t.Errorf("dracula.yaml (non-active) should not have (current) marker; got line: %q", line)
		}
		if strings.Contains(line, "nord.yaml") && strings.Contains(line, "(current)") {
			t.Errorf("nord.yaml (non-active) should not have (current) marker; got line: %q", line)
		}
	}
}
