// app_related_cursor_skip_test.go — pins the user-visible defect that the
// detail view's related-panel cursor lands on dimmed (non-actionable) rows,
// while the main menu's cursor skips over dimmed (confirmed-empty) entries.
// The two surfaces must behave identically.
//
// Menu-skip semantics (read from internal/app/menu.go:113-143,
// menuSkipUnavailable, and internal/app/actions_nav.go's callers):
//   - ActionMoveDown/Up move ms.Cursor by ±1, then menuSkipUnavailable scans
//     forward in `direction` for the first entry where the item is NOT
//     confirmed-empty (known && count==0 && !truncated is the skip
//     predicate; "not dimmed" is !known || count>0 || truncated).
//   - If the scan runs off the end without finding a landable entry, it
//     reverses and scans backward from (start-direction) toward and past the
//     original position.
//   - If nothing in the visible list is landable, ms.Cursor is left wherever
//     the caller set it before the skip call (stays put).
//   - ActionMoveTop sets Cursor=0 then skips forward (+1): lands on the FIRST
//     non-dim entry. ActionMoveBottom sets Cursor=len-1 then skips backward
//     (-1): lands on the LAST non-dim entry.
//   - ActionSelectIndex (click path) does NOT call menuSkipUnavailable — it
//     clamps the cursor directly to the clicked index with no skip; landing
//     on a dimmed entry is allowed, but subsequent handleActionSelect
//     (shared Enter/click-select path) blocks navigation because
//     ms.Availability[key] is known, count==0, and not truncated.
//
// Dim predicate for the related panel (internal/app/detail_cursor.go:217,
// isActionableDetailRow) delegates to the single shared predicate
// resource.IsRelatedActionable(state, count, approximate) — the SAME
// predicate buildDetailRelatedBlocks uses to set RelatedBlock.Actionable
// (internal/app/detail_body.go:435). A row is dimmed (non-actionable) when:
// State is RelatedLoading or RelatedError, or State is RelatedResolved with
// Count==0. It is actionable when State is RelatedDeferred or RelatedUnknown,
// or State is RelatedResolved with Count>0 (or Approximate with Count>0).
//
// detail_cursor.go's ActionMoveUp/Down/Top/Bottom for the related-focused
// branch (RelatedFocus==true) currently have NO skip logic at all — they
// simply clamp RelatedCursor to [0, relatedCount-1]. This file's tests are
// RED at HEAD for that reason: they assert menu-parity skip behavior that
// does not yet exist in the related-panel cursor path.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newRelatedSkipController builds a Controller with a ScreenDetail on the
// stack, related rows injected via the public ApplyDetailRelated seam, and
// RelatedFocus turned on so ActionMoveUp/Down/Top/Bottom drive RelatedCursor
// (mirrors newDetailController in detail_render_parity_test.go, but adds the
// InitDetailRelatedRows + ToggleFocus steps this suite needs).
func newRelatedSkipController(t *testing.T, rows []app.DetailRelatedRow) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(resource.Resource{ID: "i-0abc123def456789a", Name: "prod-backend-01"}, "ec2")
	// Seed RelatedVisible via a synthetic def-backed init, then overwrite with
	// the exact rows this test wants (ApplyDetailRelated alone does not set
	// RelatedVisible, and ActionToggleFocus requires it to be true).
	c.InitDetailRelatedRows("ec2")
	c.ApplyDetailRelated(rows)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	return c
}

// relatedRow builds a DetailRelatedRow. actionable controls whether the row
// is dimmed: pass count=0 for a dimmed (confirmed-empty) row, count>0 for a
// non-dimmed row — matching resource.IsRelatedActionable's count>0 branch.
func relatedRow(targetType string, count int) app.DetailRelatedRow {
	return app.DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: targetType,
		Count:       count,
	}
}

// relatedCursorAndActionable reads back RelatedCursor and the Actionable flag
// of the row currently under the cursor from the controller's own snapshot —
// the same DetailBody.Related[].Actionable the renderer would dim.
func relatedCursorAndActionable(t *testing.T, vs app.ViewState) (cursor int, actionable bool) {
	t.Helper()
	if vs.Body.Detail == nil {
		t.Fatal("ViewState.Body.Detail is nil")
	}
	rc := vs.Body.Detail.RelatedCursor
	if rc < 0 || rc >= len(vs.Body.Detail.Related) {
		t.Fatalf("RelatedCursor=%d out of range for %d related rows", rc, len(vs.Body.Detail.Related))
	}
	return rc, vs.Body.Detail.Related[rc].Actionable
}

// =============================================================================
// 1. Down-move skips over a dimmed row.
// =============================================================================

// TestRelatedCursor_MoveDown_SkipsDimmedRow verifies that moving down from a
// non-dim row past a dimmed row lands on the next non-dim row, not on the
// dimmed one — mirroring menuSkipUnavailable's forward-scan behavior.
func TestRelatedCursor_MoveDown_SkipsDimmedRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),  // index 0: non-dim, cursor starts here
		relatedRow("vpc", 0), // index 1: dimmed (confirmed-empty)
		relatedRow("eni", 2), // index 2: non-dim
	}
	c := newRelatedSkipController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveDown})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 2 {
		t.Errorf("RelatedCursor after MoveDown = %d, want 2 (should skip dimmed index 1)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want a non-dim landing row", cursor)
	}
}

// =============================================================================
// 2. Up-move skips over a dimmed row (symmetric).
// =============================================================================

// TestRelatedCursor_MoveUp_SkipsDimmedRow verifies the symmetric up-move case:
// starting at the last non-dim row and moving up past a dimmed row lands on
// the next non-dim row above it.
func TestRelatedCursor_MoveUp_SkipsDimmedRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),  // index 0: non-dim
		relatedRow("vpc", 0), // index 1: dimmed
		relatedRow("eni", 2), // index 2: non-dim, cursor starts here
	}
	c := newRelatedSkipController(t, rows)
	// Land the cursor on index 2 first (two down-moves would already invoke
	// skip logic; jump there directly by moving to bottom instead, which this
	// suite tests separately — here we drive down twice from 0 assuming the
	// fixed behavior, but to isolate MoveUp we instead seed via two MoveDowns
	// on a controller with no dimmed rows in between is not available, so we
	// use ActionMoveBottom to land on the last row directly).
	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveUp})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 0 {
		t.Errorf("RelatedCursor after MoveBottom+MoveUp = %d, want 0 (should skip dimmed index 1)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want a non-dim landing row", cursor)
	}
}

// =============================================================================
// 3. Jump-to-top / jump-to-bottom land on first/last non-dim row.
// =============================================================================

// TestRelatedCursor_MoveTop_LandsOnFirstNonDimRow verifies that jump-to-top
// skips a leading dimmed row and lands on the first non-dim row — mirroring
// handleActionMoveTop's Cursor=0 + menuSkipUnavailable(+1) sequence.
func TestRelatedCursor_MoveTop_LandsOnFirstNonDimRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("vpc", 0), // index 0: dimmed
		relatedRow("sg", 4),  // index 1: non-dim — expected landing
		relatedRow("eni", 2), // index 2: non-dim
	}
	c := newRelatedSkipController(t, rows)
	c.Apply(app.Action{Kind: app.ActionMoveBottom})

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveTop})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 1 {
		t.Errorf("RelatedCursor after MoveTop = %d, want 1 (first non-dim row, skipping dimmed index 0)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want the first non-dim row", cursor)
	}
}

// TestRelatedCursor_MoveBottom_LandsOnLastNonDimRow verifies that
// jump-to-bottom skips a trailing dimmed row and lands on the last non-dim
// row — mirroring handleActionMoveBottom's Cursor=len-1 +
// menuSkipUnavailable(-1) sequence.
func TestRelatedCursor_MoveBottom_LandsOnLastNonDimRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 4),  // index 0: non-dim
		relatedRow("eni", 2), // index 1: non-dim — expected landing
		relatedRow("vpc", 0), // index 2: dimmed
	}
	c := newRelatedSkipController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveBottom})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 1 {
		t.Errorf("RelatedCursor after MoveBottom = %d, want 1 (last non-dim row, skipping dimmed index 2)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want the last non-dim row", cursor)
	}
}

// =============================================================================
// 4. Every row below is dimmed: cursor stays put.
// =============================================================================

// TestRelatedCursor_MoveDown_AllRemainingDimmed_CursorStaysPut verifies that
// when every row in the scan direction is dimmed, the cursor does not move
// past the last non-dim row it started from — mirroring menuSkipUnavailable's
// documented "stays put" fallback when no landable entry exists in either
// direction from the current position onward.
func TestRelatedCursor_MoveDown_AllRemainingDimmed_CursorStaysPut(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),  // index 0: non-dim, cursor starts here
		relatedRow("vpc", 0), // index 1: dimmed
		relatedRow("eni", 0), // index 2: dimmed
	}
	c := newRelatedSkipController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionMoveDown})

	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 0 {
		t.Errorf("RelatedCursor after MoveDown with all-dimmed remainder = %d, want 0 (stay put — no landable row below)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want the cursor to have stayed on the original non-dim row", cursor)
	}
}

// =============================================================================
// 5. Click/index-select on a dimmed row mirrors menu parity.
// =============================================================================

// TestRelatedCursor_ClickOnDimmedRow_NoNavigation verifies the click-path
// parity claimed by actions_nav.go:290-293's comment: like the main menu's
// ActionSelectIndex (which clamps directly with no skip, then relies on
// handleActionSelect's Availability guard to block navigation on a
// confirmed-empty entry), clicking a dimmed related row via
// ActionRelatedSelect must not navigate. isActionableDetailRow gates
// navigation in handleActionRelatedSelect (targetRow == nil ||
// !isActionableDetailRow(*targetRow) => no-op), so this assertion targets the
// TaskRequest side effect (must be empty) as the parity signal — this part of
// the contract already exists in production code independent of the
// skip-logic defect, so it is expected to be GREEN at HEAD (noted below).
func TestRelatedCursor_ClickOnDimmedRow_NoNavigation(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),  // index 0: non-dim
		relatedRow("vpc", 0), // index 1: dimmed — clicked
	}
	c := newRelatedSkipController(t, rows)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "1"})

	if len(tasks) != 0 {
		t.Errorf("ActionRelatedSelect on dimmed row produced %d tasks, want 0 (no navigation on a dimmed/non-actionable row)", len(tasks))
	}
}

// =============================================================================
// 6. Menu-parity: both surfaces use the same skip semantics.
// =============================================================================

// TestRelatedCursor_MenuParity_SameDimNonDimPatternSameLandingSequence drives
// the main menu and the related panel with the SAME dim/non-dim pattern
// (non-dim, dimmed, non-dim, dimmed, non-dim) through the same sequence of
// moves (MoveDown, MoveDown, MoveTop, MoveBottom) and asserts both surfaces
// land on the same relative index sequence — pinning that the related panel
// must adopt the identical skip algorithm the menu already has.
func TestRelatedCursor_MenuParity_SameDimNonDimPatternSameLandingSequence(t *testing.T) {
	// --- Menu side ---
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	menuCtrl := app.New(core)

	all := resource.AllResourceTypes()
	if len(all) < 5 {
		t.Fatalf("resource.AllResourceTypes() has %d entries, need at least 5 for this pattern", len(all))
	}
	// Pattern: index 0 non-dim, 1 dimmed, 2 non-dim, 3 dimmed, 4 non-dim —
	// same shape as the related-panel rows fixture below.
	// Every catalog entry from index 5 onward is ALSO marked dimmed (confirmed
	// empty) so MoveBottom's backward scan from the true last catalog index
	// skips the whole tail and lands on index 4 — bounding the comparison
	// window to exactly the 5 entries under test, matching the related
	// panel's 5-row fixture below index-for-index.
	dimPattern := []int{3, 0, 2, 0, 1} // per-index Count: >0 = non-dim, 0 = dimmed
	for i, count := range dimPattern {
		menuCtrl.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{
				ResourceType: all[i].ShortName,
				Count:        count,
				Truncated:    false,
			},
		})
	}
	for i := 5; i < len(all); i++ {
		menuCtrl.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{
				ResourceType: all[i].ShortName,
				Count:        0,
				Truncated:    false,
			},
		})
	}

	menuLanding := func(vs app.ViewState) int { return vs.Body.Menu.Selected }
	menuActionable := func(vs app.ViewState, idx int) bool {
		e := vs.Body.Menu.Entries[idx]
		return !(e.AvailKnown && e.Availability == 0 && !e.AvailTruncated)
	}

	vsMenu, _ := menuCtrl.Apply(app.Action{Kind: app.ActionMoveDown})
	menuSeq := []int{menuLanding(vsMenu)}
	vsMenu, _ = menuCtrl.Apply(app.Action{Kind: app.ActionMoveDown})
	menuSeq = append(menuSeq, menuLanding(vsMenu))
	vsMenu, _ = menuCtrl.Apply(app.Action{Kind: app.ActionMoveTop})
	menuSeq = append(menuSeq, menuLanding(vsMenu))
	vsMenu, _ = menuCtrl.Apply(app.Action{Kind: app.ActionMoveBottom})
	menuSeq = append(menuSeq, menuLanding(vsMenu))

	if !menuActionable(vsMenu, menuSeq[len(menuSeq)-1]) {
		t.Fatalf("test setup: final menu landing index %d is dimmed, want non-dim", menuSeq[len(menuSeq)-1])
	}

	// --- Related-panel side: same 5-row dim/non-dim pattern. ---
	rows := []app.DetailRelatedRow{
		relatedRow("t0", 3), // 0: non-dim
		relatedRow("t1", 0), // 1: dimmed
		relatedRow("t2", 2), // 2: non-dim
		relatedRow("t3", 0), // 3: dimmed
		relatedRow("t4", 1), // 4: non-dim
	}
	relCtrl := newRelatedSkipController(t, rows)

	vsRel, _ := relCtrl.Apply(app.Action{Kind: app.ActionMoveDown})
	relSeq := []int{vsRel.Body.Detail.RelatedCursor}
	vsRel, _ = relCtrl.Apply(app.Action{Kind: app.ActionMoveDown})
	relSeq = append(relSeq, vsRel.Body.Detail.RelatedCursor)
	vsRel, _ = relCtrl.Apply(app.Action{Kind: app.ActionMoveTop})
	relSeq = append(relSeq, vsRel.Body.Detail.RelatedCursor)
	vsRel, _ = relCtrl.Apply(app.Action{Kind: app.ActionMoveBottom})
	relSeq = append(relSeq, vsRel.Body.Detail.RelatedCursor)

	// Both sequences start at index 0 (non-dim), and the pattern is IDENTICAL
	// (non-dim, dimmed, non-dim, dimmed, non-dim) between the menu's first 5
	// catalog entries (after PatchMenuAvailability) and the related rows, so
	// the landing-index sequences must match exactly if both surfaces share
	// the same skip algorithm.
	if len(menuSeq) != len(relSeq) {
		t.Fatalf("sequence length mismatch: menu=%v related=%v", menuSeq, relSeq)
	}
	for i := range menuSeq {
		if menuSeq[i] != relSeq[i] {
			t.Errorf("landing-sequence step %d: menu landed on index %d, related-panel landed on index %d — the two surfaces must use identical skip semantics for the same dim/non-dim pattern (moves: MoveDown, MoveDown, MoveTop, MoveBottom)", i, menuSeq[i], relSeq[i])
		}
	}
}
