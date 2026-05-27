package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// TestIssue119_StackedWidth_ToggleRelated verifies that at 80-99 columns
// (stacked mode), pressing r still toggles the related panel off and on.
func TestIssue119_StackedWidth_ToggleRelated(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 85, 30, true)
	defer cleanup()

	before := stripAnsi(d.View())
	if !strings.Contains(before, "RELATED") {
		t.Fatalf("precondition failed: expected RELATED panel to be visible at width=85; got:\n%s", before)
	}

	d, _ = pressDetailKey(d, "r")
	afterHide := stripAnsi(d.View())
	if strings.Contains(afterHide, "RELATED") {
		t.Errorf("at width=85, first r press should hide related panel; got:\n%s", afterHide)
	}

	d, _ = pressDetailKey(d, "r")
	afterShow := stripAnsi(d.View())
	if !strings.Contains(afterShow, "RELATED") {
		t.Errorf("at width=85, second r press should show related panel again; got:\n%s", afterShow)
	}
}

// TestIssue140_RelatedListTitle_ContainsSourceContext verifies that count>1
// related navigation includes the source EC2 identifier in the destination list title.
func TestIssue140_RelatedListTitle_ContainsSourceContext(t *testing.T) {
	m := newRelatedDemoModel(t)

	ec2Res := resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Fields: map[string]string{"instance_id": "i-0a1b2c3d4e5f60001", "status": "running"},
	}
	m = navigateToEC2DetailRelated(t, m, ec2Res)

	alarmResources := []resource.Resource{
		{ID: "alarm-title-1", Name: "high-cpu-alarm", Fields: map[string]string{"status": "alarm"}},
		{ID: "alarm-title-2", Name: "status-check-alarm", Fields: map[string]string{"status": "ok"}},
		{ID: "alarm-title-3", Name: "unrelated-alarm", Fields: map[string]string{"status": "ok"}},
	}
	m = applyRelatedResourcesLoaded(m, "alarm", alarmResources)

	m, _ = relatedApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "alarm",
		SourceType:     "ec2",
		SourceResource: ec2Res,
		RelatedIDs:     []string{"alarm-title-1", "alarm-title-2"},
	})

	view := stripAnsi(relatedViewContent(m))

	if !strings.Contains(view, "alarms(2)") {
		t.Fatalf("precondition failed: expected filtered alarm list count in title, got:\n%s", view)
	}
	if !strings.Contains(view, ec2Res.ID) {
		t.Errorf("related list title must include source EC2 id %q; got:\n%s", ec2Res.ID, view)
	}
}
