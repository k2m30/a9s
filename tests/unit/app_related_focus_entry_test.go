package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newRelatedFocusEntryController builds a Controller on a ScreenDetail holding
// rows, with RelatedFocus off so the caller drives ActionToggleFocus.
func newRelatedFocusEntryController(t *testing.T, rows []app.DetailRelatedRow) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenDetail},
	})
	c.EnsureDetailState(resource.Resource{ID: "key-0abc123def456789a", Name: "prod-kms-key"}, "kms")
	c.InitDetailRelatedRows("kms")
	c.ApplyDetailRelated(rows)
	return c
}

func TestRelatedFocusEntry_Tab_SkipsLeadingDimmedRows(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0),
		relatedRow("ami", 0),
		relatedRow("alias", 1),
		relatedRow("grant", 3),
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

func TestRelatedFocusEntry_Tab_AllRowsDimmed_CursorStaysZero(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0),
		relatedRow("ami", 0),
		relatedRow("grant", 0),
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

func TestRelatedFocusEntry_TabAwayThenBack_ReappliesSkipRule(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0),
		relatedRow("alias", 2),
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

	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if vs.Body.Detail.RelatedFocused {
		t.Fatal("second Tab did not release RelatedFocus")
	}

	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus})
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

func TestRelatedFocusEntry_Tab_MatchesMoveTopLanding(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("ebs-volume", 0),
		relatedRow("ami", 0),
		relatedRow("alias", 1),
		relatedRow("grant", 0),
		relatedRow("policy", 4),
	}

	tabCtrl := newRelatedFocusEntryController(t, rows)
	vsTab, _ := tabCtrl.Apply(app.Action{Kind: app.ActionToggleFocus})
	tabCursor := vsTab.Body.Detail.RelatedCursor

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

func TestRelatedFocusEntry_Tab_PartialReplayMix_SkipsBareRows(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("target-group", 0),
		relatedRow("subnet", 0),
		relatedRow("vpc", 1),
		relatedRow("security-group", 0),
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

	vs, _ = c.Apply(app.Action{Kind: app.ActionToggleFocus})
	if vs.Body.Detail.RelatedFocused {
		t.Fatal("second Tab did not release RelatedFocus on an all-bare related panel — focus is trapped")
	}
}

// A deferred "(?)" pivot is actionable (Enter re-dispatches it in place) but
// not drillable, so Tab lands on the first row Enter navigates from.
func TestRelatedFocusEntry_Tab_PrefersDrillableOverDeferredUnknown(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("tg", 0),
		{
			TargetType: "alarm", DisplayName: "alarm",
			State: domain.RelatedUnknown, Count: 0,
		},
		{
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
	drill, ok := c.SelectedRelatedRow()
	if !ok || drill.TargetType != "eip" {
		t.Errorf("SelectedRelatedRow() = %+v ok=%v, want the eip pivot (Enter navigates), not the deferred alarm '(?)'", drill, ok)
	}
}

func TestRelatedFocusEntry_Tab_DeferredOnly_FallsBackToActionable(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("tg", 0),
		{
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
