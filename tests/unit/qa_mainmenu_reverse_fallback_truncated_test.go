package unit

// qa_mainmenu_reverse_fallback_truncated_test.go — Regression test for the
// skipUnavailable reverse-fallback bug.
//
// Root cause: skipUnavailable (internal/tui/views/mainmenu.go:159) has an
// asymmetry between its forward loop and its reverse fallback:
//
//   Forward loop  (line 172): !known || count > 0 || isTruncated
//   Reverse fallback (line 185): !known || count > 0          ← missing || isTruncated
//
// Result: when the forward loop exhausts all items in the given direction and
// the reverse fallback scans the opposite direction, truncated-zero types
// (known=true, count=0, truncated=true) are silently skipped even though they
// ARE navigable (more pages may exist — count is a lower bound).
//
// Test layout
// -----------
// AllResourceTypes()[0] = "ec2"     (AlwaysHealthy=false)
// AllResourceTypes()[1] = "ecs-svc" (AlwaysHealthy=false)
//
//  Scenario — cursor at 0, press Up:
//    scroll.Up() → cursor stays at 0 (clamped)
//    skipUnavailable(-1):
//      forward(-1): item[0] is ec2 — known, count=0, truncated=false → NOT navigable → cur=-1, exit
//      reverse: cur = 0 - (-1) = 1 → item[1] is ecs-svc — known, count=0, truncated=true
//        BUG:  !known(F) || count>0(F)             = false → skip → cursor stays at 0
//        FIXED: !known(F) || count>0(F) || trunc(T) = true  → SetCursor(1) → return
//
//  TestMainMenu_ReverseFallback_TruncatedZeroIsNavigable:
//    FAILS today (cursor stays at ec2 / index 0).
//    PASSES after the fix adds || isTruncated to the reverse fallback.
//
//  TestMainMenu_ForwardNav_TruncatedZeroIsNavigable:
//    Baseline pin — forward loop already has the fix, always passes.

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// menuUpKeyMsg returns a KeyPressMsg matching the "k" (Up) binding.
func menuUpKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "k"}
}

// menuDownKeyMsg returns a KeyPressMsg matching the "j" (Down) binding.
func menuDownKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "j"}
}

// setupReverseFallbackMenu builds a MainMenuModel with:
//   - item[0] = AllResourceTypes()[0] ("ec2"):
//     availability=0, truncated=false → confirmed empty, NOT navigable
//   - item[1] = AllResourceTypes()[1] ("ecs-svc"):
//     availability=0, truncated=true  → truncated-zero, IS navigable
//
// All other items are left unknown (no availability set) so skipUnavailable
// naturally terminates the reverse loop via the !known branch.
//
// Cursor starts at index 0 (top of list).
func setupReverseFallbackMenu(t *testing.T) views.MainMenuModel {
	t.Helper()

	all := resource.AllResourceTypes()
	if len(all) < 2 {
		t.Skip("AllResourceTypes() has fewer than 2 entries — test not applicable")
	}

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 50)

	item0 := all[0].ShortName // "ec2"
	item1 := all[1].ShortName // "ecs-svc"

	// item[0]: known, zero, not truncated — confirmed empty, must NOT be navigable.
	m.SetAvailability(item0, 0)
	m.SetTruncated(item0, false)

	// item[1]: known, zero, truncated — truncated-zero, MUST be navigable.
	m.SetAvailability(item1, 0)
	m.SetTruncated(item1, true)

	// Cursor starts at 0 (default).
	return m
}

// TestMainMenu_ReverseFallback_TruncatedZeroIsNavigable verifies that the
// reverse fallback loop in skipUnavailable respects the isTruncated guard.
//
// Setup: cursor at index 0 (ec2 — confirmed empty). Press Up.
//   - scroll.Up() keeps cursor at 0 (already at top).
//   - skipUnavailable(-1) forward pass: item[0] (ec2) is empty → exits immediately.
//   - reverse pass: item[1] (ecs-svc) is truncated-zero → should be navigable.
//
// FAILS today: reverse pass misses isTruncated guard → cursor stays at 0 (ec2).
// PASSES after fix: reverse pass finds ecs-svc navigable → cursor moves to 1.
func TestMainMenu_ReverseFallback_TruncatedZeroIsNavigable(t *testing.T) {
	m := setupReverseFallbackMenu(t)

	all := resource.AllResourceTypes()
	item1 := all[1].ShortName // "ecs-svc"

	// Cursor is at 0 (item0=ec2 — confirmed empty). Press Up — stays at 0 due to clamp.
	// skipUnavailable(-1) reverse fallback should land on item1 (ecs-svc).
	m, _ = m.Update(menuUpKeyMsg())

	got := m.SelectedItem().ShortName
	if got != item1 {
		t.Errorf(
			"reverse fallback skipped truncated-zero %q: cursor at %q (index 0), want %q (index 1)\n"+
				"Likely cause: reverse fallback missing '|| isTruncated' guard in skipUnavailable",
			item1, got, item1,
		)
	}
}

// TestMainMenu_ForwardNav_TruncatedZeroIsNavigable verifies that the forward
// loop in skipUnavailable already handles truncated-zero types correctly.
// This is a baseline pin: it should pass both before and after the fix.
//
// Setup: cursor at index 0 (ec2 — confirmed empty). Press Down.
//   - scroll.Down() moves cursor to 1 (ecs-svc).
//   - skipUnavailable(+1) forward pass: item[1] (ecs-svc) is truncated-zero → navigable.
//   - Cursor stays at 1.
func TestMainMenu_ForwardNav_TruncatedZeroIsNavigable(t *testing.T) {
	m := setupReverseFallbackMenu(t)

	all := resource.AllResourceTypes()
	item1 := all[1].ShortName // "ecs-svc"

	// Cursor is at 0. Press Down → cursor moves to 1.
	// skipUnavailable(+1): item[1] (ecs-svc) is truncated-zero → forward pass already
	// handles isTruncated correctly → cursor stays at 1.
	m, _ = m.Update(menuDownKeyMsg())

	got := m.SelectedItem().ShortName
	if got != item1 {
		t.Errorf(
			"forward loop failed to land on truncated-zero %q: got %q\n"+
				"Unexpected: forward loop should already handle isTruncated (baseline pin)",
			item1, got,
		)
	}
}
