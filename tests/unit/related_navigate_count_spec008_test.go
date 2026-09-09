package unit_test

// related_navigate_count_spec008_test.go — Spec-008: handleRelatedNavigate
// behavior: the TargetID case opens detail, not a list, and RelatedIDs>1
// creates a filtered list.
//
// A related pivot that narrows to exactly ONE resource must open that
// resource's DETAIL view (fields + related), for EVERY target type, in both
// lanes (TUI and web) — never the target's enter-keyed child view, which
// would make the two lanes diverge since the web lane always renders
// detail. Child views stay reachable by pressing Enter inside the target's
// own list. TestApp_008_RelatedNavigate_SingleID_OpensDrillTarget (tg) and
// TestApp_008_RelatedNavigate_SingleRelatedIDs_CacheMiss_AutoOpensDrillTarget
// (asg) pin this rule.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ---------------------------------------------------------------------------
// Local helpers (unit_test package cannot access unit package internals)
// ---------------------------------------------------------------------------

// relatedApplyMsg sends a message through the tui.Model's Update.
func relatedApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	return tuitest.Step(m, msg)
}

// relatedViewContent returns the stripped content string from View().
func relatedViewContent(m tui.Model) string {
	return m.View().Content
}

// newRelatedDemoModel creates a tui.Model in demo mode, sized for testing.
func newRelatedDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
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
	m, _ = relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: resourceType,
		Resources:    resources,
	})
	return m
}

// applyRelatedFollowUp runs cmd (if any) and feeds its result message(s) back
// through the model, same as the single-message `if cmd != nil { follow :=
// cmd(); m, _ = relatedApplyMsg(m, follow) }` pattern used throughout this
// file — except it is batch-safe: tui.Model.Update has no tea.BatchMsg case,
// so a cmd that resolves to tea.Batch(navigateCmd, otherCmd) (e.g. an
// auto-open Navigate batched alongside a ProbeEnrich task dispatch) would
// otherwise silently drop every sub-message, including the Navigate the test
// is waiting on. Each sub-command is applied in order, one batch level deep —
// deep enough for the auto-open-single-detail batches this file exercises,
// without recursing into unrelated async follow-ups (e.g. a tea.Tick-driven
// flash-clear) the way a generic recursive drain would.
func applyRelatedFollowUp(m tui.Model, cmd tea.Cmd) tui.Model {
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		m, _ = relatedApplyMsg(m, msg)
		return m
	}
	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if subMsg == nil {
			continue
		}
		m, _ = relatedApplyMsg(m, subMsg)
	}
	return m
}

// ---------------------------------------------------------------------------
// Count=1: single related resource should open DETAIL view, not list
// ---------------------------------------------------------------------------

// TestApp_008_RelatedNavigate_SingleID_OpensDrillTarget verifies that when a
// RelatedNavigateMsg arrives with a single TargetID (count=1 path), the model
// opens the target resource's DETAIL view: a related-panel Count=1 pivot
// always lands on detail, even for types like tg that register
// Children[Key="enter"] (tg_health). Child views stay reachable by pressing
// Enter in tg's own list.
func TestApp_008_RelatedNavigate_SingleID_OpensDrillTarget(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	tgRes := resource.Resource{
		ID:     "tg-spec008-single",
		Name:   "my-target-group",
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

	if strings.Contains(view, "tg_health") {
		t.Errorf("RelatedNavigateMsg with TargetID on tg must NOT enter the tg_health child view (2026-07-06 rule: Count=1 pivot always opens detail); got:\n%s", view)
	}
	if strings.Contains(view, "tg(1)") {
		t.Errorf("RelatedNavigateMsg with TargetID=%q must not open a filtered list; got:\n%s", "tg-spec008-single", view)
	}
	if !strings.Contains(view, "detail -- tg-spec008-single") {
		t.Errorf("RelatedNavigateMsg for tg must open the target's DETAIL view; got:\n%s", view)
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
	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "ami",
		Resources: []resource.Resource{
			{ID: "ami-single-1", Name: "ami-single", Fields: map[string]string{"status": "available"}},
			{ID: "ami-other-1", Name: "ami-other", Fields: map[string]string{"status": "available"}},
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
// the right-column path: RelatedIDs with one element must auto-open the target's
// DETAIL view — NOT leave the operator stranded on a 1-row filtered list, and NOT
// the enter-keyed child view even when one is registered. asg registers
// Children[Key="enter"]=asg_activities but the Count=1 pivot must still land
// on asg's own detail.
func TestApp_008_RelatedNavigate_SingleRelatedIDs_CacheMiss_AutoOpensDrillTarget(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	// No preloaded asg cache: this exercises the right-column cache-miss flow.
	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "asg",
		SourceResource: ec2Res,
		RelatedIDs:     []string{"asg-single-1"},
	})

	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "asg",
		Resources: []resource.Resource{
			{ID: "asg-single-1", Name: "asg-single", Fields: map[string]string{"status": "InService"}},
			{ID: "asg-other-1", Name: "asg-other", Fields: map[string]string{"status": "InService"}},
		},
	})
	m = m2
	// asg is issue-capable, so the auto-open Navigate now arrives batched
	// alongside a ProbeEnrich task dispatch (tea.Batch) — drain batch-safe so
	// the Navigate still reaches the model instead of being silently dropped.
	m = applyRelatedFollowUp(m, cmd)

	view := stripAnsi(relatedViewContent(m))
	if strings.Contains(view, "asg_activities") {
		t.Fatalf("single related right-column cache-miss path must NOT auto-open the asg_activities child view (2026-07-06 rule: Count=1 pivot always opens detail); got:\n%s", view)
	}
	if strings.Contains(view, "asg(1/") || strings.Contains(view, "asg(1)") {
		t.Fatalf("single related right-column cache-miss path must not leave user in list view; got:\n%s", view)
	}
	if !strings.Contains(view, "detail -- asg-single-1") {
		t.Fatalf("single related right-column cache-miss path must auto-open the asg DETAIL view; got:\n%s", view)
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
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceResource: ec2Res,
		TargetID:       "alarm-page2-target",
	})

	m2, cmd := relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-page1-other", Name: "page1-other", Fields: map[string]string{"status": "ok"}},
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

	m2, cmd = relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-page2-target", Name: "page2-target", Fields: map[string]string{"status": "alarm"}},
			{ID: "alarm-page2-other", Name: "page2-other", Fields: map[string]string{"status": "ok"}},
		},
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    2,
			TotalHint:   3,
		},
		Append: true,
	})
	m = m2
	m = applyRelatedFollowUp(m, cmd)

	view := stripAnsi(relatedViewContent(m))
	// alarm has Children[Key="enter"]=alarm_history, but a related pivot that
	// narrows to exactly ONE resource always opens that resource's detail
	// view. The load-more mechanics under test are unchanged: once the later
	// page yields the target, the user must not be left on a dead-end 1-row
	// list.
	if strings.Contains(view, "alarm_history") {
		t.Fatalf("exact-ID related navigation must NOT auto-open alarm_history (2026-07-06 rule: Count=1 pivot always opens detail); got:\n%s", view)
	}
	if !strings.Contains(view, "detail -- alarm-page2-target") {
		t.Fatalf("exact-ID related navigation should auto-open the alarm DETAIL view once a later page contains the target; got:\n%s", view)
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
		{ID: "alarm-spec008-1", Name: "high-cpu-alarm", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-spec008-2", Name: "status-check-alarm", Fields: map[string]string{"status": "ok"}},
		{ID: "alarm-spec008-3", Name: "unrelated-alarm", Fields: map[string]string{"status": "ok"}},
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
		{ID: "alarm-count-1", Name: "cpu-alarm", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-count-2", Name: "memory-alarm", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-count-3", Name: "disk-alarm", Fields: map[string]string{"status": "ok"}},
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
		{ID: "alarm-related-1", Name: "related-one", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-unrelated-1", Name: "unrelated-one", Fields: map[string]string{"status": "ok"}},
	})
	m, _ = relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-related-1", Name: "related-one", Fields: map[string]string{"status": "alarm"}},
			{ID: "alarm-unrelated-1", Name: "unrelated-one", Fields: map[string]string{"status": "ok"}},
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
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceResource: ec2Res,
		RelatedIDs:     []string{"alarm-related-1", "alarm-related-2"},
	})

	m2, _ := relatedApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceFilteredList,
		ResourceType: "alarm",
		Resources: []resource.Resource{
			{ID: "alarm-related-2", Name: "related-two", Fields: map[string]string{"status": "alarm"}},
			{ID: "alarm-unrelated-2", Name: "unrelated-two", Fields: map[string]string{"status": "ok"}},
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
		Result:       resource.KnownRelated("tg", nil, false),
	}
	_, cmd := relatedApplyMsg(m, checkMsg)

	if cmd != nil {
		resultMsg := cmd()
		if _, isNav := resultMsg.(messages.RelatedNavigate); isNav {
			t.Error("RelatedCheckResultMsg with Count=0 must not produce RelatedNavigateMsg")
		}
	}
}

// ---------------------------------------------------------------------------
// Count=1 drill rule: detail for every target type, including childless
// ones. Parameterized over three enter-child types (s3, tg, asg) plus one
// childless type (kms): with no Children[Key="enter"] to redirect through,
// kms lands on detail without the rule and must continue to do so.
// ---------------------------------------------------------------------------

func TestApp_008_RelatedNavigate_CountOne_AlwaysOpensDetail(t *testing.T) {
	cases := []struct {
		name       string
		targetType string
		res        resource.Resource
	}{
		{
			name:       "s3_has_enter_child",
			targetType: "s3",
			res: resource.Resource{
				ID:     "a9s-drill-rule-bucket",
				Name:   "a9s-drill-rule-bucket",
				Fields: map[string]string{"name": "a9s-drill-rule-bucket"},
			},
		},
		{
			name:       "tg_has_enter_child",
			targetType: "tg",
			res: resource.Resource{
				ID:     "tg-drill-rule-single",
				Name:   "drill-rule-target-group",
				Fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/drill-rule-target-group/def456"},
			},
		},
		{
			name:       "asg_has_enter_child",
			targetType: "asg",
			res: resource.Resource{
				ID:     "asg-drill-rule-single",
				Name:   "drill-rule-asg",
				Fields: map[string]string{"status": "InService"},
			},
		},
		{
			name:       "kms_is_childless",
			targetType: "kms",
			res: resource.Resource{
				ID:     "arn:aws:kms:us-east-1:123456789012:key/drill-rule-key",
				Name:   "drill-rule-key",
				Fields: map[string]string{"status": "Enabled"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newRelatedDemoModel(t)

			ec2Res := resource.Resource{
				ID:     "i-0a1b2c3d4e5f60001",
				Name:   "web-prod-01",
				Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
			}
			m = navigateToEC2DetailRelated(t, m, ec2Res)

			// Prime the target-type cache so RelatedNavigate takes the
			// NavigationKindDetail cache-hit branch.
			m = applyRelatedResourcesLoaded(m, tc.targetType, []resource.Resource{tc.res})

			m, cmd := relatedApplyMsg(m, messages.RelatedNavigate{
				TargetType:     tc.targetType,
				SourceResource: ec2Res,
				RelatedIDs:     []string{tc.res.ID},
			})
			m = applyRelatedFollowUp(m, cmd)

			view := stripAnsi(relatedViewContent(m))
			if !strings.Contains(view, "detail -- "+tc.res.ID) {
				t.Errorf("Count=1 related pivot to %s must open the target's DETAIL view; got:\n%s", tc.targetType, view)
			}
		})
	}
}
