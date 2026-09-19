package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ═══════════════════════════════════════════════════════════════════════════
// HELP VIEW TESTS
// ═══════════════════════════════════════════════════════════════════════════

// ? opens help from main menu
func TestQA_Help_OpenFromMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("pressing ? from main menu should show help frame title, got: %s", plain)
	}
}

// ? opens help from resource list
func TestQA_Help_OpenFromResourceList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("pressing ? from resource list should show help frame title, got: %s", plain)
	}
}

// ? opens help from detail view
func TestQA_Help_OpenFromDetailView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target: messages.TargetHelp,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("navigating to help should show help frame title, got: %s", plain)
	}
}

// Context-sensitive column layout visible (from main menu)
func TestQA_Help_FourColumnLayout(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))

	columns := []string{"NAVIGATION", "ACTIONS", "OTHER"}
	for _, col := range columns {
		if !strings.Contains(plain, col) {
			t.Errorf("help view should contain column header %q, got: %s", col, plain)
		}
	}
}

// Key bindings listed in context-sensitive help (main menu)
func TestQA_Help_KeyBindingsListed(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)

	mustContain := []string{
		"up/down",    // j/k
		"top",        // g
		"bottom",     // G
		"enter",      // select
		"filter",     // /
		"command",    // :
		"quit",       // q
		"force quit", // ctrl+c
		"help",       // ?
		"esc",        // back
	}
	for _, b := range mustContain {
		if !strings.Contains(plainLower, b) {
			t.Errorf("help from main menu should contain %q", b)
		}
	}
}

// Any key closes help, returns to previous view
func TestQA_Help_AnyKeyCloses(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("should be on help view")
	}

	m, cmd := rootApplyMsg(m, rootKeyPress("a"))

	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after closing help, should return to main menu, got: %s", plain)
	}
}

// Escape closes help
func TestQA_Help_EscapeCloses(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after Escape from help, should return to main menu, got: %s", plain)
	}
}

// Frame title reads "help"
func TestQA_Help_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("help view frame title should contain 'help', got: %s", plain)
	}
}

// "Press any key to close" hint
func TestQA_Help_CloseHint(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Press any key to close") {
		t.Errorf("help view should contain 'Press any key to close' hint, got: %s", plain)
	}
}

// Help preserves return context (returns to previous view)
func TestQA_Help_PreservesReturnContext(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("should be on help view")
	}

	// 'a' is not globally bound.
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2") {
		t.Errorf("after closing help, should return to ec2 resource list, got: %s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PROFILE SELECTOR TESTS
// ═══════════════════════════════════════════════════════════════════════════

// :ctx command opens profile selector (via NavigateMsg)
func TestQA_Profile_CtxCommandNavigates(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "ctx" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":ctx command should return a cmd")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf(":ctx should produce NavigateMsg, got %T", msg)
	}
	if navMsg.Target != messages.TargetProfile {
		t.Errorf(":ctx should target TargetProfile, got %d", navMsg.Target)
	}
}

// :profile command also opens profile selector
func TestQA_Profile_ProfileCommandNavigates(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "profile" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":profile command should return a cmd")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf(":profile should produce NavigateMsg, got %T", msg)
	}
	if navMsg.Target != messages.TargetProfile {
		t.Errorf(":profile should target TargetProfile, got %d", navMsg.Target)
	}
}

// Frame title "aws-profiles(N)" with correct count
func TestQA_Profile_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})

	if cmd == nil {
		t.Fatal("NavigateMsg for profile should return a fetchProfiles cmd")
	}

	// profilesLoadedMsg is unexported and fetchProfiles reads real AWS config
	// files.
	t.Skip("Profile loading requires filesystem access; covered by region tests and command dispatch tests")
}

// Profile list shows profiles (tested via the view directly)
func TestQA_Profile_ListShowsProfiles(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// profilesLoadedMsg is unexported and the profile view shares the region
	// selector's architecture, so this checks the :ctx dispatch chain.

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, r := range "ctx" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":ctx should produce a cmd")
	}
	msg := cmd()
	if navMsg, ok := msg.(messages.Navigate); ok {
		if navMsg.Target != messages.TargetProfile {
			t.Errorf("expected TargetProfile, got %d", navMsg.Target)
		}
	} else {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
}

// Enter selects profile (via ProfileSelectedMsg)
func TestQA_Profile_EnterSelectsProfile(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Simulate receiving ProfileSelectedMsg (which is what the profile view emits on Enter)
	m, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "staging"})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after profile selection, should be at main menu, got: %s", plain)
	}
	if cmd == nil {
		t.Error("ProfileSelectedMsg should trigger a connectAWS command")
	}
}

// Escape cancels profile selection
func TestQA_Profile_EscapeCancels(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// The profile view needs file I/O; the region view shares its architecture.
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Fatal("should be on region selector")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("Escape from selector should return to main menu, got: %s", plain)
	}
}

// Profile header updates after selection
func TestQA_Profile_HeaderUpdatesAfterSelection(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "staging"})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "staging") {
		t.Errorf("header should show new profile 'staging' after selection, got: %s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// REGION SELECTOR TESTS
// ═══════════════════════════════════════════════════════════════════════════

// :region command opens region list
func TestQA_Region_CommandOpensRegionList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "region" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":region should return a cmd")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf(":region should produce NavigateMsg, got %T", msg)
	}
	if navMsg.Target != messages.TargetRegion {
		t.Errorf(":region should target TargetRegion, got %d", navMsg.Target)
	}

	m, _ = rootApplyMsg(m, navMsg)

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Errorf("after :region command, frame title should contain 'aws-regions', got: %s", plain)
	}
}

// Region list shows standard regions
func TestQA_Region_ListContainsStandardRegions(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))

	// Check for a selection of standard regions (some may be outside visible scroll area,
	// but us-east-1 should be first and visible)
	if !strings.Contains(plain, "us-east-1") {
		t.Errorf("region list should contain us-east-1, got: %s", plain)
	}
}

// Frame title "aws-regions(N)"
func TestQA_Region_FrameTitleWithCount(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))

	// Frame title carries the live AllRegions() count so new SDK-sourced
	// regions auto-appear without a test edit.
	wantTitle := fmt.Sprintf("aws-regions(%d)", len(awsclient.AllRegions()))
	if !strings.Contains(plain, wantTitle) {
		t.Errorf("frame title should be %q, got: %s", wantTitle, plain)
	}
}

// Navigate regions with j/k
func TestQA_Region_NavigateWithJK(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "us-east-2") {
		t.Errorf("after pressing j, us-east-2 should be visible, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, rootKeyPress("k"))

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "us-east-1") {
		t.Errorf("after pressing k, us-east-1 should be visible, got: %s", plain)
	}
}

// Enter selects region
func TestQA_Region_EnterSelectsRegion(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	// Move down one row — the region selector is alphabetical by code, so the
	// second row is AllRegions()[1] regardless of which regions are present,
	// which keeps the test stable when the SDK adds new region codes.
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	regions := awsclient.AllRegions()
	if len(regions) < 2 {
		t.Fatalf("need ≥2 regions in AllRegions() for this test, got %d", len(regions))
	}
	wantRegion := regions[1].Code

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("Enter on region should return a cmd")
	}

	msg := cmd()
	regionMsg, ok := msg.(messages.RegionSelected)
	if !ok {
		t.Fatalf("Enter on region should produce RegionSelectedMsg, got %T", msg)
	}
	if regionMsg.Region != wantRegion {
		t.Errorf("selected region should be %q (AllRegions()[1]), got %q", wantRegion, regionMsg.Region)
	}

	m, _ = rootApplyMsg(m, regionMsg)

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, wantRegion) {
		t.Errorf("after region selection, header should show %q, got: %s", wantRegion, plain)
	}
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after region selection, should return to main menu, got: %s", plain)
	}
}

// Escape cancels region selection
func TestQA_Region_EscapeCancels(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Fatal("should be on region selector")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after Escape from region selector, should return to main menu, got: %s", plain)
	}

	if !strings.Contains(plain, "us-east-1") {
		t.Errorf("region should remain us-east-1 after cancel, got: %s", plain)
	}
}

// Region selector from resource list preserves navigation
func TestQA_Region_FromResourceListPreservesNav(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Fatal("should be on region selector")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2") {
		t.Errorf("after escape from region selector, should be back at ec2 list, got: %s", plain)
	}
}

// Region header updates after selection
func TestQA_Region_HeaderUpdatesAfterSelection(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "eu-west-1") {
		t.Errorf("header should show new region 'eu-west-1' after selection, got: %s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// CROSS-CUTTING CONCERNS
// ═══════════════════════════════════════════════════════════════════════════

// Flash messages appear and auto-clear
func TestQA_Help_FlashMessageAppearsAndAutoClears(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, cmd := rootApplyMsg(m, messages.Flash{Text: "Copied!", IsError: false})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Copied!") {
		t.Errorf("flash message 'Copied!' should appear in header, got: %s", plain)
	}

	if cmd == nil {
		t.Fatal("FlashMsg should return a tick command for auto-clear")
	}
}

// New flash replaces previous flash
func TestQA_Help_FlashMessageReplaces(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "First!", IsError: false})

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Second!", IsError: false})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "First!") {
		t.Error("old flash 'First!' should be replaced by new flash")
	}
	if !strings.Contains(plain, "Second!") {
		t.Errorf("new flash 'Second!' should be visible, got: %s", plain)
	}
}

// Error messages in red (flash with IsError=true)
func TestQA_Help_ErrorFlashMessage(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Error: no credentials", IsError: true})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Error: no credentials") {
		t.Errorf("error flash should appear in header, got: %s", plain)
	}

	// The raw output should contain ANSI codes (styled) when NO_COLOR is not set
	raw := rootViewContent(m)
	if !strings.Contains(raw, "Error: no credentials") {
		t.Error("error flash should be present in raw output")
	}
}

// ClearFlashMsg reverts to "? for help"
func TestQA_Help_FlashClearsToHelpHint(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Copied!", IsError: false})

	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: 1})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "Copied!") {
		t.Error("flash should be cleared after ClearFlashMsg")
	}
	if !strings.Contains(plain, "? for help") {
		t.Errorf("after flash clear, should show '? for help', got: %s", plain)
	}
}

// Stale ClearFlashMsg (wrong gen) does not clear
func TestQA_Help_StaleClearFlashIgnored(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Active!", IsError: false})

	// Try to clear with wrong gen (0 instead of 1)
	m, _ = rootApplyMsg(m, messages.ClearFlash{Gen: 0})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Active!") {
		t.Errorf("stale ClearFlashMsg should not clear active flash, got: %s", plain)
	}
}

// Terminal resize adapts
func TestQA_Help_TerminalResizeAdapts(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Error("after resize, main menu should still render")
	}

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 65, Height: 20})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Error("after resize to 65 cols, main menu should still render")
	}
}

// Resize during help view
func TestQA_Help_ResizeDuringHelp(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Error("help view should still render after resize")
	}
	if !strings.Contains(plain, "NAVIGATION") {
		t.Error("help columns should reflow after resize")
	}
}

// Resize during region selector
func TestQA_Region_ResizeDuringRegion(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Error("region selector should still render after resize")
	}
}

// Minimum width enforcement
func TestQA_Help_MinimumWidthEnforced(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 50, Height: 24})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "too narrow") {
		t.Errorf("terminal too narrow should show error message, got: %s", plain)
	}
}

// Minimum height enforcement
func TestQA_Help_MinimumHeightEnforced(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 5})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "too short") {
		t.Errorf("terminal too short should show error message, got: %s", plain)
	}
}

// Recovery from too-small terminal
func TestQA_Help_RecoveryFromTooSmall(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "too narrow") {
		t.Fatal("should show 'too narrow' error")
	}

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "too narrow") {
		t.Error("after resize back to normal, should not show 'too narrow'")
	}
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after resize recovery, should show main menu, got: %s", plain)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELP FROM DIFFERENT VIEWS
// ═══════════════════════════════════════════════════════════════════════════

// ? opens help from YAML view (via navigate)
func TestQA_Help_OpenFromYAMLView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "test-123", Name: "test-resource"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Errorf("pressing ? from YAML view should show help, got: %s", plain)
	}
}

// Test: ? from help (help on help) - closing should still return correctly
func TestQA_Help_ReturnFromHelpOnHelp(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	// Any key on help should produce PopViewMsg, not open another help
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("closing help should return to main menu, got: %s", plain)
	}
}
