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
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = relatedApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

// navigateToEC2DetailRelated navigates the given model to the EC2 detail view.
func navigateToEC2DetailRelated(t *testing.T, m tui.Model, res resource.Resource) tui.Model {
	t.Helper()
	m, _ = relatedApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &res,
	})
	return m
}

// applyRelatedResourcesLoaded delivers a ResourcesLoadedMsg for the given type.
func applyRelatedResourcesLoaded(m tui.Model, resourceType string, resources []resource.Resource) tui.Model {
	m, _ = relatedApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: resourceType,
		Resources:    resources,
	})
	return m
}

// ---------------------------------------------------------------------------
// Count=1: single related resource should open DETAIL view, not list
// ---------------------------------------------------------------------------

// TestApp_008_RelatedNavigate_SingleID_OpensDetail verifies that when a
// RelatedNavigateMsg arrives with a single TargetID (count=1 path), the model
// pushes a detail view for that resource rather than a filtered list.
//
// FAILS AT RUNTIME until handleRelatedNavigate pushes TargetDetail for single IDs.
func TestApp_008_RelatedNavigate_SingleID_OpensDetail(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	tgRes := resource.Resource{ID: "tg-spec008-single", Name: "my-target-group", Status: "active"}
	m = applyRelatedResourcesLoaded(m, "tg", []resource.Resource{tgRes})

	// Deliver RelatedNavigateMsg with TargetID set (single resource navigation).
	m, _ = relatedApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "tg",
		TargetID:   "tg-spec008-single",
	})

	view := stripAnsi(relatedViewContent(m))

	if !strings.Contains(view, "my-target-group") {
		t.Errorf("RelatedNavigateMsg with TargetID should open detail view for %q, got view:\n%s", "my-target-group", view)
	}
	if strings.Contains(view, "tg(1)") {
		t.Errorf("RelatedNavigateMsg with TargetID=%q must open DETAIL, not filtered list; got:\n%s", "tg-spec008-single", view)
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

	m, _ = relatedApplyMsg(m, messages.RelatedNavigateMsg{
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

	m, _ = relatedApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType: "alarm",
		RelatedIDs: []string{"alarm-count-1", "alarm-count-2"},
	})

	view := stripAnsi(relatedViewContent(m))

	// Frame title should indicate count=2 for filtered alarm list
	if !strings.Contains(view, "2") {
		t.Errorf("frame/view should indicate count=2 for filtered alarm list; got:\n%s", view)
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

	checkMsg := messages.RelatedCheckResultMsg{
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
		if _, isNav := resultMsg.(messages.RelatedNavigateMsg); isNav {
			t.Error("RelatedCheckResultMsg with Count=0 must not produce RelatedNavigateMsg")
		}
	}
}
