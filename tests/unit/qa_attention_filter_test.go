package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestAttentionFilter_DefaultDisabled(t *testing.T) {
	var af views.AttentionFilter
	if af.IsEnabled() {
		t.Error("new AttentionFilter should be disabled by default")
	}
}

func TestAttentionFilter_Toggle(t *testing.T) {
	var af views.AttentionFilter

	af.Toggle()
	if !af.IsEnabled() {
		t.Error("after first Toggle, should be enabled")
	}

	af.Toggle()
	if af.IsEnabled() {
		t.Error("after second Toggle, should be disabled")
	}
}

func TestAttentionFilter_SetEnabled(t *testing.T) {
	var af views.AttentionFilter

	af.SetEnabled(true)
	if !af.IsEnabled() {
		t.Error("SetEnabled(true) should enable")
	}

	af.SetEnabled(false)
	if af.IsEnabled() {
		t.Error("SetEnabled(false) should disable")
	}
}

func TestAttentionFilter_SetEnabledIdempotent(t *testing.T) {
	var af views.AttentionFilter

	af.SetEnabled(true)
	af.SetEnabled(true)
	if !af.IsEnabled() {
		t.Error("double SetEnabled(true) should still be enabled")
	}
}
