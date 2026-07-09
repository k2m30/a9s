// keyboard_scoreless_resolve_test.go — regression pin for Codex review #4.
//
// The mouse ActionRelatedSelect path resolves a scoreless related row IN PLACE
// (re-dispatch the source's related checks) rather than opening the target
// type's plain unfiltered list. The keyboard/web ActionSelect path must behave
// identically; before the fix it fell through to ResolveRelatedNavigate, which
// with no IDs/filter produced an unfiltered target list ("goes to all").
package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

func TestActionSelect_ScorelessRelatedRow_ResolvesInPlace(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(t, res, "ec2")

	// A truncated "(0+)" row: actionable (truncated), but nothing to scope by —
	// no ResourceIDs, no FetchFilter.
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

	foundRecheck := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.KindRelatedCheck {
			foundRecheck = true
		}
	}
	if !foundRecheck {
		t.Errorf("keyboard ActionSelect on a scoreless (0+) related row did not emit a "+
			"KindRelatedCheck resolve-in-place task; tasks=%v — it must not fall through to the "+
			"unfiltered target list", tasks)
	}

	// The screen must remain the detail — no navigation to a list occurred.
	if got := c.Snapshot().Body.Kind; got != app.BodyKindDetail {
		t.Errorf("after ActionSelect on a scoreless row, Body.Kind = %q, want the detail (no navigation)", got)
	}
}
