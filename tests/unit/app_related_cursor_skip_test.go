package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newRelatedSkipController builds a Controller on a ScreenDetail holding rows,
// with RelatedFocus on.
func newRelatedSkipController(t *testing.T, rows []app.DetailRelatedRow) *app.Controller {
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
	c.EnsureDetailState(resource.Resource{ID: "i-0abc123def456789a", Name: "prod-backend-01"}, "ec2")
	// ActionToggleFocus requires RelatedVisible, which ApplyDetailRelated alone
	// leaves unset.
	c.InitDetailRelatedRows("ec2")
	c.ApplyDetailRelated(rows)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	return c
}

// relatedRow builds a DetailRelatedRow; count 0 makes it dimmed
// (confirmed-empty).
func relatedRow(targetType string, count int) app.DetailRelatedRow {
	return app.DetailRelatedRow{
		TargetType:  targetType,
		DisplayName: targetType,
		Count:       count,
	}
}

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

func TestRelatedCursor_MoveDown_SkipsDimmedRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),
		relatedRow("vpc", 0),
		relatedRow("eni", 2),
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

func TestRelatedCursor_MoveUp_SkipsDimmedRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),
		relatedRow("vpc", 0),
		relatedRow("eni", 2),
	}
	c := newRelatedSkipController(t, rows)
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

func TestRelatedCursor_MoveTop_LandsOnFirstNonDimRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("vpc", 0),
		relatedRow("sg", 4),
		relatedRow("eni", 2),
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

func TestRelatedCursor_MoveBottom_LandsOnLastNonDimRow(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 4),
		relatedRow("eni", 2),
		relatedRow("vpc", 0),
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

func TestRelatedCursor_MoveDown_AllRemainingDimmed_CursorStaysPut(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),
		relatedRow("vpc", 0),
		relatedRow("eni", 0),
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

func TestRelatedCursor_ClickOnDimmedRow_NoNavigation(t *testing.T) {
	rows := []app.DetailRelatedRow{
		relatedRow("sg", 3),
		relatedRow("vpc", 0),
	}
	c := newRelatedSkipController(t, rows)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "1"})

	if len(tasks) != 0 {
		t.Errorf("ActionRelatedSelect on dimmed row produced %d tasks, want 0 (no navigation on a dimmed/non-actionable row)", len(tasks))
	}
}

func TestRelatedCursor_MenuParity_SameDimNonDimPatternSameLandingSequence(t *testing.T) {
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	menuCtrl := newBlessedController(t, core)

	all := resource.AllResourceTypes()
	if len(all) < 5 {
		t.Fatalf("resource.AllResourceTypes() has %d entries, need at least 5 for this pattern", len(all))
	}
	// Every catalog entry from index 5 onward is also dimmed so MoveBottom's
	// backward scan lands on index 4, bounding the comparison to the 5 entries
	// under test.
	dimPattern := []int{3, 0, 2, 0, 1} // per-index Count: >0 = non-dim, 0 = dimmed
	// Only a count confirmed this session dims a row, so every Origin is
	// "verified".
	for i, count := range dimPattern {
		menuCtrl.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{
				ResourceType: all[i].ShortName,
				Count:        count,
				Truncated:    false,
				Origin:       runtime.OriginVerified,
			},
		})
	}
	for i := 5; i < len(all); i++ {
		menuCtrl.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{
				ResourceType: all[i].ShortName,
				Count:        0,
				Truncated:    false,
				Origin:       runtime.OriginVerified,
			},
		})
	}
	// The permanent synthetic "costs" (Cost Explorer) entry sits after every
	// resource.AllResourceTypes() entry and is not itself in `all`, so it is
	// untouched by the loop above. Dim it too so MoveBottom's backward scan
	// keeps skipping past it into the catalog tail instead of stopping
	// immediately on it — preserving the bounded 5-entry comparison window
	// this test relies on.
	menuCtrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{
			ResourceType: "costs",
			Count:        0,
			Truncated:    false,
			Origin:       runtime.OriginVerified,
		},
	})

	menuLanding := func(vs app.ViewState) int { return vs.Body.Menu.Selected }
	// The dim/skip verdict is MenuEntry.ConfirmedEmpty, computed once in the body;
	// recomputing it here would let this test pass while the cursor and the
	// renderer disagreed.
	menuActionable := func(vs app.ViewState, idx int) bool {
		return !vs.Body.Menu.Entries[idx].ConfirmedEmpty
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

	rows := []app.DetailRelatedRow{
		relatedRow("t0", 3),
		relatedRow("t1", 0),
		relatedRow("t2", 2),
		relatedRow("t3", 0),
		relatedRow("t4", 1),
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

	if len(menuSeq) != len(relSeq) {
		t.Fatalf("sequence length mismatch: menu=%v related=%v", menuSeq, relSeq)
	}
	for i := range menuSeq {
		if menuSeq[i] != relSeq[i] {
			t.Errorf("landing-sequence step %d: menu landed on index %d, related-panel landed on index %d — the two surfaces must use identical skip semantics for the same dim/non-dim pattern (moves: MoveDown, MoveDown, MoveTop, MoveBottom)", i, menuSeq[i], relSeq[i])
		}
	}
}
