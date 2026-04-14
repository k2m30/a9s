package unit

// qa_menu_attention_persist_test.go — T042: attention filter persistence tests.
//
// These tests verify that the ctrl+z attention filter state on MainMenuModel
// persists across operations that don't explicitly reset it (e.g., SetSize).
// Because the main menu lives at stack[0] and is never rebuilt, persistence
// is provided by the model's value semantics — state must survive mutations.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestMainMenuAttentionFilterToggle verifies that Toggle() flips the attention
// filter state and IsEnabled() reports the new state correctly.
func TestMainMenuAttentionFilterToggle(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	// Initially disabled.
	if m.IsEnabled() {
		t.Error("attention filter should be disabled on a fresh menu")
	}

	// Toggle on.
	m.Toggle()
	if !m.IsEnabled() {
		t.Error("attention filter should be enabled after first Toggle()")
	}

	// Toggle off again.
	m.Toggle()
	if m.IsEnabled() {
		t.Error("attention filter should be disabled after second Toggle()")
	}
}

// TestMainMenuAttentionFilterSetEnabled verifies that SetEnabled explicitly
// sets the filter state to the requested value.
func TestMainMenuAttentionFilterSetEnabled(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	m.SetEnabled(true)
	if !m.IsEnabled() {
		t.Error("IsEnabled() should be true after SetEnabled(true)")
	}

	m.SetEnabled(false)
	if m.IsEnabled() {
		t.Error("IsEnabled() should be false after SetEnabled(false)")
	}
}

// TestMainMenuAttentionFilterPersistsAcrossSetSize verifies that toggling the
// attention filter on and then calling SetSize does not reset the filter state.
// This guards against SetSize accidentally reinitializing the embedded struct.
func TestMainMenuAttentionFilterPersistsAcrossSetSize(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	// Enable filter.
	m.Toggle()
	if !m.IsEnabled() {
		t.Fatal("precondition: attention filter must be enabled before SetSize")
	}

	// SetSize should not affect the attention filter.
	m.SetSize(80, 24)

	if !m.IsEnabled() {
		t.Error("attention filter was reset by SetSize — state must persist across SetSize calls")
	}
}

// TestMainMenuAttentionFilterPersistsAcrossSetFilter verifies that the attention
// filter state survives a text filter change.
func TestMainMenuAttentionFilterPersistsAcrossSetFilter(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	m.Toggle()
	if !m.IsEnabled() {
		t.Fatal("precondition: attention filter must be enabled")
	}

	// Apply a text filter via SetFilter. The attention filter must survive.
	m.SetFilter("ec")

	if !m.IsEnabled() {
		t.Error("attention filter was reset by SetFilter — state must persist")
	}
}

// TestMainMenuAttentionFilterPersistsAcrossSetIssues verifies that calling
// SetIssues does not affect the attention filter toggle state.
func TestMainMenuAttentionFilterPersistsAcrossSetIssues(t *testing.T) {
	m := views.NewMainMenu(keys.Default())

	m.Toggle()

	m.SetIssues("ec2", 3, false)
	m.SetIssues("rds", 0, false)

	if !m.IsEnabled() {
		t.Error("attention filter was reset by SetIssues — state must persist")
	}
}
