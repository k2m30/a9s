package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ---------------------------------------------------------------------------
// C. Filter Mode (/)
// ---------------------------------------------------------------------------

func TestQA_MainMenu_SlashEntersFilterMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	plain := stripANSI(rootViewContent(m))
	// Header should show "/" instead of "? for help"
	if strings.Contains(plain, "? for help") {
		t.Error("after pressing /, header should not show '? for help'")
	}
	if !strings.Contains(plain, "/") {
		t.Error("after pressing /, header should show filter indicator '/'")
	}
}

func TestQA_MainMenu_FilterTextAppearsInHeader(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "dbi" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/dbi") {
		t.Errorf("header should show '/dbi' during filter, got:\n%s", plain)
	}
}

func TestQA_MainMenu_EscClearsFilterMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter filter mode and type something
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "redis" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Press Esc
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Error("after Esc from filter, header should show '? for help'")
	}
	if strings.Contains(plain, "/redis") {
		t.Error("after Esc, filter text should be cleared from header")
	}
}

func TestQA_MainMenu_EnterConfirmsFilterMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "test" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Press Enter to confirm filter (exits filter input mode)
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	plain := stripANSI(rootViewContent(m))
	// Should no longer be in filter mode (header goes back to normal)
	if strings.Contains(plain, "/test") {
		t.Error("after Enter, filter input cursor should be gone from header")
	}
}

func TestQA_MainMenu_BackspaceInFilterRemovesCharacter(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "ec2" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Press Backspace
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyBackspace))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/ec") {
		t.Errorf("after backspace, filter should show '/ec', got:\n%s", plain)
	}
	if strings.Contains(plain, "/ec2") {
		t.Error("after backspace, filter should not contain '/ec2'")
	}
}

// ---------------------------------------------------------------------------
// D. Command Mode (:)
// ---------------------------------------------------------------------------

func TestQA_MainMenu_ColonEntersCommandMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "? for help") {
		t.Error("after pressing :, header should not show '? for help'")
	}
}

func TestQA_MainMenu_CommandTextAppearsInHeader(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "s3" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":s3") {
		t.Errorf("header should show ':s3' during command mode, got:\n%s", plain)
	}
}

func TestQA_MainMenu_CommandNavigateEC2(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "ec2" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":ec2 should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("expected ec2, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandNavigateS3(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "s3" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":s3 should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "s3" {
		t.Errorf("expected s3, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandNavigateRDS(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "dbi" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":dbi should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "dbi" {
		t.Errorf("expected rds, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandNavigateRedis(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "redis" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":redis should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "redis" {
		t.Errorf("expected redis, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandNavigateDocDB(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "dbc" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":dbc should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "dbc" {
		t.Errorf("expected docdb, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandNavigateEKS(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "eks" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":eks should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "eks" {
		t.Errorf("expected eks, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandTabAutocompleteCompletesUniquePrefix(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "he" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyTab))
	plain := stripANSI(rootViewContent(m))
	header := strings.SplitN(plain, "\n", 2)[0]
	if !strings.Contains(header, ":help") {
		t.Fatalf("tab autocomplete should complete ':he' to ':help', got header:\n%s", header)
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("after autocomplete, enter should return a NavigateMsg command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg after autocomplete enter, got %T", msg)
	}
	if nav.Target != messages.TargetHelp {
		t.Fatalf("expected autocomplete to navigate to help, got target %v", nav.Target)
	}
}

func TestQA_MainMenu_CommandTabAutocompleteLeavesAmbiguousPrefixUnchanged(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("e"))
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyTab))

	plain := stripANSI(rootViewContent(m))
	header := strings.SplitN(plain, "\n", 2)[0]
	if !strings.Contains(header, ":e") {
		t.Fatalf("ambiguous prefix should remain ':e', got header:\n%s", header)
	}
	if strings.Contains(header, ":ec2") || strings.Contains(header, ":eks") || strings.Contains(header, ":ebs") {
		t.Fatalf("ambiguous prefix should not autocomplete to a specific command, got header:\n%s", header)
	}
}

func TestQA_MainMenu_CommandNavigateSecrets(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "secrets" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":secrets should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.ResourceType != "secrets" {
		t.Errorf("expected secrets, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_CommandQuit(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("q"))
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":q should return a quit command")
	}
}

func TestQA_MainMenu_CommandQuitLong(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "quit" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":quit should return a quit command")
	}
}

func TestQA_MainMenu_CommandCtx(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "ctx" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":ctx should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetProfile {
		t.Errorf("expected TargetProfile, got %d", nav.Target)
	}
}

func TestQA_MainMenu_CommandRegion(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "region" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal(":region should return a command")
	}
	msg := cmd()
	nav, ok := msg.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetRegion {
		t.Errorf("expected TargetRegion, got %d", nav.Target)
	}
}

func TestQA_MainMenu_UnknownCommandShowsError(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "foobar" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("unknown command should return a FlashMsg command")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Error("unknown command flash should be an error")
	}
	if !strings.Contains(flash.Text, "unknown") {
		t.Errorf("flash text should mention 'unknown', got %q", flash.Text)
	}
}

func TestQA_MainMenu_EscCancelsCommandMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "xyzzy" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	// Verify command text is visible before Esc
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":xyzzy") {
		t.Errorf("before Esc, header should show ':xyzzy', got:\n%s", plain)
	}

	// Press Esc
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Error("after Esc from command mode, header should show '? for help'")
	}
	// Use a unique string that won't appear in the menu aliases
	if strings.Contains(plain, ":xyzzy") {
		t.Error("after Esc, command text ':xyzzy' should not appear in header")
	}
}

// ---------------------------------------------------------------------------
// E. Help Overlay (?)
// ---------------------------------------------------------------------------

func TestQA_MainMenu_HelpOpensOnQuestionMark(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Error("pressing ? should open help view (frame title should contain 'help')")
	}
}

func TestQA_MainMenu_HelpShowsCategories(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	// Context-sensitive help from main menu shows NAVIGATION, ACTIONS, OTHER
	categories := []string{"NAVIGATION", "ACTIONS", "OTHER"}
	for _, cat := range categories {
		if !strings.Contains(plain, cat) {
			t.Errorf("help screen should contain category %q", cat)
		}
	}
}

func TestQA_MainMenu_HelpShowsNavigationKeys(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)
	// Context-sensitive: main menu shows navigation keys with lowercase descriptions
	navBindings := []string{"up/down", "top", "bottom"}
	for _, binding := range navBindings {
		if !strings.Contains(plainLower, binding) {
			t.Errorf("help screen should contain navigation binding %q", binding)
		}
	}
}

func TestQA_MainMenu_HelpShowsGeneralKeys(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	plainLower := strings.ToLower(plain)
	// Context-sensitive: main menu shows quit, command, filter actions
	generalBindings := []string{"quit", "command", "filter"}
	for _, binding := range generalBindings {
		if !strings.Contains(plainLower, binding) {
			t.Errorf("help screen should contain general binding %q", binding)
		}
	}
}

func TestQA_MainMenu_HelpShowsCloseHint(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Press any key to close") {
		t.Error("help screen should show 'Press any key to close' hint")
	}
}

func TestQA_MainMenu_AnyKeyClosesHelp(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Open help
	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	// Press any key (e.g., "a") to close
	m, cmd := rootApplyMsg(m, rootKeyPress("a"))
	// The help view returns a PopViewMsg via cmd
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Error("after closing help, should return to main menu with 'resource-types'")
	}
}

func TestQA_MainMenu_EscClosesHelp(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("?"))

	// Esc on help triggers PopViewMsg through the help's Update (any key pops)
	// But the root intercepts Esc and does popView directly
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Error("Esc should close help and return to main menu")
	}
}

// ---------------------------------------------------------------------------
// F. Quit
// ---------------------------------------------------------------------------

func TestQA_MainMenu_QQuits(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, rootKeyPress("q"))
	if cmd == nil {
		t.Fatal("q should return a quit command")
	}
}

func TestQA_MainMenu_CtrlCQuits(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
}

func TestQA_MainMenu_QInFilterModeDoesNotQuit(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	// Type q
	m, _ = rootApplyMsg(m, rootKeyPress("q"))

	plain := stripANSI(rootViewContent(m))
	// Should show /q in header, not quit
	if !strings.Contains(plain, "/q") {
		t.Error("q in filter mode should be treated as filter text, header should show '/q'")
	}
	// All resource types should still be visible
	if !strings.Contains(plain, "S3 Buckets") {
		t.Error("app should not have quit; resource types should still be visible")
	}
}

func TestQA_MainMenu_QInCommandModeDoesNotQuit(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	// Type q (but don't press Enter)
	m, _ = rootApplyMsg(m, rootKeyPress("q"))

	plain := stripANSI(rootViewContent(m))
	// Should show :q in header, not quit yet
	if !strings.Contains(plain, ":q") {
		t.Error("q in command mode should be treated as command text, header should show ':q'")
	}
}

func TestQA_MainMenu_CtrlCQuitsFromFilterMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "test" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even from filter mode")
	}
}

func TestQA_MainMenu_CtrlCQuitsFromCommandMode(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "test" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even from command mode")
	}
}

// ---------------------------------------------------------------------------
// G. Header Bar
// ---------------------------------------------------------------------------

func TestQA_MainMenu_HeaderShowsAppName(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "a9s") {
		t.Error("header should contain 'a9s'")
	}
}

func TestQA_MainMenu_HeaderShowsVersion(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "v1.0.2") {
		t.Errorf("header should contain 'v1.0.2', got:\n%s", plain)
	}
}

func TestQA_MainMenu_HeaderShowsProfileAndRegion(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "testprofile:us-east-1") {
		t.Errorf("header should contain 'testprofile:us-east-1', got:\n%s", plain)
	}
}

func TestQA_MainMenu_HeaderShowsHelpHint(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Errorf("header should contain '? for help' in normal mode, got:\n%s", plain)
	}
}

func TestQA_MainMenu_HeaderShowsFilterWhenActive(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "eks" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/eks") {
		t.Errorf("header should show '/eks' when filter is active")
	}
}

func TestQA_MainMenu_HeaderShowsCommandWhenActive(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "s3" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":s3") {
		t.Errorf("header should show ':s3' when command is active")
	}
}

func TestQA_MainMenu_HeaderShowsFlashOnError(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.FlashMsg{Text: "Error: unknown command", IsError: true})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Error: unknown command") {
		t.Errorf("header should show flash error text")
	}
}

func TestQA_MainMenu_HeaderShowsFlashOnSuccess(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.FlashMsg{Text: "Copied!", IsError: false})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Copied!") {
		t.Errorf("header should show flash success text")
	}
}

func TestQA_MainMenu_HeaderSpansFullWidth(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	content := rootViewContent(m)
	firstLine := strings.Split(content, "\n")[0]
	vis := lipglossWidth(firstLine)
	if vis != 80 {
		t.Errorf("header should span full terminal width 80, got %d", vis)
	}
}

// ---------------------------------------------------------------------------
// H. Frame / Border
// ---------------------------------------------------------------------------

func TestQA_MainMenu_FrameTitle(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	expectedTitle := fmt.Sprintf("resource-types(%d)", len(resource.AllResourceTypes()))
	if !strings.Contains(plain, expectedTitle) {
		t.Errorf("frame title should be %q, got:\n%s", expectedTitle, plain)
	}
}

func TestQA_MainMenu_FrameHasBorders(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "\u250c") {
		t.Error("frame should have top-left corner")
	}
	if !strings.Contains(plain, "\u2510") {
		t.Error("frame should have top-right corner")
	}
	if !strings.Contains(plain, "\u2514") {
		t.Error("frame should have bottom-left corner")
	}
	if !strings.Contains(plain, "\u2518") {
		t.Error("frame should have bottom-right corner")
	}
	if !strings.Contains(plain, "\u2502") {
		t.Error("frame should have vertical border")
	}
}

func TestQA_MainMenu_FrameFillsRemainingHeight(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	content := rootViewContent(m)
	lines := strings.Split(content, "\n")
	// Terminal height is 40, should have exactly 40 lines
	if len(lines) != 40 {
		t.Errorf("expected 40 lines total, got %d", len(lines))
	}
}

func TestQA_MainMenu_ContentRowsBoundedByVerticalBars(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	content := rootViewContent(m)
	lines := strings.Split(content, "\n")

	// Lines 1 through height-2 (0-indexed: 1 to 22) are content rows
	// They should be bounded by vertical bars
	for i := 2; i < len(lines)-1; i++ {
		plain := stripANSI(lines[i])
		if len(plain) == 0 {
			continue
		}
		if !strings.HasPrefix(plain, "\u2502") {
			t.Errorf("content line %d should start with vertical bar, got: %q", i, plain[:1])
		}
		if !strings.HasSuffix(plain, "\u2502") {
			t.Errorf("content line %d should end with vertical bar", i)
		}
	}
}

// ---------------------------------------------------------------------------
// I. Terminal Size Constraints
// ---------------------------------------------------------------------------

func TestQA_MainMenu_NarrowTerminalShowsError(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 50, Height: 24})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "too narrow") {
		t.Errorf("narrow terminal should show 'too narrow' error, got: %q", plain)
	}
}

func TestQA_MainMenu_ShortTerminalShowsError(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 5})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "too short") {
		t.Errorf("short terminal should show 'too short' error, got: %q", plain)
	}
}

func TestQA_MainMenu_ExactMinWidthRendersCorrectly(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 60, Height: 24})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "too narrow") {
		t.Error("60 columns should not trigger narrow error")
	}
	if !strings.Contains(plain, "resource-types") {
		t.Error("60 columns should still render main menu")
	}
}

func TestQA_MainMenu_ExactMinHeightRendersCorrectly(t *testing.T) {
	tui.Version = "1.0.2"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 7})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "too short") {
		t.Error("7 lines should not trigger short error")
	}
	if !strings.Contains(plain, "resource-types") {
		t.Error("7 lines should still render main menu")
	}
}

// ---------------------------------------------------------------------------
// J. Combined / Edge Case Interactions
// ---------------------------------------------------------------------------

func TestQA_MainMenu_CommandModeOverridesNormalKeys(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Press j, k, g, G, q, ? -- all should be treated as text
	for _, ch := range "jkgGq?" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":jkgGq?") {
		t.Errorf("in command mode, all chars should be command text; header should show ':jkgGq?', got:\n%s", plain)
	}
}

func TestQA_MainMenu_FilterModeOverridesNormalKeys(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	// Press j, k, g, G, q, ? -- all should be treated as filter text
	for _, ch := range "jkgGq?" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/jkgGq?") {
		t.Errorf("in filter mode, all chars should be filter text; header should show '/jkgGq?', got:\n%s", plain)
	}
}

func TestQA_MainMenu_EscInNormalModeIsNoop(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Esc on main menu with no filter should be a no-op (NOT quit).
	// Only q and ctrl+c should quit.
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	if cmd != nil {
		t.Error("Esc on main menu should be a no-op (nil cmd), not quit the app")
	}
}

func TestQA_MainMenu_HeaderTransitionsBetweenModes(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Normal mode: "? for help"
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Error("step 1: normal mode should show '? for help'")
	}

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, ch := range "s3" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/s3") {
		t.Error("step 2: filter mode should show '/s3'")
	}

	// Esc clears filter
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Error("step 3: after Esc, should show '? for help'")
	}

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "eks" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":eks") {
		t.Error("step 4: command mode should show ':eks'")
	}

	// Esc cancels command
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "? for help") {
		t.Error("step 5: after Esc, should show '? for help'")
	}
}

func TestQA_MainMenu_OnlyOneInputModeActive_FilterBlocksCommand(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	// Type : -- should be added to filter text, not enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/:") {
		t.Error("pressing : in filter mode should add to filter text, not enter command mode")
	}
}

func TestQA_MainMenu_OnlyOneInputModeActive_CommandBlocksFilter(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	// Type / -- should be added to command text, not enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":/") {
		t.Error("pressing / in command mode should add to command text, not enter filter mode")
	}
}

func TestQA_MainMenu_SelectionPersistsAcrossGAndShiftG(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// G to bottom
	m, _ = rootApplyMsg(m, rootKeyPress("G"))
	// g to top
	m, _ = rootApplyMsg(m, rootKeyPress("g"))
	// G to bottom again
	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType != "ses" {
		t.Errorf("after G, g, G, should be on ses, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_FlashClearsAfterClearMsg(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Send flash
	m, cmd := rootApplyMsg(m, messages.FlashMsg{Text: "test flash", IsError: false})

	// Flash should be visible
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test flash") {
		t.Error("flash should be visible immediately")
	}

	// Execute the tick cmd to get the ClearFlashMsg
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Flash should be cleared
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "test flash") {
		t.Error("flash should be cleared after ClearFlashMsg")
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("after flash clears, header should return to '? for help'")
	}
}

func TestQA_MainMenu_WindowResizeMaintainsState(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	// Move cursor down a few times
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	// Resize
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Verify all 10 resources still visible and we can navigate
	plain := stripANSI(rootViewContent(m))
	expectedTitle := fmt.Sprintf("resource-types(%d)", len(resource.AllResourceTypes()))
	if !strings.Contains(plain, expectedTitle) {
		t.Errorf("after resize, frame title should still show %q", expectedTitle)
	}

	// Cursor should still be at index 2 (ECS Clusters)
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command after resize")
	}
	msg := cmd()
	nav := msg.(messages.NavigateMsg)
	if nav.ResourceType != "ecs" {
		t.Errorf("cursor should remain at ecs after resize, got %q", nav.ResourceType)
	}
}

func TestQA_MainMenu_NoLineExceedsWidth(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	content := rootViewContent(m)
	for i, line := range strings.Split(content, "\n") {
		vis := lipglossWidth(line)
		if vis > 80 {
			t.Errorf("line %d exceeds terminal width 80: got %d", i, vis)
		}
	}
}
