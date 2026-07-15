// app_related_focus_entry_test.go — pins the user-visible defect
// (screenshot-confirmed, live) that pressing Tab to focus the detail RELATED
// panel lands RelatedCursor on the FIRST row even when that row is a dimmed
// dead-end (e.g. "EBS Volumes (0)" on a KMS key detail). The existing skip
// logic (detailSkipUnselectableRelated, see app_related_cursor_skip_test.go)
// only runs on movement actions (MoveUp/Down/Top/Bottom) — ActionToggleFocus
// (core/app/detail_cursor.go:162-168, the Tab handler) does nothing but
// flip ds.RelatedFocus, so RelatedCursor is left wherever it last was
// (typically 0, its zero value) with no skip applied.
//
// Required semantics (mirrors the main menu): entering focus lands the
// cursor on the FIRST actionable row; if NO row is actionable, the cursor
// stays at 0 (a bare landing on a dim row is then unavoidable, so this file
// pins that fallback deliberately rather than leaving it as an accident).
//
// These tests are RED at the current tree: ActionToggleFocus has no skip
// call, so pin 1 and pin 4 fail (cursor stays at 0 on the initial focus-grant
// instead of skipping to the first actionable row).
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newRelatedFocusEntryController builds a Controller with a ScreenDetail on
// the stack and related rows injected via ApplyDetailRelated, WITHOUT toggling
// focus — the caller drives ActionToggleFocus itself so these tests observe
// the focus-entry transition directly (unlike newRelatedSkipController in
// app_related_cursor_skip_test.go, which already grants focus as setup).
func newRelatedFocusEntryController(t *testing.T, rows []app.DetailRelatedRow) *app.Controller {
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
	c.EnsureDetailState(resource.Resource{ID: "key-0abc123def456789a", Name: "prod-kms-key"}, "kms")
	c.InitDetailRelatedRows("kms")
	c.ApplyDetailRelated(rows)
	return c
}

// =============================================================================
// 1. Tab focus-entry lands on the first ACTIONABLE row, skipping leading dims.
// =============================================================================

// TestRelatedFocusEntry_Tab_SkipsLeadingDimmedRows verifies that pressing Tab
// to focus the related panel (rows = [dim(0), dim(0), actionable(1), ...])
// lands RelatedCursor on index 2, not on the leading dimmed rows at 0/1 — the
// exact live defect (e.g. "EBS Volumes (0)" landing focus on a KMS key
// detail).
func TestRelatedFocusEntry_Tab_SkipsLeadingDimmedRows(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0), // index 0: dimmed dead-end
		relatedRow("ami", 0),        // index 1: dimmed dead-end
		relatedRow("alias", 1),      // index 2: actionable — expected landing
		relatedRow("grant", 3),      // index 3: actionable
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})

	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("ActionToggleFocus did not grant RelatedFocus")
	}
	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 2 {
		t.Errorf("RelatedCursor after Tab (focus entry) = %d, want 2 (first actionable row, skipping dimmed indices 0 and 1)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false), want the landing row to be actionable", cursor)
	}
}

// =============================================================================
// 2. All rows dim: cursor stays at 0, focus is still granted.
// =============================================================================

// TestRelatedFocusEntry_Tab_AllRowsDimmed_CursorStaysZero verifies that when
// every related row is a dead end, Tab still grants RelatedFocus (the user
// can still see/scroll the panel) but the cursor is pinned at 0 — the
// documented, deliberate fallback rather than an accidental landing.
func TestRelatedFocusEntry_Tab_AllRowsDimmed_CursorStaysZero(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0), // index 0: dimmed
		relatedRow("ami", 0),        // index 1: dimmed
		relatedRow("grant", 0),      // index 2: dimmed
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})

	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("ActionToggleFocus did not grant RelatedFocus even though the related panel is visible")
	}
	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 0 {
		t.Errorf("RelatedCursor after Tab with all-dimmed rows = %d, want 0 (deliberate fallback — no actionable row exists)", cursor)
	}
	if actionable {
		t.Errorf("row at RelatedCursor=%d reports Actionable=true, want false (test setup: every row in this fixture is a confirmed-empty dead end)", cursor)
	}
}

// =============================================================================
// 3. Re-entering focus after Tab-away re-applies the rule.
// =============================================================================

// TestRelatedFocusEntry_TabAwayThenBack_ReappliesSkipRule verifies that
// leaving related-focus (second Tab, back to the field column) and then
// re-entering it (third Tab) re-runs the same skip-to-first-actionable rule,
// not just on the very first focus grant. This guards against a fix that only
// patches the "cold start" path and forgets the toggle can flip both ways.
func TestRelatedFocusEntry_TabAwayThenBack_ReappliesSkipRule(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0), // index 0: dimmed
		relatedRow("alias", 2),      // index 1: actionable — expected landing both times
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("first Tab did not grant RelatedFocus")
	}
	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 1 || !actionable {
		t.Fatalf("first Tab: RelatedCursor=%d actionable=%v, want cursor=1 actionable=true", cursor, actionable)
	}

	// Move the cursor away from the actionable landing so the second focus
	// grant cannot pass merely by leaving a stale-but-correct cursor in place.
	vs, _ = c.Apply(app.Action{Kind: app.ActionMoveUp})
	if vs.Body.Detail.RelatedCursor != 1 {
		t.Fatalf("test setup: MoveUp from index 1 with dimmed index 0 should stay at 1 (skip), got %d", vs.Body.Detail.RelatedCursor)
	}

	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus}) // Tab away
	if vs.Body.Detail.RelatedFocused {
		t.Fatal("second Tab did not release RelatedFocus")
	}

	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus}) // Tab back
	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("third Tab did not re-grant RelatedFocus")
	}
	cursor, actionable = relatedCursorAndActionable(t, vs)
	if cursor != 1 {
		t.Errorf("RelatedCursor after Tab-away-then-back = %d, want 1 (skip rule must be re-applied on every focus grant, not just the first)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is dimmed (Actionable=false) on re-entry, want actionable", cursor)
	}
}

// =============================================================================
// 4. Parity: focus-entry landing equals MoveTop's landing (same
//    stepToSelectable semantics, starting from cursor 0).
// =============================================================================

// TestRelatedFocusEntry_Tab_MatchesMoveTopLanding verifies that the landing
// row Tab produces on a cold focus-entry is identical to the landing row
// ActionMoveTop produces from a fresh RelatedCursor=0 — both must resolve via
// the same stepToSelectable(0, visCount, +1, ...) semantics, so a fix that
// hand-rolls a different "find first actionable" loop for focus-entry cannot
// silently diverge from the movement path's algorithm.
func TestRelatedFocusEntry_Tab_MatchesMoveTopLanding(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0), // index 0: dimmed
		relatedRow("ami", 0),        // index 1: dimmed
		relatedRow("alias", 1),      // index 2: actionable
		relatedRow("grant", 0),      // index 3: dimmed
		relatedRow("policy", 4),     // index 4: actionable
	}

	// Tab side: cold focus-entry on a controller that has never had focus.
	tabCtrl := newRelatedFocusEntryController(t, rows)
	vsTab, _ := tabCtrl.Apply(app.Action{Kind: app.ActionToggleFocus})
	tabCursor := vsTab.Body.Detail.RelatedCursor

	// MoveTop side: grant focus via the existing (already-correct) skip
	// harness, move away from the top, then jump back to top explicitly.
	topCtrl := newRelatedSkipController(t, rows)
	topCtrl.Apply(app.Action{Kind: app.ActionMoveBottom})
	vsTop, _ := topCtrl.Apply(app.Action{Kind: app.ActionMoveTop})
	topCursor := vsTop.Body.Detail.RelatedCursor

	if tabCursor != topCursor {
		t.Errorf("Tab focus-entry landed on index %d but ActionMoveTop landed on index %d for the identical row pattern — focus-entry must use the same stepToSelectable semantics as MoveTop", tabCursor, topCursor)
	}
	if tabCursor != 2 {
		t.Errorf("Tab focus-entry landed on index %d, want 2 (first actionable row in the fixture)", tabCursor)
	}
}

// =============================================================================
// 5. Circular-reentry cursor guard: replay leaves some rows bare (never
//    resolved) and others actionable (resolved from cache) — Tab must still
//    land on an actionable row rather than a bare one, and an all-bare panel
//    must not trap focus.
// =============================================================================
//
// This mirrors the live circular-drill defect (detail A -> drill -> detail B
// -> drill back to A) pinned end-to-end in
// tests/unit/related_circular_reentry_test.go: a bare row is a
// DetailRelatedRow at its zero value (Count=0, Loading=false, Err="") —
// exactly what relatedRow(name, 0) already produces, and exactly what
// resource.IsRelatedActionable treats as non-actionable via its count==0
// branch. These tests extend (not duplicate) the existing all-dimmed /
// leading-dimmed pins above by exercising the specific row-shape a partial
// cache replay produces: a mix of never-touched bare rows and
// cache-resolved actionable rows in arbitrary order.

// TestRelatedFocusEntry_Tab_PartialReplayMix_SkipsBareRows verifies that
// when a related panel holds a mix of bare (never-replayed) rows and
// actionable (cache-replayed) rows — the exact shape a circular-reentry
// cache replay leaves when it only partially reseeds the panel — Tab still
// lands on the first actionable row, never on a bare one.
func TestRelatedFocusEntry_Tab_PartialReplayMix_SkipsBareRows(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("target-group", 0),   // index 0: bare (never replayed)
		relatedRow("subnet", 0),         // index 1: bare (never replayed)
		relatedRow("vpc", 1),            // index 2: actionable (replayed from cache)
		relatedRow("security-group", 0), // index 3: bare (never replayed)
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})

	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("ActionToggleFocus did not grant RelatedFocus")
	}
	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 2 {
		t.Errorf("RelatedCursor after Tab on a partial-replay-mix panel = %d, want 2 (the only actionable row, skipping bare indices 0, 1, 3)", cursor)
	}
	if !actionable {
		t.Errorf("row at RelatedCursor=%d is not actionable — Tab must never land on a bare row produced by a partial cache replay", cursor)
	}
}

// TestRelatedFocusEntry_Tab_AllRowsBareFromReplay_DoesNotTrapFocus verifies
// that when EVERY row in the panel is bare (the live defect's worst case: a
// cache-miss circular reentry that reseeds nothing at all), Tab still grants
// RelatedFocus — the user can leave the panel again with a second Tab — and
// does not hang or panic hunting for a nonexistent actionable row.
func TestRelatedFocusEntry_Tab_AllRowsBareFromReplay_DoesNotTrapFocus(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("target-group", 0),
		relatedRow("subnet", 0),
		relatedRow("vpc", 0),
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("ActionToggleFocus did not grant RelatedFocus even though every related row is bare")
	}

	// Focus must still be released by a second Tab — an all-bare panel must
	// not trap the user permanently in related-focus.
	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if vs.Body.Detail.RelatedFocused {
		t.Fatal("second Tab did not release RelatedFocus on an all-bare related panel — focus is trapped")
	}
}

// =============================================================================
// 6. Tab prefers a drillable pivot over a leading deferred "(?)" row.
// =============================================================================
//
// The live ec2 defect: the panel leads with a deferred "(?)" pivot (alarm/
// ebs-snap/backup — RelatedUnknown, count 0, no IDs) that is actionable (Enter
// re-dispatches it in place) but NOT drillable, sitting before the first
// resolved pivot carrying IDs (Elastic IPs). Tab landed on the "(?)" and Enter
// only re-dispatched it → a silent no-op. The landing must skip the deferred
// "(?)" and settle on the first row Enter would actually navigate from.

// TestRelatedFocusEntry_Tab_PrefersDrillableOverDeferredUnknown is RED before
// detailSkipToDrillable: Tab lands on the deferred alarm "(?)" at index 1.
func TestRelatedFocusEntry_Tab_PrefersDrillableOverDeferredUnknown(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("tg", 0), // index 0: resolved (0) dead-end
		{ // index 1: deferred "(?)" — actionable (re-dispatch) but NOT drillable
			TargetType: "alarm", DisplayName: "alarm",
			State: domain.RelatedUnknown, Count: 0,
		},
		{ // index 2: drillable — Enter navigates (carries IDs)
			TargetType: "eip", DisplayName: "eip",
			State: domain.RelatedResolved, Count: 1, ResourceIDs: []string{"eipalloc-0abc"},
		},
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if !vs.Body.Detail.RelatedFocused {
		t.Fatal("ActionToggleFocus did not grant RelatedFocus")
	}
	if vs.Body.Detail.RelatedCursor != 2 {
		t.Errorf("RelatedCursor after Tab = %d, want 2 (the drillable eip pivot) — Tab must skip the deferred alarm '(?)' at index 1 that only re-dispatches on Enter, so Tab+Enter drills a resource instead of no-op'ing", vs.Body.Detail.RelatedCursor)
	}
	// The drill read must return the drillable eip row, not the deferred alarm.
	drill, ok := c.SelectedRelatedRow()
	if !ok || drill.TargetType != "eip" {
		t.Errorf("SelectedRelatedRow() = %+v ok=%v, want the eip pivot (Enter navigates), not the deferred alarm '(?)'", drill, ok)
	}
}

// TestRelatedFocusEntry_Tab_DeferredOnly_FallsBackToActionable verifies the
// fallback: when the panel holds ONLY deferred "(?)" rows (no drillable pivot),
// Tab still lands on the first actionable "(?)" so it stays reachable and Enter
// can resolve it in place — detailSkipToDrillable must not strand focus at 0.
func TestRelatedFocusEntry_Tab_DeferredOnly_FallsBackToActionable(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("tg", 0), // index 0: dead-end
		{ // index 1: deferred "(?)" — the only actionable row
			TargetType: "alarm", DisplayName: "alarm",
			State: domain.RelatedUnknown, Count: 0,
		},
	}
	c := newRelatedFocusEntryController(t, rows)

	vs, _ := c.Apply(app.Action{Kind: app.ActionToggleFocus})
	cursor, actionable := relatedCursorAndActionable(t, vs)
	if cursor != 1 || !actionable {
		t.Errorf("RelatedCursor=%d actionable=%v, want cursor=1 actionable=true (fall back to the first actionable deferred row when nothing is drillable)", cursor, actionable)
	}
}
