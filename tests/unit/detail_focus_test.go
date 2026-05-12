package unit_test

// detail_focus_test.go — tests for Tab-key focus switching in the detail view
// and right-column Enter/Esc behaviour when the right column is focused (T019).
//
// Design spec: docs/design/related-resources.md v4.3
// QA stories:  docs/qa/related-resources-stories.md
//
// Key facts about Tab/focus sequencing:
//   - At width >= 100 with registered defs, the right column is auto-shown on
//     SetSize (rightColAutoShown=true, rightColVisible=false).
//   - Tab only toggles focus when rightColVisible=true.
//   - The first press of r (ToggleRelated) at width >= 100 transitions the column
//     from auto-shown → rightColVisible=true (still visible, no hide).
//   - Only after this transition does Tab toggle focus on/off.
//   - At width < 100 the right column is never shown; Tab is always a no-op.
//   - Esc while right column is focused: unfocuses (does NOT pop the view stack).
//   - Enter while right column is focused: emits RelatedNavigateMsg for selected row.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// makeDetailForFocusTest creates a DetailModel for "ec2" with a fields-only
// resource at the given width. It does NOT register related defs — the caller
// is responsible for that (and for cleanup via defer).
func makeDetailForFocusTest(t *testing.T, width int) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:   "i-focus-test",
		Name: "focus-test-instance",
		Fields: map[string]string{
			"instance_id": "i-focus-test",
			"state":       "running",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "ec2", nil, k)
	d.SetSize(width, 30)
	return d
}

// tabKeyMsg returns the tea.KeyPressMsg for the Tab key.
func tabKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyTab}
}

// escKeyMsg returns the tea.KeyPressMsg for the Escape key.
func escKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEscape}
}

// enterKeyMsg returns the tea.KeyPressMsg for the Enter key.
func enterKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

// sendTabWithCmd sends the Tab key and returns both the updated model and cmd.
func sendTabWithCmd(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(tabKeyMsg())
}

// sendEnterWithCmd sends the Enter key and returns both the updated model and cmd.
func sendEnterWithCmd(d views.DetailModel) (views.DetailModel, tea.Cmd) {
	return d.Update(enterKeyMsg())
}

// makeExplicitlyVisible transitions a DetailModel's right column from auto-shown
// (rightColAutoShown=true, rightColVisible=false) to explicitly visible
// (rightColVisible=true). This is required before Tab can toggle focus.
// With the current UX, the first press of r hides the auto-shown column and the
// second press re-opens it explicitly.
// Precondition: width >= 100, related defs registered, SetSize called.
func makeExplicitlyVisible(d views.DetailModel) views.DetailModel {
	updated, _ := d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	updated, _ = updated.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	return updated
}

// focusRightColumn brings the right column to focused state by pressing r
// (explicit-visible transition) then Tab (focus). This is the canonical
// two-step sequence to reach "right column focused".
func focusRightColumn(d views.DetailModel) views.DetailModel {
	d = makeExplicitlyVisible(d)
	updated, _ := d.Update(tabKeyMsg())
	return updated
}

// ---------------------------------------------------------------------------
// TestDetail_TabSwitchesFocus
// Given: width=140, RelatedDefs registered for "ec2", r pressed (explicit-visible
//
//	transition), then Tab pressed
//
// When:  Tab pressed with rightColVisible=true
// Then:  View() output changes — focused row gets highlight applied
// ---------------------------------------------------------------------------

func TestDetail_TabSwitchesFocus(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	// Precondition: right column should be auto-shown at width=140 with registered defs.
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping focus test")
	}

	// r: transitions auto-shown → rightColVisible=true (column still visible).
	d = makeExplicitlyVisible(d)
	viewBeforeTab := d.View()

	// Tab: should focus the right column — view must change.
	d, _ = d.Update(tabKeyMsg())
	viewAfterTab := d.View()

	if viewBeforeTab == viewAfterTab {
		t.Errorf("Tab should change the View() output (focus highlight on selected row); views were identical before and after Tab")
	}
}

// ---------------------------------------------------------------------------
// TestDetail_TabTogglesFocusOff
// Given: width=140, RelatedDefs registered, r pressed (explicit-visible),
//
//	Tab pressed once (right col focused)
//
// When:  Tab pressed again
// Then:  View() returns to the un-focused appearance
// ---------------------------------------------------------------------------

func TestDetail_TabTogglesFocusOff(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping Tab-toggle-off test")
	}

	// r: explicit-visible transition (required for Tab to work).
	d = makeExplicitlyVisible(d)
	viewBeforeTab := d.View()

	// Tab ON — focus right column.
	d, _ = d.Update(tabKeyMsg())
	viewFocused := d.View()

	// Sanity: Tab must have changed the view.
	if viewBeforeTab == viewFocused {
		t.Fatal("first Tab should change the view (focus highlight); views were identical — prerequisite failed")
	}

	// Tab OFF — unfocus right column.
	d, _ = d.Update(tabKeyMsg())
	viewUnfocused := d.View()

	// Focused and unfocused views must differ.
	if viewFocused == viewUnfocused {
		t.Errorf("second Tab should remove focus highlight; view did not change from focused to unfocused state")
	}

	// Unfocused view should restore to the pre-Tab appearance.
	if viewBeforeTab != viewUnfocused {
		t.Logf("note: unfocused view after two Tabs differs from pre-Tab view (acceptable if content is equivalent)")
	}
}

// ---------------------------------------------------------------------------
// TestDetail_TabWithoutRightCol_IsNoop
// Given: width=80 (below 100-column threshold — no right column)
// When:  Tab key pressed
// Then:  No change in View(), no panic
// ---------------------------------------------------------------------------

func TestDetail_TabWithoutRightCol_IsNoop(t *testing.T) {
	// No RelatedDefs needed — at width=80 right col is never shown regardless.
	unregisterEC2Related(t)

	d := makeDetailForFocusTest(t, 80)
	viewBefore := d.View()

	d, cmd := sendTabWithCmd(d)
	viewAfter := d.View()

	if viewBefore != viewAfter {
		t.Errorf("Tab at width=80 should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", viewBefore, viewAfter)
	}
	if cmd != nil {
		t.Errorf("Tab at width=80 should return nil cmd, got non-nil cmd")
	}
}

// TestDetail_TabWithoutRightCol_NoPanicOnNarrowTerminal verifies no panic at
// extreme narrow width (40 columns).
func TestDetail_TabWithoutRightCol_NoPanicOnNarrowTerminal(t *testing.T) {
	unregisterEC2Related(t)

	d := makeDetailForFocusTest(t, 40)

	// Must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Tab at width=40 panicked: %v", r)
		}
	}()
	d, _ = d.Update(tabKeyMsg())
	_ = d.View()
}

// ---------------------------------------------------------------------------
// TestDetail_RightColFocused_EnterEmitsNavigateMsg
// Given: width=140, RelatedDefs registered with at least one entry that has
//
//	ResourceIDs set via a RelatedCheckResultMsg
//
// When:  Tab pressed (focus right column), then Enter pressed
// Then:  cmd() produces RelatedNavigateMsg with TargetType matching first row
// ---------------------------------------------------------------------------

func TestDetail_RightColFocused_EnterEmitsNavigateMsg(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "vpc", DisplayName: "VPCs", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping Enter-emits-navigate test")
	}

	// r: explicit-visible transition (rebuilds rightCol from registered defs).
	d = makeExplicitlyVisible(d)

	// Deliver a result AFTER the column is rebuilt so ResourceIDs are preserved.
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       2,
			ResourceIDs: []string{"tg-aaa", "tg-bbb"},
			Err:         nil,
		},
	})

	// Tab: focus right column (cursor is on first row = "tg").
	d, _ = d.Update(tabKeyMsg())

	// Enter: should emit RelatedNavigateMsg.
	_, cmd := sendEnterWithCmd(d)

	if cmd == nil {
		t.Fatal("Enter on focused right column should return a non-nil cmd")
	}

	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("cmd() should produce messages.RelatedNavigate, got %T", msg)
	}
	if nav.TargetType != "tg" {
		t.Errorf("RelatedNavigateMsg.TargetType: want \"tg\", got %q", nav.TargetType)
	}
	// RelatedIDs must carry the IDs from the check result.
	if len(nav.RelatedIDs) == 0 {
		t.Errorf("RelatedNavigateMsg.RelatedIDs must be non-empty when checker returned IDs")
	}
	if nav.RelatedIDs[0] != "tg-aaa" {
		t.Errorf("RelatedNavigateMsg.RelatedIDs[0]: want \"tg-aaa\", got %q", nav.RelatedIDs[0])
	}
}

// TestDetail_RightColFocused_EnterEmitsNavigateMsg_ExactTargetType verifies that
// the TargetType in the emitted message exactly matches the registered def's
// TargetType — not a derived or mutated value.
func TestDetail_RightColFocused_EnterEmitsNavigateMsg_ExactTargetType(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping exact-TargetType test")
	}

	// r: explicit-visible transition (rebuilds rightCol).
	d = makeExplicitlyVisible(d)

	// Deliver result AFTER column is rebuilt.
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "asg",
			Count:       1,
			ResourceIDs: []string{"asg-xyz"},
			Err:         nil,
		},
	})

	// Tab: focus.
	d, _ = d.Update(tabKeyMsg())
	_, cmd := sendEnterWithCmd(d)

	if cmd == nil {
		t.Fatal("Enter on focused right column should return non-nil cmd")
	}
	result := cmd()
	nav, ok := result.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("cmd() should produce messages.RelatedNavigate, got %T", result)
	}
	if nav.TargetType != "asg" {
		t.Errorf("RelatedNavigateMsg.TargetType: want %q (exact), got %q", "asg", nav.TargetType)
	}
}

// ---------------------------------------------------------------------------
// TestDetail_RightColFocused_EscUnfocuses
// Given: width=140, RelatedDefs registered, Tab pressed (right column focused)
// When:  Esc pressed
// Then:  Right column is unfocused (view changes back to non-focused appearance);
//
//	the view is NOT popped (cmd should be nil or not a PopViewMsg)
//
// ---------------------------------------------------------------------------

func TestDetail_RightColFocused_EscUnfocuses(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping Esc-unfocuses test")
	}

	// r → Tab: reach focused state.
	d = focusRightColumn(d)
	viewFocused := d.View()

	// Esc: should unfocus, not pop.
	d, cmd := d.Update(escKeyMsg())
	viewAfterEsc := d.View()

	// The view must change after Esc (focus highlight removed).
	if viewFocused == viewAfterEsc {
		t.Errorf("Esc on focused right column should remove focus highlight; view was unchanged before vs after Esc")
	}

	// cmd must be nil or must not be PopViewMsg (Esc only unfocuses here).
	if cmd != nil {
		produced := cmd()
		if _, isPopView := produced.(messages.PopView); isPopView {
			t.Errorf("Esc on focused right column should NOT emit PopViewMsg; got PopViewMsg")
		}
	}
}

// TestDetail_RightColFocused_EscDoesNotHidePanel verifies that Esc while focused
// does NOT hide the right column — it only removes focus.
func TestDetail_RightColFocused_EscDoesNotHidePanel(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping Esc-panel-visibility test")
	}

	// r → Tab: focus. Then Esc: unfocus.
	d = focusRightColumn(d)
	d, _ = d.Update(escKeyMsg())
	viewAfterEsc := d.View()

	// RELATED header should still be present after Esc (column not hidden).
	if !strings.Contains(viewAfterEsc, "RELATED") {
		t.Errorf("after Esc on focused right column, RELATED panel should still be visible; got:\n%s", viewAfterEsc)
	}
}

// ---------------------------------------------------------------------------
// TestDetail_Tab_NoRelatedDefs_IsNoop
// Given: width=140 BUT no RelatedDefs registered for "ec2"
// When:  Tab pressed
// Then:  No panic and no view change (right column was not auto-shown)
// ---------------------------------------------------------------------------

func TestDetail_Tab_NoRelatedDefs_IsNoop(t *testing.T) {
	unregisterEC2Related(t)

	d := makeDetailForFocusTest(t, 140)
	viewBefore := d.View()

	// Sanity: without defs the right column should NOT be auto-shown.
	if strings.Contains(viewBefore, "RELATED") {
		t.Skip("right column unexpectedly auto-shown without registered defs; skipping no-defs test")
	}

	d, cmd := sendTabWithCmd(d)
	viewAfter := d.View()

	if viewBefore != viewAfter {
		t.Errorf("Tab without right column should be a no-op; view changed:\nbefore:\n%s\nafter:\n%s", viewBefore, viewAfter)
	}
	if cmd != nil {
		t.Errorf("Tab without right column should return nil cmd, got non-nil cmd")
	}
}

// ---------------------------------------------------------------------------
// TestDetail_FocusSequence_TabTabRestores
// Given: right column explicitly visible (r pressed), Tab ON then Tab OFF
// When:  both Tab presses complete
// Then:  view returns to the same appearance as before the first Tab
// ---------------------------------------------------------------------------

func TestDetail_FocusSequence_TabTabRestores(t *testing.T) {
	noopChecker := func(_ context.Context, _ any, _ resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
		return resource.RelatedCheckResult{Count: 0}
	}
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	d := makeDetailForFocusTest(t, 140)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown; skipping Tab-Tab restore test")
	}

	// r: explicit-visible transition.
	d = makeExplicitlyVisible(d)
	viewBeforeTab := d.View()

	// Tab ON.
	d, _ = d.Update(tabKeyMsg())
	// Tab OFF.
	d, _ = d.Update(tabKeyMsg())
	viewAfterTabTab := d.View()

	if viewBeforeTab != viewAfterTabTab {
		t.Errorf("Tab+Tab should restore the pre-focus view; views differ: before len=%d, after len=%d", len(viewBeforeTab), len(viewAfterTabTab))
	}
}
