package unit_test

// detail_review_fixes_test.go — tests for the 6 review-issue fixes applied to
// detail.go and rightcolumn.go.
//
// Fix 1: Tab works on auto-shown panel (rightColShowing() includes auto-shown state)
// Fix 3: Independent field cursor — fieldCursor tracks position independently of viewport scroll
// Fix 5: Enter blocked on unavailable right-column rows (loading or count == 0)
// Fix 6: Count == -1 renders without number in right column
//
// TDD: these tests are written BEFORE the fixes land. Some will fail until the
// coder fixes are merged. That is the expected red-state.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// makeDetailForReviewFixes creates a DetailModel for "ec2" with an optional
// viewConfig and a simple field-only resource. Width/height are set by the caller.
func makeDetailWithConfig(t *testing.T, cfg *config.ViewsConfig, width, height int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-review-fixes",
		Name: "review-fixes-instance",
		Fields: map[string]string{
			"InstanceId":   "i-review-fixes",
			"VpcId":        "vpc-aabbcc00",
			"SubnetId":     "subnet-11223344",
			"ImageId":      "ami-deadbeef",
			"InstanceType": "t3.micro",
			"State":        "running",
			"LaunchTime":   "2026-01-01T00:00:00Z",
			"Platform":     "linux",
			"Architecture": "x86_64",
			"Tags":         "Name=review-fixes",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(width, height)
	return d
}

// makeDetailWide creates a DetailModel at width=140 (triggers auto-show when
// related defs are registered) with no viewConfig.
func makeDetailWide(t *testing.T) views.DetailModel {
	t.Helper()
	return makeDetailWithConfig(t, nil, 140, 30)
}

// makeDetailNarrow creates a DetailModel at width=80, height=5 (not all fields
// visible) with a viewConfig containing 10 Detail paths.
func makeDetailNarrow(t *testing.T) views.DetailModel {
	t.Helper()
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{
					{Path: "InstanceId"}, {Path: "VpcId"}, {Path: "SubnetId"}, {Path: "ImageId"}, {Path: "InstanceType"},
					{Path: "State"}, {Path: "LaunchTime"}, {Path: "Platform"}, {Path: "Architecture"}, {Path: "Tags"},
				},
			},
		},
	}
	return makeDetailWithConfig(t, cfg, 80, 5)
}

// pressDown sends a j/down key to the DetailModel.
func pressDown(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
}

// pressUp sends a k/up key to the DetailModel.
func pressUp(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
}

// pressEnterDetail sends Enter to the DetailModel.
func pressEnterDetail(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// pressTabDetail sends Tab to the DetailModel.
func pressTabDetail(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
}

// registerEC2Defs registers one RelatedDef for "ec2" and returns a cleanup func
// that restores the original defs so test order (shuffle) doesn't matter.
func registerEC2Defs(defs []resource.RelatedDef) func() {
	orig := resource.GetRelated("ec2")
	resource.RegisterRelated("ec2", defs)
	return func() {
		if orig == nil {
			resource.UnregisterRelated("ec2")
		} else {
			resource.RegisterRelated("ec2", orig)
		}
	}
}

// deliverResult delivers a RelatedCheckResultMsg to the DetailModel.
func deliverResult(d views.DetailModel, msg messages.RelatedCheckResultMsg) views.DetailModel {
	updated, _ := d.Update(msg)
	return updated
}

// ---------------------------------------------------------------------------
// Fix 1: Tab works on auto-shown panel
// ---------------------------------------------------------------------------

// TestDetail_TabWorksOnAutoShownPanel verifies that Tab toggles focus when the
// right column was auto-shown on SetSize (without pressing r first).
// The fix: Tab checks rightColShowing() which includes auto-shown state.
func TestDetail_TabWorksOnAutoShownPanel(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	// Precondition: right column must be auto-shown (no r press).
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140 with registered defs; cannot test auto-shown Tab")
	}

	// Capture view before Tab — right column is auto-shown but NOT focused.
	viewBeforeTab := d.View()

	// Send Tab WITHOUT pressing r first. With the fix, Tab should work because
	// rightColShowing() returns true for auto-shown state.
	d, _ = pressTabDetail(d)
	viewAfterTab := d.View()

	// After Tab, the focused row should get a highlight style — view must change.
	if viewBeforeTab == viewAfterTab {
		t.Errorf("Tab on auto-shown panel should change View() output (focus highlight on selected row); views were identical before and after Tab")
	}
}

// TestDetail_TabOnAutoShownPanel_RightColVisible verifies that after Tab focuses
// the auto-shown right column, the RELATED header is still present.
func TestDetail_TabOnAutoShownPanel_RightColVisible(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping")
	}

	// Tab without r first.
	d, _ = pressTabDetail(d)

	if !strings.Contains(d.View(), "RELATED") {
		t.Errorf("after Tab on auto-shown panel, RELATED header should still be visible; got:\n%s", d.View())
	}
}

// TestDetail_TabOnAutoShownPanel_ThenTabUnfocuses verifies that pressing Tab
// twice on an auto-shown panel focuses then unfocuses the right column.
func TestDetail_TabOnAutoShownPanel_ThenTabUnfocuses(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping")
	}

	// Tab 1: focus auto-shown panel.
	d, _ = pressTabDetail(d)
	viewFocused := d.View()

	// Tab 2: unfocus.
	d, _ = pressTabDetail(d)
	viewUnfocused := d.View()

	if viewFocused == viewUnfocused {
		t.Errorf("second Tab should remove focus highlight; focused and unfocused views should differ")
	}
}

// ---------------------------------------------------------------------------
// Fix 3: Independent field cursor
// ---------------------------------------------------------------------------

// TestDetail_FieldCursorIndependentFromScroll verifies that pressing j 3 times
// moves fieldCursor to index 3, and Enter emits RelatedNavigateMsg with
// TargetType from the field at index 3 (SubnetId → "subnet"), NOT from index 0.
func TestDetail_FieldCursorIndependentFromScroll(t *testing.T) {
	// Register NavigableField for "SubnetId" → "subnet" (path index 2 in 0-based).
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "SubnetId", TargetType: "subnet"},
	})

	// Ensure no related defs so right column isn't shown (avoids Tab interactions).
	unregisterEC2Related(t)

	// Width=80, height=5: not all 10 fields visible simultaneously.
	d := makeDetailNarrow(t)

	// Press j 3 times: fieldCursor should be at index 3 (SubnetId is at index 2
	// in the 10-path list, but we press down 3 times from index 0 → land on
	// index 3 which is "ImageId", or index 2 which is "SubnetId" after 2 presses).
	// The paths are: [0]InstanceId [1]VpcId [2]SubnetId [3]ImageId [4]InstanceType ...
	// Press down twice to reach SubnetId at index 2.
	var cmd tea.Cmd
	d, _ = pressDown(d)
	d, _ = pressDown(d)
	// Now fieldCursor == 2 → SubnetId (navigable → "subnet").

	_, cmd = pressEnterDetail(d)

	if cmd == nil {
		t.Fatal("Enter on a navigable field (SubnetId) should return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce messages.RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "subnet" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q (field at cursor index 2 = SubnetId), got %q", "subnet", nav.TargetType)
	}
	// TargetID must be the value from the SubnetId field.
	if nav.TargetID != "subnet-11223344" {
		t.Errorf("RelatedNavigateMsg.TargetID: want %q, got %q", "subnet-11223344", nav.TargetID)
	}
}

// TestDetail_FieldCursorClamps verifies the cursor never goes out of bounds.
// Press j 10 times (with 10 fields, cursor should stop at 9).
// Press k 20 times (cursor should stop at 0).
// Enter should always reference a valid field (no panic, no index out of range).
func TestDetail_FieldCursorClamps(t *testing.T) {
	// Register a navigable field so Enter has something to fire on.
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "InstanceId", TargetType: "ec2"},
	})

	unregisterEC2Related(t)

	d := makeDetailNarrow(t)

	// Press j 10 times — cursor must clamp at len(fieldList)-1.
	for range 10 {
		d, _ = pressDown(d)
	}

	// Press k 20 times — cursor must clamp at 0.
	for range 20 {
		d, _ = pressUp(d)
	}

	// Enter must not panic and should produce a cmd (cursor is at 0 = InstanceId = navigable).
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Enter after clamped cursor panicked: %v", r)
		}
	}()

	d, cmd := pressEnterDetail(d)
	_ = d

	// At index 0 (InstanceId is navigable), cmd must be non-nil.
	if cmd == nil {
		t.Error("Enter on navigable field at index 0 (after clamping) should return non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedNavigateMsg after clamped cursor, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q, got %q", "ec2", nav.TargetType)
	}
}

// TestDetail_FieldCursorDown_TwoFieldsClamps verifies the two-field boundary.
// With only 2 fields, pressing j once lands at index 1, pressing again stays at 1.
func TestDetail_FieldCursorDown_TwoFieldsClamps(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "InstanceId", TargetType: "ec2"},
		{FieldPath: "VpcId", TargetType: "vpc"},
	})
	unregisterEC2Related(t)

	// 2-field config.
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{{Path: "InstanceId"}, {Path: "VpcId"}},
			},
		},
	}
	res := resource.Resource{
		ID:   "i-two-fields",
		Name: "two-fields",
		Fields: map[string]string{
			"InstanceId": "i-two-fields",
			"VpcId":      "vpc-two-fields",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", cfg, k)
	d.SetSize(80, 24)

	// Press down twice — should clamp at index 1.
	d, _ = pressDown(d)
	d, _ = pressDown(d)

	// Enter: cursor at 1 → VpcId → "vpc".
	d, cmd := pressEnterDetail(d)
	_ = d

	if cmd == nil {
		t.Fatal("Enter on VpcId (cursor at index 1) should return non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "vpc" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q (VpcId at index 1), got %q", "vpc", nav.TargetType)
	}
}

// TestDetail_FieldCursorUp_AtZeroStaysZero verifies pressing k at index 0 keeps
// the cursor at 0 and Enter still fires on the first field.
func TestDetail_FieldCursorUp_AtZeroStaysZero(t *testing.T) {
	replaceEC2NavigableFields(t, []resource.NavigableField{
		{FieldPath: "InstanceId", TargetType: "ec2"},
	})
	unregisterEC2Related(t)

	d := makeDetailNarrow(t)

	// Press k 5 times from index 0 — cursor must stay at 0.
	for range 5 {
		d, _ = pressUp(d)
	}

	d, cmd := pressEnterDetail(d)
	_ = d

	if cmd == nil {
		t.Fatal("Enter at clamped index 0 should return non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "ec2" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q at index 0, got %q", "ec2", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// Fix 5: Enter blocked on unavailable right-column rows
// ---------------------------------------------------------------------------

// TestRightColumn_EnterBlockedOnLoadingRow verifies that pressing Enter on the
// focused right column BEFORE any RelatedCheckResultMsg arrives returns nil cmd.
// Row is in loading state (count == -1, loading == true).
func TestRightColumn_EnterBlockedOnLoadingRow(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping loading-row Enter test")
	}

	// Transition auto-shown → explicitly visible so Tab works.
	d = makeExplicitlyVisible(d)

	// Tab: focus right column. No result delivered yet — row is loading.
	d, _ = pressTabDetail(d)

	// Enter on a loading row: should return nil cmd (blocked).
	_, cmd := pressEnterDetail(d)

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.RelatedNavigateMsg); ok {
			t.Errorf("Enter on loading row should NOT emit RelatedNavigateMsg; got RelatedNavigateMsg with msg=%+v", msg)
		}
	}
}

// TestRightColumn_EnterBlockedOnZeroCountRow verifies that pressing Enter on a
// right-column row with count == 0 returns nil cmd (nothing to navigate to).
func TestRightColumn_EnterBlockedOnZeroCountRow(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping zero-count Enter test")
	}

	// Transition auto-shown → explicitly visible.
	d = makeExplicitlyVisible(d)

	// Deliver result with Count == 0 (no related resources found).
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       0,
			ResourceIDs: nil,
			Err:         nil,
		},
	})

	// Tab: focus right column.
	d, _ = pressTabDetail(d)

	// Enter on a zero-count row: should return nil cmd.
	_, cmd := pressEnterDetail(d)

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.RelatedNavigateMsg); ok {
			t.Errorf("Enter on zero-count row should NOT emit RelatedNavigateMsg; got RelatedNavigateMsg with msg=%+v", msg)
		}
	}
}

// TestRightColumn_EnterAllowedOnPositiveCountRow verifies that pressing Enter on
// a right-column row with count > 0 emits a RelatedNavigateMsg.
func TestRightColumn_EnterAllowedOnPositiveCountRow(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping positive-count Enter test")
	}

	// Transition auto-shown → explicitly visible (rebuilds rightCol).
	d = makeExplicitlyVisible(d)

	// Deliver result with Count == 2 (positive — navigation allowed).
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       2,
			ResourceIDs: []string{"tg-1", "tg-2"},
			Err:         nil,
		},
	})

	// Tab: focus right column (cursor on first row = "tg").
	d, _ = pressTabDetail(d)

	// Enter on positive-count row: must emit RelatedNavigateMsg.
	_, cmd := pressEnterDetail(d)

	if cmd == nil {
		t.Fatal("Enter on positive-count row should return non-nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("cmd() should produce messages.RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "tg" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q, got %q", "tg", nav.TargetType)
	}
	if len(nav.RelatedIDs) != 2 {
		t.Errorf("RelatedNavigateMsg.RelatedIDs: want 2 IDs, got %d", len(nav.RelatedIDs))
	}
	if nav.RelatedIDs[0] != "tg-1" {
		t.Errorf("RelatedNavigateMsg.RelatedIDs[0]: want %q, got %q", "tg-1", nav.RelatedIDs[0])
	}
}

// TestRightColumn_EnterBlockedOnLoadingRow_AutoShown verifies the same blocking
// behavior when the right column was never explicitly toggled (auto-shown path).
func TestRightColumn_EnterBlockedOnLoadingRow_AutoShown(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping auto-shown loading-block test")
	}

	// Tab WITHOUT pressing r: auto-shown → Tab focuses (Fix 1 + Fix 5 combined).
	d, _ = pressTabDetail(d)

	// Enter before any result: loading row — must be blocked.
	_, cmd := pressEnterDetail(d)

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(messages.RelatedNavigateMsg); ok {
			t.Errorf("Enter on auto-shown loading row should NOT emit RelatedNavigateMsg; got %+v", msg)
		}
	}
}

// ---------------------------------------------------------------------------
// Fix 6: Count == -1 renders without number
// ---------------------------------------------------------------------------

// TestRightColumn_NegativeOneCountRendersWithoutNumber verifies that when a
// RelatedCheckResultMsg delivers Count == -1 (unknown / not applicable), the
// right column does NOT render "(-1)" but DOES show the display name.
func TestRightColumn_NegativeOneCountRendersWithoutNumber(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping negative-one-count render test")
	}

	// Deliver Count == -1 (not a simple "no results" — unknown/not-applicable).
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       -1,
			ResourceIDs: nil,
			Err:         nil,
		},
	})

	view := d.View()

	// Must NOT contain the string "(-1)" — that would be confusing to the user.
	if strings.Contains(view, "(-1)") {
		t.Errorf("View() must not contain \"(-1)\" when count is -1 (unknown); got:\n%s", view)
	}

	// Must still show the display name (row is present, just without a count number).
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("View() must still contain display name %q when count is -1; got:\n%s", "Target Groups", view)
	}
}

// TestRightColumn_NegativeOneCountRendersWithoutNumber_AfterAutoShow verifies
// the same behavior when the right column was auto-shown (no explicit toggle).
func TestRightColumn_NegativeOneCountRendersWithoutNumber_AfterAutoShow(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping")
	}

	// Deliver -1 for "tg", positive for "asg".
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       -1,
			ResourceIDs: nil,
			Err:         nil,
		},
	})
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "asg",
			Count:       3,
			ResourceIDs: []string{"asg-1", "asg-2", "asg-3"},
			Err:         nil,
		},
	})

	view := d.View()

	// "(-1)" must not appear.
	if strings.Contains(view, "(-1)") {
		t.Errorf("View() must not contain \"(-1)\"; got:\n%s", view)
	}

	// Both display names must still appear.
	if !strings.Contains(view, "Target Groups") {
		t.Errorf("View() must contain %q; got:\n%s", "Target Groups", view)
	}
	if !strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("View() must contain %q; got:\n%s", "Auto Scaling Groups", view)
	}

	// Positive count for asg must appear normally.
	if !strings.Contains(view, "(3)") {
		t.Errorf("View() must contain \"(3)\" for asg with count=3; got:\n%s", view)
	}
}

// TestRightColumn_NegativeOneCount_NotZeroCount verifies that count==-1 is
// rendered differently from count==0: count==0 shows "(0)", count==-1 shows no number.
func TestRightColumn_NegativeOneCount_NotZeroCount(t *testing.T) {
	cleanup := registerEC2Defs([]resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})
	defer cleanup()

	d := makeDetailWide(t)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping")
	}

	// tg: count -1, asg: count 0.
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       -1,
			ResourceIDs: nil,
			Err:         nil,
		},
	})
	d = deliverResult(d, messages.RelatedCheckResultMsg{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "asg",
			Count:       0,
			ResourceIDs: nil,
			Err:         nil,
		},
	})

	view := d.View()

	// count==-1: no number at all.
	if strings.Contains(view, "(-1)") {
		t.Errorf("count==-1 must not render as \"(-1)\"; got:\n%s", view)
	}

	// count==0: must show "(0)" per existing design.
	if !strings.Contains(view, "(0)") {
		t.Errorf("count==0 must still render as \"(0)\"; got:\n%s", view)
	}
}
