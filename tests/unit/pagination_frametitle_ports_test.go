// pagination_frametitle_ports_test.go pins the "(N+)" truncated-count vs
// "(N)" exact-count FrameTitle transition across Ctrl+R-reset, top-level
// re-fetch, empty-list re-fetch, and cross-resource-type switch on the live
// seam: Controller.ApplyResourcesLoaded -> ListFrameTitle / ListSelected.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
)

// wave3PagResources returns n EC2 instances with sequential zero-padded IDs
// starting at 0.
func wave3PagResources(n int) []resource.Resource {
	return wave3PagResourcesFrom(0, n)
}

// wave3PagResourcesFrom returns n EC2 instances with sequential zero-padded
// IDs starting at start — used to generate a
// disjoint-ID second page (AWS pagination never repeats an ID across pages).
func wave3PagResourcesFrom(start, n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", start+i)
		out[i] = resource.Resource{
			ID: id, Name: id,
			Fields: map[string]string{"instance_id": id, "name": id, "state": "running"},
		}
	}
	return out
}

func TestPaginationFrameTitle_CtrlR_ResetsPagination(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))

	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)
	if title := c.ListFrameTitle(); title != "ec2(200+)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(200+)", title)
	}

	// Load-more appends page 2 (disjoint IDs, matching real AWS pagination).
	c.ApplyResourcesLoaded("ec2", wave3PagResourcesFrom(200, 200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p3"}, true)
	if title := c.ListFrameTitle(); title != "ec2(400+)" {
		t.Fatalf("after load-more: expected %q, got %q", "ec2(400+)", title)
	}

	// Ctrl+R: a full re-fetch replaces with first page only (Append=false) —
	// mirrors app_handlers.go's fetchResources dispatch on refresh.
	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-fresh-p2"}, false)
	if title := c.ListFrameTitle(); title != "ec2(200+)" {
		t.Errorf("after Ctrl+R refresh: expected %q, got %q", "ec2(200+)", title)
	}
}

func TestPaginationFrameTitle_CtrlR_TopLevel_ReFetchesAllPages(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))

	c.ApplyResourcesLoaded("ec2", wave3PagResources(150), nil, false)
	if title := c.ListFrameTitle(); title != "ec2(150)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(150)", title)
	}

	// Top-level fetchers exhaust all pages internally, so a re-fetch arrives
	// with nil/false pagination even if the underlying count changed.
	c.ApplyResourcesLoaded("ec2", wave3PagResources(160), nil, false)
	if title := c.ListFrameTitle(); title != "ec2(160)" {
		t.Errorf("after re-fetch: expected %q, got %q", "ec2(160)", title)
	}
}

func TestPaginationFrameTitle_CtrlR_EmptyList_ReFetches(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))

	c.ApplyResourcesLoaded("ec2", []resource.Resource{}, nil, false)
	if title := c.ListFrameTitle(); title != "ec2(0)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(0)", title)
	}

	c.ApplyResourcesLoaded("ec2", wave3PagResources(1), nil, false)
	if title := c.ListFrameTitle(); title != "ec2(1)" {
		t.Errorf("after refresh from empty: expected %q, got %q", "ec2(1)", title)
	}
}

func TestPaginationFrameTitle_DetailAndBack_PreservesLoadedData(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))

	// 200 + 200 + 200 = 600 total across three pages.
	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)
	c.ApplyResourcesLoaded("ec2", wave3PagResourcesFrom(200, 200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p3"}, true)
	c.ApplyResourcesLoaded("ec2", wave3PagResourcesFrom(400, 200), &resource.PaginationMeta{IsTruncated: false}, true)

	titleBefore := c.ListFrameTitle()
	if titleBefore != "ec2(600)" {
		t.Fatalf("precondition: expected %q, got %q", "ec2(600)", titleBefore)
	}

	for range 5 {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	selected, ok := c.ListSelected()
	if !ok {
		t.Fatal("expected a selected resource")
	}

	// A detail round-trip does not touch the list controller state at all
	// (the list screen stays on the stack underneath detail) — reasserting
	// after the cursor move is the live-path equivalent of "state survives
	// return from detail".
	titleAfter := c.ListFrameTitle()
	if titleAfter != titleBefore {
		t.Errorf("frame title changed after detail round-trip: %q -> %q", titleBefore, titleAfter)
	}
	selectedAfter, ok := c.ListSelected()
	if !ok || selectedAfter.ID != selected.ID {
		t.Errorf("cursor moved after detail round-trip: %q -> (ok=%v) %q", selected.ID, ok, selectedAfter.ID)
	}
}

func TestPaginationFrameTitle_SwitchingResourceType_ResetsPagination(t *testing.T) {
	c1 := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	c1.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok"}, false)
	if title := c1.ListFrameTitle(); title != "ec2(200+)" {
		t.Fatalf("precondition: ec2 %q", title)
	}

	// Switching resource type in production pushes a fresh list screen —
	// modeled here as an independent controller for "dbi", starting in
	// loading state (no rows yet).
	c2 := openListControllerWithConfig(t, "dbi", configForType("dbi"))
	if title := c2.ListFrameTitle(); title != "dbi" {
		t.Errorf("new list screen should show loading title %q, got %q", "dbi", title)
	}

	c2.ApplyResourcesLoaded("dbi", wave3PagResources(50), nil, false)
	if title := c2.ListFrameTitle(); title != "dbi(50)" {
		t.Errorf("expected %q, got %q", "dbi(50)", title)
	}

	// The ec2 controller is untouched by the dbi switch.
	if title := c1.ListFrameTitle(); title != "ec2(200+)" {
		t.Errorf("ec2 controller state changed by unrelated dbi switch: got %q", title)
	}
}

// TestPaginationFrameTitle_RapidRefresh_ReplaceClean pins that several rapid
// Ctrl+R replace results
// arriving in sequence must leave the model reflecting only the last one,
// with no stale "+" truncation marker once the final page is exact.
func TestPaginationFrameTitle_RapidRefresh_ReplaceClean(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	for attempt := range 3 {
		count := 100 + attempt*10
		c.ApplyResourcesLoaded("ec2", wave3PagResources(count), nil, false)
	}

	title := c.ListFrameTitle()
	if title != "ec2(120)" {
		t.Errorf("after 3 rapid refreshes: expected %q, got %q", "ec2(120)", title)
	}
	if _, ok := c.ListSelected(); !ok {
		t.Error("selected resource missing after rapid refreshes")
	}
	if strings.Contains(title, "+") {
		t.Errorf("frame title should not show '+' after non-truncated refresh, got %q", title)
	}
}

// ===========================================================================
// buildListFrameTitle's LoadingMore and text-filter branches (list_body.go).
// ===========================================================================

// TestPaginationFrameTitle_LoadingMore_ShowsAndClears pins that the
// "(N+ loading...)" inline suffix appears
// while ActionLoadMore is in flight and disappears (replaced by the plain
// count) once the appended page lands.
func TestPaginationFrameTitle_LoadingMore_ShowsAndClears(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	c.Apply(app.Action{Kind: app.ActionLoadMore})
	if title := c.ListFrameTitle(); title != "ec2(200+ loading...)" {
		t.Fatalf("while load-more in flight: expected %q, got %q", "ec2(200+ loading...)", title)
	}

	c.ApplyResourcesLoaded("ec2", wave3PagResourcesFrom(200, 50), &resource.PaginationMeta{IsTruncated: false}, true)
	title := c.ListFrameTitle()
	if strings.Contains(title, "loading...") {
		t.Errorf("after append lands: title must not still show 'loading...', got %q", title)
	}
	if title != "ec2(250)" {
		t.Errorf("after append lands: expected %q, got %q", "ec2(250)", title)
	}
}

// TestPaginationFrameTitle_TruncatedWithFilter pins that an active text filter on a
// still-truncated list shows "(filtered/total+)".
func TestPaginationFrameTitle_TruncatedWithFilter(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	c.ApplyResourcesLoaded("ec2", wave3PagResources(200), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	// "i-0000" matches i-00000..i-00009 = 10 of the 200 loaded rows.
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "i-0000"})

	if title := c.ListFrameTitle(); title != "ec2(10/200+)" {
		t.Errorf("expected exact title %q, got %q", "ec2(10/200+)", title)
	}
}

// TestPaginationFrameTitle_AllLoadedWithFilter pins that an active text filter on a
// fully-loaded (non-truncated) list shows the exact "(filtered/total)" pair,
// with no trailing "+".
func TestPaginationFrameTitle_AllLoadedWithFilter(t *testing.T) {
	c := openListControllerWithConfig(t, "ec2", configForType("ec2"))
	c.ApplyResourcesLoaded("ec2", wave3PagResources(523), &resource.PaginationMeta{IsTruncated: false}, false)

	// "i-0000" matches i-00000..i-00009 = 10 of the 523 loaded rows.
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "i-0000"})

	if title := c.ListFrameTitle(); title != "ec2(10/523)" {
		t.Errorf("expected %q, got %q", "ec2(10/523)", title)
	}
}
