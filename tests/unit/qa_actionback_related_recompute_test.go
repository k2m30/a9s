// qa_actionback_related_recompute_test.go — RED regression pin for a P2 bug
// found by Codex in the v3.47.0 #38 landing: the web/headless Back path never
// re-dispatches the related-check recompute that owner decision #38
// (2026-07-06) requires.
//
// #38 made a transient "(?)" related row (State: RelatedUnknown, no
// FetchFilter — see resource.IsRelatedActionable) actionable in every
// renderer: Enter/select on
// such a row opens the target type's plain top-level list, the same
// navigation a menu entry would produce. Returning to the source detail must
// then RECOMPUTE that pivot's count now that the target's cache is warm — but
// the re-dispatch (a messages.RelatedCheckStarted-equivalent) was wired ONLY
// into the TUI's Escape handler (internal/tui/app_stack.go's
// recomputeRelatedOnReveal, invoked from app_input.go). The renderer-agnostic
// Back path — internal/app/actions_nav.go's handleActionBack, the ONLY Back
// handler for web/headless — just pops the screen:
//
//	func (c *Controller) handleActionBack(_ Action) (ViewState, []runtime.TaskRequest) {
//	    c.applyIntents([]runtime.UIIntent{runtime.PopScreen{}})
//	    return c.snapshot(), nil
//	}
//
// No TaskRequest is ever returned, so the revealed detail's stale "(?)" row
// for the pivot the user just drilled through never recomputes for a
// web/headless client — only the TUI (via its own Escape-key path, not
// ActionBack) gets the fix.
//
// Fixture pair: "ng" (node group) -> "ebs" (EBS Volumes), the same pair
// qa_related_transient_unknown_drill_test.go pins at the TUI/keyboard
// altitude (that file's Pin 3, GREEN today — the TUI Escape path already
// works). This file mirrors that setup at the controller/headless altitude,
// driving the drill via app.ActionRelatedSelect (the renderer-agnostic
// counterpart of the TUI's keyboard Enter on a focused related row) and the
// return via app.ActionBack (the renderer-agnostic counterpart of Esc),
// asserting on the TaskRequest a real Core.HandleRelatedCheckStarted-shaped
// re-dispatch would produce (runtime.KindRelatedCheck, Scope "ng/<id>") —
// exactly what internal/runtime/related.go's HandleRelatedCheckStarted
// returns, and what openRelatedDetail (internal/app/navigate.go) returns
// on a cache-miss fresh detail open, so a real fix would make this
// assertion pass without inventing a new task shape.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// actionBackRecomputeNGResource mirrors transientUnknownNGResource in
// qa_related_transient_unknown_drill_test.go — a node-group resource.Resource
// with no registered fetcher of its own.
func actionBackRecomputeNGResource() resource.Resource {
	return resource.Resource{
		ID:   "prod-workers",
		Name: "prod-workers",
		Fields: map[string]string{
			"nodegroup_name": "prod-workers",
			"cluster_name":   "prod-cluster",
			"status":         "ACTIVE",
		},
	}
}

// scopeNGToEBSOnlyForActionBackTest captures the REAL, live ng->ebs
// RelatedDef ("EBS Volumes", checkNGEBS) from the production registry, then
// re-registers "ng" with only that single def for the duration of the
// calling test — restored via t.Cleanup(resource.CleanupRelatedForTest).
// Mirrors scopeNGToEBSOnly in qa_related_transient_unknown_drill_test.go
// (duplicated here rather than shared: that helper lives in package unit,
// this file is package unit_test to reach newTestController).
func scopeNGToEBSOnlyForActionBackTest(t *testing.T) resource.RelatedDef {
	t.Helper()
	var ebsDef resource.RelatedDef
	found := false
	for _, def := range resource.GetRelated("ng") {
		if def.TargetType == "ebs" {
			ebsDef = def
			found = true
			break
		}
	}
	if !found {
		t.Fatal(`test setup: no ng->ebs RelatedDef registered in production (expected "EBS Volumes", checkNGEBS)`)
	}
	resource.SetRelatedForTest("ng", []resource.RelatedDef{ebsDef})
	t.Cleanup(func() { resource.CleanupRelatedForTest("ng") })
	return ebsDef
}

// TestActionBack_AfterTransientUnknownRelatedDrill_RedispatchesRelatedCheck
// drives the full #38 user journey at the controller/headless altitude:
//  1. Open the "ng" detail and seed its lone related row (scoped to ebs) with
//     the transient "(?)" state (RelatedUnknown, no FetchFilter, not loading) —
//     exactly what a real cold-cache checkNGEBS result delivers via
//     ApplyDetailRelatedResultForResource (the same merge every related-result
//     path uses, including the TUI adapter's messages.RelatedCheckResult
//     handling).
//  2. Drill into the pivot via app.ActionRelatedSelect (row index 0) — the
//     web UI's row-click path, and the renderer-agnostic counterpart of the
//     TUI's keyboard Enter on a focused related row. This must push a plain
//     "ebs" resource list (owner decision #38, already fixed and pinned by
//     qa_related_transient_unknown_drill_test.go's Pin 1/2).
//  3. Return via app.ActionBack — the ONLY Back action web/headless clients
//     have; there is no separate "Esc" action in this lane.
//  4. Assert the tasks returned by the ActionBack Apply call contain a
//     runtime.KindRelatedCheck TaskRequest scoped to "ng/prod-workers" — the
//     re-dispatch that would let the "(?)" row recompute once the target
//     cache is warm, mirroring HandleRelatedCheckStarted's own task shape.
func TestActionBack_AfterTransientUnknownRelatedDrill_RedispatchesRelatedCheck(t *testing.T) {
	ebsDef := scopeNGToEBSOnlyForActionBackTest(t)
	ctrl := newTestController(t)

	ngRes := actionBackRecomputeNGResource()

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(ngRes, "ng")
	ctrl.InitDetailRelatedRows("ng")
	ctrl.ApplyDetailRelatedResultForResource("ng", ngRes.ID, ebsDef.DisplayName, "ebs", domain.RelatedUnknown, 0, false, "", false, nil, nil)

	preDrill := ctrl.Snapshot()
	if preDrill.Body.Detail == nil || len(preDrill.Body.Detail.Related) != 1 {
		t.Fatalf("test setup: expected exactly 1 related row on the ng detail before the drill; got Body.Detail=%+v", preDrill.Body.Detail)
	}
	row := preDrill.Body.Detail.Related[0]
	if row.CountDisplay != "(?)" || !row.Actionable {
		t.Fatalf(`test setup: expected a transient "(?)" actionable row before the drill; got CountDisplay=%q Actionable=%v`, row.CountDisplay, row.Actionable)
	}

	// Drill: controller-level related-row select (web UI row-click path).
	drillSnap, drillTasks := ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if drillSnap.Body.List == nil {
		t.Fatalf("test setup: ActionRelatedSelect on the transient \"(?)\" row did not push a resource list (Body.List is nil) — cannot exercise the Back path. Tasks: %+v", drillTasks)
	}
	hasFetch := false
	for _, task := range drillTasks {
		if task.Key.Kind == runtime.KindFetchResources && task.Key.Scope == "ebs" {
			hasFetch = true
		}
	}
	if !hasFetch {
		t.Fatalf("test setup: drill did not dispatch a KindFetchResources task for \"ebs\" — cannot exercise the Back path. Tasks: %+v", drillTasks)
	}

	// Back: pop the drilled-into ebs list, revealing the ng detail again —
	// the ONLY Back action available to a web/headless caller.
	backSnap, backTasks := ctrl.Apply(app.Action{Kind: app.ActionBack})
	if backSnap.Body.Detail == nil || ctrl.GetDetailResource().ID != ngRes.ID || ctrl.GetDetailResourceType() != "ng" {
		t.Fatalf("test setup: ActionBack did not reveal the ng detail (resource=%+v, type=%q) — cannot assert on the recompute dispatch", ctrl.GetDetailResource(), ctrl.GetDetailResourceType())
	}

	wantScope := "ng/" + ngRes.ID
	hasRelatedCheck := false
	for _, task := range backTasks {
		if task.Key.Kind == runtime.KindRelatedCheck && task.Key.Scope == wantScope {
			hasRelatedCheck = true
		}
	}
	if !hasRelatedCheck {
		t.Errorf("BUG: ActionBack revealing the ng detail did not re-dispatch a KindRelatedCheck task (want Scope %q); got tasks: %+v. internal/app/actions_nav.go's handleActionBack only pops the screen stack (runtime.PopScreen{}) and always returns nil tasks — the TUI's parallel Escape path (internal/tui/app_stack.go's recomputeRelatedOnReveal, wired from app_input.go) re-dispatches a RelatedCheckStarted-equivalent, but the shared/headless ActionBack path has no analogous hook, so the %q pivot's \"(?)\" row on the ng detail never recomputes for a web/headless client without a manual refresh.",
			wantScope, backTasks, ebsDef.DisplayName)
	}
}
