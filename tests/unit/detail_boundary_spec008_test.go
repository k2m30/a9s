package unit_test

// detail_boundary_spec008_test.go — Spec-008: j/k boundary clamping in DetailModel.
//
// NOTE: `fieldCursor` and `fieldList` already exist on this branch.
// The ONLY missing item is the exported getter `FieldCursor() int`.
// This file uses `//go:build spec008` tag until the coder adds the getter.
//
// Once the coder adds:
//   func (m DetailModel) FieldCursor() int { return m.fieldCursor }
// remove the `//go:build spec008` tag from this file so it runs in normal CI.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeSpec008Detail builds a DetailModel with the given field list and size.
// The resource carries a Fields map keyed by the same strings used in ViewDef.Detail.
func makeSpec008Detail(t *testing.T, fieldPaths []string, width, height int) views.DetailModel {
	t.Helper()
	fields := make(map[string]string, len(fieldPaths))
	for _, p := range fieldPaths {
		fields[p] = p + "-value"
	}
	res := resource.Resource{
		ID:     "i-spec008",
		Name:   "spec008-instance",
		Fields: fields,
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {Detail: fieldPaths},
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(width, height)
	return d
}

// pressJKey sends a single j keypress through the DetailModel Update.
func pressJKey(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
}

// pressKKey sends a single k keypress through the DetailModel Update.
func pressKKey(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
}

// ---------------------------------------------------------------------------
// Compile-gate: FieldCursor() must exist
// ---------------------------------------------------------------------------

// TestDetail_008_FieldCursorGetter_Exists verifies the exported FieldCursor()
// accessor exists on DetailModel.
// FAILS TO COMPILE until the coder adds:
//
//	func (m DetailModel) FieldCursor() int { return m.fieldCursor }
func TestDetail_008_FieldCursorGetter_Exists(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId"}, 80, 20)
	cursor := d.FieldCursor() // compile error until getter is added
	if cursor < 0 {
		t.Errorf("FieldCursor() returned negative value %d", cursor)
	}
}

// ---------------------------------------------------------------------------
// j at last field clamps cursor
// ---------------------------------------------------------------------------

// TestDetail_008_JAtLastField_CursorClamped verifies that pressing j when the
// cursor is already at the last navigable field does not advance the cursor
// beyond the last index.
//
// Given: 3 fields in navViewConfig, height large enough to show all
// When: cursor is navigated to index 2 (last), then j is pressed again
// Then: FieldCursor() == 2 (unchanged, not 3)
func TestDetail_008_JAtLastField_CursorClamped(t *testing.T) {
	fieldPaths := []string{"InstanceId", "VpcId", "SubnetId"}
	d := makeSpec008Detail(t, fieldPaths, 80, 20)

	// Navigate to last field (index 2)
	d, _ = pressJKey(d)
	d, _ = pressJKey(d)
	if d.FieldCursor() != 2 {
		t.Fatalf("precondition: expected cursor at 2 after 2 j presses on 3-field list, got %d", d.FieldCursor())
	}

	// Press j one more time at the boundary
	d, _ = pressJKey(d)
	if d.FieldCursor() != 2 {
		t.Errorf("j at last field (index 2 of 3) must clamp cursor at 2, got %d", d.FieldCursor())
	}
}

// TestDetail_008_JAtLastField_10Fields verifies clamping with a 10-field list.
//
// Given: 10 fields, cursor navigated to index 9 (last)
// When: j is pressed again
// Then: FieldCursor() == 9
func TestDetail_008_JAtLastField_10Fields(t *testing.T) {
	fieldPaths := []string{
		"InstanceId", "VpcId", "SubnetId", "PrivateIpAddress", "PublicIpAddress",
		"InstanceType", "State", "LaunchTime", "ImageId", "KeyName",
	}
	d := makeSpec008Detail(t, fieldPaths, 80, 40)

	// Navigate to last field
	for range 9 {
		d, _ = pressJKey(d)
	}
	if d.FieldCursor() != 9 {
		t.Fatalf("precondition: expected cursor at 9 after 9 j presses on 10-field list, got %d", d.FieldCursor())
	}

	// Press j at boundary
	d, _ = pressJKey(d)
	if d.FieldCursor() != 9 {
		t.Errorf("j at last field (index 9 of 10) must clamp cursor at 9, got %d", d.FieldCursor())
	}
}

// ---------------------------------------------------------------------------
// k at first field clamps cursor
// ---------------------------------------------------------------------------

// TestDetail_008_KAtFirstField_CursorClamped verifies that pressing k when the
// cursor is at index 0 does not move it to -1 or wrap around.
//
// Given: 3 fields, cursor at 0 (initial state)
// When: k is pressed
// Then: FieldCursor() == 0
func TestDetail_008_KAtFirstField_CursorClamped(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId", "SubnetId"}, 80, 20)

	// Cursor starts at 0 by default
	if d.FieldCursor() != 0 {
		t.Fatalf("precondition: expected initial cursor at 0, got %d", d.FieldCursor())
	}

	// Press k at boundary
	d, _ = pressKKey(d)
	if d.FieldCursor() != 0 {
		t.Errorf("k at first field (index 0) must clamp cursor at 0, got %d", d.FieldCursor())
	}
}

// TestDetail_008_KAtFirstField_MultipleKPresses verifies that multiple k presses
// at the boundary all keep cursor at 0.
func TestDetail_008_KAtFirstField_MultipleKPresses(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId"}, 80, 20)

	for range 5 {
		d, _ = pressKKey(d)
	}
	if d.FieldCursor() != 0 {
		t.Errorf("5 k presses at top boundary must keep cursor at 0, got %d", d.FieldCursor())
	}
}

// ---------------------------------------------------------------------------
// Basic j/k navigation (preconditions for boundary tests)
// ---------------------------------------------------------------------------

// TestDetail_008_JMovesDown verifies that j advances the cursor from 0 to 1.
func TestDetail_008_JMovesDown(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId", "SubnetId"}, 80, 20)

	d, _ = pressJKey(d)
	if d.FieldCursor() != 1 {
		t.Errorf("j from cursor 0 should move to 1, got %d", d.FieldCursor())
	}
}

// TestDetail_008_KMovesUp verifies that k moves the cursor back up from 1 to 0.
func TestDetail_008_KMovesUp(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId", "SubnetId"}, 80, 20)

	d, _ = pressJKey(d) // cursor = 1
	d, _ = pressKKey(d) // cursor should return to 0
	if d.FieldCursor() != 0 {
		t.Errorf("k from cursor 1 should return to 0, got %d", d.FieldCursor())
	}
}

// TestDetail_008_InitialCursorIsZero verifies that a freshly created DetailModel
// starts with FieldCursor() == 0.
func TestDetail_008_InitialCursorIsZero(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId", "VpcId"}, 80, 20)
	if d.FieldCursor() != 0 {
		t.Errorf("initial FieldCursor() must be 0, got %d", d.FieldCursor())
	}
}

// TestDetail_008_SingleField_BothDirectionsClamped verifies that a 1-field
// detail view clamps both j and k at 0.
func TestDetail_008_SingleField_BothDirectionsClamped(t *testing.T) {
	d := makeSpec008Detail(t, []string{"InstanceId"}, 80, 20)

	d, _ = pressJKey(d)
	if d.FieldCursor() != 0 {
		t.Errorf("j on single-field detail must clamp cursor at 0, got %d", d.FieldCursor())
	}

	d, _ = pressKKey(d)
	if d.FieldCursor() != 0 {
		t.Errorf("k on single-field detail must clamp cursor at 0, got %d", d.FieldCursor())
	}
}

// TestDetail_008_NoPanic_EmptyFields verifies that pressing j/k on a DetailModel
// with no configured fields does not panic.
func TestDetail_008_NoPanic_EmptyFields(t *testing.T) {
	res := resource.Resource{ID: "i-noop", Name: "noop", Fields: map[string]string{}}
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{"ec2": {Detail: []string{}}}}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(80, 20)

	// These must not panic
	d, _ = pressJKey(d) //nolint:ineffassign,staticcheck // crash-verification test
	d, _ = pressKKey(d) //nolint:ineffassign,staticcheck // crash-verification test
}
