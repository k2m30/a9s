package unit

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/ui"
)

// ===========================================================================
// CommandInput tests
// ===========================================================================

func TestCommandInput_NewCommandInput_DefaultCommands(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	if len(ci.Suggestions) == 0 {
		t.Fatal("NewCommandInput(nil) should populate default commands")
	}
	// Verify some known default commands are present.
	want := map[string]bool{"ec2": false, "rds": false, "s3": false, "quit": false, "region": false}
	for _, s := range ci.Suggestions {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Errorf("expected default command %q in suggestions", k)
		}
	}
}

func TestCommandInput_NewCommandInput_CustomCommands(t *testing.T) {
	custom := []string{"alpha", "beta"}
	ci := ui.NewCommandInput(custom)
	if len(ci.Suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(ci.Suggestions))
	}
	if ci.Suggestions[0] != "alpha" || ci.Suggestions[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", ci.Suggestions)
	}
}

func TestCommandInput_HandleKey_AppendChar(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	executed, cmd := ci.HandleKey("a")
	if executed {
		t.Error("single char should not finalize input")
	}
	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if ci.Text != "a" {
		t.Errorf("expected Text=%q, got %q", "a", ci.Text)
	}
	if ci.Cursor != 1 {
		t.Errorf("expected Cursor=1, got %d", ci.Cursor)
	}
}

func TestCommandInput_HandleKey_Enter(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "ec2"
	ci.Cursor = 3
	executed, cmd := ci.HandleKey("enter")
	if !executed {
		t.Error("enter should finalize input")
	}
	if cmd != "ec2" {
		t.Errorf("expected command %q, got %q", "ec2", cmd)
	}
	// After enter, state should be reset.
	if ci.Text != "" {
		t.Errorf("expected Text cleared after enter, got %q", ci.Text)
	}
	if ci.Cursor != 0 {
		t.Errorf("expected Cursor=0 after enter, got %d", ci.Cursor)
	}
}

func TestCommandInput_HandleKey_Escape(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "some"
	ci.Cursor = 4
	ci.Active = true
	executed, cmd := ci.HandleKey("escape")
	if !executed {
		t.Error("escape should finalize input")
	}
	if cmd != "" {
		t.Errorf("escape should return empty command, got %q", cmd)
	}
	if ci.Text != "" {
		t.Errorf("expected Text cleared after escape, got %q", ci.Text)
	}
	if ci.Active {
		t.Error("expected Active=false after escape")
	}
}

func TestCommandInput_HandleKey_Backspace(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "abc"
	ci.Cursor = 3
	executed, cmd := ci.HandleKey("backspace")
	if executed {
		t.Error("backspace should not finalize input")
	}
	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if ci.Text != "ab" {
		t.Errorf("expected Text=%q, got %q", "ab", ci.Text)
	}
	if ci.Cursor != 2 {
		t.Errorf("expected Cursor=2, got %d", ci.Cursor)
	}
}

func TestCommandInput_HandleKey_BackspaceOnEmpty(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	executed, cmd := ci.HandleKey("backspace")
	if executed {
		t.Error("backspace on empty should not finalize")
	}
	if cmd != "" {
		t.Errorf("expected empty command, got %q", cmd)
	}
	if ci.Text != "" {
		t.Errorf("expected empty Text, got %q", ci.Text)
	}
	if ci.Cursor != 0 {
		t.Errorf("expected Cursor=0, got %d", ci.Cursor)
	}
}

func TestCommandInput_HandleKey_MultiCharThenEnter(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	for _, ch := range "rds" {
		ci.HandleKey(string(ch))
	}
	if ci.Text != "rds" {
		t.Fatalf("expected Text=%q after typing, got %q", "rds", ci.Text)
	}
	executed, cmd := ci.HandleKey("enter")
	if !executed {
		t.Error("enter should finalize")
	}
	if cmd != "rds" {
		t.Errorf("expected command %q, got %q", "rds", cmd)
	}
}

func TestCommandInput_HandleKey_IgnoresMultiRuneKey(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	// Keys longer than 1 rune (that aren't special) are ignored.
	ci.HandleKey("ctrl+c")
	if ci.Text != "" {
		t.Errorf("expected empty Text for multi-rune key, got %q", ci.Text)
	}
}

func TestCommandInput_BestMatch_EC2(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "ec"
	match := ci.BestMatch()
	if match != "ec2" {
		t.Errorf("expected BestMatch=%q for %q, got %q", "ec2", "ec", match)
	}
}

func TestCommandInput_BestMatch_R(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "r"
	match := ci.BestMatch()
	// "r" should match the first suggestion starting with "r"
	// From defaultCommands: "region", "rds", "redis" — order is region first.
	validMatches := []string{"rds", "redis", "region", "root"}
	found := false
	for _, v := range validMatches {
		if match == v {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected BestMatch for %q to be one of %v, got %q", "r", validMatches, match)
	}
}

func TestCommandInput_BestMatch_Quit(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "q"
	match := ci.BestMatch()
	if match != "q" {
		t.Errorf("expected BestMatch=%q for %q, got %q", "q", "q", match)
	}
}

func TestCommandInput_BestMatch_Empty(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = ""
	match := ci.BestMatch()
	if match != "" {
		t.Errorf("expected empty BestMatch for empty text, got %q", match)
	}
}

func TestCommandInput_BestMatch_NoMatch(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "xyz"
	match := ci.BestMatch()
	if match != "" {
		t.Errorf("expected empty BestMatch for %q, got %q", "xyz", match)
	}
}

func TestCommandInput_BestMatch_CaseInsensitive(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "EC"
	match := ci.BestMatch()
	if match != "ec2" {
		t.Errorf("expected case-insensitive BestMatch=%q, got %q", "ec2", match)
	}
}

func TestCommandInput_View_ContainsColonAndText(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "ec"
	view := ci.View()
	if !strings.HasPrefix(view, ":") {
		t.Errorf("View should start with ':', got %q", view)
	}
	if !strings.Contains(view, "ec") {
		t.Errorf("View should contain typed text 'ec', got %q", view)
	}
}

func TestCommandInput_View_EmptyText(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	view := ci.View()
	if view != ":" {
		t.Errorf("View with empty text should be ':', got %q", view)
	}
}

func TestCommandInput_View_ContainsAutocompleteSuffix(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "ec"
	view := ci.View()
	// The view should contain "ec" and then the autocomplete suffix "2"
	// (possibly with ANSI styling). Check that the rendered output has
	// more content than just ":ec".
	if len(view) <= len(":ec") {
		t.Errorf("View should include autocomplete suffix, got %q", view)
	}
}

func TestCommandInput_Reset(t *testing.T) {
	ci := ui.NewCommandInput(nil)
	ci.Text = "test"
	ci.Cursor = 4
	ci.Active = true
	ci.Reset()
	if ci.Text != "" {
		t.Errorf("expected Text cleared, got %q", ci.Text)
	}
	if ci.Cursor != 0 {
		t.Errorf("expected Cursor=0, got %d", ci.Cursor)
	}
	if ci.Active {
		t.Error("expected Active=false after Reset")
	}
}

// ===========================================================================
// RenderHeader tests
// ===========================================================================

func TestRenderHeader_Normal(t *testing.T) {
	out := ui.RenderHeader("a9s", "1.0", "dev", "us-east-1", false, 80)
	if !strings.Contains(out, "a9s") {
		t.Errorf("header should contain app name, got %q", out)
	}
	if !strings.Contains(out, "dev") {
		t.Errorf("header should contain profile, got %q", out)
	}
	if !strings.Contains(out, "us-east-1") {
		t.Errorf("header should contain region, got %q", out)
	}
}

func TestRenderHeader_Loading(t *testing.T) {
	out := ui.RenderHeader("a9s", "1.0", "dev", "us-east-1", true, 80)
	if !strings.Contains(out, "a9s") {
		t.Errorf("loading header should contain app name, got %q", out)
	}
	// The spinner character is U+28BE (⣾)
	if !strings.Contains(out, "\u28BE") {
		t.Errorf("loading header should contain spinner character, got %q", out)
	}
}

func TestRenderHeader_WidthRespected(t *testing.T) {
	out := ui.RenderHeader("a9s", "1.0", "prod", "eu-west-1", false, 60)
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		// Strip ANSI escape codes for length measurement
		cleaned := stripANSI(line)
		if len(cleaned) > 60 {
			t.Errorf("header line exceeds width 60: len=%d, line=%q", len(cleaned), cleaned)
		}
	}
}

func TestRenderHeader_EmptyProfileRegion(t *testing.T) {
	// Should not panic with empty strings.
	out := ui.RenderHeader("a9s", "0.1", "", "", false, 80)
	if !strings.Contains(out, "a9s") {
		t.Errorf("header should contain app name even with empty profile/region, got %q", out)
	}
}

func TestRenderHeader_ZeroWidth(t *testing.T) {
	// Width=0 means no width constraint; should not panic.
	out := ui.RenderHeader("a9s", "1.0", "dev", "us-west-2", false, 0)
	if !strings.Contains(out, "a9s") {
		t.Errorf("header with width=0 should still contain app name, got %q", out)
	}
}

func TestRenderHeader_LoadingZeroWidth(t *testing.T) {
	out := ui.RenderHeader("a9s", "1.0", "dev", "us-west-2", true, 0)
	if !strings.Contains(out, "\u28BE") {
		t.Errorf("loading header with width=0 should still contain spinner, got %q", out)
	}
}

func TestRenderHeader_ContainsVersion(t *testing.T) {
	out := ui.RenderHeader("a9s", "2.5.3", "default", "ap-southeast-1", false, 100)
	if !strings.Contains(out, "2.5.3") {
		t.Errorf("header should contain version string, got %q", out)
	}
}

// ===========================================================================
// RenderStatusBar tests
// ===========================================================================

func TestRenderStatusBar_NormalMode(t *testing.T) {
	out := ui.RenderStatusBar(ui.NormalMode, "", 0, false, 80)
	for _, hint := range []string{"? help", ": command", "/ filter"} {
		if !strings.Contains(out, hint) {
			t.Errorf("NormalMode status bar should contain %q, got %q", hint, out)
		}
	}
}

func TestRenderStatusBar_CommandMode(t *testing.T) {
	out := ui.RenderStatusBar(ui.CommandMode, "ec2", 0, false, 80)
	if !strings.Contains(out, ":") {
		t.Errorf("CommandMode status bar should contain ':', got %q", out)
	}
	if !strings.Contains(out, "ec2") {
		t.Errorf("CommandMode status bar should contain command text, got %q", out)
	}
}

func TestRenderStatusBar_FilterMode(t *testing.T) {
	out := ui.RenderStatusBar(ui.FilterMode, "web", 5, false, 80)
	if !strings.Contains(out, "/") {
		t.Errorf("FilterMode status bar should contain '/', got %q", out)
	}
	if !strings.Contains(out, "web") {
		t.Errorf("FilterMode status bar should contain filter text, got %q", out)
	}
	if !strings.Contains(out, "5") {
		t.Errorf("FilterMode status bar should contain match count, got %q", out)
	}
}

func TestRenderStatusBar_FilterMode_ZeroMatches(t *testing.T) {
	out := ui.RenderStatusBar(ui.FilterMode, "nonexistent", 0, false, 80)
	if !strings.Contains(out, "0 matches") {
		t.Errorf("FilterMode status bar should show '0 matches', got %q", out)
	}
}

func TestRenderStatusBar_ErrorMode(t *testing.T) {
	out := ui.RenderStatusBar(ui.ErrorMode, "connection failed", 0, true, 80)
	if !strings.Contains(out, "connection failed") {
		t.Errorf("ErrorMode status bar should contain error text, got %q", out)
	}
}

func TestRenderStatusBar_ErrorMode_ZeroWidth(t *testing.T) {
	out := ui.RenderStatusBar(ui.ErrorMode, "error occurred", 0, true, 0)
	if !strings.Contains(out, "error occurred") {
		t.Errorf("ErrorMode with width=0 should still contain error text, got %q", out)
	}
}

func TestRenderStatusBar_LoadingMode(t *testing.T) {
	out := ui.RenderStatusBar(ui.LoadingMode, "", 0, false, 80)
	if !strings.Contains(out, "Loading") {
		t.Errorf("LoadingMode status bar should contain 'Loading', got %q", out)
	}
}

func TestRenderStatusBar_Width0_NoPanic(t *testing.T) {
	// No width constraint; should not panic in any mode.
	modes := []ui.StatusMode{ui.NormalMode, ui.CommandMode, ui.FilterMode, ui.ErrorMode, ui.LoadingMode}
	for _, mode := range modes {
		out := ui.RenderStatusBar(mode, "test", 3, false, 0)
		if out == "" && mode != ui.NormalMode {
			// At minimum, some output is expected for most modes.
		}
		// The real test is that no panic occurred.
	}
}

func TestRenderStatusBar_CommandMode_EmptyText(t *testing.T) {
	out := ui.RenderStatusBar(ui.CommandMode, "", 0, false, 80)
	if !strings.Contains(out, ":") {
		t.Errorf("CommandMode with empty text should still contain ':', got %q", out)
	}
}

func TestRenderStatusBar_NormalMode_QuitHint(t *testing.T) {
	out := ui.RenderStatusBar(ui.NormalMode, "", 0, false, 80)
	if !strings.Contains(out, "q quit") {
		t.Errorf("NormalMode should contain 'q quit' hint, got %q", out)
	}
}

// ===========================================================================
// RenderBreadcrumbs tests
// ===========================================================================

func TestRenderBreadcrumbs_SingleSegment(t *testing.T) {
	out := ui.RenderBreadcrumbs([]string{"main"}, 80)
	if !strings.Contains(out, "main") {
		t.Errorf("breadcrumbs should contain 'main', got %q", out)
	}
}

func TestRenderBreadcrumbs_MultipleSegments(t *testing.T) {
	out := ui.RenderBreadcrumbs([]string{"main", "ec2", "i-12345"}, 80)
	if !strings.Contains(out, "main") {
		t.Errorf("breadcrumbs should contain 'main', got %q", out)
	}
	if !strings.Contains(out, "ec2") {
		t.Errorf("breadcrumbs should contain 'ec2', got %q", out)
	}
	if !strings.Contains(out, "i-12345") {
		t.Errorf("breadcrumbs should contain 'i-12345', got %q", out)
	}
	// Segments should be joined with " › " (U+203A)
	if !strings.Contains(out, "\u203A") {
		t.Errorf("breadcrumbs should contain '›' separator, got %q", out)
	}
}

func TestRenderBreadcrumbs_EmptySegments(t *testing.T) {
	// Should not panic with empty slice.
	out := ui.RenderBreadcrumbs([]string{}, 80)
	_ = out
}

func TestRenderBreadcrumbs_NilSegments(t *testing.T) {
	// Should not panic with nil slice.
	out := ui.RenderBreadcrumbs(nil, 80)
	_ = out
}

func TestRenderBreadcrumbs_WidthRespected(t *testing.T) {
	out := ui.RenderBreadcrumbs([]string{"main", "ec2"}, 40)
	// Just verify it doesn't panic and produces output containing our segments.
	if !strings.Contains(out, "main") {
		t.Errorf("breadcrumbs should contain 'main', got %q", out)
	}
	if !strings.Contains(out, "ec2") {
		t.Errorf("breadcrumbs should contain 'ec2', got %q", out)
	}
}

func TestRenderBreadcrumbs_ZeroWidth(t *testing.T) {
	// Width=0 means no constraint; should not panic.
	out := ui.RenderBreadcrumbs([]string{"main"}, 0)
	if !strings.Contains(out, "main") {
		t.Errorf("breadcrumbs with width=0 should still contain 'main', got %q", out)
	}
}

func TestRenderBreadcrumbs_SeparatorPresent(t *testing.T) {
	out := ui.RenderBreadcrumbs([]string{"a", "b", "c"}, 80)
	cleaned := stripANSI(out)
	// Check that separators join segments
	if !strings.Contains(cleaned, "a \u203A b \u203A c") {
		t.Errorf("expected segments joined with ' › ', got %q", cleaned)
	}
}

// ===========================================================================
// HelpModel tests
// ===========================================================================

func TestHelp_ViewContainsSections(t *testing.T) {
	// Set NO_COLOR so styling does not add ANSI that obscures text.
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	h.Width = 80
	h.Height = 40
	view := h.View()

	sections := []string{"Global", "Navigation", "Actions", "Sorting"}
	for _, sec := range sections {
		if !strings.Contains(view, sec) {
			t.Errorf("help view should contain section %q, got %q", sec, view)
		}
	}
}

func TestHelp_ViewContainsSpecificKeys(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	h.Width = 80
	h.Height = 40
	view := h.View()

	keys := []string{":", "/", "?", "d", "y", "x", "c", "j", "k"}
	for _, key := range keys {
		if !strings.Contains(view, key) {
			t.Errorf("help view should contain key %q", key)
		}
	}
}

func TestHelp_ViewContainsKeybindings(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	view := h.View()

	bindings := []string{
		"command", "filter", "help", "back",
		"down", "up", "top", "bottom", "select",
		"describe", "JSON view", "reveal secret", "copy ID",
		"by name", "by status", "by age",
	}
	for _, b := range bindings {
		if !strings.Contains(view, b) {
			t.Errorf("help view should contain binding description %q", b)
		}
	}
}

func TestHelp_ViewContainsCloseHint(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	view := h.View()
	if !strings.Contains(view, "Press any key to close help") {
		t.Error("help view should contain close hint")
	}
}

func TestHelp_ViewContainsKeybindingsTitle(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	view := h.View()
	if !strings.Contains(view, "Keybindings") {
		t.Error("help view should contain 'Keybindings' title")
	}
}

func TestHelp_WidthHeightSet_RendersWithinBounds(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	h.Width = 60
	h.Height = 30
	view := h.View()

	// Verify the help renders non-empty content with the given dimensions.
	if view == "" {
		t.Fatal("help view should not be empty")
	}
	// Verify it contains the expected content sections.
	if !strings.Contains(view, "Global") {
		t.Error("help view should contain 'Global' section")
	}
	if !strings.Contains(view, "Keybindings") {
		t.Error("help view should contain 'Keybindings' title")
	}
}

func TestHelp_SmallWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	h.Width = 20
	h.Height = 10
	// Should not panic with very small dimensions.
	view := h.View()
	if view == "" {
		t.Error("help view should not be empty even with small dimensions")
	}
}

func TestHelp_ZeroDimensions(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	// Width=0 and Height=0, should not panic.
	view := h.View()
	if view == "" {
		t.Error("help view should not be empty with zero dimensions")
	}
}

func TestHelp_NewHelpModel_Defaults(t *testing.T) {
	h := ui.NewHelpModel()
	if h.Width != 0 {
		t.Errorf("expected default Width=0, got %d", h.Width)
	}
	if h.Height != 0 {
		t.Errorf("expected default Height=0, got %d", h.Height)
	}
}

func TestHelp_ViewContainsHistoryKeys(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	view := h.View()
	if !strings.Contains(view, "[") {
		t.Error("help view should contain '[' for history back")
	}
	if !strings.Contains(view, "]") {
		t.Error("help view should contain ']' for history forward")
	}
}

func TestHelp_ViewContainsSortKeys(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	view := h.View()
	if !strings.Contains(view, "N") {
		t.Error("help view should contain 'N' for sort by name")
	}
	if !strings.Contains(view, "S") {
		t.Error("help view should contain 'S' for sort by status")
	}
	if !strings.Contains(view, "A") {
		t.Error("help view should contain 'A' for sort by age")
	}
}

func TestHelp_NOCOLORRendering(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	h := ui.NewHelpModel()
	h.Width = 80
	view := h.View()
	// With NO_COLOR, the output should still contain all sections.
	if !strings.Contains(view, "Global") {
		t.Error("NO_COLOR help view should still contain 'Global' section")
	}
}

func TestHelp_ColorRendering(t *testing.T) {
	// Ensure NO_COLOR is not set.
	prevNoColor := os.Getenv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	defer func() {
		if prevNoColor != "" {
			os.Setenv("NO_COLOR", prevNoColor)
		}
	}()

	h := ui.NewHelpModel()
	h.Width = 80
	view := h.View()
	if !strings.Contains(view, "Global") {
		t.Error("colored help view should still contain 'Global' section")
	}
}

// ===========================================================================
// StatusMode constants tests
// ===========================================================================

func TestUI_StatusModeConstants(t *testing.T) {
	// Verify the status mode enum values are distinct and correct.
	modes := []ui.StatusMode{ui.NormalMode, ui.CommandMode, ui.FilterMode, ui.ErrorMode, ui.LoadingMode}
	seen := make(map[ui.StatusMode]bool)
	for _, m := range modes {
		if seen[m] {
			t.Errorf("duplicate StatusMode value: %d", m)
		}
		seen[m] = true
	}
	if len(seen) != 5 {
		t.Errorf("expected 5 distinct StatusMode values, got %d", len(seen))
	}
}

