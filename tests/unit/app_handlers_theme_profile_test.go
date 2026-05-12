package unit

// app_handlers_theme_profile_test.go — behavioral tests for zero-hit handlers
// in internal/tui/app_handlers.go:
//   - handleThemeSelected (0.0%)  — ThemeSelectedMsg → FlashMsg on error paths
//   - handleProfilesLoaded (0.0%) — profilesLoadedMsg → profile selector pushed
//
// handleThemeSelected is triggered by sending ThemeSelectedMsg directly.
// handleProfilesLoaded is triggered by triggering fetchProfiles() via
// NavigateMsg{Target: TargetProfile}, executing its cmd, and re-sending
// the opaque profilesLoadedMsg back to rootApplyMsg.
//
// profilesLoadedMsg is unexported from package tui — we obtain it by executing
// the cmd returned from NavigateMsg{Target: TargetProfile} after setting
// AWS_CONFIG_FILE to a synthetic config file containing known profiles.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// handleThemeSelected
// ─────────────────────────────────────────────────────────────────────────────

// TestHandleThemeSelected_InvalidThemeName verifies that sending a ThemeSelectedMsg
// with an absolute path (which ThemePath rejects) returns a FlashMsg{IsError: true}.
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

// TestHandleThemeSelected_ThemeFileNotFound verifies that sending a ThemeSelectedMsg
// with a syntactically valid but non-existent filename returns FlashMsg{IsError: true}.
func TestHandleThemeSelected_ThemeFileNotFound(t *testing.T) {
	withTuiVersion(t, "test")
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	m := newRootSizedModel()

	// Valid filename format but the file does not exist on disk.
	_, cmd := rootApplyMsg(m, messages.ThemeSelected{Theme: "nonexistent-theme-xyz.yaml"})
	if cmd == nil {
		t.Fatal("handleThemeSelected with missing theme file should return a cmd")
	}
	msg := cmd()
	flash, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flash.IsError {
		t.Errorf("FlashMsg.IsError = false, want true (file not found)")
	}
	if !strings.Contains(flash.Text, "Cannot read theme") && !strings.Contains(flash.Text, "cannot read") {
		t.Errorf("FlashMsg.Text = %q, want to mention cannot read theme", flash.Text)
	}
}

// TestHandleThemeSelected_EmptyThemeName verifies that an empty theme name
// is rejected by ThemePath and returns a FlashMsg{IsError: true}.
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

// TestHandleThemeSelected_TraversalRejected verifies that a theme name containing
// ".." is rejected by ThemePath.
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

// ─────────────────────────────────────────────────────────────────────────────
// handleProfilesLoaded
// ─────────────────────────────────────────────────────────────────────────────

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

// TestHandleProfilesLoaded_PushesProfileSelectorView verifies that when
// profilesLoadedMsg arrives the model's stack gains a profile-selector view.
// We trigger fetchProfiles() indirectly via NavigateMsg{Target: TargetProfile},
// execute the returned cmd, and pipe the opaque profilesLoadedMsg back into
// the model — exercising handleProfilesLoaded without constructing the
// unexported type directly.
func TestHandleProfilesLoaded_PushesProfileSelectorView(t *testing.T) {
	withTuiVersion(t, "test")

	// Write a real AWS config with known profiles and redirect DefaultConfigPath.
	cfgPath := writeAWSConfig(t, []string{"default", "staging", "prod"})
	t.Setenv("AWS_CONFIG_FILE", cfgPath)

	// Create model without demo clients so profile switching is not blocked.
	m := tui.New("default", "us-east-1")
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

	// Feed profilesLoadedMsg back into the model to trigger handleProfilesLoaded.
	updatedM, cmd := rootApplyMsg(m, loadedMsg)

	// handleProfilesLoaded always returns nil cmd.
	if cmd != nil {
		t.Errorf("handleProfilesLoaded should return nil cmd, got non-nil")
	}

	// The profile selector should now be on the view stack.
	viewOutput := stripANSI(rootViewContent(updatedM))
	if !strings.Contains(viewOutput, "staging") && !strings.Contains(viewOutput, "prod") {
		t.Errorf("after profilesLoadedMsg, view should show profile names; got:\n%s", viewOutput)
	}
}

// TestHandleProfilesLoaded_SingleProfile verifies edge case: only one profile
// in the list still results in the selector being pushed and nil cmd.
