// theme_selector_test.go — theme selector: (current) marker correctness and
// Enter -> ThemeSelected emission.
//
// views.NewTheme/FrameTitle/View are DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (selector.go). Retargeted onto
// the live seams: NewTransientSelector + app.SelectorBody + RenderSelector for
// the marker-rendering pins (same live path as selector_render_parity_test.go),
// and newLiveSelector (tui_selector_test.go) for the Update()-driven Enter
// selection pin — ThemeSelected is the one message kind not exercised there.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// T037 — theme selector (current) indicator
// ===========================================================================

func TestNewTheme_FrameTitleAndCurrentIndicator(t *testing.T) {
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml"}
	body := app.SelectorBody{
		Items:      themeFiles,
		Selected:   1,
		AllItems:   themeFiles,
		ActiveItem: "dracula.yaml",
		Title:      "themes",
	}
	m := views.NewTransientSelector(80, 24)
	plain := stripANSI(m.RenderSelector(body))

	if !strings.Contains(plain, "dracula.yaml") {
		t.Errorf("RenderSelector output does not contain %q; got:\n%s", "dracula.yaml", plain)
	}
	if !strings.Contains(plain, "(current)") {
		t.Errorf("RenderSelector output does not contain %q for active item; got:\n%s", "(current)", plain)
	}

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
	m := newLiveSelector(themeFiles, "tokyo-night.yaml", "themes", func(s string) tea.Msg {
		return messages.ThemeSelected{Theme: s}
	}, k)

	m, _ = m.Update(selectorKeyPress("j"))

	_, cmd := m.Update(selectorSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on dracula.yaml should produce a command, got nil")
	}

	msg := cmd()
	tsm, ok := msg.(messages.ThemeSelected)
	if !ok {
		t.Fatalf("expected ThemeSelectedMsg, got %T: %v", msg, msg)
	}
	if tsm.Theme != "dracula.yaml" {
		t.Errorf("ThemeSelectedMsg.Theme = %q, want %q", tsm.Theme, "dracula.yaml")
	}
}

// ===========================================================================
// T060 — theme selector marks the correct theme as current, not others
// ===========================================================================

func TestNewTheme_MarksCorrectThemeAsCurrent(t *testing.T) {
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml", "nord.yaml"}
	body := app.SelectorBody{
		Items:      themeFiles,
		Selected:   0,
		AllItems:   themeFiles,
		ActiveItem: "tokyo-night.yaml",
		Title:      "themes",
	}
	m := views.NewTransientSelector(80, 24)
	plain := stripANSI(m.RenderSelector(body))

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

	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "dracula.yaml") && strings.Contains(line, "(current)") {
			t.Errorf("dracula.yaml (non-active) should not have (current) marker; got line: %q", line)
		}
		if strings.Contains(line, "nord.yaml") && strings.Contains(line, "(current)") {
			t.Errorf("nord.yaml (non-active) should not have (current) marker; got line: %q", line)
		}
	}
}
