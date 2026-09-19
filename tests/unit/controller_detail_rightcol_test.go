// A RelatedCheckResult landing on a controller-backed detail screen must
// update DetailState.RelatedRows: a related row in RelatedLoading is not
// actionable, and once ApplyDetailRelatedResultForResource lands a resolved
// count the same row dispatches a task on ActionRelatedSelect.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ctrlDetailResourceType is a synthetic resource type used only in this file
// to avoid polluting the global related registry used by other tests.
const ctrlDetailResourceType = "ctrl-detail-reg-test-ec2"

// buildCtrlBackedDetailController creates an app.Controller with one related
// def registered and a ScreenDetail pushed and seeded for res, ready to
// drive ApplyDetailRelatedResultForResource + ActionRelatedSelect directly.
func buildCtrlBackedDetailController(t *testing.T) *app.Controller {
	t.Helper()

	resource.SetRelatedForTest(ctrlDetailResourceType, []resource.RelatedDef{
		{
			TargetType:  "tg",
			DisplayName: "Target Groups",
			Checker:     noopChecker,
		},
	})
	t.Cleanup(func() { resource.CleanupRelatedForTest(ctrlDetailResourceType) })

	res := resource.Resource{
		ID:   "i-ctrl001",
		Name: "ctrl-test-instance",
		Fields: map[string]string{
			"instance_id": "i-ctrl001",
			"state":       "running",
		},
	}

	c := newTestController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID: runtime.ScreenDetail,
			Context: runtime.ScreenContext{
				ResourceType: ctrlDetailResourceType,
				ResourceID:   res.ID,
			},
		},
	})
	c.EnsureDetailState(res, ctrlDetailResourceType)
	c.InitDetailRelatedRows(ctrlDetailResourceType)

	return c
}

func TestDetailController_RelatedCheckResult_EnablesRelatedSelect(t *testing.T) {
	c := buildCtrlBackedDetailController(t)

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after EnsureDetailState + InitDetailRelatedRows")
	}
	if len(body.Related) != 1 || body.Related[0].Name != "Target Groups" {
		t.Fatalf("expected exactly 1 related row 'Target Groups' after InitDetailRelatedRows, got %+v", body.Related)
	}
	if !body.Related[0].Loading {
		t.Fatal("test setup: related row must start Loading before the result lands")
	}

	// A loading row is not actionable.
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	_, preTasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if len(preTasks) != 0 {
		t.Fatalf("ActionRelatedSelect on a still-Loading related row dispatched %d tasks, want 0; tasks: %+v", len(preTasks), preTasks)
	}

	c.ApplyDetailRelatedResultForResource(ctrlDetailResourceType, "i-ctrl001", "Target Groups", "tg",
		domain.RelatedResolved, 3, false, "", false, []string{"tg-1", "tg-2", "tg-3"}, nil)

	postBody := c.Snapshot().Body.Detail
	if postBody.Related[0].Loading {
		t.Fatal("Fix6: related row still Loading after ApplyDetailRelatedResultForResource — result did not land")
	}
	if postBody.Related[0].Count != 3 {
		t.Fatalf("Fix6: related row Count = %d, want 3 after ApplyDetailRelatedResultForResource", postBody.Related[0].Count)
	}

	_, postTasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if len(postTasks) == 0 {
		t.Fatal("Fix6: ActionRelatedSelect on the now-resolved actionable related row dispatched 0 tasks — result landing did not make the row selectable")
	}
}
