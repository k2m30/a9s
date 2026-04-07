package unit

// Tests for §7.3: ctrl+z key must appear in BottomHints() on every resource list.
//
// Bug vector: the coder adds ctrl+z as a key binding but forgets to register it
// in BottomHints(), so the user has no discoverable hint that the feature exists.
// These tests catch that omission.
//
// These tests WILL FAIL until the coder adds the ctrl+z hint to BottomHints()
// in internal/tui/views/resourcelist.go.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestBottomHints_IncludesCtrlZ asserts that ctrl+z appears in BottomHints()
// for a standard (non-ct-events) resource list. The hint must be global, not
// resource-type-specific, because attentionOnly applies to all resource types.
func TestBottomHints_IncludesCtrlZ(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(200, 20)
	m, _ = m.Init()

	hints := m.BottomHints()
	for _, h := range hints {
		if h.Key == "ctrl+z" {
			return // found — test passes
		}
	}

	got := make([]string, 0, len(hints))
	for _, h := range hints {
		got = append(got, h.Key)
	}
	t.Errorf("BottomHints missing ctrl+z hint for ec2 resource list; got keys: %v", got)
}

// TestBottomHints_CtrlZ_PresentOnCtEventsToo asserts that ctrl+z also appears
// in BottomHints() for ct-events, confirming the hint is not suppressed for
// the primary use-case resource type.
func TestBottomHints_CtrlZ_PresentOnCtEventsToo(t *testing.T) {
	td := resource.FindResourceType("ct-events")
	if td == nil {
		t.Fatal("ct-events resource type not found")
	}
	k := keys.Default()
	m := views.NewResourceList(*td, config.DefaultConfig(), k)
	m.SetSize(200, 20)
	m, _ = m.Init()

	hints := m.BottomHints()
	for _, h := range hints {
		if h.Key == "ctrl+z" {
			return // found — test passes
		}
	}

	got := make([]string, 0, len(hints))
	for _, h := range hints {
		got = append(got, h.Key)
	}
	t.Errorf("BottomHints missing ctrl+z hint for ct-events resource list; got keys: %v", got)
}
