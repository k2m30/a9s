package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// revealKeyPress creates a tea.KeyPressMsg for a printable character.
func revealKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func TestQA_Reveal_ViewRendersSecretValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	m.SetSize(80, 24)
	out := m.View()
	if !strings.Contains(out, "hunter2") {
		t.Errorf("reveal view should contain secret value 'hunter2', got: %s", out)
	}
}

func TestQA_Reveal_ViewBeforeSetSize(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	out := m.View()
	if out != "Initializing..." {
		t.Errorf("reveal view before SetSize should be 'Initializing...', got: %q", out)
	}
}

// revealScrollStep drives the stored viewport.Model directly and round-trips
// it through RevealModel.GetViewport()/SetViewport(), as production's
// rsKindReveal case in internal/tui/app_stack.go does.
func revealScrollStep(m *views.RevealModel, msg tea.KeyMsg) {
	vp, _ := m.GetViewport().Update(msg)
	m.SetViewport(vp)
}

func TestQA_Reveal_ScrollWithViewport(t *testing.T) {
	k := keys.Default()
	longValue := strings.Repeat("line\n", 20)
	m := views.NewReveal("scroll-test", longValue, k)
	m.SetSize(80, 5)

	revealScrollStep(&m, revealKeyPress("j"))
	out := m.View()
	if out == "" {
		t.Error("View() returned empty after scroll down with j")
	}

	revealScrollStep(&m, revealKeyPress("k"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after scroll up with k")
	}

	// bubbles viewport has no default 'G' binding (charm.land/bubbles/v2/viewport
	// DefaultKeyMap), so this checks the unhandled key does not panic, as with
	// production's raw rs.viewport.Update(msg) dispatch.
	revealScrollStep(&m, revealKeyPress("G"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to bottom with G")
	}

	// No default 'g' binding either.
	revealScrollStep(&m, revealKeyPress("g"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to top with g")
	}

	revealScrollStep(&m, tea.KeyPressMsg{Code: tea.KeyDown})
	revealScrollStep(&m, tea.KeyPressMsg{Code: tea.KeyUp})
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after arrow key scrolling")
	}
}

func TestQA_Reveal_Resize(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("resize-test", "secret-value-here", k)
	m.SetSize(80, 24)
	out1 := m.View()
	if out1 == "" || out1 == "Initializing..." {
		t.Fatalf("View() after first SetSize returned %q", out1)
	}

	m.SetSize(120, 30)
	out2 := m.View()
	if out2 == "" || out2 == "Initializing..." {
		t.Fatalf("View() after resize returned %q", out2)
	}

	if !strings.Contains(out1, "secret-value-here") {
		t.Errorf("View() after first SetSize missing secret value, got: %s", out1)
	}
	if !strings.Contains(out2, "secret-value-here") {
		t.Errorf("View() after resize missing secret value, got: %s", out2)
	}
}

func TestQA_Reveal_MultilineValue(t *testing.T) {
	k := keys.Default()
	var sb strings.Builder
	for i := range 15 {
		sb.WriteString("line-")
		sb.WriteString(strings.Repeat("x", i+1))
		sb.WriteString("\n")
	}
	multiLine := sb.String()

	m := views.NewReveal("multi-test", multiLine, k)
	m.SetSize(80, 5)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatalf("View() returned %q for multi-line value", out)
	}

	if !strings.Contains(out, "line-x") {
		t.Errorf("View() should contain first line 'line-x', got: %s", out)
	}
}

func TestQA_Reveal_EmptyValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("empty", "", k)
	m.SetSize(80, 24)
	out := m.View()

	// Must not be "Initializing..." since SetSize was called
	if out == "Initializing..." {
		t.Error("View() returned 'Initializing...' after SetSize, even with empty value")
	}
}

func TestQA_Reveal_LongValue(t *testing.T) {
	k := keys.Default()
	longVal := strings.Repeat("A", 1500)
	m := views.NewReveal("long-test", longVal, k)
	m.SetSize(80, 24)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatalf("View() returned %q for long value", out)
	}
}

func TestQA_Reveal_EscBubblesUp(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("esc-test", "secret123", k)
	m.SetSize(80, 24)

	// Production routes Esc like any other reveal key: straight to the stored
	// viewport (rsKindReveal in app_stack.go).
	vp, cmd := m.GetViewport().Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.SetViewport(vp)

	// The viewport ignores Esc; cmd is nil or a viewport-internal command.
	_ = cmd

	out := m.View()
	if out == "" {
		t.Error("View() returned empty after Esc key")
	}
	if !strings.Contains(out, "secret123") {
		t.Errorf("View() should still contain 'secret123' after Esc, got: %s", out)
	}
}

func TestQA_Reveal_JSONValue_PrettyPrintedInView(t *testing.T) {
	jsonSecret := `{"api_key":"sk-123456","endpoint":"https://api.example.com","retries":3}`
	k := keys.Default()
	m := views.NewReveal("my-secret", jsonSecret, k)
	m.SetSize(80, 24)

	out := m.View()

	if strings.Contains(out, `{"api_key"`) {
		t.Errorf("reveal view should pretty-print JSON, not show raw blob; got: %s", out)
	}
	if !strings.Contains(out, "api_key") {
		t.Errorf("reveal view should contain key 'api_key'; got: %s", out)
	}
	if !strings.Contains(out, "sk-123456") {
		t.Errorf("reveal view should contain value 'sk-123456'; got: %s", out)
	}
}

func TestQA_Reveal_JSONValue_ColonInKeys_StaysQuoted(t *testing.T) {
	// Secret keys with colons must stay quoted in JSON format (not YAML).
	jsonSecret := `{"enterprise_pass:demo":"This*is*our*1st*BETA","mongodb_pass:root":"secret123"}`
	k := keys.Default()
	m := views.NewReveal("integration_test", jsonSecret, k)
	m.SetSize(80, 24)

	out := m.View()

	if !strings.Contains(out, `"enterprise_pass:demo"`) {
		t.Errorf("reveal view should show quoted key with colon; got: %s", out)
	}
	if !strings.Contains(out, "This*is*our*1st*BETA") {
		t.Errorf("reveal view should show secret value; got: %s", out)
	}
}

func TestQA_Reveal_NonJSON_RenderedAsIs(t *testing.T) {
	plainSecret := "my-plain-password-123"
	k := keys.Default()
	m := views.NewReveal("plain-secret", plainSecret, k)
	m.SetSize(80, 24)

	out := m.View()

	if !strings.Contains(out, plainSecret) {
		t.Errorf("reveal view should show plain secret as-is; got: %s", out)
	}
}
