package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
// SetAvailability: empty type is greyed out
// ---------------------------------------------------------------------------

func TestQA_Availability_EmptyTypeGreyedOut(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", false)

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
	hasResources, ok := avail["ec2"]
	if !ok {
		t.Fatal("GetAvailability() missing ec2 entry")
	}
	if hasResources {
		t.Error("ec2 availability should be false")
	}
}

// ---------------------------------------------------------------------------
// SetAvailability: present type renders normally
// ---------------------------------------------------------------------------

func TestQA_Availability_PresentTypeNormal(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	m.SetAvailability("ec2", true)

	output := m.View()

	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should appear when marked as available")
	}

	avail := m.GetAvailability()
	if !avail["ec2"] {
		t.Error("ec2 availability should be true")
	}
}

// ---------------------------------------------------------------------------
// Unknown type in availability map: renders normally
// ---------------------------------------------------------------------------

func TestQA_Availability_UnknownTypeNormal(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	// Set availability for some type but NOT ec2.
	m.SetAvailability("s3", false)

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
	// Mark the second item as empty. Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, false)

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
	m.SetAvailability("ec2", false)
	m.SetAvailability("s3", true)

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
	m.SetAvailability("ec2", true)
	m.SetAvailability("s3", false)
	m.SetAvailability("rds", true)

	avail = m.GetAvailability()
	if len(avail) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(avail))
	}

	// Verify values.
	if !avail["ec2"] {
		t.Error("ec2 should be true")
	}
	if avail["s3"] {
		t.Error("s3 should be false")
	}
	if !avail["rds"] {
		t.Error("rds should be true")
	}

	// Mutating the returned map should not affect the model.
	avail["ec2"] = false
	avail["new_type"] = true

	fresh := m.GetAvailability()
	if !fresh["ec2"] {
		t.Error("mutating GetAvailability() return value should not affect model: ec2 should still be true")
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
	m.SetAvailability("ec2", false)
	m.SetFilter("ec2")

	output := m.View()

	// EC2 should still appear in filtered results even though it's marked empty.
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("filter matching greyed-out type should still show EC2 Instances")
	}

	// Availability state should be preserved through filtering.
	avail := m.GetAvailability()
	if avail["ec2"] {
		t.Error("ec2 availability should still be false after filtering")
	}
}

// ---------------------------------------------------------------------------
// Multiple empty types
// ---------------------------------------------------------------------------

func TestQA_Availability_MultipleEmpty(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)

	emptyTypes := []string{"ec2", "rds", "lambda", "eks", "vpc"}
	for _, shortName := range emptyTypes {
		m.SetAvailability(shortName, false)
	}

	avail := m.GetAvailability()
	if len(avail) != len(emptyTypes) {
		t.Fatalf("expected %d entries, got %d", len(emptyTypes), len(avail))
	}

	for _, shortName := range emptyTypes {
		if avail[shortName] {
			t.Errorf("%s should be false (empty)", shortName)
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

	m.SetAvailability("ec2", false)
	avail := m.GetAvailability()
	if avail["ec2"] {
		t.Error("ec2 should be false initially")
	}

	m.SetAvailability("ec2", true)
	avail = m.GetAvailability()
	if !avail["ec2"] {
		t.Error("ec2 should be true after overwrite")
	}
}

// ---------------------------------------------------------------------------
// All resource types: verify SetAvailability works for every registered type
// ---------------------------------------------------------------------------

func TestQA_Availability_AllResourceTypes(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)

	allTypes := resource.AllResourceTypes()
	// Mark every other type as empty.
	for i, rt := range allTypes {
		m.SetAvailability(rt.ShortName, i%2 == 0)
	}

	avail := m.GetAvailability()
	if len(avail) != len(allTypes) {
		t.Fatalf("expected %d entries, got %d", len(allTypes), len(avail))
	}

	for i, rt := range allTypes {
		expected := i%2 == 0
		if avail[rt.ShortName] != expected {
			t.Errorf("%s: availability = %v, want %v", rt.ShortName, avail[rt.ShortName], expected)
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
	m.SetAvailability("ec2", false)
	m.SetAvailability("s3", true)

	// Should not panic at small terminal size.
	output := m.View()
	if output == "" {
		t.Error("View() should not be empty at 40x10")
	}
}

func TestQA_Availability_LargeTerminal(t *testing.T) {
	m := newSizedMainMenu(t, 200, 50)
	m.SetAvailability("ec2", false)

	output := m.View()
	if !strings.Contains(output, "EC2 Instances") {
		t.Error("EC2 Instances should appear at 200x50")
	}
}

func TestQA_Availability_ZeroSize(t *testing.T) {
	m := newSizedMainMenu(t, 0, 0)
	m.SetAvailability("ec2", false)

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
	// Mark items[1] as empty. Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, false)

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
	// Mark items[1] as empty.
	m.SetAvailability(allTypes[1].ShortName, false)

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
	// Mark items[1] and items[2] as empty. Cursor starts at items[0].
	m.SetAvailability(allTypes[1].ShortName, false)
	m.SetAvailability(allTypes[2].ShortName, false)

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
	// Mark items[0] and items[1] as empty.
	m.SetAvailability(allTypes[0].ShortName, false)
	m.SetAvailability(allTypes[1].ShortName, false)

	// Move cursor past the empty items first.
	for i := 0; i < 5; i++ {
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
	// Mark the last 2 items as empty.
	m.SetAvailability(allTypes[lastIdx].ShortName, false)
	m.SetAvailability(allTypes[lastIdx-1].ShortName, false)

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
	// Mark ALL items as empty.
	for _, rt := range allTypes {
		m.SetAvailability(rt.ShortName, false)
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
	for i := 0; i < 20; i++ {
		m, _ = m.Update(menuKeyDown())
	}
	for i := 0; i < 20; i++ {
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
	// Mark items[0] as available and items[2] as empty.
	// items[1] is NOT in the map — should be treated as navigable.
	m.SetAvailability(allTypes[0].ShortName, true)
	m.SetAvailability(allTypes[2].ShortName, false)

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
	// Mark ALL items as empty so cursor is stuck on an empty item.
	for _, rt := range allTypes {
		m.SetAvailability(rt.ShortName, false)
	}

	// Press Enter — should return nil cmd (no NavigateMsg).
	_, cmd := m.Update(menuKeyEnter())

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.NavigateMsg); ok {
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
	// Mark items[4] as empty.
	m.SetAvailability(allTypes[4].ShortName, false)

	m, _ = m.Update(menuKeyPageDown())

	selected := m.SelectedItem()
	if selected.ShortName == allTypes[4].ShortName {
		t.Errorf("PageDown should skip empty item at page boundary (%s)", allTypes[4].ShortName)
	}

	// Cursor should have landed on a non-empty item.
	avail := m.GetAvailability()
	hasRes, known := avail[selected.ShortName]
	if known && !hasRes {
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

	// Mark the second filtered item as empty on the original menu.
	m.SetAvailability(secondFiltered.ShortName, false)

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

	// Mark the last item as empty.
	m.SetAvailability(allTypes[lastIdx].ShortName, false)

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

	// Mark items[0] as empty.
	m.SetAvailability(allTypes[0].ShortName, false)

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

	// Mark every other item as empty (odd indices).
	for i, rt := range allTypes {
		if i%2 == 1 {
			m.SetAvailability(rt.ShortName, false)
		} else {
			m.SetAvailability(rt.ShortName, true)
		}
	}

	// Navigate Down through all items — should only visit even-indexed items.
	visited := []string{m.SelectedItem().ShortName}
	for i := 0; i < len(allTypes); i++ {
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
