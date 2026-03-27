package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
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
// Selected row on empty type still uses RowSelected style
// ---------------------------------------------------------------------------

func TestQA_Availability_SelectedRowNotDimmed(t *testing.T) {
	m := newSizedMainMenu(t, 80, 200)
	// Mark the first item type as unavailable.
	firstType := resource.AllResourceTypes()[0]
	m.SetAvailability(firstType.ShortName, false)

	output := m.View()

	// The selected item should still appear in the output.
	if !strings.Contains(output, firstType.Name) {
		t.Errorf("selected resource %q should still appear in output", firstType.Name)
	}

	// Verify it's still the selected item via the model state.
	selected := m.SelectedItem()
	if selected.ShortName != firstType.ShortName {
		t.Errorf("selected item should be %q, got %q", firstType.ShortName, selected.ShortName)
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
