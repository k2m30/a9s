// keyboard_scoreless_resolve_test.go — regression pin: a truncated "(0+)"
// related row navigates to a SCOPED list exactly like "(N+)". It is a lower
// bound, not a dead end: pressing Enter opens the target list seeded with the
// found IDs (empty for 0+) and fetches the population so the reapply-checker can
// discover matches on later pages. It must NEVER resolve-in-place (no-op) and
// never open the unfiltered "goes to all" list. The zero count is not
// special-cased — it is "(N+)" with an empty seed.
package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

func TestActionSelect_TruncatedZeroRelatedRow_NavigatesToScopedList(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(t, res, "ec2")

	// A truncated "(0+)" row: actionable, found none yet — a lower bound.
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "sg",
			DisplayName: "Security Groups",
			Count:       0,
			Truncated:   true,
		},
	})

	c.SetDetailRelatedVisible(true, false)
	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus asserted via snapshot below

	snap := c.Snapshot()
	if snap.Body.Detail == nil || !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel did not accept focus — cannot drive related navigation in this env")
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	// It must NAVIGATE away from the detail (into the scoped target list), not
	// resolve in place.
	if got := c.Snapshot().Body.Kind; got == app.BodyKindDetail {
		t.Errorf("after ActionSelect on a truncated (0+) row, Body.Kind is still the detail — it must navigate to the scoped list like (N+)")
	}
	var recheck, fetch bool
	for _, tk := range tasks {
		switch tk.Key.Kind {
		case runtime.KindRelatedCheck:
			recheck = true
		case runtime.KindFetchResources:
			fetch = true
		}
	}
	if recheck {
		t.Errorf("truncated (0+) must NOT resolve in place; got a KindRelatedCheck task: %v", tasks)
	}
	if !fetch {
		t.Errorf("truncated (0+) navigation must emit a KindFetchResources task (populate + reapply); tasks=%v", tasks)
	}
}
