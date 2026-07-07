package unit_test

// rightcolumn_actionable_test.go — regression tests for isActionableRow
// approximate-zero handling.
//
// Background (contract updated — resolved count==0 is NEVER actionable):
//   resource.IsRelatedActionable now treats a RESOLVED count==0 as never
//   actionable, even when approximate==true. ApproximateZero() (related.go:219)
//   sets Count:0, so "(0)" rows must not be drillable into an empty view.
//   RelatedDeferred pivots (server-side FetchFilter navigation) remain
//   actionable regardless of Count/the approximate flag.  These tests are
//   regression guards to ensure this invariant is never accidentally
//   reverted back to "approximate implies actionable".
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

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Direct resource.IsRelatedActionable table test — the single source of truth
// consumed by isActionableRow. Covers the full contract from related.go:290.
//
// Each case name preserves its pre-task-#58 identity (count/hasFetchFilter/
// loading/hasErr framing) for traceability; the state column is the mapping
// migration rule 2/3 assigns: loading->RelatedLoading, hasErr->RelatedError,
// hasFetchFilter(count<0)->RelatedDeferred, bare count<0->RelatedUnknown,
// else->RelatedResolved (zero value). hasFetchFilter is no longer a function
// parameter, so the two "*_WithFetchFilter_NEW_NotActionable" resolved-zero
// cases now share identical (state,count,approximate) inputs with their
// no-filter siblings — kept as separate cases rather than deleted, since both
// still assert the real "resolved zero is never actionable" invariant.
// ---------------------------------------------------------------------------

func TestIsRelatedActionable_Table(t *testing.T) {
	cases := []struct {
		name           string
		state          domain.RelatedRowState
		count          int
		approximate    bool
		wantActionable bool
	}{
		{"DefiniteZero_NoFilter", domain.RelatedResolved, 0, false, false},
		{"ApproxZero_NoFilter_NEW_NotActionable", domain.RelatedResolved, 0, true, false},
		{"DefiniteZero_WithFetchFilter_NEW_NotActionable", domain.RelatedResolved, 0, false, false},
		{"UnknownCount_WithFetchFilter_Actionable", domain.RelatedDeferred, 0, false, true},
		{"UnknownCount_NoFilter_Actionable", domain.RelatedUnknown, 0, false, true},
		{"PositiveCount_NoFilter_Actionable", domain.RelatedResolved, 3, false, true},
		{"PositiveCount_Approximate_Actionable", domain.RelatedResolved, 3, true, true},
		{"Loading_BlocksRegardlessOfCount", domain.RelatedLoading, 5, false, false},
		{"Error_BlocksRegardlessOfCount", domain.RelatedError, 5, false, false},
		{"ApproxZero_WithFetchFilter_NEW_NotActionable", domain.RelatedResolved, 0, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resource.IsRelatedActionable(tc.state, tc.count, tc.approximate)
			if got != tc.wantActionable {
				t.Errorf("IsRelatedActionable(state=%v, count=%d, approximate=%v) = %v, want %v",
					tc.state, tc.count, tc.approximate, got, tc.wantActionable)
			}
		})
	}
}

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
	resource.SetRelatedForTest("approx-test-ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})
	cleanup := func() { resource.CleanupRelatedForTest("approx-test-ec2") }

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
// the given state, count, approximate flag, fetchFilter, and error.
func injectApproxResult(
	d views.DetailModel,
	state domain.RelatedRowState,
	count int,
	approximate bool,
	fetchFilter map[string]string,
	err error,
) views.DetailModel {
	msg := messages.RelatedCheckResult{
		ResourceType: "approx-test-ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			State:       state,
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
	_, ok := msg.(messages.RelatedNavigate)
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
// Expected: NOT actionable. NEW contract: a resolved count==0 is never
// actionable even when approximate==true — ApproximateZero() rows must not
// let the user drill into an empty view.
func TestIsActionableRow_ApproxZero_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	// Get focus while row is loading (always succeeds).
	d = focusRightColWhileLoading(t, d)

	// Inject the approximate-zero result.
	d = injectApproxResult(d, domain.RelatedResolved, 0, true, nil, nil)

	// Enter must NOT produce RelatedNavigateMsg — resolved zero is a dead end
	// regardless of the approximate flag.
	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("Enter on approximate-zero row (count=0, approximate=true, no fetchFilter) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_ApproxZero_WithFilter — count=0, approximate=true, fetchFilter={"x":"y"}
// Expected: NOT actionable. NEW contract: count==0 is never actionable even
// with a fetchFilter set, because FetchFilter pivots always carry Count:-1
// (never 0) in production — a resolved Count:0 with a fetchFilter is still a
// definite empty result and must not be navigable.
func TestIsActionableRow_ApproxZero_WithFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, domain.RelatedResolved, 0, true, map[string]string{"x": "y"}, nil)

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("Enter on approximate-zero row (count=0, approximate=true, fetchFilter set) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_DefiniteZero_WithFilter — count=0, approximate=false, fetchFilter={"x":"y"}
// Expected: NOT actionable. A resolved zero count must block navigation even
// when a fetchFilter is present — only count==-1 (unknown) is rescued by
// hasFetchFilter.
func TestIsActionableRow_DefiniteZero_WithFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, domain.RelatedResolved, 0, false, map[string]string{"x": "y"}, nil)

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("Enter on definite-zero row (count=0, approximate=false, fetchFilter set) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_DefiniteZero_NoFilter — count=0, approximate=false, no fetchFilter
// Expected: NOT actionable (existing behavior, must keep passing)
func TestIsActionableRow_DefiniteZero_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, domain.RelatedResolved, 0, false, nil, nil)

	msg := pressEnterCmd(d)
	if isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: definite-zero row (count=0, approximate=false) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg")
	}
}

// TestIsActionableRow_CountMinusOne_NoFilter — count=-1, approximate=false, no fetchFilter
// Expected: actionable (owner decision #38, 2026-07-06: a transient "(?)" row
// — resolved-unknown, cold-cache count==-1 with no FetchFilter — is now a
// drillable pivot into the target type's plain top-level list, not a
// dead end. See qa_related_transient_unknown_drill_test.go for the
// app/controller-level end-to-end pin of this contract.)
func TestIsActionableRow_CountMinusOne_NoFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, domain.RelatedUnknown, 0, false, nil, nil)

	msg := pressEnterCmd(d)
	if !isApproxNavMsg(msg) {
		t.Errorf("REGRESSION: count=-1 row without fetchFilter must produce RelatedNavigateMsg (owner decision #38: transient unknown rows are now actionable); got %T", msg)
	}
}

// TestIsActionableRow_CountMinusOne_WithFilter — count=-1, approximate=false, fetchFilter={"x":"y"}
// Expected: actionable (existing behavior — must keep passing)
func TestIsActionableRow_CountMinusOne_WithFilter(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = focusRightColWhileLoading(t, d)
	d = injectApproxResult(d, domain.RelatedDeferred, 0, false, map[string]string{"x": "y"}, nil)

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
	injectMsg := messages.RelatedCheckResult{
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
	injectMsg := messages.RelatedCheckResult{
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
	d = injectApproxResult(d, domain.RelatedError, 0, true, nil, errors.New("boom"))

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

// TestIsActionableRow_HasActionableRows_ApproxZero_BlocksFocus
// NEW contract: a resolved approximate-zero row is never actionable, so with
// only that single row registered, "l" must NOT transfer focus (mirrors the
// existing definite-zero behavior below).
func TestIsActionableRow_HasActionableRows_ApproxZero_BlocksFocus(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not visible — cannot test l-key focus behavior")
	}

	// Inject approximate-zero BEFORE any focus attempt.
	d = injectApproxResult(d, domain.RelatedResolved, 0, true, nil, nil)

	// "l" focuses right column only when HasActionableRows()==true. Under the
	// new contract, approximate-zero is not actionable, so focus must NOT transfer.
	_, focused := pressScrollRightDetail(d)
	if focused {
		t.Errorf("REGRESSION: 'l' key must NOT transfer focus when the only row is a resolved approximate-zero row (count==0 is never actionable); view changed after l press")
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

	d = injectApproxResult(d, domain.RelatedResolved, 0, false, nil, nil)

	_, focused := pressScrollRightDetail(d)
	if focused {
		t.Errorf("REGRESSION: 'l' key must NOT transfer focus when all rows are definite-zero; view changed (implies HasActionableRows==true, which is wrong)")
	}
}

// ---------------------------------------------------------------------------
// Render-level smoke tests
// ---------------------------------------------------------------------------

// TestIsActionableRow_ApproxZero_ViewShape verifies the "(0)" suffix
// rendering for approximate-zero rows. Per AS-378, the renderer collapses
// "(0+)" → "(0)" so the integration test (which asserts the literal
// substring `"<Pivot> (<N>)"` for every count >= 0) is satisfied and the
// design-spec table (`docs/design/related-resources.md §5.3`) stays the
// SSOT. Approximate-ness is signaled via the RowNormal style (vs DimText
// for confirmed-zero), and navigability is preserved by isActionableRow,
// not by the text suffix.
//   - Part 1: approximate-zero row renders as "Target Groups (0)" without
//     the "+" marker.
//   - Part 2: after focus transition via loading state, the same "(0)"
//     suffix is present.
func TestIsActionableRow_ApproxZero_ViewShape(t *testing.T) {
	ensureNoColor(t)

	// --- Part 1: (0) present in unfocused view, "(0+)" must be absent ---
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = injectApproxResult(d, domain.RelatedResolved, 0, true, nil, nil)
	plain := stripAnsi(d.View())
	if !strings.Contains(plain, "(0)") {
		t.Errorf("approximate-zero row must render as 'Target Groups (0)' in View(); got:\n%s", plain)
	}
	if strings.Contains(plain, "(0+)") {
		t.Errorf("approximate-zero row must NOT render the '+' marker after AS-378; got:\n%s", plain)
	}
	if !strings.Contains(plain, "Target Groups") {
		t.Errorf("approximate-zero row display name 'Target Groups' missing from View(); got:\n%s", plain)
	}

	// --- Part 2: (0) present after focus transition via loading state ---
	d2, cleanup2 := buildApproxDetail(t)
	defer cleanup2()

	d2 = focusRightColWhileLoading(t, d2)
	d2 = injectApproxResult(d2, domain.RelatedResolved, 0, true, nil, nil)

	plain2 := stripAnsi(d2.View())
	if !strings.Contains(plain2, "(0)") {
		t.Errorf("approximate-zero row must still show '(0)' when right column is focused; got:\n%s", plain2)
	}
	if strings.Contains(plain2, "(0+)") {
		t.Errorf("approximate-zero row must NOT render '(0+)' when focused after AS-378; got:\n%s", plain2)
	}
}

// TestIsActionableRow_DefiniteZero_ViewShape_NoPlusSign verifies that a
// definite-zero (approximate=false) row renders as "(0)", not "(0+)".
// Guards against rendering regression where approximate flag is ignored.
func TestIsActionableRow_DefiniteZero_ViewShape_NoPlusSign(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildApproxDetail(t)
	defer cleanup()

	d = injectApproxResult(d, domain.RelatedResolved, 0, false, nil, nil)
	plain := stripAnsi(d.View())

	if strings.Contains(plain, "(0+)") {
		t.Errorf("definite-zero row must render as '(0)', not '(0+)'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("definite-zero row must render as 'Target Groups (0)'; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Cursor-skip tests: mixed rows (some count==0, one count>0) — the cursor
// must never land on a resolved count==0 row, mirroring the root list's
// "never select a non-actionable row" behavior.
// ---------------------------------------------------------------------------
//
// buildMixedRelatedDetail registers three RelatedDefs on "ec2":
//   - "Zero A" (targetType "tg")  — resolved to count=0 (approximate=true, a
//     realistic ApproximateZero() result)
//   - "Zero B" (targetType "vpc") — resolved to count=0 (approximate=false)
//   - "Positive" (targetType "asg") — resolved to count=3 (actionable)
//
// All three results are injected before any cursor movement, then focus is
// acquired via the explicit-visible + Tab sequence used by the sibling
// detail_focus_test.go helpers (focusRightColumn, replaceEC2Related).

// buildMixedRelatedDetail returns a focused DetailModel with the three rows
// described above, and a cleanup func the caller must defer.
func buildMixedRelatedDetail(t *testing.T) (views.DetailModel, func()) {
	t.Helper()
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Zero A", Checker: noopChecker},
		{TargetType: "vpc", DisplayName: "Zero B", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Positive", Checker: noopChecker},
	})
	cleanup := func() { unregisterEC2Related(t) }

	d := makeDetailForFocusTest(t, 140)
	if !strings.Contains(d.View(), "RELATED") {
		cleanup()
		t.Skip("right column not auto-shown at width=140; skipping mixed cursor-skip test")
	}

	d = focusRightColumn(d)

	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType:   "ec2",
		DefDisplayName: "Zero A",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       0,
			Approximate: true,
		},
	})
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType:   "ec2",
		DefDisplayName: "Zero B",
		Result: resource.RelatedCheckResult{
			TargetType:  "vpc",
			Count:       0,
			Approximate: false,
		},
	})
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType:   "ec2",
		DefDisplayName: "Positive",
		Result: resource.RelatedCheckResult{
			TargetType:  "asg",
			Count:       3,
			ResourceIDs: []string{"asg-1", "asg-2", "asg-3"},
		},
	})

	return d, cleanup
}

// TestRightColumn_CursorSkipsZeroRows_DownNavigation verifies that moving the
// cursor down from the first row never lands on either "Zero A" or "Zero B"
// (both resolved to count==0), landing only on "Positive" (count=3).
func TestRightColumn_CursorSkipsZeroRows_DownNavigation(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildMixedRelatedDetail(t)
	defer cleanup()

	// ensureCursorValid (invoked on every RelatedCheckResult) should have
	// already parked the cursor on the sole actionable row ("Positive").
	// Pressing Down repeatedly must never move it onto a zero row: since
	// "Positive" is the only actionable row, Down is a no-op and Enter must
	// always resolve to TargetType "asg".
	for i := 0; i < 4; i++ {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		msg := pressEnterCmd(d)
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			t.Fatalf("iteration %d: Enter did not produce RelatedNavigate; got %T (cursor may have landed on a non-actionable zero row)", i, msg)
		}
		if nav.TargetType != "asg" {
			t.Errorf("iteration %d: cursor landed on TargetType %q; want \"asg\" (the only count>0 row) — a zero row must never be selectable", i, nav.TargetType)
		}
	}
}

// TestRightColumn_CursorSkipsZeroRows_UpNavigation mirrors the Down test using
// the Up key, confirming the skip logic is symmetric.
func TestRightColumn_CursorSkipsZeroRows_UpNavigation(t *testing.T) {
	ensureNoColor(t)
	d, cleanup := buildMixedRelatedDetail(t)
	defer cleanup()

	for i := 0; i < 4; i++ {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "k"})
		msg := pressEnterCmd(d)
		nav, ok := msg.(messages.RelatedNavigate)
		if !ok {
			t.Fatalf("iteration %d: Enter did not produce RelatedNavigate; got %T (cursor may have landed on a non-actionable zero row)", i, msg)
		}
		if nav.TargetType != "asg" {
			t.Errorf("iteration %d: cursor landed on TargetType %q; want \"asg\" (the only count>0 row) — a zero row must never be selectable", i, nav.TargetType)
		}
	}
}

// TestRightColumn_EnterOnAllZeroRows_NoNavigate verifies that when every
// registered row resolves to count==0 (mix of approximate true/false, no
// count>0 row present at all), Enter never emits RelatedNavigate — there is
// no reachable actionable row to land on.
func TestRightColumn_EnterOnAllZeroRows_NoNavigate(t *testing.T) {
	ensureNoColor(t)
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Zero A", Checker: noopChecker},
		{TargetType: "vpc", DisplayName: "Zero B", Checker: noopChecker},
	})
	defer unregisterEC2Related(t)

	d := makeDetailForFocusTest(t, 140)
	if !strings.Contains(d.View(), "RELATED") {
		t.Skip("right column not auto-shown at width=140; skipping all-zero test")
	}
	d = focusRightColumn(d)

	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType:   "ec2",
		DefDisplayName: "Zero A",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       0,
			Approximate: true,
		},
	})
	d, _ = d.Update(messages.RelatedCheckResult{
		ResourceType:   "ec2",
		DefDisplayName: "Zero B",
		Result: resource.RelatedCheckResult{
			TargetType:  "vpc",
			Count:       0,
			Approximate: false,
		},
	})

	// Try navigating around — with no actionable row reachable, Enter must
	// never emit RelatedNavigate regardless of cursor position.
	for i := 0; i < 3; i++ {
		msg := pressEnterCmd(d)
		if isApproxNavMsg(msg) {
			t.Errorf("iteration %d: Enter on all-zero rows (no actionable row present) must NOT produce RelatedNavigateMsg; got RelatedNavigateMsg", i)
		}
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
}
