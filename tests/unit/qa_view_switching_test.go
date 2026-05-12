package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ════════════════════════════════════════════════════════════════════════════
// QA: Inter-navigation between JSON and YAML views
//
// Verifies that pressing d/y/J in JSON or YAML view emits a NavigateMsg
// targeting the correct view with ReplaceCurrent=true.
// ════════════════════════════════════════════════════════════════════════════

// switchTestResource returns a minimal resource suitable for JSON/YAML view construction.
func switchTestResource() resource.Resource {
	return resource.Resource{
		ID:   "test-1",
		Name: "test-resource",
		Fields: map[string]string{
			"Name": "test",
		},
	}
}

// execCmd runs a cmd and returns the resulting tea.Msg, or nil if cmd is nil.
func execCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ── JSON view ────────────────────────────────────────────────────────────────

// TestJSONView_PressD_EmitsNavigateToDetail verifies that pressing 'd' in the
// JSON view emits a NavigateMsg targeting TargetDetail with ReplaceCurrent=true.
func TestJSONView_PressD_EmitsNavigateToDetail(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewJSON(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "d"})
	msg := execCmd(cmd)

	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'd' in JSON view: expected NavigateMsg, got %T (%v)", msg, msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("NavigateMsg.Target = %v, want TargetDetail (%v)", nav.Target, messages.TargetDetail)
	}
	if !nav.ReplaceCurrent {
		t.Errorf("NavigateMsg.ReplaceCurrent = false, want true")
	}
	if nav.ResourceType != "ec2-instances" {
		t.Errorf("NavigateMsg.ResourceType = %q, want %q", nav.ResourceType, "ec2-instances")
	}
}

// TestJSONView_PressY_EmitsNavigateToYAML verifies that pressing 'y' in the
// JSON view emits a NavigateMsg targeting TargetYAML with ReplaceCurrent=true.
func TestJSONView_PressY_EmitsNavigateToYAML(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewJSON(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "y"})
	msg := execCmd(cmd)

	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'y' in JSON view: expected NavigateMsg, got %T (%v)", msg, msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("NavigateMsg.Target = %v, want TargetYAML (%v)", nav.Target, messages.TargetYAML)
	}
	if !nav.ReplaceCurrent {
		t.Errorf("NavigateMsg.ReplaceCurrent = false, want true")
	}
	if nav.ResourceType != "ec2-instances" {
		t.Errorf("NavigateMsg.ResourceType = %q, want %q", nav.ResourceType, "ec2-instances")
	}
}

// TestJSONView_PressJ_ReturnsNilCmd verifies that pressing 'J' in the JSON view
// returns a nil cmd (no-op — already on JSON).
func TestJSONView_PressJ_ReturnsNilCmd(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewJSON(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "J"})
	if cmd != nil {
		t.Fatalf("pressing 'J' in JSON view: expected nil cmd (already on JSON), got non-nil cmd")
	}
}

// ── YAML view ────────────────────────────────────────────────────────────────

// TestYAMLView_PressD_EmitsNavigateToDetail verifies that pressing 'd' in the
// YAML view emits a NavigateMsg targeting TargetDetail with ReplaceCurrent=true.
func TestYAMLView_PressD_EmitsNavigateToDetail(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewYAML(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "d"})
	msg := execCmd(cmd)

	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'd' in YAML view: expected NavigateMsg, got %T (%v)", msg, msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("NavigateMsg.Target = %v, want TargetDetail (%v)", nav.Target, messages.TargetDetail)
	}
	if !nav.ReplaceCurrent {
		t.Errorf("NavigateMsg.ReplaceCurrent = false, want true")
	}
	if nav.ResourceType != "ec2-instances" {
		t.Errorf("NavigateMsg.ResourceType = %q, want %q", nav.ResourceType, "ec2-instances")
	}
}

// TestYAMLView_PressJ_EmitsNavigateToJSON verifies that pressing 'J' in the
// YAML view emits a NavigateMsg targeting TargetJSON with ReplaceCurrent=true.
func TestYAMLView_PressJ_EmitsNavigateToJSON(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewYAML(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "J"})
	msg := execCmd(cmd)

	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("pressing 'J' in YAML view: expected NavigateMsg, got %T (%v)", msg, msg)
	}
	if nav.Target != messages.TargetJSON {
		t.Errorf("NavigateMsg.Target = %v, want TargetJSON (%v)", nav.Target, messages.TargetJSON)
	}
	if !nav.ReplaceCurrent {
		t.Errorf("NavigateMsg.ReplaceCurrent = false, want true")
	}
	if nav.ResourceType != "ec2-instances" {
		t.Errorf("NavigateMsg.ResourceType = %q, want %q", nav.ResourceType, "ec2-instances")
	}
}

// TestYAMLView_PressY_ReturnsNilCmd verifies that pressing 'y' in the YAML view
// returns a nil cmd (no-op — already on YAML).
func TestYAMLView_PressY_ReturnsNilCmd(t *testing.T) {
	res := switchTestResource()
	k := keys.Default()
	m := views.NewYAML(res, "ec2-instances", k)
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyPressMsg{Code: -1, Text: "y"})
	if cmd != nil {
		t.Fatalf("pressing 'y' in YAML view: expected nil cmd (already on YAML), got non-nil cmd")
	}
}
