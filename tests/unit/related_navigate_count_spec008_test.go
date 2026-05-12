package unit_test

// related_navigate_count_spec008_test.go — Spec-008: handleRelatedNavigate behavior.
//
// Tests for handleRelatedNavigate() bug fixes.
// Bug 1: TargetID case opens list instead of detail.
// Bug 2: RelatedIDs>1 creates unfiltered list.
//
// Types RelatedCheckResultMsg, RelatedNavigateMsg, and resource.RelatedCheckResult
// already exist on this branch (006 infrastructure).
//
// TestApp_008_RelatedNavigate_* tests FAIL AT RUNTIME until handleRelatedNavigate
// is fixed in app_handlers.go.
// TestApp_008_RelatedCheckResult_Count0_NoNavigation PASSES NOW (regression guard).

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// Local helpers (unit_test package cannot access unit package internals)
// ---------------------------------------------------------------------------

// relatedApplyMsg sends a message through the tui.Model's Update.
func relatedApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// relatedViewContent returns the stripped content string from View().
func relatedViewContent(m tui.Model) string {
	return m.View().Content
}

// newRelatedDemoModel creates a tui.Model in demo mode, sized for testing.
func newRelatedDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = relatedApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// navigateToEC2DetailRelated navigates the given model to the EC2 detail view.
func navigateToEC2DetailRelated(t *testing.T, m tui.Model, res resource.Resource) tui.Model {
	t.Helper()
	m, _ = relatedApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	return m
}

// applyRelatedResourcesLoaded delivers a ResourcesLoadedMsg for the given type.
func applyRelatedResourcesLoaded(m tui.Model, resourceType string, resources []resource.Resource) tui.Model {
	m, _ = relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: resourceType,
		Resources:    resources,
	})
	return m
}

// ---------------------------------------------------------------------------
// Count=1: single related resource should open DETAIL view, not list
// ---------------------------------------------------------------------------

// TestApp_008_RelatedNavigate_SingleID_OpensDrillTarget verifies that when a
// RelatedNavigateMsg arrives with a single TargetID (count=1 path), the model
// mirrors manual Enter on the target's list row: for types with
// Children[Key="enter"] registered, it must enter that child view rather
// than push generic detail or a filtered list. tg's enter-child is
// tg_health.
func TestApp_008_RelatedNavigate_SingleID_OpensDrillTarget(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	tgRes := resource.Resource{
		ID:     "tg-spec008-single",
		Name:   "my-target-group",
		Status: "active",
		Fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-target-group/abc123"},
	}
	m = applyRelatedResourcesLoaded(m, "tg", []resource.Resource{tgRes})

	// Deliver RelatedNavigateMsg with TargetID set (single resource navigation).
	m, cmd := relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType: "tg",
		TargetID:   "tg-spec008-single",
	})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = relatedApplyMsg(m, msg)
		}
	}

	view := stripAnsi(relatedViewContent(m))

	// tg registers Children[Key="enter"] → tg_health, so fast path enters it.
	if !strings.Contains(view, "tg_health") {
		t.Errorf("RelatedNavigateMsg with TargetID on tg (enter-child registered) must enter tg_health child view; got:\n%s", view)
	}
	if strings.Contains(view, "tg(1)") {
		t.Errorf("RelatedNavigateMsg with TargetID=%q must not open a filtered list; got:\n%s", "tg-spec008-single", view)
	}
	if strings.Contains(view, "detail -- tg-spec008-single") {
		t.Errorf("RelatedNavigateMsg for tg (with enter-child) must not push plain detail; got:\n%s", view)
	}
}

// TestApp_008_RelatedNavigate_SingleID_CacheMiss_AutoOpensDetail verifies that
// when TargetID is known but target cache is empty, the intermediate related list
// auto-opens detail as soon as ResourcesLoaded leaves exactly one filtered row.
func TestApp_008_RelatedNavigate_SingleID_CacheMiss_AutoOpensDetail(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	// No preloaded ami cache: this exercises the fetch/list fallback path.
	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "ami",
		SourceResource: ec2Res,
		TargetID:       "ami-single-1",
	})

	// Fetch result contains multiple AMIs; related ID filter leaves one.
	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources: []resource.Resource{
			{ID: "ami-single-1", Name: "ami-single", Status: "available"},
			{ID: "ami-other-1", Name: "ami-other", Status: "available"},
		},
	})
	m = m2
	if cmd != nil {
		if follow := cmd(); follow != nil {
			m, _ = relatedApplyMsg(m, follow)
		}
	}

	view := stripAnsi(relatedViewContent(m))
	if !strings.Contains(view, "detail --") || !strings.Contains(view, "ami-single-1") {
		t.Fatalf("single related TargetID cache-miss path must auto-open AMI detail; got:\n%s", view)
	}
	if strings.Contains(view, "ami(1/") || strings.Contains(view, "ami(1)") {
		t.Fatalf("single related TargetID cache-miss path must not leave user in list view; got:\n%s", view)
	}
}

// TestApp_008_RelatedNavigate_SingleRelatedIDs_CacheMiss_AutoOpensDrillTarget verifies
// the right-column path: RelatedIDs with one element must auto-open the type's drill
// target (child view if Children[Key="enter"] is registered, else detail) — NOT leave
// the operator stranded on a 1-row filtered list. For asg the Enter-child is
// `asg_activities`, so the test asserts that view loads after auto-navigation.
func TestApp_008_RelatedNavigate_SingleRelatedIDs_CacheMiss_AutoOpensDrillTarget(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	// No preloaded asg cache: this exercises the right-column cache-miss flow.
	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "asg",
		SourceResource: ec2Res,
		RelatedIDs:     []string{"asg-single-1"},
	})

	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "asg",
		Resources: []resource.Resource{
			{ID: "asg-single-1", Name: "asg-single", Status: "InService"},
			{ID: "asg-other-1", Name: "asg-other", Status: "InService"},
		},
	})
	m = m2
	if cmd != nil {
		if follow := cmd(); follow != nil {
			m, _ = relatedApplyMsg(m, follow)
		}
	}

	view := stripAnsi(relatedViewContent(m))
	// asg has Children[Key="enter"]=asg_activities — auto-open must mirror
	// manual Enter and land on the child view, not the generic detail.
	if !strings.Contains(view, "asg_activities") {
		t.Fatalf("single related right-column cache-miss path must auto-open asg Enter-child (asg_activities); got:\n%s", view)
	}
	if strings.Contains(view, "asg(1/") || strings.Contains(view, "asg(1)") {
		t.Fatalf("single related right-column cache-miss path must not leave user in list view; got:\n%s", view)
	}
}

// TestApp_008_RelatedNavigate_SingleID_CacheMiss_LoadsMoreUntilTargetFound verifies
// that exact-ID related navigation does not dead-end on page 1 when the known
// target lives on a later page.
func TestApp_008_RelatedNavigate_SingleID_CacheMiss_LoadsMoreUntilTargetFound(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceResource: ec2Res,
		TargetID:       "alarm-page2-target",
	})

	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-page1-other", Name: "page1-other", Status: "ok"},
		},
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page-2",
			PageSize:    1,
			TotalHint:   -1,
		},
	})
	m = m2
	if cmd == nil {
		t.Fatal("first page without the exact target should request LoadMore")
	}
	loadMore, ok := cmd().(messages.LoadMore)
	if !ok {
		t.Fatalf("expected LoadMoreMsg after page-1 miss, got %T", cmd())
	}
	if loadMore.ContinuationToken != "page-2" {
		t.Fatalf("LoadMoreMsg continuation token = %q, want %q", loadMore.ContinuationToken, "page-2")
	}

	m2, cmd = relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-page2-target", Name: "page2-target", Status: "alarm"},
			{ID: "alarm-page2-other", Name: "page2-other", Status: "ok"},
		},
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    2,
			TotalHint:   3,
		},
		Append: true,
	})
	m = m2
	if cmd != nil {
		if follow := cmd(); follow != nil {
			m, _ = relatedApplyMsg(m, follow)
		}
	}

	view := stripAnsi(relatedViewContent(m))
	// alarm has Children[Key="enter"]=alarm_history — auto-open must open
	// the Enter-child, not the generic detail. The test still guards the
	// key property: once the later page yields the target, the user must
	// not be left on a dead-end 1-row list.
	if !strings.Contains(view, "alarm_history") {
		t.Fatalf("exact-ID related navigation should auto-open alarm Enter-child (alarm_history) once a later page contains the target; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Count>1: multiple related resources must be filtered to only those IDs
// ---------------------------------------------------------------------------

// TestApp_008_RelatedNavigate_MultipleIDs_ShowsOnlyThoseResources verifies that
// when a RelatedNavigateMsg arrives with multiple RelatedIDs, the resulting list
// view shows only the matching resources and NOT unrelated ones.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by exact IDs.
func TestApp_008_RelatedNavigate_MultipleIDs_ShowsOnlyThoseResources(t *testing.T) {
	m := newRelatedDemoModel(t)

	alarmResources := []resource.Resource{
		{ID: "alarm-spec008-1", Name: "high-cpu-alarm", Status: "alarm"},
		{ID: "alarm-spec008-2", Name: "status-check-alarm", Status: "ok"},
		{ID: "alarm-spec008-3", Name: "unrelated-alarm", Status: "ok"},
	}
	m = applyRelatedResourcesLoaded(m, "alarm", alarmResources)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType: "alarm",
		RelatedIDs: []string{"alarm-spec008-1", "alarm-spec008-2"},
	})

	view := stripAnsi(relatedViewContent(m))

	if !strings.Contains(view, "high-cpu-alarm") {
		t.Errorf("view must contain related alarm %q; got:\n%s", "high-cpu-alarm", view)
	}
	if !strings.Contains(view, "status-check-alarm") {
		t.Errorf("view must contain related alarm %q; got:\n%s", "status-check-alarm", view)
	}
	if strings.Contains(view, "unrelated-alarm") {
		t.Errorf("view must NOT contain unrelated alarm %q when RelatedIDs are set; got:\n%s", "unrelated-alarm", view)
	}
}

// TestApp_008_RelatedNavigate_MultipleIDs_FrameTitleHasCount verifies that when
// a multi-ID RelatedNavigateMsg is applied, the frame title reflects the count
// of filtered resources.
//
// FAILS AT RUNTIME until handleRelatedNavigate filters by exact IDs.
func TestApp_008_RelatedNavigate_MultipleIDs_FrameTitleHasCount(t *testing.T) {
	m := newRelatedDemoModel(t)

	alarmResources := []resource.Resource{
		{ID: "alarm-count-1", Name: "cpu-alarm", Status: "alarm"},
		{ID: "alarm-count-2", Name: "memory-alarm", Status: "alarm"},
		{ID: "alarm-count-3", Name: "disk-alarm", Status: "ok"},
	}
	m = applyRelatedResourcesLoaded(m, "alarm", alarmResources)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType: "alarm",
		RelatedIDs: []string{"alarm-count-1", "alarm-count-2"},
	})

	view := stripAnsi(relatedViewContent(m))

	// Frame title should indicate count=2 for filtered alarm list
	if !strings.Contains(view, "2") {
		t.Errorf("frame/view should indicate count=2 for filtered alarm list; got:\n%s", view)
	}
}

// TestApp_008_RelatedNavigate_MultipleIDs_LoadMoreStaysConstrained verifies that
// when a cache-hit related list is paginated, loading more keeps the exact-ID
// related subset instead of appending unrelated rows from later pages.
func TestApp_008_RelatedNavigate_MultipleIDs_LoadMoreStaysConstrained(t *testing.T) {
	m := newRelatedDemoModel(t)

	// Prime cache via a real alarm list load so pagination metadata is retained.
	m, _ = relatedApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "alarm",
	})
	m = applyRelatedResourcesLoaded(m, "alarm", []resource.Resource{
		{ID: "alarm-related-1", Name: "related-one", Status: "alarm"},
		{ID: "alarm-unrelated-1", Name: "unrelated-one", Status: "ok"},
	})
	m, _ = relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-related-1", Name: "related-one", Status: "alarm"},
			{ID: "alarm-unrelated-1", Name: "unrelated-one", Status: "ok"},
		},
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page-2",
			PageSize:    2,
			TotalHint:   -1,
		},
	})

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceResource: ec2Res,
		RelatedIDs:     []string{"alarm-related-1", "alarm-related-2"},
	})

	m2, _ := relatedApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-related-2", Name: "related-two", Status: "alarm"},
			{ID: "alarm-unrelated-2", Name: "unrelated-two", Status: "ok"},
		},
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    2,
			TotalHint:   4,
		},
		Append: true,
	})
	m = m2

	view := stripAnsi(relatedViewContent(m))
	if !strings.Contains(view, "related-one") || !strings.Contains(view, "related-two") {
		t.Fatalf("related list should continue to show all matching related IDs after load more; got:\n%s", view)
	}
	if strings.Contains(view, "unrelated-one") || strings.Contains(view, "unrelated-two") {
		t.Fatalf("related list must not leak unrelated rows after load more; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// RelatedCheckResultMsg: count=0 regression guard
// ---------------------------------------------------------------------------

// TestApp_008_RelatedCheckResult_Count0_NoNavigation verifies that a
// RelatedCheckResultMsg with Count=0 does not produce navigation.
//
// PASSES NOW — regression guard.
func TestApp_008_RelatedCheckResult_Count0_NoNavigation(t *testing.T) {
	m := newRelatedDemoModel(t)

	checkMsg := messages.RelatedCheckResult{
		ResourceType: "ec2",
		Result: resource.RelatedCheckResult{
			TargetType:  "tg",
			Count:       0,
			ResourceIDs: nil,
		},
	}
	_, cmd := relatedApplyMsg(m, checkMsg)

	if cmd != nil {
		resultMsg := cmd()
		if _, isNav := resultMsg.(messages.RelatedNavigate); isNav {
			t.Error("RelatedCheckResultMsg with Count=0 must not produce RelatedNavigateMsg")
		}
	}
}
