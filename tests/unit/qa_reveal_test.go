package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/views"
)

// revealKeyPress creates a tea.KeyPressMsg for a printable character.
func revealKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ════════════════════════════════════════════════════════════════════════════
// 1. View renders secret value after SetSize
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_ViewRendersSecretValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	m.SetSize(80, 24)
	out := m.View()
	if !strings.Contains(out, "hunter2") {
		t.Errorf("reveal view should contain secret value 'hunter2', got: %s", out)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 2. View before SetSize returns "Initializing..."
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_ViewBeforeSetSize(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	out := m.View()
	if out != "Initializing..." {
		t.Errorf("reveal view before SetSize should be 'Initializing...', got: %q", out)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 3. FrameTitle returns secret name
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_FrameTitle(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	title := m.FrameTitle()
	if title != "secret-name" {
		t.Errorf("FrameTitle() = %q, want 'secret-name'", title)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 4. CopyContent returns value and message
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_CopyContent(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	val, msg := m.CopyContent()
	if val != "hunter2" {
		t.Errorf("CopyContent() value = %q, want 'hunter2'", val)
	}
	if msg != "Secret copied to clipboard" {
		t.Errorf("CopyContent() message = %q, want 'Secret copied to clipboard'", msg)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 5. SecretValue returns raw value
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_SecretValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	val := m.SecretValue()
	if val != "hunter2" {
		t.Errorf("SecretValue() = %q, want 'hunter2'", val)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 6. HeaderWarning contains "Secret visible"
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_HeaderWarning(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	warning := m.HeaderWarning()
	plain := stripANSI(warning)
	if !strings.Contains(plain, "Secret visible") {
		t.Errorf("HeaderWarning() stripped = %q, want it to contain 'Secret visible'", plain)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 7. GetHelpContext returns HelpFromReveal
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_GetHelpContext(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("secret-name", "hunter2", k)
	ctx := m.GetHelpContext()
	if ctx != views.HelpFromReveal {
		t.Errorf("GetHelpContext() = %v, want HelpFromReveal (%v)", ctx, views.HelpFromReveal)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 8. Scroll with viewport keys does not panic
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_ScrollWithViewport(t *testing.T) {
	k := keys.Default()
	// 20 lines of content in a 5-line viewport — content will overflow
	longValue := strings.Repeat("line\n", 20)
	m := views.NewReveal("scroll-test", longValue, k)
	m.SetSize(80, 5)

	// Scroll down with 'j'
	m, _ = m.Update(revealKeyPress("j"))
	out := m.View()
	if out == "" {
		t.Error("View() returned empty after scroll down with j")
	}

	// Scroll up with 'k'
	m, _ = m.Update(revealKeyPress("k"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after scroll up with k")
	}

	// Jump to bottom with 'G'
	m, _ = m.Update(revealKeyPress("G"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to bottom with G")
	}

	// Jump to top with 'g'
	m, _ = m.Update(revealKeyPress("g"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to top with g")
	}

	// Arrow keys
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after arrow key scrolling")
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 9. Resize does not panic and produces valid output
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_Resize(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("resize-test", "secret-value-here", k)
	m.SetSize(80, 24)
	out1 := m.View()
	if out1 == "" || out1 == "Initializing..." {
		t.Fatalf("View() after first SetSize returned %q", out1)
	}

	// Resize to a different size
	m.SetSize(120, 30)
	out2 := m.View()
	if out2 == "" || out2 == "Initializing..." {
		t.Fatalf("View() after resize returned %q", out2)
	}

	// Verify both outputs contain the secret value
	if !strings.Contains(out1, "secret-value-here") {
		t.Errorf("View() after first SetSize missing secret value, got: %s", out1)
	}
	if !strings.Contains(out2, "secret-value-here") {
		t.Errorf("View() after resize missing secret value, got: %s", out2)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 10. Multi-line value renders correctly in small viewport
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_MultilineValue(t *testing.T) {
	k := keys.Default()
	var sb strings.Builder
	for i := 0; i < 15; i++ {
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

	// Should contain at least the first line visible in the viewport
	if !strings.Contains(out, "line-x") {
		t.Errorf("View() should contain first line 'line-x', got: %s", out)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 11. Empty value does not panic
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_EmptyValue(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("empty", "", k)
	m.SetSize(80, 24)
	out := m.View()

	// Must not be "Initializing..." since SetSize was called
	if out == "Initializing..." {
		t.Error("View() returned 'Initializing...' after SetSize, even with empty value")
	}

	// FrameTitle should still work
	title := m.FrameTitle()
	if title != "empty" {
		t.Errorf("FrameTitle() = %q, want 'empty'", title)
	}

	// CopyContent should return empty value
	val, msg := m.CopyContent()
	if val != "" {
		t.Errorf("CopyContent() value = %q, want empty string", val)
	}
	if msg != "Secret copied to clipboard" {
		t.Errorf("CopyContent() msg = %q, want 'Secret copied to clipboard'", msg)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 12. Long single-line value does not panic
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_LongValue(t *testing.T) {
	k := keys.Default()
	longVal := strings.Repeat("A", 1500)
	m := views.NewReveal("long-test", longVal, k)
	m.SetSize(80, 24)
	out := m.View()

	if out == "" || out == "Initializing..." {
		t.Fatalf("View() returned %q for long value", out)
	}

	// SecretValue should return the full long value
	if m.SecretValue() != longVal {
		t.Errorf("SecretValue() length = %d, want %d", len(m.SecretValue()), len(longVal))
	}

	// CopyContent should return full value
	val, _ := m.CopyContent()
	if val != longVal {
		t.Errorf("CopyContent() value length = %d, want %d", len(val), len(longVal))
	}
}

// ════════════════════════════════════════════════════════════════════════════
// 13. Esc key bubbles up (viewport ignores it, returns nil cmd)
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_EscBubblesUp(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("esc-test", "secret123", k)
	m.SetSize(80, 24)

	// Send Esc to the reveal model
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	// The reveal model delegates to viewport which does not handle Esc.
	// It should not crash. The cmd may be nil (viewport ignores Esc)
	// or it may be a viewport internal command — either way, no panic.
	_ = cmd

	// Model should still be functional after Esc
	out := m.View()
	if out == "" {
		t.Error("View() returned empty after Esc key")
	}
	if !strings.Contains(out, "secret123") {
		t.Errorf("View() should still contain 'secret123' after Esc, got: %s", out)
	}
}
