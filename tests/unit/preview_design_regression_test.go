package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func previewApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

func previewView(m tui.Model) string {
	return stripAnsi(m.View().Content)
}

func newPreviewDemoModel(t *testing.T, w, h int) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = previewApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func previewEC2Resource() resource.Resource {
	return resource.Resource{
		ID:     "i-0a1b2c3d4e5f60001",
		Name:   "web-prod-01",
		Status: "running",
		Fields: map[string]string{
			"InstanceId": "i-0a1b2c3d4e5f60001",
			"VpcId":      "vpc-0abc123def456789a",
			"SubnetId":   "subnet-0aaa111111111111a",
			"ImageId":    "ami-0abc123def456789a",
		},
	}
}

func TestPreview_RightColumnFilter_HidesNonMatchingRows(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	if !strings.Contains(stripAnsi(d.View()), "Auto Scaling Groups") {
		t.Fatal("precondition failed: expected Auto Scaling Groups row")
	}

	// Focus right column, then apply /cloud filter.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	for _, ch := range []string{"/", "c", "l", "o", "u", "d"} {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: ch})
	}
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	view := stripAnsi(d.View())
	if strings.Contains(view, "Auto Scaling Groups") {
		t.Errorf("after /cloud filter in right column, non-matching type must be hidden; got:\n%s", view)
	}
	if !strings.Contains(view, "CloudWatch Alarms") {
		t.Errorf("after /cloud filter, cloud-related rows should remain visible; got:\n%s", view)
	}
}

func TestPreview_DetailCopyContent_CopiesCurrentFieldValue(t *testing.T) {
	d, cleanup := ec2StoryDetailWithConfig(t, 120, 30, true)
	defer cleanup()

	content, _ := d.CopyContent()
	if content != "i-0a1b2c3d4e5f60001" {
		t.Errorf("detail CopyContent should copy the active field value; got %q", content)
	}
}

func TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	// tg=0 (dim), asg=2 (available), others dim.
	for _, msg := range []messages.RelatedCheckResultMsg{
		{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "tg", Count: 0}},
		{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "asg", Count: 2, ResourceIDs: []string{"asg-1", "asg-2"}}},
		{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "alarm", Count: 0}},
		{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "cfn", Count: 0}},
	} {
		d, _ = d.Update(msg)
	}

	// Focus right column and press Enter. Expected: first actionable row (asg) is selected.
	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on focused right column should emit RelatedNavigateMsg for first non-dim row")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("expected RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "asg" {
		t.Errorf("focused right column should skip dim rows and land on asg; got target %q", nav.TargetType)
	}
}

func TestPreview_RightColumnScroll_KeepsDeepCursorRowVisible(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.RegisterRelated("ec2", oldDefs) })

	defs := make([]resource.RelatedDef, 0, 20)
	for i := 1; i <= 20; i++ {
		target := fmt.Sprintf("t%02d", i)
		defs = append(defs, resource.RelatedDef{
			TargetType:  target,
			DisplayName: fmt.Sprintf("Type %02d", i),
			Checker:     resource.NoopChecker,
		})
	}
	resource.RegisterRelated("ec2", defs)

	m := newPreviewDemoModel(t, 120, 8)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	// Focus right column and move cursor deep into the list.
	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	for range 14 {
		m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	}

	view := previewView(m)
	if !strings.Contains(view, "Type 15") {
		t.Errorf("right column should scroll to keep focused deep row visible (expected Type 15); got:\n%s", view)
	}
}

func TestPreview_DetailHelp_IncludesRelatedFilterAndCopyValue(t *testing.T) {
	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: '?', Text: "?"})
	view := previewView(m)

	if !strings.Contains(view, "filter list") {
		t.Errorf("detail help should include related-column '/ Filter list' binding; got:\n%s", view)
	}
	if !strings.Contains(view, "copy value") {
		t.Errorf("detail help should describe c as 'copy value'; got:\n%s", view)
	}
	if !strings.Contains(view, "RELATED") {
		t.Errorf("detail help should include RELATED section; got:\n%s", view)
	}
}

// compile-time guard: keep this file in external-test package where stripAnsi helper is available.
var _ = views.DetailModel{}
