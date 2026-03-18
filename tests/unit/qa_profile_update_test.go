package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/views"
)

// ═══════════════════════════════════════════════════════════════════════════
// ProfileModel.Update direct tests
// ═══════════════════════════════════════════════════════════════════════════

func profileKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func profileSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func newProfileModel() views.ProfileModel {
	profiles := []string{"profile-1", "profile-2", "profile-3"}
	k := keys.Default()
	m := views.NewProfile(profiles, "profile-2", k)
	m.SetSize(80, 20)
	return m
}

// PU-01: j/down moves cursor down
func TestQA_ProfileUpdate_JMovesDown(t *testing.T) {
	m := newProfileModel()

	// Cursor starts at 0; press j to move to 1
	m, _ = m.Update(profileKeyPress("j"))

	view := m.View()
	plain := stripANSI(view)

	// After moving down, profile-2 should be at cursor position (index 1).
	// The View highlights the cursor row with RowSelected style.
	// We verify the view renders without panic and contains all profiles.
	if !strings.Contains(plain, "profile-1") {
		t.Errorf("view should contain profile-1, got: %s", plain)
	}
	if !strings.Contains(plain, "profile-2") {
		t.Errorf("view should contain profile-2, got: %s", plain)
	}
	if !strings.Contains(plain, "profile-3") {
		t.Errorf("view should contain profile-3, got: %s", plain)
	}

	// Press j again to move to index 2
	m, _ = m.Update(profileKeyPress("j"))

	// Press Enter at index 2 to verify cursor is at profile-3
	m, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm, ok := msg.(messages.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("expected ProfileSelectedMsg, got %T", msg)
	}
	if psm.Profile != "profile-3" {
		t.Errorf("after j j Enter, expected profile-3, got %s", psm.Profile)
	}
}

// PU-02: down arrow moves cursor down
func TestQA_ProfileUpdate_DownArrowMovesDown(t *testing.T) {
	m := newProfileModel()

	m, _ = m.Update(profileSpecialKey(tea.KeyDown))

	// Verify cursor is now at index 1 by pressing Enter
	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm, ok := msg.(messages.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("expected ProfileSelectedMsg, got %T", msg)
	}
	if psm.Profile != "profile-2" {
		t.Errorf("after down-arrow Enter, expected profile-2, got %s", psm.Profile)
	}
}

// PU-03: k/up moves cursor up
func TestQA_ProfileUpdate_KMovesUp(t *testing.T) {
	m := newProfileModel()

	// Move down first, then up
	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))
	// Cursor at index 2
	m, _ = m.Update(profileKeyPress("k"))
	// Cursor at index 1

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm, ok := msg.(messages.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("expected ProfileSelectedMsg, got %T", msg)
	}
	if psm.Profile != "profile-2" {
		t.Errorf("after j j k Enter, expected profile-2, got %s", psm.Profile)
	}
}

// PU-04: up arrow moves cursor up
func TestQA_ProfileUpdate_UpArrowMovesUp(t *testing.T) {
	m := newProfileModel()

	m, _ = m.Update(profileSpecialKey(tea.KeyDown))
	m, _ = m.Update(profileSpecialKey(tea.KeyDown))
	m, _ = m.Update(profileSpecialKey(tea.KeyUp))

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-2" {
		t.Errorf("after down down up Enter, expected profile-2, got %s", psm.Profile)
	}
}

// PU-05: cursor stops at top boundary
func TestQA_ProfileUpdate_CursorStopsAtTop(t *testing.T) {
	m := newProfileModel()

	// Cursor at 0; pressing k should keep it at 0
	m, _ = m.Update(profileKeyPress("k"))
	m, _ = m.Update(profileKeyPress("k"))
	m, _ = m.Update(profileKeyPress("k"))

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-1" {
		t.Errorf("cursor should stop at top; expected profile-1, got %s", psm.Profile)
	}
}

// PU-06: cursor stops at bottom boundary
func TestQA_ProfileUpdate_CursorStopsAtBottom(t *testing.T) {
	m := newProfileModel()

	// Move down past the end (3 profiles, max index is 2)
	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-3" {
		t.Errorf("cursor should stop at bottom; expected profile-3, got %s", psm.Profile)
	}
}

// PU-07: Enter at cursor position 0 returns correct profile
func TestQA_ProfileUpdate_EnterAtPosition0(t *testing.T) {
	m := newProfileModel()

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm, ok := msg.(messages.ProfileSelectedMsg)
	if !ok {
		t.Fatalf("expected ProfileSelectedMsg, got %T", msg)
	}
	if psm.Profile != "profile-1" {
		t.Errorf("Enter at position 0 should select profile-1, got %s", psm.Profile)
	}
}

// PU-08: Enter at cursor position 1 returns correct profile
func TestQA_ProfileUpdate_EnterAtPosition1(t *testing.T) {
	m := newProfileModel()

	m, _ = m.Update(profileKeyPress("j"))

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-2" {
		t.Errorf("Enter at position 1 should select profile-2, got %s", psm.Profile)
	}
}

// PU-09: Enter at cursor position 2 returns correct profile
func TestQA_ProfileUpdate_EnterAtPosition2(t *testing.T) {
	m := newProfileModel()

	m, _ = m.Update(profileKeyPress("j"))
	m, _ = m.Update(profileKeyPress("j"))

	_, cmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-3" {
		t.Errorf("Enter at position 2 should select profile-3, got %s", psm.Profile)
	}
}

// PU-10: g/G keys are not handled by ProfileModel (no Top/Bottom binding)
// ProfileModel only handles Up, Down, Enter. g/G are effectively no-ops.
func TestQA_ProfileUpdate_GKeysNoOp(t *testing.T) {
	m := newProfileModel()

	// Move cursor to index 1
	m, _ = m.Update(profileKeyPress("j"))

	// Press g (Top) - not handled by ProfileModel.Update
	m, cmd := m.Update(profileKeyPress("g"))
	if cmd != nil {
		t.Error("g key should not produce a command in ProfileModel")
	}

	// Cursor should still be at index 1
	_, enterCmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if enterCmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := enterCmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-2" {
		t.Errorf("g should be no-op; cursor should stay at profile-2, got %s", psm.Profile)
	}
}

// PU-11: G key is not handled by ProfileModel (no Top/Bottom binding)
func TestQA_ProfileUpdate_ShiftGKeyNoOp(t *testing.T) {
	m := newProfileModel()

	// Press G (Bottom) - not handled by ProfileModel.Update
	m, cmd := m.Update(profileKeyPress("G"))
	if cmd != nil {
		t.Error("G key should not produce a command in ProfileModel")
	}

	// Cursor should still be at index 0
	_, enterCmd := m.Update(profileSpecialKey(tea.KeyEnter))
	if enterCmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := enterCmd()
	psm := msg.(messages.ProfileSelectedMsg)
	if psm.Profile != "profile-1" {
		t.Errorf("G should be no-op; cursor should stay at profile-1, got %s", psm.Profile)
	}
}

// PU-12: FrameTitle returns "aws-profiles(3)" for 3 profiles
func TestQA_ProfileUpdate_FrameTitle(t *testing.T) {
	m := newProfileModel()

	title := m.FrameTitle()
	if title != "aws-profiles(3)" {
		t.Errorf("FrameTitle() = %q, want %q", title, "aws-profiles(3)")
	}
}

// PU-13: FrameTitle count reflects actual profile count
func TestQA_ProfileUpdate_FrameTitleCount(t *testing.T) {
	profiles := []string{"a", "b", "c", "d", "e"}
	k := keys.Default()
	m := views.NewProfile(profiles, "b", k)

	title := m.FrameTitle()
	if title != "aws-profiles(5)" {
		t.Errorf("FrameTitle() = %q, want %q", title, "aws-profiles(5)")
	}
}

// PU-14: View shows (current) marker for active profile
func TestQA_ProfileUpdate_ActiveProfileMarker(t *testing.T) {
	m := newProfileModel()

	view := m.View()
	plain := stripANSI(view)

	if !strings.Contains(plain, "(current)") {
		t.Errorf("view should show (current) marker for active profile, got: %s", plain)
	}

	// profile-2 is active; its line should contain (current)
	lines := strings.Split(plain, "\n")
	foundCurrent := false
	for _, line := range lines {
		if strings.Contains(line, "profile-2") && strings.Contains(line, "(current)") {
			foundCurrent = true
			break
		}
	}
	if !foundCurrent {
		t.Errorf("profile-2 line should have (current) marker, got: %s", plain)
	}
}

// PU-15: Update returns nil cmd for non-matching key
func TestQA_ProfileUpdate_UnhandledKeyReturnsNilCmd(t *testing.T) {
	m := newProfileModel()

	_, cmd := m.Update(profileKeyPress("x"))
	if cmd != nil {
		t.Error("unhandled key 'x' should return nil cmd")
	}
}

// PU-16: Update with non-KeyMsg returns model unchanged
func TestQA_ProfileUpdate_NonKeyMsgPassthrough(t *testing.T) {
	m := newProfileModel()

	// Send a WindowSizeMsg (not a KeyMsg)
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
	// Model should still work after receiving non-key message
	title := m2.FrameTitle()
	if title != "aws-profiles(3)" {
		t.Errorf("model should be unchanged after non-key msg; FrameTitle() = %q", title)
	}
}

// PU-17: Init returns model and nil cmd
func TestQA_ProfileUpdate_Init(t *testing.T) {
	m := newProfileModel()

	m2, cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil cmd")
	}
	if m2.FrameTitle() != "aws-profiles(3)" {
		t.Errorf("Init() should return same model; FrameTitle() = %q", m2.FrameTitle())
	}
}

// PU-18: View with empty profiles
func TestQA_ProfileUpdate_EmptyProfiles(t *testing.T) {
	k := keys.Default()
	m := views.NewProfile([]string{}, "", k)
	m.SetSize(80, 20)

	view := m.View()
	if !strings.Contains(view, "No profiles available") {
		t.Errorf("empty profiles should show 'No profiles available', got: %s", view)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RevealModel.Update direct tests
// ═══════════════════════════════════════════════════════════════════════════

func newRevealModel(secretName, value string) views.RevealModel {
	k := keys.Default()
	m := views.NewReveal(secretName, value, k)
	return m
}

// RU-01: SetSize initializes viewport so View() returns content
func TestQA_RevealUpdate_SetSizeInitializesViewport(t *testing.T) {
	m := newRevealModel("my-secret", "super-secret-value-123")

	// Before SetSize, View should show "Initializing..."
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("before SetSize, View() should return 'Initializing...', got: %q", view)
	}

	// After SetSize, View should show the secret value
	m.SetSize(80, 20)
	view = m.View()
	if !strings.Contains(view, "super-secret-value-123") {
		t.Errorf("after SetSize, View() should contain secret value, got: %q", view)
	}
}

// RU-02: SecretValue returns the raw value
func TestQA_RevealUpdate_SecretValue(t *testing.T) {
	m := newRevealModel("my-secret", "the-actual-secret")

	val := m.SecretValue()
	if val != "the-actual-secret" {
		t.Errorf("SecretValue() = %q, want %q", val, "the-actual-secret")
	}
}

// RU-03: HeaderWarning returns styled warning text
func TestQA_RevealUpdate_HeaderWarning(t *testing.T) {
	m := newRevealModel("my-secret", "value")

	warning := m.HeaderWarning()
	plain := stripANSI(warning)
	if !strings.Contains(plain, "Secret visible") {
		t.Errorf("HeaderWarning() should contain 'Secret visible', got: %q", plain)
	}
	if !strings.Contains(plain, "press esc to close") {
		t.Errorf("HeaderWarning() should contain 'press esc to close', got: %q", plain)
	}
}

// RU-04: FrameTitle returns the secret name
func TestQA_RevealUpdate_FrameTitle(t *testing.T) {
	m := newRevealModel("prod/db-password", "s3cret")

	title := m.FrameTitle()
	if title != "prod/db-password" {
		t.Errorf("FrameTitle() = %q, want %q", title, "prod/db-password")
	}
}

// RU-05: Viewport scroll works after SetSize - down key
func TestQA_RevealUpdate_ViewportScrollDown(t *testing.T) {
	// Create a long secret value that overflows a small viewport
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line-" + strings.Repeat("x", 5) + "-" + string(rune('0'+i%10))
	}
	longValue := strings.Join(lines, "\n")

	m := newRevealModel("scroll-secret", longValue)
	m.SetSize(80, 5) // small viewport to force scrolling

	viewBefore := m.View()

	// Send down key to scroll viewport
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	viewAfter := m.View()

	// Scrolling should change the view if content overflows
	if viewBefore == viewAfter {
		t.Error("down key should scroll viewport when content overflows")
	}
}

// RU-06: Viewport scroll works - up key after down
func TestQA_RevealUpdate_ViewportScrollUp(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line-" + strings.Repeat("y", 5) + "-" + string(rune('0'+i%10))
	}
	longValue := strings.Join(lines, "\n")

	m := newRevealModel("scroll-secret", longValue)
	m.SetSize(80, 5)

	// Scroll down first
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	viewScrolled := m.View()

	// Scroll back up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	viewAfterUp := m.View()

	if viewScrolled == viewAfterUp {
		t.Error("up key should scroll viewport back when content overflows")
	}
}

// RU-07: Update before SetSize (not ready) returns nil cmd
func TestQA_RevealUpdate_UpdateBeforeReady(t *testing.T) {
	m := newRevealModel("my-secret", "value")

	// Without SetSize, m.ready is false
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cmd != nil {
		t.Error("Update before SetSize should return nil cmd")
	}

	// View should still show "Initializing..."
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("View() before SetSize should be 'Initializing...', got: %q", view)
	}
}

// RU-08: Init returns model and nil cmd
func TestQA_RevealUpdate_Init(t *testing.T) {
	m := newRevealModel("my-secret", "value")

	m2, cmd := m.Init()
	if cmd != nil {
		t.Error("Init() should return nil cmd")
	}
	if m2.SecretValue() != "value" {
		t.Errorf("Init() should preserve model; SecretValue() = %q", m2.SecretValue())
	}
}

// RU-09: SetSize can be called multiple times (resize)
func TestQA_RevealUpdate_ResizeViewport(t *testing.T) {
	m := newRevealModel("my-secret", "secret-content-here")

	m.SetSize(80, 20)
	view1 := m.View()
	if !strings.Contains(view1, "secret-content-here") {
		t.Error("view should contain secret after first SetSize")
	}

	// Resize
	m.SetSize(40, 10)
	view2 := m.View()
	if !strings.Contains(view2, "secret-content-here") {
		t.Error("view should still contain secret after resize")
	}
}

// RU-10: SecretValue is independent of viewport state
func TestQA_RevealUpdate_SecretValueIndependent(t *testing.T) {
	m := newRevealModel("my-secret", "immutable-secret")
	m.SetSize(80, 5)

	// Scroll around
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})

	// SecretValue should still return the full original value
	if m.SecretValue() != "immutable-secret" {
		t.Errorf("SecretValue() should be stable after scrolling, got %q", m.SecretValue())
	}
}

// RU-11: Multiline secret renders correctly
func TestQA_RevealUpdate_MultilineSecret(t *testing.T) {
	multiline := "key1: value1\nkey2: value2\nkey3: value3"
	m := newRevealModel("json-secret", multiline)
	m.SetSize(80, 20)

	view := m.View()
	if !strings.Contains(view, "key1") {
		t.Error("multiline secret should contain key1")
	}
	if !strings.Contains(view, "key3") {
		t.Error("multiline secret should contain key3")
	}
}

// RU-12: Empty secret value
func TestQA_RevealUpdate_EmptySecret(t *testing.T) {
	m := newRevealModel("empty-secret", "")
	m.SetSize(80, 20)

	// Should not panic
	view := m.View()
	_ = view

	val := m.SecretValue()
	if val != "" {
		t.Errorf("SecretValue() for empty secret should be empty, got %q", val)
	}
}
