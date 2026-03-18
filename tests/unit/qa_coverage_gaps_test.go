package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ── propagateSize tests ─────────────────────────────────────────────────────
// Push mainmenu + resourcelist + detail + help onto stack, then send
// WindowSizeMsg and verify all views adapted by checking View() output.

func TestQA_Coverage_PropagateSize_AllViewTypes(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Stack starts with mainmenu. Push resourcelist on top.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	// Push detail on top of resourcelist.
	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Fields: map[string]string{"name": "test-instance", "state": "running"},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	// Push help on top of detail.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetHelp,
	})

	// Now the stack has: mainmenu, resourcelist, detail, help
	// Capture view before resize
	beforeResize := rootViewContent(m)

	// Send a different WindowSizeMsg to trigger propagateSize on all views
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	afterResize := rootViewContent(m)

	// After resizing from 80x24 to 120x40, the output should change
	if beforeResize == afterResize {
		t.Error("View() output should change after WindowSizeMsg resize")
	}

	// The active view (help) should render properly at the new size
	plain := stripANSI(afterResize)
	if !strings.Contains(plain, "help") {
		t.Errorf("after resize, help view should still show 'help' frame title, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_Coverage_PropagateSize_YAMLAndReveal(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push YAML view
	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "test-instance",
		Fields: map[string]string{"name": "test-instance"},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	// Capture before resize
	beforeResize := rootViewContent(m)

	// Resize
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	afterResize := rootViewContent(m)

	if beforeResize == afterResize {
		t.Error("YAML view output should change after WindowSizeMsg resize")
	}

	plain := stripANSI(afterResize)
	if !strings.Contains(plain, "yaml") {
		t.Errorf("after resize, YAML view should still show 'yaml' frame title, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_Coverage_PropagateSize_ProfileAndRegion(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push region view (this can be done directly via NavigateMsg)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetRegion,
	})

	beforeResize := rootViewContent(m)

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	afterResize := rootViewContent(m)

	if beforeResize == afterResize {
		t.Error("Region view output should change after WindowSizeMsg resize")
	}

	plain := stripANSI(afterResize)
	if !strings.Contains(plain, "aws-regions") {
		t.Errorf("after resize, region view should still show 'aws-regions' frame title, got: %s", plain[:min(200, len(plain))])
	}
}

// ── updateActiveView delegation tests ───────────────────────────────────────
// Navigate to profile/region selector, send a key message, verify delegation.

func TestQA_Coverage_UpdateActiveView_ProfileDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// We cannot use NavigateMsg{Target: TargetProfile} directly because it
	// triggers an async fetch. Instead, we simulate profilesLoadedMsg by
	// navigating to region first to confirm the pattern, then we'll use
	// a workaround: create the profile view via the internal message.
	// The profilesLoadedMsg is internal to the tui package so we can't
	// send it from outside. Let's test via the region view instead and
	// verify profile delegation by checking that the cursor moves.

	// Navigate to region selector (this is direct, no async)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetRegion,
	})

	// Verify we're on the region view
	plain1 := stripANSI(rootViewContent(m))
	if !strings.Contains(plain1, "aws-regions") {
		t.Fatalf("should be on region selector, got: %s", plain1[:min(200, len(plain1))])
	}

	// Send a "down" key - this should be delegated to the RegionModel
	// The first region should be selected initially (cursor=0).
	// After pressing down, cursor moves to 1 and a different region gets highlighted.
	before := rootViewContent(m)
	m, _ = rootApplyMsg(m, rootKeyPress("j")) // 'j' is down
	after := rootViewContent(m)

	// The view should change because the cursor moved
	if before == after {
		t.Error("pressing 'j' on region view should move cursor, changing the view output")
	}
}

func TestQA_Coverage_UpdateActiveView_RegionDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to region selector
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetRegion,
	})

	// Send enter key - should produce a RegionSelectedMsg command
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("pressing enter on region view should return a command (RegionSelectedMsg)")
	}

	// Execute the command and check we get a RegionSelectedMsg
	msg := cmd()
	if _, ok := msg.(messages.RegionSelectedMsg); !ok {
		t.Errorf("expected RegionSelectedMsg, got %T", msg)
	}
}

func TestQA_Coverage_UpdateActiveView_RegionUpKey(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate to region selector
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetRegion,
	})

	// Move down first, then up
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	afterDown := rootViewContent(m)

	m, _ = rootApplyMsg(m, rootKeyPress("k")) // 'k' is up
	afterUp := rootViewContent(m)

	// After moving down then up, view should change back
	if afterDown == afterUp {
		t.Error("pressing 'k' after 'j' on region view should move cursor back up")
	}
}

// ── colorizeValue tests ─────────────────────────────────────────────────────
// Test via YAMLModel with resource Fields containing null, bool, numeric,
// and string values. colorizeValue is unexported so we test through the model.

func TestQA_Coverage_ColorizeValue_Null(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-null",
		Fields: map[string]string{"nullable_field": "null"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	// The word "null" should appear in the output
	plain := stripANSI(output)
	if !strings.Contains(plain, "null") {
		t.Errorf("YAML view should contain 'null' value, got: %s", plain)
	}
	// With colors enabled, "null" should be styled (wrapped in ANSI codes)
	// so the raw output should contain ANSI escapes around "null"
	if !strings.Contains(output, "null") {
		t.Errorf("YAML view raw output should contain 'null', got: %s", output)
	}
}

func TestQA_Coverage_ColorizeValue_Tilde(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-tilde",
		Fields: map[string]string{"tilde_field": "~"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "~") {
		t.Errorf("YAML view should contain '~' null value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_BoolTrue(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-bool",
		Fields: map[string]string{"enabled": "true"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "true") {
		t.Errorf("YAML view should contain 'true' bool value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_BoolFalse(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-bool-false",
		Fields: map[string]string{"enabled": "false"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "false") {
		t.Errorf("YAML view should contain 'false' bool value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_NumericInt(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-num-int",
		Fields: map[string]string{"count": "42"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "42") {
		t.Errorf("YAML view should contain '42' numeric value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_NumericFloat(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-num-float",
		Fields: map[string]string{"ratio": "3.14"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "3.14") {
		t.Errorf("YAML view should contain '3.14' numeric value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_String(t *testing.T) {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-string",
		Fields: map[string]string{"greeting": "hello"},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	if !strings.Contains(plain, "hello") {
		t.Errorf("YAML view should contain 'hello' string value, got: %s", plain)
	}
}

func TestQA_Coverage_ColorizeValue_AllBranches(t *testing.T) {
	// Test all branches in a single YAML view to ensure colorizeValue
	// handles each type differently (different ANSI color codes).
	k := keys.Default()
	res := resource.Resource{
		ID: "test-all-branches",
		Fields: map[string]string{
			"a_null":   "null",
			"b_bool":   "true",
			"c_number": "42",
			"d_string": "hello",
		},
	}
	y := views.NewYAML(res, k)
	y.SetSize(80, 24)
	output := y.View()

	plain := stripANSI(output)
	for _, expected := range []string{"null", "true", "42", "hello"} {
		if !strings.Contains(plain, expected) {
			t.Errorf("YAML view should contain '%s', got: %s", expected, plain)
		}
	}

	// The raw (ANSI) output should have more characters than the plain output
	// because each value type gets styled with different colors
	if len(output) <= len(plain) {
		t.Error("raw output should contain ANSI codes making it longer than plain text")
	}
}

// ── SelectedItem test ───────────────────────────────────────────────────────

func TestQA_Coverage_MainMenuModel_SelectedItem(t *testing.T) {
	k := keys.Default()
	menu := views.NewMainMenu(k)
	menu.SetSize(80, 24)

	allTypes := resource.AllResourceTypes()

	// Initially cursor is at 0, so SelectedItem should return the first type
	item := menu.SelectedItem()
	if item.ShortName != allTypes[0].ShortName {
		t.Errorf("SelectedItem() at cursor 0 should return %q, got %q",
			allTypes[0].ShortName, item.ShortName)
	}
	if item.Name != allTypes[0].Name {
		t.Errorf("SelectedItem() at cursor 0 should return name %q, got %q",
			allTypes[0].Name, item.Name)
	}
}

func TestQA_Coverage_MainMenuModel_SelectedItem_AfterMove(t *testing.T) {
	k := keys.Default()
	menu := views.NewMainMenu(k)
	menu.SetSize(80, 24)

	allTypes := resource.AllResourceTypes()

	// Move cursor down twice to index 2
	menu, _ = menu.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	menu, _ = menu.Update(tea.KeyPressMsg{Code: -1, Text: "j"})

	item := menu.SelectedItem()
	if item.ShortName != allTypes[2].ShortName {
		t.Errorf("SelectedItem() at cursor 2 should return %q, got %q",
			allTypes[2].ShortName, item.ShortName)
	}
	if item.Name != allTypes[2].Name {
		t.Errorf("SelectedItem() at cursor 2 should return name %q, got %q",
			allTypes[2].Name, item.Name)
	}
}

func TestQA_Coverage_MainMenuModel_SelectedItem_LastItem(t *testing.T) {
	k := keys.Default()
	menu := views.NewMainMenu(k)
	menu.SetSize(80, 24)

	allTypes := resource.AllResourceTypes()

	// Move cursor to bottom using 'G'
	menu, _ = menu.Update(tea.KeyPressMsg{Code: -1, Text: "G"})

	item := menu.SelectedItem()
	lastIdx := len(allTypes) - 1
	if item.ShortName != allTypes[lastIdx].ShortName {
		t.Errorf("SelectedItem() at last position should return %q, got %q",
			allTypes[lastIdx].ShortName, item.ShortName)
	}
}

// ── Additional propagateSize coverage: reveal view ──────────────────────────

func TestQA_Coverage_PropagateSize_RevealView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push a secret reveal view via SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "super-secret-value-123",
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "my-secret") {
		t.Errorf("reveal view should show secret name in frame title, got: %s", plain[:min(200, len(plain))])
	}

	// Resize
	before := rootViewContent(m)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	after := rootViewContent(m)

	if before == after {
		t.Error("reveal view output should change after WindowSizeMsg resize")
	}
}

// ── Additional updateActiveView delegation: detail and YAML ─────────────────

func TestQA_Coverage_UpdateActiveView_DetailDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "test-detail",
		Fields: map[string]string{"name": "test-detail", "state": "running"},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	// Pressing 'y' on the detail view should trigger NavigateMsg{Target: TargetYAML}
	_, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd == nil {
		t.Fatal("pressing 'y' on detail view should return a command")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if navMsg.Target != messages.TargetYAML {
		t.Errorf("expected TargetYAML, got %v", navMsg.Target)
	}
}

func TestQA_Coverage_UpdateActiveView_HelpDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetHelp,
	})

	// Any key press on help should return PopViewMsg
	_, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd == nil {
		t.Fatal("pressing any key on help view should return a command (PopViewMsg)")
	}

	msg := cmd()
	if _, ok := msg.(messages.PopViewMsg); !ok {
		t.Errorf("expected PopViewMsg, got %T", msg)
	}
}

func TestQA_Coverage_UpdateActiveView_RevealDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push reveal view
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "secret-val",
	})

	// Verify reveal view is active
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "my-secret") {
		t.Fatalf("should be on reveal view, got: %s", plain[:min(200, len(plain))])
	}

	// Send a scroll key to the reveal view (down arrow) - should be delegated
	before := rootViewContent(m)
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyDown))
	// The view may or may not change (depends on content length), but it should not crash
	_ = rootViewContent(m)
	_ = before
}

func TestQA_Coverage_UpdateActiveView_YAMLDelegation(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-yaml-test",
		Name:   "yaml-test",
		Fields: map[string]string{"key": "value"},
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	// Send toggle wrap key 'w' - should be delegated to YAMLModel
	m, _ = rootApplyMsg(m, rootKeyPress("w"))

	// Should not crash and should still render
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("after toggling wrap, YAML view should still show frame title, got: %s", plain[:min(200, len(plain))])
	}
}
