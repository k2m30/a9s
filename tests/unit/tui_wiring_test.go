package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// ── Clipboard copy tests ────────────────────────────────────────────────────

func TestWiring_CopyInResourceList_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load some resources
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in resource list should return a command for clipboard copy")
	}

	// Execute the command — should return a FlashMsg or CopiedMsg
	msg := cmd()
	switch v := msg.(type) {
	case messages.FlashMsg:
		if v.IsError {
			// Clipboard may fail in CI, but should still produce a FlashMsg
			t.Logf("clipboard copy returned error flash: %s (expected in headless env)", v.Text)
		}
	case messages.CopiedMsg:
		if v.Content != "i-abc123" {
			t.Errorf("CopiedMsg.Content should be 'i-abc123', got %q", v.Content)
		}
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInDetailView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123"},
	}

	// Navigate to detail
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in detail view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK — clipboard may succeed or fail
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInYAMLView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123", "name": "web-server"},
	}

	// Navigate to YAML
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in YAML view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInRevealView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	// Push a reveal view via SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "s3cr3t-value",
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in reveal view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

// ── Refresh (ctrl+r) tests ──────────────────────────────────────────────────

func TestWiring_RefreshInResourceList_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Press ctrl+r to refresh
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("pressing ctrl+r in resource list should return a command for fetching resources")
	}

	// Execute the cmd — should yield APIErrorMsg (nil clients) or ResourcesLoadedMsg
	msg := cmd()
	switch msg.(type) {
	case messages.APIErrorMsg:
		// Expected: no clients initialized
	case messages.ResourcesLoadedMsg:
		// Would happen if clients were set
	case messages.FlashMsg:
		// Refreshing flash is also OK
	default:
		t.Errorf("expected APIErrorMsg or ResourcesLoadedMsg, got %T", msg)
	}
}

func TestWiring_RefreshNotOnMainMenu(t *testing.T) {
	m := newRootSizedModel()

	// Press ctrl+r on the main menu — should not trigger refresh
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// On main menu, ctrl+r should produce nil cmd (no refresh)
	if cmd != nil {
		t.Error("pressing ctrl+r on main menu should not trigger any command")
	}
}

// ── Reveal (x key) tests ────────────────────────────────────────────────────

func TestWiring_RevealForSecrets_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to secrets resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	// Load a secret
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources: []resource.Resource{
			{ID: "my-secret", Name: "my-secret", Status: "active", Fields: map[string]string{"secret_name": "my-secret"}},
		},
	})

	// Press 'x' to reveal
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd == nil {
		t.Fatal("pressing 'x' on secrets resource list should return a reveal fetch command")
	}

	// Execute the cmd — should yield FlashMsg (nil clients) or SecretRevealedMsg
	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// Expected: no clients initialized
	case messages.SecretRevealedMsg:
		// Would happen if clients were set
	default:
		t.Errorf("expected FlashMsg or SecretRevealedMsg, got %T", msg)
	}
}

func TestWiring_RevealNotForNonSecrets(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load a resource
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	// Press 'x' — should NOT trigger reveal
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		// Execute to check it's not a reveal command
		msg := cmd()
		switch msg.(type) {
		case messages.SecretRevealedMsg:
			t.Error("pressing 'x' on non-secrets resource should not trigger reveal")
		case messages.NavigateMsg:
			nm := msg.(messages.NavigateMsg)
			if nm.Target == messages.TargetReveal {
				t.Error("pressing 'x' on non-secrets resource should not navigate to reveal")
			}
		}
	}
}

// ── View config loading tests ───────────────────────────────────────────────

func TestWiring_ViewConfigLoadedOnClientsReady(t *testing.T) {
	m := newRootSizedModel()

	// Send ClientsReadyMsg — viewConfig should be loaded
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil, Err: nil})

	// Navigate to resource list — it should work (viewConfig used internally)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Verify the view renders without panic
	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render resource list view after config loading")
	}
}

func TestWiring_ViewConfigLoadedAtInit(t *testing.T) {
	m := newRootSizedModel()

	// The Init cmd should have loaded the config. Send the InitConnectMsg
	// that Init returns.
	cmd := m.Init()
	if cmd != nil {
		msg := cmd()
		// Apply the InitConnectMsg
		m, _ = rootApplyMsg(m, msg)
	}

	// Navigate to resource list with viewConfig
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Verify it renders
	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render resource list view even without views.yaml file")
	}
}

// ── SecretRevealedMsg push test ─────────────────────────────────────────────

func TestWiring_SecretRevealedMsg_PushesRevealView(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	// Send SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "prod/db-password",
		Value:      "hunter2",
	})

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render reveal view")
	}
	// The frame title should contain the secret name
	if !containsSubstring(plain, "prod/db-password") {
		t.Errorf("reveal view should show secret name in frame title, got: %s", truncateForLog(plain))
	}
}

func TestWiring_SecretRevealedMsg_Error(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		Err: errForTest("access denied"),
	})

	plain := stripANSI(rootViewContent(m))
	if !containsSubstring(plain, "reveal failed") {
		t.Errorf("should show error flash for reveal failure, got: %s", truncateForLog(plain))
	}
}

// ── Helper functions ────────────────────────────────────────────────────────

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func truncateForLog(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

type testError string

func errForTest(msg string) error {
	return testError(msg)
}

func (e testError) Error() string {
	return string(e)
}
