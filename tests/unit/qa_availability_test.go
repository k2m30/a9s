package unit

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newSizedMainMenu creates a MainMenuModel with the given terminal dimensions
// and NO_COLOR mode enabled for deterministic string assertions.
func newSizedMainMenu(t *testing.T, w, h int) views.MainMenuModel {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		styles.Reinit()
	})
	m := views.NewMainMenu(keys.Default())
	m.SetSize(w, h)
	return m
}

// menuKeyDown creates a tea.KeyPressMsg for the "j" (down) key.
func menuKeyDown() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "j"}
}

// menuKeyUp creates a tea.KeyPressMsg for the "k" (up) key.
func menuKeyUp() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "k"}
}

// menuKeyTop creates a tea.KeyPressMsg for the "g" (top) key.
func menuKeyTop() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "g"}
}

// menuKeyBottom creates a tea.KeyPressMsg for the "G" (bottom) key.
func menuKeyBottom() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: "G"}
}

// menuKeyEnter creates a tea.KeyPressMsg for the Enter key.
func menuKeyEnter() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// menuKeyPageDown creates a tea.KeyPressMsg for the PageDown key.
func menuKeyPageDown() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyPgDown}
}

// availItoa is a test helper to convert int to string without importing strconv.
func availItoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ---------------------------------------------------------------------------
// Normal render (no availability set)
// ---------------------------------------------------------------------------

func TestQA_Availability_NormalRender(t *testing.T) {
	// Use height=200 so all resource types + category headers fit in the viewport.
	m := newSizedMainMenu(t, 80, 200)

	output := m.View()

	// With no availability set, all resource types should appear in the output
	// and none should be dimmed (since we can't distinguish dim without ANSI codes
	// in NO_COLOR mode, we just verify all types are present).
	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		if !strings.Contains(output, rt.Name) {
			t.Errorf("menu missing resource type %q in View() output", rt.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// SetAvailability: empty type (count=0) is greyed out
// ---------------------------------------------------------------------------

func TestQA_Availability_EmptyTypeGreyedOut(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)

	output := m.View()

	// EC2 Instances should still appear in the output (not removed).
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should still be visible when marked as empty")
	}

	// The availability info should be stored correctly.
	avail := m.GetAvailability()
	if avail == nil {
		t.Fatal("GetAvailability() should not return nil after SetAvailability")
	}
	count, ok := avail["ec2"]
	if !ok {
		t.Fatal("GetAvailability() missing ec2 entry")
	}
	if count != 0 {
		t.Errorf("ec2 count should be 0, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// SetAvailability: present type (count>0) renders normally
// ---------------------------------------------------------------------------

func TestQA_Availability_PresentTypeNormal(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 5)

	output := m.View()

	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should appear when marked as available")
	}

	avail := m.GetAvailability()
	if avail["ec2"] != 5 {
		t.Errorf("ec2 count should be 5, got %d", avail["ec2"])
	}
}

// ---------------------------------------------------------------------------
// Unknown type in availability map: renders normally
// ---------------------------------------------------------------------------

func TestQA_Availability_UnknownTypeNormal(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	// Set availability for some type but NOT ec2.
	m.SetAvailability("s3", 0)

	output := m.View()

	// EC2 should render normally (not in the availability map at all).
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should render normally when not in availability map")
	}

	avail := m.GetAvailability()
	if _, ok := avail["ec2"]; ok {
		t.Error("ec2 should not be in the availability map")
	}
}

// ---------------------------------------------------------------------------
// Cursor cannot land on empty type — Down skips it
// ---------------------------------------------------------------------------

func TestQA_Availability_CursorCannotLandOnEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types for this test")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark the second item as empty (count=0). Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, 0)

	// Press Down — should skip items[1] and land on items[2].
	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName == allTypes[1].ShortName {
		t.Errorf("cursor should NOT land on empty type %q", allTypes[1].ShortName)
	}
	if selected.ShortName != allTypes[2].ShortName {
		t.Errorf("cursor should land on items[2] %q, got %q", allTypes[2].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// ClearAvailability resets everything
// ---------------------------------------------------------------------------

func TestQA_Availability_ClearAvailability(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)
	m.SetAvailability("s3", 7)

	// Verify availability was set.
	avail := m.GetAvailability()
	if len(avail) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(avail))
	}

	// Clear.
	m.ClearAvailability()

	avail = m.GetAvailability()
	if avail != nil {
		t.Errorf("GetAvailability() after ClearAvailability() should return nil, got %v", avail)
	}

	// All items should render normally after clear.
	output := m.View()
	for _, rt := range resource.AllResourceTypes() {
		if !strings.Contains(output, rt.Name) {
			t.Errorf("after ClearAvailability(), menu should show %q", rt.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// GetAvailability returns a copy (mutation safety)
// ---------------------------------------------------------------------------

func TestQA_Availability_GetAvailability(t *testing.T) {
	m := newSizedMainMenu(t, 80, 40)

	// Initially nil.
	avail := m.GetAvailability()
	if avail != nil {
		t.Errorf("GetAvailability() should return nil when nothing set, got %v", avail)
	}

	// Set some values.
	m.SetAvailability("ec2", 12)
	m.SetAvailability("s3", 0)
	m.SetAvailability("rds", 3)

	avail = m.GetAvailability()
	if len(avail) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(avail))
	}

	// Verify exact values.
	if avail["ec2"] != 12 {
		t.Errorf("ec2 should be 12, got %d", avail["ec2"])
	}
	if avail["s3"] != 0 {
		t.Errorf("s3 should be 0, got %d", avail["s3"])
	}
	if avail["rds"] != 3 {
		t.Errorf("rds should be 3, got %d", avail["rds"])
	}

	// Mutating the returned map should not affect the model.
	avail["ec2"] = 0
	avail["new_type"] = 99

	fresh := m.GetAvailability()
	if fresh["ec2"] != 12 {
		t.Errorf("mutating GetAvailability() return value should not affect model: ec2 should still be 12, got %d", fresh["ec2"])
	}
	if _, ok := fresh["new_type"]; ok {
		t.Error("mutating GetAvailability() return value should not affect model: new_type should not exist")
	}
}

// ---------------------------------------------------------------------------
// SetCheckProgress updates internal state
// ---------------------------------------------------------------------------

func TestQA_Availability_SetCheckProgress(t *testing.T) {
	m := newSizedMainMenu(t, 80, 40)

	// Should not panic with various values.
	m.SetCheckProgress(0, 62)
	m.SetCheckProgress(31, 62)
	m.SetCheckProgress(62, 62)

	// View should still render without errors.
	output := m.View()
	if output == "" {
		t.Error("View() should not be empty after SetCheckProgress")
	}
}

// ---------------------------------------------------------------------------
// Filter with greyed-out type: still shown but dimmed
// ---------------------------------------------------------------------------

func TestQA_Availability_FilterWithGreyedOut(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)
	m.SetFilter("ec2")

	output := m.View()

	// EC2 should still appear in filtered results even though it's marked empty.
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("filter matching greyed-out type should still show EC2 Instances")
	}

	// Availability state should be preserved through filtering.
	avail := m.GetAvailability()
	if avail["ec2"] != 0 {
		t.Errorf("ec2 count should still be 0 after filtering, got %d", avail["ec2"])
	}
}

// ---------------------------------------------------------------------------
// Multiple empty types
// ---------------------------------------------------------------------------

func TestQA_Availability_MultipleEmpty(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)

	emptyTypes := []string{"ec2", "rds", "lambda", "eks", "vpc"}
	for _, shortName := range emptyTypes {
		m.SetAvailability(shortName, 0)
	}

	avail := m.GetAvailability()
	if len(avail) != len(emptyTypes) {
		t.Fatalf("expected %d entries, got %d", len(emptyTypes), len(avail))
	}

	for _, shortName := range emptyTypes {
		if avail[shortName] != 0 {
			t.Errorf("%s should be 0 (empty), got %d", shortName, avail[shortName])
		}
	}

	// All types (including empty ones) should still appear in the output.
	output := m.View()
	allTypes := resource.AllResourceTypes()
	for _, rt := range allTypes {
		if !strings.Contains(output, rt.Name) {
			t.Errorf("menu should contain %q even when some types are empty", rt.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// SetAvailability overwrites previous value
// ---------------------------------------------------------------------------

func TestQA_Availability_OverwriteValue(t *testing.T) {
	m := newSizedMainMenu(t, 80, 40)

	m.SetAvailability("ec2", 0)
	avail := m.GetAvailability()
	if avail["ec2"] != 0 {
		t.Errorf("ec2 should be 0 initially, got %d", avail["ec2"])
	}

	m.SetAvailability("ec2", 15)
	avail = m.GetAvailability()
	if avail["ec2"] != 15 {
		t.Errorf("ec2 should be 15 after overwrite, got %d", avail["ec2"])
	}
}

// ---------------------------------------------------------------------------
// All resource types: verify SetAvailability works for every registered type
// ---------------------------------------------------------------------------

func TestQA_Availability_AllResourceTypes(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)

	allTypes := resource.AllResourceTypes()
	// Mark every other type: even indices get count=i+1, odd indices get count=0.
	for i, rt := range allTypes {
		if i%2 == 0 {
			m.SetAvailability(rt.ShortName, i+1)
		} else {
			m.SetAvailability(rt.ShortName, 0)
		}
	}

	avail := m.GetAvailability()
	if len(avail) != len(allTypes) {
		t.Fatalf("expected %d entries, got %d", len(allTypes), len(avail))
	}

	for i, rt := range allTypes {
		if i%2 == 0 {
			expected := i + 1
			if avail[rt.ShortName] != expected {
				t.Errorf("%s: count = %d, want %d", rt.ShortName, avail[rt.ShortName], expected)
			}
		} else if avail[rt.ShortName] != 0 {
			t.Errorf("%s: count = %d, want 0", rt.ShortName, avail[rt.ShortName])
		}
	}

	// All types should still appear in the view.
	output := m.View()
	for _, rt := range allTypes {
		if !strings.Contains(output, rt.Name) {
			t.Errorf("menu should contain %q regardless of availability", rt.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge: various terminal sizes
// ---------------------------------------------------------------------------

func TestQA_Availability_SmallTerminal(t *testing.T) {
	m := newSizedMainMenu(t, 40, 10)
	m.SetAvailability("ec2", 0)
	m.SetAvailability("s3", 3)

	// Should not panic at small terminal size.
	output := m.View()
	if output == "" {
		t.Error("View() should not be empty at 40x10")
	}
}

func TestQA_Availability_LargeTerminal(t *testing.T) {
	m := newSizedMainMenu(t, 200, 50)
	m.SetAvailability("ec2", 0)

	output := m.View()
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should appear at 200x50")
	}
}

func TestQA_Availability_ZeroSize(t *testing.T) {
	m := newSizedMainMenu(t, 0, 0)
	m.SetAvailability("ec2", 0)

	// Should not panic.
	output := m.View()
	// Output may be minimal but should not be empty (filteredItems is non-empty).
	_ = output
}

// ---------------------------------------------------------------------------
// Navigation skip: Down skips over a single empty type
// ---------------------------------------------------------------------------

func TestQA_Availability_DownSkipsEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark items[1] as empty (count=0). Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, 0)

	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[2].ShortName {
		t.Errorf("Down should skip empty items[1] (%s) and land on items[2] (%s), got %s",
			allTypes[1].ShortName, allTypes[2].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Up skips over a single empty type
// ---------------------------------------------------------------------------

func TestQA_Availability_UpSkipsEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark items[1] as empty (count=0).
	m.SetAvailability(allTypes[1].ShortName, 0)

	// Move cursor to items[2] (Down once skips items[1], lands on items[2]).
	m, _ = m.Update(menuKeyDown())
	if m.SelectedItem().ShortName != allTypes[2].ShortName {
		t.Fatalf("setup: expected cursor at items[2] (%s), got %s",
			allTypes[2].ShortName, m.SelectedItem().ShortName)
	}

	// Now press Up — should skip items[1] and land on items[0].
	m, _ = m.Update(menuKeyUp())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[0].ShortName {
		t.Errorf("Up should skip empty items[1] (%s) and land on items[0] (%s), got %s",
			allTypes[1].ShortName, allTypes[0].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Down skips multiple consecutive empty items
// ---------------------------------------------------------------------------

func TestQA_Availability_DownSkipsMultipleEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need at least 4 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark items[1] and items[2] as empty (count=0). Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, 0)
	m.SetAvailability(allTypes[2].ShortName, 0)

	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[3].ShortName {
		t.Errorf("Down should skip empty items[1] (%s) and items[2] (%s) and land on items[3] (%s), got %s",
			allTypes[1].ShortName, allTypes[2].ShortName, allTypes[3].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Top (g) lands on first non-empty item
// ---------------------------------------------------------------------------

func TestQA_Availability_TopSkipsEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need at least 4 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark items[0] and items[1] as empty (count=0).
	m.SetAvailability(allTypes[0].ShortName, 0)
	m.SetAvailability(allTypes[1].ShortName, 0)

	// Move cursor past the empty items first.
	for range 5 {
		m, _ = m.Update(menuKeyDown())
	}

	// Now press Top (g).
	m, _ = m.Update(menuKeyTop())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[2].ShortName {
		t.Errorf("Top should land on first non-empty item (%s), got %s",
			allTypes[2].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Bottom (G) lands on last non-empty item
// ---------------------------------------------------------------------------

func TestQA_Availability_BottomSkipsEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	lastIdx := len(allTypes) - 1
	// Mark the last 2 items as empty (count=0).
	m.SetAvailability(allTypes[lastIdx].ShortName, 0)
	m.SetAvailability(allTypes[lastIdx-1].ShortName, 0)

	// Press Bottom (G).
	m, _ = m.Update(menuKeyBottom())

	selected := m.SelectedItem()
	expectedIdx := lastIdx - 2
	if selected.ShortName != allTypes[expectedIdx].ShortName {
		t.Errorf("Bottom should land on last non-empty item (%s at index %d), got %s",
			allTypes[expectedIdx].ShortName, expectedIdx, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: all items empty — cursor stays at 0, no panic
// ---------------------------------------------------------------------------

func TestQA_Availability_AllEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Skip("no resource types registered")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark ALL items as empty (count=0).
	for _, rt := range allTypes {
		m.SetAvailability(rt.ShortName, 0)
	}

	// When ALL items are empty, skipUnavailable gives up (no infinite loop).
	// The cursor moves normally via scroll.Down/Up since there is no navigable
	// item to snap to. The key invariant is: no panic, no infinite loop.

	// Press Down — should not panic.
	m, _ = m.Update(menuKeyDown())

	// Press Up — should not panic.
	m, _ = m.Update(menuKeyUp())

	// Press Top — no panic.
	m, _ = m.Update(menuKeyTop())

	// Press Bottom — no panic.
	m, _ = m.Update(menuKeyBottom())

	// Multiple rapid presses — no infinite loop.
	for range 20 {
		m, _ = m.Update(menuKeyDown())
	}
	for range 20 {
		m, _ = m.Update(menuKeyUp())
	}

	// View should not panic.
	output := m.View()
	if output == "" {
		t.Error("View() should not be empty even when all items are empty")
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: no availability (nil) — all items navigable
// ---------------------------------------------------------------------------

func TestQA_Availability_NoAvailability(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 2 {
		t.Skip("need at least 2 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Don't set any availability — nil map means all items are navigable.

	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[1].ShortName {
		t.Errorf("with nil availability, Down should move to items[1] (%s), got %s",
			allTypes[1].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: items NOT in availability map are navigable
// ---------------------------------------------------------------------------

func TestQA_Availability_UnknownItemsNavigable(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark items[0] as available (count=5) and items[2] as empty (count=0).
	// items[1] is NOT in the map — should be treated as navigable.
	m.SetAvailability(allTypes[0].ShortName, 5)
	m.SetAvailability(allTypes[2].ShortName, 0)

	// Cursor at items[0], press Down.
	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[1].ShortName {
		t.Errorf("unknown item (not in availability map) should be navigable; expected items[1] (%s), got %s",
			allTypes[1].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Enter on empty type is a no-op (defensive)
// ---------------------------------------------------------------------------

func TestQA_Availability_EnterBlockedOnEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Skip("no resource types registered")
	}

	m := newSizedMainMenu(t, 80, 200)
	// Mark ALL items as empty (count=0) so cursor is stuck on an empty item.
	for _, rt := range allTypes {
		m.SetAvailability(rt.ShortName, 0)
	}

	// Press Enter — should return nil cmd (no NavigateMsg).
	_, cmd := m.Update(menuKeyEnter())

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.Navigate); ok {
			t.Error("Enter on empty type should NOT produce a NavigateMsg")
		}
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: PageDown lands on non-empty item
// ---------------------------------------------------------------------------

func TestQA_Availability_PageDownSkipsEmpty(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 10 {
		t.Skip("need at least 10 resource types for PageDown test")
	}

	// Use a small height so PageDown doesn't jump to the end of the list.
	m := newSizedMainMenu(t, 80, 5)

	// With height=5, pageSize = height-1 = 4, so PageDown lands near items[4].
	// Mark items[4] as empty (count=0).
	m.SetAvailability(allTypes[4].ShortName, 0)

	m, _ = m.Update(menuKeyPageDown())

	selected := m.SelectedItem()
	if selected.ShortName == allTypes[4].ShortName {
		t.Errorf("PageDown should skip empty item at page boundary (%s)", allTypes[4].ShortName)
	}

	// Cursor should have landed on a non-empty item.
	avail := m.GetAvailability()
	count, known := avail[selected.ShortName]
	if known && count == 0 {
		t.Errorf("PageDown landed on known-empty item %s", selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: filter + availability skip works together
// ---------------------------------------------------------------------------

func TestQA_Availability_FilterWithSkip(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 5 {
		t.Skip("need at least 5 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)

	// Use a filter that matches multiple items (>= 2 chars required).
	// "ec" should match EC2, ECS, ECR, etc.
	m.SetFilter("ec")

	// Get the first filtered item.
	firstFiltered := m.SelectedItem()
	if firstFiltered.ShortName == "" {
		t.Skip("filter 'ec' matched no items")
	}

	// Press Down without availability to discover the second filtered item.
	mCopy := m
	mCopy, _ = mCopy.Update(menuKeyDown())
	secondFiltered := mCopy.SelectedItem()
	if secondFiltered.ShortName == firstFiltered.ShortName {
		t.Skip("filter 'ec' matched only 1 item")
	}

	// Mark the second filtered item as empty (count=0) on the original menu.
	m.SetAvailability(secondFiltered.ShortName, 0)

	// Press Down — should skip the empty second item.
	m, _ = m.Update(menuKeyDown())

	selected := m.SelectedItem()
	if selected.ShortName == secondFiltered.ShortName {
		t.Errorf("Down should skip empty filtered item %s", secondFiltered.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Down at end of list does not land on empty items
// ---------------------------------------------------------------------------

func TestQA_Availability_DownAtEndStays(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)
	lastIdx := len(allTypes) - 1

	// Mark the last item as empty (count=0).
	m.SetAvailability(allTypes[lastIdx].ShortName, 0)

	// Press Bottom to get near the end.
	m, _ = m.Update(menuKeyBottom())

	// Should land on the last non-empty item.
	selected := m.SelectedItem()
	if selected.ShortName == allTypes[lastIdx].ShortName {
		t.Errorf("Bottom should not land on empty last item %s", allTypes[lastIdx].ShortName)
	}

	// Press Down again — should stay put since the only item below is empty.
	prev := m.SelectedItem()
	m, _ = m.Update(menuKeyDown())
	current := m.SelectedItem()
	if current.ShortName != prev.ShortName {
		t.Errorf("Down at end should stay on %s, moved to %s", prev.ShortName, current.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: Up at top stays when items[0] is empty
// ---------------------------------------------------------------------------

func TestQA_Availability_UpAtTopStays(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 3 {
		t.Skip("need at least 3 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)

	// Mark items[0] as empty (count=0).
	m.SetAvailability(allTypes[0].ShortName, 0)

	// Press Top — should skip items[0] and land on items[1].
	m, _ = m.Update(menuKeyTop())

	selected := m.SelectedItem()
	if selected.ShortName != allTypes[1].ShortName {
		t.Errorf("Top should skip empty items[0] and land on items[1] (%s), got %s",
			allTypes[1].ShortName, selected.ShortName)
	}

	// Press Up — should stay at items[1] since items[0] is empty.
	m, _ = m.Update(menuKeyUp())
	selected = m.SelectedItem()
	if selected.ShortName != allTypes[1].ShortName {
		t.Errorf("Up at top should stay on items[1] (%s) since items[0] is empty, got %s",
			allTypes[1].ShortName, selected.ShortName)
	}
}

// ---------------------------------------------------------------------------
// Navigation skip: all resource types — mark every other empty, navigate through
// ---------------------------------------------------------------------------

func TestQA_Availability_NavigateAllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need at least 4 resource types")
	}

	m := newSizedMainMenu(t, 80, 200)

	// Mark every other item as empty (odd indices get count=0).
	for i, rt := range allTypes {
		if i%2 == 1 {
			m.SetAvailability(rt.ShortName, 0)
		} else {
			m.SetAvailability(rt.ShortName, i+1)
		}
	}

	// Navigate Down through all items — should only visit even-indexed items.
	visited := []string{m.SelectedItem().ShortName}
	for range allTypes {
		prev := m.SelectedItem().ShortName
		m, _ = m.Update(menuKeyDown())
		curr := m.SelectedItem().ShortName
		if curr != prev {
			visited = append(visited, curr)
		}
	}

	// Build set of empty (odd-indexed) items.
	emptySet := make(map[string]bool)
	for i, rt := range allTypes {
		if i%2 == 1 {
			emptySet[rt.ShortName] = true
		}
	}

	// Verify none of the visited items are empty.
	for _, shortName := range visited {
		if emptySet[shortName] {
			t.Errorf("cursor visited empty item %s during navigation", shortName)
		}
	}
}

// ---------------------------------------------------------------------------
// Count display: count appears in View() output
// ---------------------------------------------------------------------------

func TestQA_Availability_CountDisplayed(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 12)

	output := m.View()

	// The count "(12)" should appear near the EC2 row.
	if !strings.Contains(output, "(12)") {
		t.Errorf("View() should contain count '(12)' for ec2 with count=12, output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Count display: zero count appears for empty types
// ---------------------------------------------------------------------------

func TestQA_Availability_ZeroCountDisplayed(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)

	output := m.View()

	// The count "(0)" should appear near the EC2 row.
	if !strings.Contains(output, "(0)") {
		t.Errorf("View() should contain count '(0)' for ec2 with count=0, output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Count display: unknown types show no count
// ---------------------------------------------------------------------------

func TestQA_Availability_UnknownNoCount(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	// Don't set availability for any type.

	output := m.View()

	// Output should NOT contain any "(N)" patterns (parenthesized numbers).
	countPattern := regexp.MustCompile(`\(\d+\)`)
	if countPattern.MatchString(output) {
		t.Errorf("View() should not contain any count patterns when no availability is set, found match in output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Count display: count appears on selected row too
// ---------------------------------------------------------------------------

func TestQA_Availability_CountInSelectedRow(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Skip("no resource types registered")
	}

	// Set count for the first item (which is selected by default).
	m.SetAvailability(allTypes[0].ShortName, 5)

	output := m.View()

	// The count "(5)" should appear even on the selected (highlighted) row.
	if !strings.Contains(output, "(5)") {
		t.Errorf("View() should contain count '(5)' on selected row for %s, output:\n%s",
			allTypes[0].ShortName, output)
	}
}

// ---------------------------------------------------------------------------
// Count display: various count values
// ---------------------------------------------------------------------------

func TestQA_Availability_CountDisplayVariousValues(t *testing.T) {
	// Use actual registered resource type short names from AllResourceTypes().
	allTypes := resource.AllResourceTypes()
	if len(allTypes) < 4 {
		t.Skip("need at least 4 resource types")
	}

	testCases := []struct {
		name     string
		index    int
		count    int
		expected string
	}{
		{"single digit", 0, 1, "(1)"},
		{"double digit", 1, 42, "(42)"},
		{"triple digit", 2, 100, "(100)"},
		{"large count", 3, 1000, "(1000)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newSizedMainMenu(t, 80, 200)
			shortName := allTypes[tc.index].ShortName
			m.SetAvailability(shortName, tc.count)

			output := m.View()
			if !strings.Contains(output, tc.expected) {
				t.Errorf("View() should contain %q for %s with count=%d",
					tc.expected, shortName, tc.count)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Count display: all resource types show their counts
// ---------------------------------------------------------------------------

func TestQA_Availability_CountDisplayAllResourceTypes(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	allTypes := resource.AllResourceTypes()

	// Set unique counts for every resource type.
	for i, rt := range allTypes {
		m.SetAvailability(rt.ShortName, (i+1)*10)
	}

	output := m.View()

	// Verify each count appears in the output.
	for i, rt := range allTypes {
		expected := "(" + availItoa((i+1)*10) + ")"
		if !strings.Contains(output, expected) {
			t.Errorf("View() should contain %q for %s (count=%d)",
				expected, rt.ShortName, (i+1)*10)
		}
	}
}

// ---------------------------------------------------------------------------
// Truncated count display: "(5+)" when probe returned a truncated first page
// ---------------------------------------------------------------------------

func TestQA_Availability_TruncatedCountDisplay(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 5)
	m.SetTruncated("ec2", true)

	output := m.View()

	// Must contain "(5+)" — the plus indicates more resources exist.
	if !strings.Contains(output, "(5+)") {
		t.Errorf("View() should contain '(5+)' for truncated ec2 count=5, output:\n%s", output)
	}

	// Must NOT contain "(5)" without the plus — verify it's the truncated form.
	// We do this by checking that "(5)" only ever appears as part of "(5+)".
	withoutPlus := strings.ReplaceAll(output, "(5+)", "")
	if strings.Contains(withoutPlus, "(5)") {
		t.Errorf("View() should NOT contain bare '(5)' without plus for truncated ec2, output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Non-truncated count display: "(3)" without plus when not truncated
// ---------------------------------------------------------------------------

func TestQA_Availability_NonTruncatedCountDisplay(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 3)
	// Don't call SetTruncated — defaults to not truncated.

	output := m.View()

	// Must contain "(3)" — no plus suffix.
	if !strings.Contains(output, "(3)") {
		t.Errorf("View() should contain '(3)' for non-truncated ec2 count=3, output:\n%s", output)
	}

	// Must NOT contain "(3+)".
	if strings.Contains(output, "(3+)") {
		t.Errorf("View() should NOT contain '(3+)' for non-truncated ec2, output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// Truncated zero count: zero + truncated should not crash, shows "(0)"
// ---------------------------------------------------------------------------

func TestQA_Availability_TruncatedZeroCount(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 0)
	m.SetTruncated("ec2", true)

	output := m.View()

	// Truncated-zero must display as "(0+)" — the + signals the probe only saw
	// page 1 and it happened to be empty, but more pages exist.
	if !strings.Contains(output, "(0+)") {
		t.Errorf("View() should contain '(0+)' for ec2 with count=0 and truncated=true, output:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// GetTruncated returns a copy of the truncated state map
// ---------------------------------------------------------------------------

func TestQA_Availability_GetTruncated(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)

	// Before setting anything, GetTruncated should return nil.
	if got := m.GetTruncated(); got != nil {
		t.Errorf("GetTruncated() should return nil before any SetTruncated calls, got: %v", got)
	}

	// Set truncated for some types, not others.
	m.SetTruncated("ec2", true)
	m.SetTruncated("rds", true)
	m.SetTruncated("s3", false)

	got := m.GetTruncated()
	if got == nil {
		t.Fatal("GetTruncated() should not return nil after SetTruncated calls")
	}

	// Verify correct values.
	if !got["ec2"] {
		t.Errorf("GetTruncated()[\"ec2\"] = false, want true")
	}
	if !got["rds"] {
		t.Errorf("GetTruncated()[\"rds\"] = false, want true")
	}
	if got["s3"] {
		t.Errorf("GetTruncated()[\"s3\"] = true, want false")
	}

	// Verify it's a copy — mutation should not affect the model.
	got["ec2"] = false
	got2 := m.GetTruncated()
	if !got2["ec2"] {
		t.Errorf("GetTruncated() returned a reference, not a copy — mutation propagated")
	}
}

// ---------------------------------------------------------------------------
// ClearAvailability also clears truncated state
// ---------------------------------------------------------------------------

func TestQA_Availability_ClearAvailability_ClearsTruncated(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", 5)
	m.SetTruncated("ec2", true)
	m.SetAvailability("rds", 10)
	m.SetTruncated("rds", true)

	// Verify state is set.
	if avail := m.GetAvailability(); avail == nil || avail["ec2"] != 5 {
		t.Fatal("precondition: availability should be set before clear")
	}
	if trunc := m.GetTruncated(); trunc == nil || !trunc["ec2"] {
		t.Fatal("precondition: truncated should be set before clear")
	}

	m.ClearAvailability()

	// Both maps should be nil after clear.
	if avail := m.GetAvailability(); avail != nil {
		t.Errorf("GetAvailability() should return nil after ClearAvailability(), got: %v", avail)
	}
	if trunc := m.GetTruncated(); trunc != nil {
		t.Errorf("GetTruncated() should return nil after ClearAvailability(), got: %v", trunc)
	}
}

// ---------------------------------------------------------------------------
// Truncated count display: all resource types show "(N+)" when truncated
// ---------------------------------------------------------------------------

func TestQA_Availability_TruncatedCountAllResourceTypes(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	allTypes := resource.AllResourceTypes()

	// Set unique counts with truncation for every resource type.
	for i, rt := range allTypes {
		count := (i + 1) * 10
		m.SetAvailability(rt.ShortName, count)
		m.SetTruncated(rt.ShortName, true)
	}

	output := m.View()

	// Verify each count appears with "+" suffix in the output.
	for i, rt := range allTypes {
		count := (i + 1) * 10
		expected := "(" + availItoa(count) + "+)"
		if !strings.Contains(output, expected) {
			t.Errorf("View() should contain %q for truncated %s (count=%d)",
				expected, rt.ShortName, count)
		}
	}
}
