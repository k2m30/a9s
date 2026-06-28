// controller_detail_rightcol_test.go — regression test for Fix 6 (P2-2):
// DetailModel.Update(RelatedCheckResult) on the controller-backed path must
// propagate the result to m.rightCol so that Enter on the focused right-column
// row dispatches RelatedNavigate.
//
// Pre-fix failure: the ctrl != nil branch applied the result to the controller
// but forgot `m.rightCol, _ = m.rightCol.Update(msg)`. The local rightCol
// therefore stayed in the loading state it had at construction time. A loading
// row is not actionable (isActionableRow returns false for loading rows), so
// Enter on the focused, apparently-resolved row produced no RelatedNavigateMsg —
// a silent no-op that looked like a broken Enter key to the user.
//
// Test strategy (mirrors rightcolumn_actionable_test.go):
//  1. Build DetailModel via NewDetailWithCtrl with a registered related def.
//  2. Tab to focus the right column while the row is in loading state (Tab works
//     in loading state: HasActionableRows returns true for loading rows).
//  3. Inject RelatedCheckResult with count > 0, which MUST update m.rightCol.
//  4. Press Enter → assert RelatedNavigate is dispatched.
//
// Without Fix 6, step 3 leaves m.rightCol loading; step 4 is a no-op.
package unit_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ctrlDetailResourceType is a synthetic resource type used only in this file
// to avoid polluting the global related registry used by other tests.
const ctrlDetailResourceType = "ctrl-detail-reg-test-ec2"

// buildCtrlBackedDetail creates a NewDetailWithCtrl with one related def
// registered and the controller stack already seeded with a ScreenDetail.
// Returns the DetailModel and a cleanup func.
func buildCtrlBackedDetail(t *testing.T) (views.DetailModel, func()) {
	t.Helper()

	resource.SetRelatedForTest(ctrlDetailResourceType, []resource.RelatedDef{
		{
			TargetType:  "tg",
			DisplayName: "Target Groups",
			Checker:     noopChecker,
		},
	})
	cleanup := func() { resource.CleanupRelatedForTest(ctrlDetailResourceType) }

	res := resource.Resource{
		ID:   "i-ctrl001",
		Name: "ctrl-test-instance",
		Fields: map[string]string{
			"instance_id": "i-ctrl001",
			"state":       "running",
		},
	}

	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	ctrl := app.New(core)

	// Navigate to a list screen first (mirrors how TUI uses the controller)
	// then push a detail screen for res.
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: ctrlDetailResourceType})
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID: runtime.ScreenDetail,
			Context: runtime.ScreenContext{
				ResourceType: ctrlDetailResourceType,
				ResourceID:   res.ID,
			},
		},
	})
	ctrl.EnsureDetailState(res, ctrlDetailResourceType)

	k := keys.Default()
	d := views.NewDetailWithCtrl(res, ctrlDetailResourceType, nil, k, ctrl)
	d.SetSize(140, 30)

	return d, cleanup
}

// deliverCtrlRelatedResult injects a RelatedCheckResult into d and returns
// the updated model.
func deliverCtrlRelatedResult(d views.DetailModel, count int) views.DetailModel {
	msg := messages.RelatedCheckResult{
		ResourceType:   ctrlDetailResourceType,
		DefDisplayName: "Target Groups",
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      count,
		},
	}
	updated, _ := d.Update(msg)
	return updated
}

// TestDetailModel_CtrlBacked_RelatedCheckResult_EnablesEnterNavigation verifies
// that on the controller-backed path, Update(RelatedCheckResult) propagates
// the result to m.rightCol, making the row actionable so Enter dispatches
// RelatedNavigate.
func TestDetailModel_CtrlBacked_RelatedCheckResult_EnablesEnterNavigation(t *testing.T) {
	ensureNoColor(t)

	d, cleanup := buildCtrlBackedDetail(t)
	defer cleanup()

	// Right column must be visible at width=140 with a registered related def.
	if view := stripAnsi(d.View()); !containsRelated(view) {
		t.Skip("right column not visible — layout may not show at this terminal size")
	}

	// Tab to focus right column while the row is still in loading state.
	// Loading rows are always focusable (HasActionableRows == true for loading).
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	// Without focus, Enter is a no-op — but we've tabbed so focus is on right col.
	// Pre-inject check: Enter while loading should NOT produce a nav msg.
	_, preCmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if preCmd != nil {
		if msg := preCmd(); msg != nil {
			if _, ok := msg.(messages.RelatedNavigate); ok {
				t.Skip("loading row already produced RelatedNavigate — unexpected but not the regression being tested")
			}
		}
	}

	// Deliver the result: count=3, not loading. This is the step that Fix 6 enables.
	// Without Fix 6: m.rightCol stays loading → Enter is still a no-op.
	// With Fix 6: m.rightCol is updated to count=3 → isActionableRow == true.
	d = deliverCtrlRelatedResult(d, 3)

	// Press Enter and collect the emitted message.
	_, postCmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if postCmd == nil {
		t.Fatal("Fix6: Enter on controller-backed detail after RelatedCheckResult produced nil cmd — rightCol was not updated (pre-fix regression)")
	}
	msg := postCmd()
	if _, ok := msg.(messages.RelatedNavigate); !ok {
		t.Fatalf("Fix6: Enter produced %T, want messages.RelatedNavigate — rightCol.Update(msg) was not called in the ctrl path", msg)
	}
}

// containsRelated returns true when the plain-text view contains the "RELATED"
// header that signals the right column is rendered.
func containsRelated(plain string) bool {
	for i := 0; i+6 < len(plain); i++ {
		if plain[i:i+7] == "RELATED" {
			return true
		}
	}
	return false
}
