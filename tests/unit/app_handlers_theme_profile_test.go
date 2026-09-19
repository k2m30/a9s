package unit

// profilesLoadedMsg is unexported from package tui, so tests obtain it by
// executing the cmd returned from NavigateMsg{Target: TargetProfile} with
// AWS_CONFIG_FILE pointing at a synthetic config.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestHandleThemeSelected_InvalidThemeName(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	// Absolute path is rejected by config.ThemePath with "absolute paths not allowed".
	_, cmd := rootApplyMsg(m, messages.ThemeSelected{Theme: "/etc/passwd"})
	if cmd == nil {
		t.Fatal("handleThemeSelected with invalid theme name should return a cmd")
	}
	msg := cmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (invalid theme path)")
	}
	if !strings.Contains(flash.Text, "Invalid theme") && !strings.Contains(flash.Text, "invalid") && !strings.Contains(flash.Text, "absolute") {
		t.Errorf("FlashMsg.Text = %q, want to mention invalid theme", flash.Text)
	}
}

// The theme-selected flow is two round trips:
// ThemeSelected → TaskKindReadThemeFile dispatch → messages.ThemeFileRead →
// HandleThemeFileRead → FlashIntent (error). The test drives both steps:
// the first cmd produces messages.ThemeFileRead{Err: ...}; feeding that
// back through the root model produces the user-visible FlashMsg.
func TestHandleThemeSelected_ThemeFileNotFound(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	m := newRootSizedModel()

	// Step 1: ThemeSelected → ReadThemeFile task → ThemeFileRead{Err}.
	_, cmd := rootApplyMsg(m, messages.ThemeSelected{Theme: "nonexistent-theme-xyz.yaml"})
	if cmd == nil {
		t.Fatal("handleThemeSelected with missing theme file should return a cmd (read task)")
	}
	readResult := cmd()
	tfr, ok := readResult.(messages.ThemeFileRead)
	if !ok {
		t.Fatalf("expected messages.ThemeFileRead from read task, got %T", readResult)
	}
	if tfr.Err == nil {
		t.Fatalf("expected ThemeFileRead.Err to be set for missing file, got nil")
	}

	// Step 2: HandleThemeFileRead branches on Err and emits a FlashIntent, which
	// is re-emitted as messages.Flash.
	_, flashCmd := rootApplyMsg(m, tfr)
	if flashCmd == nil {
		t.Fatal("handleThemeFileRead on read error should return a cmd (flash re-emit)")
	}
	msg := flashCmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected messages.Flash from HandleThemeFileRead error branch, got %T", msg)
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (file not found)")
	}
	if !strings.Contains(flash.Text, "Cannot read theme") && !strings.Contains(flash.Text, "cannot read") {
		t.Errorf("FlashMsg.Text = %q, want to mention cannot read theme", flash.Text)
	}
}

func TestHandleThemeSelected_EmptyThemeName(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, messages.ThemeSelected{Theme: ""})
	if cmd == nil {
		t.Fatal("handleThemeSelected with empty theme name should return a cmd")
	}
	msg := cmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (empty theme name)")
	}
}

func TestHandleThemeSelected_TraversalRejected(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, messages.ThemeSelected{Theme: "../evil.yaml"})
	if cmd == nil {
		t.Fatal("handleThemeSelected with traversal theme name should return a cmd")
	}
	msg := cmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (traversal attempt rejected)")
	}
}

// writeAWSConfig writes a minimal AWS config file with the given profile names
// to a temp directory and returns the path to the config file.
func writeAWSConfig(t *testing.T, profiles []string) string {
	t.Helper()
	tmp := t.TempDir()
	p := filepath.Join(tmp, "config")
	var sb strings.Builder
	for _, name := range profiles {
		if name == "default" {
			fmt.Fprintf(&sb, "[default]\nregion = us-east-1\n\n")
		} else {
			fmt.Fprintf(&sb, "[profile %s]\nregion = us-east-1\n\n", name)
		}
	}
	if err := os.WriteFile(p, []byte(sb.String()), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

func TestHandleProfilesLoaded_PushesProfileSelectorView(t *testing.T) {
	withTuiVersion(t, "test")

	// Write a real AWS config with known profiles and redirect DefaultConfigPath.
	cfgPath := writeAWSConfig(t, []string{"default", "staging", "prod"})
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	// Create model without demo clients so profile switching is not blocked.
	m := newBlessedModel(t, "default", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Trigger fetchProfiles() — NavigateMsg{Target: TargetProfile} dispatches to
	// handleNavigate which calls m.fetchProfiles() and returns its cmd.
	_, fetchCmd := rootApplyMsg(m, messages.Navigate{Target: messages.TargetProfile})
	if fetchCmd == nil {
		t.Fatal("NavigateMsg{TargetProfile} should return a cmd (fetchProfiles)")
	}

	// Execute the cmd — should return profilesLoadedMsg (opaque tea.Msg).
	loadedMsg := fetchCmd()
	// If the config read failed for some reason, loadedMsg might be FlashMsg.
	if _, isFlash := loadedMsg.(messages.Flash); isFlash {
		t.Fatalf("fetchProfiles returned FlashMsg — config file may be malformed: %v", loadedMsg)
	}

	updatedM, cmd := rootApplyMsg(m, loadedMsg)

	if cmd != nil {
		t.Errorf("handleProfilesLoaded should return nil cmd, got non-nil")
	}

	viewOutput := stripANSI(rootViewContent(updatedM))
	if !strings.Contains(viewOutput, "staging") && !strings.Contains(viewOutput, "prod") {
		t.Errorf("after profilesLoadedMsg, view should show profile names; got:\n%s", viewOutput)
	}
}

// The TUI adapter's handleThemeFileRead parses the bytes (styles.ThemeFromYAML)
// and sets ParseErr for invalid YAML; the runtime then emits a "Bad theme
// YAML: …" error flash and applies no theme.
func TestHandleThemeFileRead_MalformedYAML_EmitsErrorFlashNoApply(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	m := newRootSizedModel()

	// Capture the view before sending the malformed theme so we can assert
	// the theme was NOT applied (the default theme label stays unchanged).
	viewBefore := stripANSI(rootViewContent(m))

	// Feed ThemeFileRead with malformed YAML bytes and Err==nil — this is the
	// path that exercises the adapter's styles.ThemeFromYAML call.
	malformedBytes := []byte(":\n  not yaml at all\n: : :\n")
	_, flashCmd := rootApplyMsg(m, messages.ThemeFileRead{
		Theme: "broken.yaml",
		Bytes: malformedBytes,
		Err:   nil,
	})
	if flashCmd == nil {
		t.Fatal("handleThemeFileRead with malformed YAML should return a cmd (flash re-emit)")
	}
	msg := flashCmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected messages.Flash, got %T: %v", msg, msg)
	}
	if !flash.IsError {
		t.Errorf("Flash.IsError = false, want true for malformed YAML")
	}
	if !strings.HasPrefix(flash.Text, "Bad theme YAML: ") {
		t.Errorf("Flash.Text = %q, want prefix %q", flash.Text, "Bad theme YAML: ")
	}

	// Assert no apply side-effect: the view is identical to before the bad
	// theme was sent (no theme-apply message was dispatched).
	viewAfter := stripANSI(rootViewContent(m))
	if viewBefore != viewAfter {
		t.Errorf("view changed after malformed theme — theme was unexpectedly applied\nbefore: %q\nafter:  %q", viewBefore, viewAfter)
	}
}
