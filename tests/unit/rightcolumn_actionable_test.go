package unit_test

// rightcolumn_actionable_test.go — regression tests for isActionableRow
// approximate-zero handling (fixed in feat: related-panel checker completion (019)).
//
// Background:
//   rightcolumn.go:319-322 treats approximate==true as actionable:
//     if row.approximate { return true }
//   This is the correct fix for the "dead-end UI" where "(0+)" rows were
//   visible but non-navigable.  These tests are regression guards to ensure
//   this invariant is never accidentally reverted.
//
// Test strategy:
//   isActionableRow is unexported and lives in internal/tui/views.  Tests in
//   tests/unit/ cannot call it directly.  We exercise it indirectly through the
//   exported DetailModel interface.
//
//   The key probe: get right-column focus (via loading state → Tab → inject result),
//   then press Enter.  Enter on the right column emits RelatedNavigateMsg iff
//   isActionableRow returns true for the selected row.
//
//   Separately: we probe the "l" key (ScrollRight) which uses HasActionableRows()
//   to decide whether to focus the right column.  This lets us test
//   HasActionableRows indirectly: inject result FIRST, then try "l" — if the
//   right column accepts focus, HasActionableRows()==true.
//
// Design spec: docs/design/related-resources.md

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// helpers local to this file
// ---------------------------------------------------------------------------

const approxTestWidth = 140

// buildApproxDetail creates a DetailModel with a single RelatedDef "tg"
// ("Target Groups") registered for resource type "approx-test-ec2".
// At width=140 the right column is auto-shown with the row in loading state.
// The caller must defer the returned cleanup func.
func buildApproxDetail(t *testing.T) (views.DetailModel, func()) {
	t.Helper()
	resource.RegisterRelated("approx-test-ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	cleanup := func() { resource.UnregisterRelated("approx-test-ec2") }

	res := resource.Resource{
		ID:   "i-approxtest001",
		Name: "approx-test-instance",
		Fields: map[string]string{
			"instance_id": "i-approxtest001",
			"state":       "running",
		},
	}
	k := keys.Default()
	d := views.NewDetail(res, "approx-test-ec2", nil, k)
	d.SetSize(approxTestWidth, 30)
	return d, cleanup
}

// injectApproxResult injects a RelatedCheckResultMsg for targetType "tg" with
// the given count, approximate flag, fetchFilter, and error.
func injectApproxResult(
	d views.DetailModel,
	count int,
	approximate bool,
	fetchFilter map[string]string,
	err error,
) views.DetailModel {
	msg := messages.RelatedCheckResultMsg{
		ResourceType: "approx-test-ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       count,
			Approximate: approximate,
			FetchFilter: fetchFilter,
			Err:         err,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// focusRightColWhileLoading tabs to the right column BEFORE injecting any
// result.  At build time the row is in loading state, so HasActionableRows()==true
// and Tab transfers focus.  Returns the focused model.
func focusRightColWhileLoading(t *testing.T, d views.DetailModel) views.DetailModel {
	t.Helper()
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not visible — cannot test focus behavior")
	}
	updated, _ := d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	return updated
}

// pressEnterCmd sends Enter to the model and executes the returned cmd.
// Returns the emitted tea.Msg (nil if cmd is nil or returns nil).
func pressEnterCmd(d views.DetailModel) tea.Msg {
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		return nil
	}
	return cmd()
}

// isApproxNavMsg reports whether a tea.Msg is a RelatedNavigateMsg.
// Named to avoid clash with any existing isNavMsg in other test files.
func isApproxNavMsg(msg tea.Msg) bool {
	if msg == nil {
		return false
	}
	_, ok := msg.(messages.RelatedNavigateMsg)
	return ok
}

// pressScrollRightDetail sends the "l" key to the detail model.
// Returns the updated model and whether the view changed (proxy for focus transferred).
func pressScrollRightDetail(d views.DetailModel) (views.DetailModel, bool) {
	viewBefore := stripAnsi(d.View())
	updated, _ := d.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	viewAfter := stripAnsi(updated.View())
	return updated, viewBefore != viewAfter
}

// ---------------------------------------------------------------------------
// Core Enter-on-focused-row tests
// ---------------------------------------------------------------------------
//
// Strategy for all cases in the spec table:
//   Step 1: build fresh detail (row in loading state)
//   Step 2: Tab to focus right column (loading row is always focusable)
//   Step 3: inject result with target count/approximate/fetchFilter/err
//   Step 4: press Enter → check RelatedNavigateMsg
//
// This isolates isActionableRow for the post-injection state: Enter always
// calls isActionableRow(*row) at the moment it is pressed, so the test reflects
// the actual gating logic regardless of whether focus was acquired via loading.

// TestIsActionableRow_ApproxZero_NoFilter — count=0, approximate=true, no fetchFilter
// Expected: actionable (approximate flag makes row navigable regardless of count)
func TestIsActionableRow_ApproxZero_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	// Get focus while row is loading (always succeeds).
	d = focusRightColWhileLoading(t, d)

	// Inject the approximate-zero result.
	d = injectApproxResult(d, 0, true, nil, nil)

	// Enter must produce RelatedNavigateMsg — approximate rows are always navigable.
	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("Enter on approximate-zero row (count=0, approximate=true, no fetchFilter) must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_ApproxZero_WithFilter — count=0, approximate=true, fetchFilter={"x":"y"}
// Expected: actionable (approximate flag + fetchFilter both make the row navigable)
func TestIsActionableRow_ApproxZero_WithFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, 0, true, map[string]string{"x": "y"}, nil)

	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("Enter on approximate-zero row (count=0, approximate=true, fetchFilter set) must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_DefiniteZero_NoFilter — count=0, approximate=false, no fetchFilter
// Expected: NOT actionable (existing behavior, must keep passing)
func TestIsActionableRow_DefiniteZero_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, 0, false, nil, nil)

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: definite-zero row (count=0, approximate=false) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_CountMinusOne_NoFilter — count=-1, approximate=false, no fetchFilter
// Expected: NOT actionable (existing behavior)
func TestIsActionableRow_CountMinusOne_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, -1, false, nil, nil)

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: count=-1 row without fetchFilter must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_CountMinusOne_WithFilter — count=-1, approximate=false, fetchFilter={"x":"y"}
// Expected: actionable (existing behavior — must keep passing)
func TestIsActionableRow_CountMinusOne_WithFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, -1, false, map[string]string{"x": "y"}, nil)

	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: Enter on count=-1 row with fetchFilter must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_PositiveCount_NoFilter — count=5, approximate=false, no fetchFilter
// Expected: actionable (existing behavior — must keep passing)
func TestIsActionableRow_PositiveCount_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	// Inject count=5 with ResourceIDs (required when Count>0).
	injectMsg := messages.RelatedCheckResultMsg{
		ResourceType: "approx-test-ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       5,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3", "tg-4", "tg-5"},
		},
	}
	d, _ = d.Update(injectMsg)

	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: Enter on count=5 row must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_ApproxN_NoFilter — count=5, approximate=true, no fetchFilter
// Expected: actionable (count>0 path already returns true; kept as regression pin)
func TestIsActionableRow_ApproxN_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	injectMsg := messages.RelatedCheckResultMsg{
		ResourceType: "approx-test-ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       5,
			Approximate: true,
			ResourceIDs: []string{"tg-1", "tg-2", "tg-3", "tg-4", "tg-5"},
		},
	}
	d, _ = d.Update(injectMsg)

	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: Enter on count=5, approximate=true row must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_Loading_Blocks — row stays in loading state
// Expected: Enter does NOT produce RelatedNavigateMsg (loading blocks navigation)
func TestIsActionableRow_Loading_Blocks(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	// Focus while loading (Tab succeeds because loading rows are focusable).
	d = focusRightColWhileLoading(t, d)

	// Do NOT inject any result — row stays loading.
	// isActionableRow: loading==true → return false.
	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("loading row must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_Error_Blocks — count=0, approximate=true, err="boom"
// Expected: NOT actionable when err != nil
func TestIsActionableRow_Error_Blocks(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, 0, true, nil, errors.New("boom"))

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("error row must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg (err blocks actionability)")
	}
}

// ---------------------------------------------------------------------------
// HasActionableRows probe: does "l" key focus the right column after injection?
// ---------------------------------------------------------------------------
//
// The "l" (ScrollRight) key uses HasActionableRows() to decide whether to focus.
// Probing "l" AFTER injecting a result (from an unfocused state) directly tests
// whether HasActionableRows() considers the injected row actionable.

// TestIsActionableRow_HasActionableRows_ApproxZero_EnablesFocus
// Expected: after injecting approximate-zero, "l" must transfer focus
// (HasActionableRows returns true for approximate rows)
func TestIsActionableRow_HasActionableRows_ApproxZero_EnablesFocus(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not visible — cannot test l-key focus behavior")
	}

	// Inject approximate-zero BEFORE any focus attempt.
	d = injectApproxResult(d, 0, true, nil, nil)

	// "l" focuses right column only when HasActionableRows()==true.
	d, focused := pressScrollRightDetail(d)
	if !focused {
		t.Errorf("'l' key must transfer focus to right column when approximate-zero row is present (HasActionableRows must return true); view unchanged after l press")
		return
	}

	// With focus transferred, Enter must also produce RelatedNavigateMsg.
	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("Enter on focused approximate-zero row must produce RelatedNavigateMsg; got %T", msg)
	}
}

// TestIsActionableRow_HasActionableRows_DefiniteZero_BlocksFocus
// Expected: after injecting definite-zero, "l" must NOT transfer focus (existing behavior)
func TestIsActionableRow_HasActionableRows_DefiniteZero_BlocksFocus(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not visible — cannot test l-key focus behavior")
	}

	d = injectApproxResult(d, 0, false, nil, nil)

	_, focused := pressScrollRightDetail(d)
	if focused {
		t.Errorf("REGRESSION: 'l' key must NOT transfer focus when all rows are definite-zero; view changed (implies HasActionableRows==true, which is wrong)")
	}
}

// ---------------------------------------------------------------------------
// Render-level smoke tests
// ---------------------------------------------------------------------------

// TestIsActionableRow_ApproxZero_ViewShape verifies "(0+)" suffix rendering.
//   - Part 1: approximate-zero row must show "(0+)" (PASSES NOW).
//   - Part 2: after focus via loading state, "(0+)" must still appear in focused view.
func TestIsActionableRow_ApproxZero_ViewShape(t *testing.T) {
	ensureNoColor(t)

	// --- Part 1: (0+) present in unfocused view ---
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = injectApproxResult(d, 0, true, nil, nil)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(0+)") {
		t.Errorf("approximate-zero row must render as 'Target Groups (0+)' in View(); got:\n%s", plain)
	}
	if !strings.Contains(plain, "Target Groups") {
		t.Errorf("approximate-zero row display name 'Target Groups' missing from View(); got:\n%s", plain)
	}

	// --- Part 2: (0+) present after focus transition via loading state ---
	d2, cleanup2 := buildApproxDetail(t)
	defer cleanup2()

	d2 = focusRightColWhileLoading(t, d2)
	d2 = injectApproxResult(d2, 0, true, nil, nil)

	plain2 := stripAnsi(d2.View())
	if !strings.Contains(plain2, "(0+)") {
		t.Errorf("approximate-zero row must still show '(0+)' when right column is focused; got:\n%s", plain2)
	}
}

// TestIsActionableRow_DefiniteZero_ViewShape_NoPlusSign verifies that a
// definite-zero (approximate=false) row renders as "(0)", not "(0+)".
// Guards against rendering regression where approximate flag is ignored.
func TestIsActionableRow_DefiniteZero_ViewShape_NoPlusSign(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = injectApproxResult(d, 0, false, nil, nil)
	plain := stripAnsi(d.View())

	if strings.Contains(plain, "(0+)") {
		t.Errorf("definite-zero row must render as '(0)', not '(0+)'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("definite-zero row must render as 'Target Groups (0)'; got:\n%s", plain)
	}
}
