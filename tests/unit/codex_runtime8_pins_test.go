package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A page routed to the canonical screen beneath a same-type related drill is
// that screen's page alone: the drill on top never sees it, so its reapply
// checker is not run against rows that belong to another list.
func TestLateCanonicalPage_DoesNotFeedTheDrillOnTopChecker(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	screen1 := seedCanonical200(t, ctrl, core, profile, region)

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "sg-src", Name: "sg-src", Type: "sg"}, "sg")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: provenancePinType, DisplayName: "EC2 Instances", Count: 3, ResourceIDs: []string{"i-drill-0", "i-drill-1", "i-drill-2"}},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-drill"),
		Provenance:   messages.FetchProvenanceFilteredList,
	})

	var seen [][]resource.Resource
	ctrl.PatchListReapplyChecker(func(_ context.Context, _ any, _ domain.Resource, cache domain.ResourceCache) domain.RelatedCheckResult {
		seen = append(seen, cache[provenancePinType].Resources)
		return domain.RelatedCheckResult{}
	}, resource.Resource{ID: "sg-src", Type: "sg"})

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(250, "i-late-canon"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ScreenID:     screen1,
	})

	if len(seen) != 0 {
		t.Fatalf("the drill's reapply checker ran %d time(s) against a page owned by the canonical screen beneath it; a page feeds only the list it names", len(seen))
	}
	if snap := ctrl.Snapshot(); snap.Body.List == nil || len(snap.Body.List.Rows) != 3 {
		t.Fatalf("drill rows changed after the late canonical page")
	}
	if snap := core.Session().RowStore.Snapshot(provenancePinType); len(snap.Rows) != 250 {
		t.Fatalf("RowStore rows = %d after the late canonical page, want 250: the page still reaches its own screen and the shared rows", len(snap.Rows))
	}
}

// A screen popped before its page arrives has dispatched nothing later, so
// its page is not superseded: it reaches no screen and still feeds the shared
// row state, which the next open of the type seeds from. A page a live screen
// has overtaken with a later request is still dropped.
func TestPoppedScreensPage_StillFeedsTheRowStore(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	screen1 := seedCanonical200(t, ctrl, core, profile, region)

	task := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: provenancePinType}, ScreenID: screen1}
	core.StampListFetchSeq(&task)
	if task.ListSeq == 0 {
		t.Fatal("test setup problem: the canonical screen's request must carry a sequence")
	}
	ctrl.Apply(app.Action{Kind: app.ActionBack})
	if ctrl.GetListInstance() == screen1 {
		t.Fatal("test setup problem: the canonical screen must be popped before its page arrives")
	}

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(250, "i-late"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ScreenID:     screen1,
		ListSeq:      task.ListSeq,
	})
	if snap := core.Session().RowStore.Snapshot(provenancePinType); len(snap.Rows) != 250 {
		t.Fatalf("RowStore rows = %d after the popped screen's page, want 250: a popped screen's page is not superseded, it feeds the shared rows", len(snap.Rows))
	}

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})
	screen2 := ctrl.GetListInstance()
	first := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: provenancePinType}, ScreenID: screen2}
	core.StampListFetchSeq(&first)
	second := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: provenancePinType}, ScreenID: screen2}
	core.StampListFetchSeq(&second)
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(300, "i-overtaken"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ScreenID:     screen2,
		ListSeq:      first.ListSeq,
	})
	if snap := core.Session().RowStore.Snapshot(provenancePinType); len(snap.Rows) != 250 {
		t.Fatalf("RowStore rows = %d after an overtaken page, want 250 unchanged: a live screen's later request still supersedes the earlier one", len(snap.Rows))
	}
}
