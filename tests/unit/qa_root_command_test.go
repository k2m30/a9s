package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// Command execution: :root and :main
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_RootCommand_EmitsNavigateToMainMenu verifies that typing `:root` and
// pressing Enter emits NavigateMsg{Target: TargetMainMenu}.
func TestQA_RootCommand_EmitsNavigateToMainMenu(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "root" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":root + Enter should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf(":root should emit NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetMainMenu {
		t.Errorf(":root should emit NavigateMsg{Target: TargetMainMenu}, got target=%d", nav.Target)
	}
}

// TestQA_MainCommand_EmitsNavigateToMainMenu verifies that typing `:main` and
// pressing Enter emits NavigateMsg{Target: TargetMainMenu}.
func TestQA_MainCommand_EmitsNavigateToMainMenu(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "main" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":main + Enter should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf(":main should emit NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetMainMenu {
		t.Errorf(":main should emit NavigateMsg{Target: TargetMainMenu}, got target=%d", nav.Target)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Tab completion for :root and :main
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_RootCommand_TabCompletion verifies that typing `:ro` + Tab completes
// to "root" in the command input buffer (visible in header).
func TestQA_RootCommand_TabCompletion(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("r"))
	m, _ = rootApplyMsg(m, rootKeyPress("o"))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyTab))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":root") {
		t.Errorf("tab after ':ro' should complete to ':root' in header, got:\n%s", plain)
	}
}

// TestQA_MainCommand_TabCompletion verifies that typing `:ma` + Tab completes
// to "main" (it is the only built-in or resource command starting with "ma").
func TestQA_MainCommand_TabCompletion(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("m"))
	m, _ = rootApplyMsg(m, rootKeyPress("a"))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyTab))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":main") {
		t.Errorf("tab after ':ma' should complete to ':main' in header, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Navigate-to-root handling: stack truncation
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_RootCommand_PopsToMainMenu_FromResourceList verifies that sending
// NavigateMsg{Target: TargetMainMenu} from the resource list returns to the
// main menu.
func TestQA_RootCommand_PopsToMainMenu_FromResourceList(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetMainMenu})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after NavigateMsg{TargetMainMenu} from resource list, expected main menu with 'resource-types', got:\n%s", plain)
	}
}

// TestQA_RootCommand_PopsToMainMenu_FromDeepStack verifies that sending
// NavigateMsg{Target: TargetMainMenu} from a deep stack (menu → list → detail)
// returns all the way to the main menu in one step.
func TestQA_RootCommand_PopsToMainMenu_FromDeepStack(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	res := &resource.Resource{ID: "my-bucket", Name: "my-bucket"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetMainMenu})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after NavigateMsg{TargetMainMenu} from deep stack, expected main menu with 'resource-types', got:\n%s", plain)
	}
}

// TestQA_RootCommand_NoopAtMainMenu verifies that sending
// NavigateMsg{Target: TargetMainMenu} when already at the main menu does not
// crash and leaves the model in the main menu state.
func TestQA_RootCommand_NoopAtMainMenu(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetMainMenu})

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Fatal("View() should not be empty after TargetMainMenu at main menu")
	}
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after NavigateMsg{TargetMainMenu} at main menu, still expected 'resource-types', got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Help COMMANDS section — via full app (main menu and resource list)
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_HelpContext_MainMenu_ShowsCommandsSection verifies that the COMMANDS
// section title appears in help opened from the main menu.
func TestQA_HelpContext_MainMenu_ShowsCommandsSection(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("main menu help should contain 'COMMANDS' section, got:\n%s", plain)
	}
}

// TestQA_HelpContext_MainMenu_CommandsSectionContent verifies that the
// individual colon commands appear under the COMMANDS section.
func TestQA_HelpContext_MainMenu_CommandsSectionContent(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("main menu help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}

// TestQA_HelpContext_ResourceList_ShowsCommandsSection verifies that the
// COMMANDS section appears in help opened from a resource list.
func TestQA_HelpContext_ResourceList_ShowsCommandsSection(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("resource list help should contain 'COMMANDS' section, got:\n%s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Help COMMANDS section — direct HelpModel tests (all remaining contexts)
// ═══════════════════════════════════════════════════════════════════════════

// TestQA_HelpContext_Detail_ShowsCommandsSection verifies that the COMMANDS
// section appears in help opened from the detail view.
func TestQA_HelpContext_Detail_ShowsCommandsSection(t *testing.T) {
	h := views.NewHelp(keys.Default(), views.HelpFromDetail)
	h.SetSize(120, 40)
	plain := stripANSI(h.View())

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("detail help should contain 'COMMANDS' section, got:\n%s", plain)
	}
	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("detail help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}

// TestQA_HelpContext_YAML_ShowsCommandsSection verifies that the COMMANDS
// section appears in help opened from the YAML view.
func TestQA_HelpContext_YAML_ShowsCommandsSection(t *testing.T) {
	h := views.NewHelp(keys.Default(), views.HelpFromYAML)
	h.SetSize(120, 40)
	plain := stripANSI(h.View())

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("yaml help should contain 'COMMANDS' section, got:\n%s", plain)
	}
	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("yaml help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}

// TestQA_HelpContext_JSON_ShowsCommandsSection verifies that the COMMANDS
// section appears in help opened from the JSON view.
func TestQA_HelpContext_JSON_ShowsCommandsSection(t *testing.T) {
	h := views.NewHelp(keys.Default(), views.HelpFromJSON)
	h.SetSize(120, 40)
	plain := stripANSI(h.View())

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("json help should contain 'COMMANDS' section, got:\n%s", plain)
	}
	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("json help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}

// TestQA_HelpContext_Selector_ShowsCommandsSection verifies that the COMMANDS
// section appears in help opened from the selector (profile/region picker).
func TestQA_HelpContext_Selector_ShowsCommandsSection(t *testing.T) {
	h := views.NewHelp(keys.Default(), views.HelpFromSelector)
	h.SetSize(120, 40)
	plain := stripANSI(h.View())

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("selector help should contain 'COMMANDS' section, got:\n%s", plain)
	}
	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("selector help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}

// TestQA_HelpContext_Reveal_ShowsCommandsSection verifies that the COMMANDS
// section appears in help opened from the reveal view.
func TestQA_HelpContext_Reveal_ShowsCommandsSection(t *testing.T) {
	h := views.NewHelp(keys.Default(), views.HelpFromReveal)
	h.SetSize(120, 40)
	plain := stripANSI(h.View())

	if !strings.Contains(strings.ToLower(plain), "commands") {
		t.Errorf("reveal help should contain 'COMMANDS' section, got:\n%s", plain)
	}
	mustContain := []string{":q", ":ctx", ":profile", ":region", ":theme", ":help", ":root", ":main"}
	for _, entry := range mustContain {
		if !strings.Contains(plain, entry) {
			t.Errorf("reveal help COMMANDS section should contain %q, got:\n%s", entry, plain)
		}
	}
}
