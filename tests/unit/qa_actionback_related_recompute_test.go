// Selecting a scoreless "(?)"
// related row through the renderer-agnostic app.ActionRelatedSelect.
//
// A transient "(?)" related row (State: RelatedUnknown, no FetchFilter — see
// resource.IsRelatedActionable) is actionable in every renderer. With no
// ResourceIDs and no FetchFilter there is no target list to open, so the row
// resolves in place on the source detail.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// actionBackRecomputeNGResource is a node-group resource.Resource with no
// registered fetcher of its own.
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

// TestActionBack_AfterTransientUnknownRelatedDrill_RedispatchesRelatedCheck:
// ActionRelatedSelect on a scoreless row pushes no target list, stays on the
// ng detail, and re-dispatches a runtime.KindRelatedCheck scoped to the
// source so the row firms up in place.
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
	if row.CountDisplay != "" || !row.Actionable {
		t.Fatalf(`test setup: expected a transient blank (no-badge) actionable row before the drill; got CountDisplay=%q Actionable=%v`, row.CountDisplay, row.Actionable)
	}

	drillSnap, drillTasks := ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if drillSnap.Body.List != nil {
		t.Fatalf("BUG: ActionRelatedSelect on a scoreless row must NOT push a target list (goes-to-all); got Body.List=%+v", drillSnap.Body.List)
	}
	if drillSnap.Body.Detail == nil || ctrl.GetDetailResource().ID != ngRes.ID || ctrl.GetDetailResourceType() != "ng" {
		t.Fatalf("BUG: ActionRelatedSelect on a scoreless row must stay on the ng detail; got resource=%+v type=%q",
			ctrl.GetDetailResource(), ctrl.GetDetailResourceType())
	}
	wantScope := "ng/" + ngRes.ID
	hasRelatedCheck := false
	for _, task := range drillTasks {
		if task.Key.Kind == runtime.KindRelatedCheck && task.Key.Scope == wantScope {
			hasRelatedCheck = true
		}
	}
	if !hasRelatedCheck {
		t.Errorf("BUG: ActionRelatedSelect on a scoreless row (%q) must re-dispatch KindRelatedCheck (scope %q) to resolve in place; got tasks: %+v",
			ebsDef.DisplayName, wantScope, drillTasks)
	}
}
