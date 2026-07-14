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

// FrameTitle/CopyContent/SecretValue/HeaderWarning/GetHelpContext are DEAD on
// RevealModel per specs/022-codebase-cleanup/wave3-map-text.md (reveal.go:
// "DEAD: ... FrameTitle, CopyContent, SecretValue (copy uses rs.revealValue),
// GetHelpContext, HeaderWarning"). The live equivalents:
//   - copy label + exact revealed value: wave3_text_ports_test.go's
//     TestWave3Port_RevealCopy_CopiesExactValue (drives the real
//     handleCopy/rsKindReveal seam via rs.revealValue, not m.CopyContent()).
//   - GetHelpContext / rs.helpContext for reveal: newRevealRS wires
//     helpContext: views.HelpFromReveal unconditionally (renderer.go); no
//     live branch reads RevealModel.GetHelpContext() at all.

// ════════════════════════════════════════════════════════════════════════════
// 8. Scroll with viewport keys does not panic
// ════════════════════════════════════════════════════════════════════════════

// RevealModel.Update() is DEAD per wave3-map-text.md ("wrap toggle — live is
// rs.revealWrap"): production drives reveal scroll by updating the stored
// viewport.Model DIRECTLY (rsKindReveal case, internal/tui/app_stack.go)
// and round-trips it via RevealModel.GetViewport()/SetViewport() — never
// through RevealModel.Update(). revealScrollStep mirrors that live seam.
func revealScrollStep(m *views.RevealModel, msg tea.KeyMsg) {
	vp, _ := m.GetViewport().Update(msg)
	m.SetViewport(vp)
}

func TestQA_Reveal_ScrollWithViewport(t *testing.T) {
	k := keys.Default()
	// 20 lines of content in a 5-line viewport — content will overflow
	longValue := strings.Repeat("line\n", 20)
	m := views.NewReveal("scroll-test", longValue, k)
	m.SetSize(80, 5)

	// Scroll down with 'j'
	revealScrollStep(&m, revealKeyPress("j"))
	out := m.View()
	if out == "" {
		t.Error("View() returned empty after scroll down with j")
	}

	// Scroll up with 'k'
	revealScrollStep(&m, revealKeyPress("k"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after scroll up with k")
	}

	// Jump to bottom with 'G' — bubbles viewport has no default 'G' binding
	// (see charm.land/bubbles/v2/viewport's DefaultKeyMap): this only proves
	// the unhandled key doesn't panic, same as production's raw
	// rs.viewport.Update(msg) dispatch.
	revealScrollStep(&m, revealKeyPress("G"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to bottom with G")
	}

	// Jump to top with 'g' — same caveat as 'G' above.
	revealScrollStep(&m, revealKeyPress("g"))
	out = m.View()
	if out == "" {
		t.Error("View() returned empty after jump to top with g")
	}

	// Arrow keys
	revealScrollStep(&m, tea.KeyPressMsg{Code: tea.KeyDown})
	revealScrollStep(&m, tea.KeyPressMsg{Code: tea.KeyUp})
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
}

// FrameTitle() is DEAD (no live caller, not resource-type-specific — see
// qa_docdb_test.go's identical retirement note). The empty-value copy-label
// edge case (m.CopyContent() above) is ported onto the live
// handleCopy/rsKindReveal seam as TestWave3Port_RevealCopy_EmptyValue in
// wave3_text_ports_test.go — handleCopy's rsKindReveal branch has no
// empty-guard, so an empty secret still copies with the normal label.

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
}

// SecretValue()/CopyContent() are DEAD (copy uses rs.revealValue directly —
// see reveal.go's DEAD list). The exact-value copy round-trip they checked is
// already covered length-agnostically by wave3_text_ports_test.go's
// TestWave3Port_RevealCopy_CopiesExactValue on the live handleCopy seam; no
// length-specific behavior branch exists to justify a second pin.

// ════════════════════════════════════════════════════════════════════════════
// 13. Esc key bubbles up (viewport ignores it, returns nil cmd)
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_EscBubblesUp(t *testing.T) {
	k := keys.Default()
	m := views.NewReveal("esc-test", "secret123", k)
	m.SetSize(80, 24)

	// Production routes Esc the same way as any other reveal key: straight to
	// the stored viewport (rsKindReveal in app_stack.go), never through
	// RevealModel.Update() — see revealScrollStep's doc comment above.
	vp, cmd := m.GetViewport().Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m.SetViewport(vp)

	// The viewport does not handle Esc. It should not crash. The cmd may be
	// nil (viewport ignores Esc) or it may be a viewport internal command —
	// either way, no panic.
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

// ════════════════════════════════════════════════════════════════════════════
// JSON pretty-printing in reveal view
// ════════════════════════════════════════════════════════════════════════════

func TestQA_Reveal_JSONValue_PrettyPrintedInView(t *testing.T) {
	jsonSecret := `{"api_key":"sk-123456","endpoint":"https://api.example.com","retries":3}`
	k := keys.Default()
	m := views.NewReveal("my-secret", jsonSecret, k)
	m.SetSize(80, 24)

	out := m.View()

	// Should show indented JSON, not the compact single-line blob.
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

	// JSON format preserves quotes around keys with colons — unambiguous.
	if !strings.Contains(out, `"enterprise_pass:demo"`) {
		t.Errorf("reveal view should show quoted key with colon; got: %s", out)
	}
	if !strings.Contains(out, "This*is*our*1st*BETA") {
		t.Errorf("reveal view should show secret value; got: %s", out)
	}
}

// TestQA_Reveal_JSONValue_CopyReturnsRaw's m.CopyContent() pin (dead per
// wave3-map-text.md) is ported onto the live handleCopy/rsKindReveal seam as
// wave3_text_ports_test.go's TestWave3Port_RevealCopy_JSONValueStaysRaw.

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
