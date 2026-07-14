package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
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
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "web-prod-01",
		Fields: map[string]string{
			"InstanceId": "i-0a1b2c3d4e5f60001",
			"VpcId":      "vpc-0abc123def456789a",
			"SubnetId":   "subnet-0aaa111111111111a",
			"ImageId":    "ami-0abc123def456789a",
			"status":     "running",
		},
	}
}

// TestPreview_RightColumnFilter_HidesNonMatchingRows (DetailModel.View()) and
// TestPreview_DetailCopyContent_CopiesCurrentFieldValue (DetailModel.
// CopyContent()) deleted (round 4, specs/022-codebase-cleanup, MIXED verdict:
// delete Detail .View()/.CopyContent halves). Live equivalents:
// TestBug_Root_RightColumnFilter_TypingFiltersRows (rightcolumn_root_filter_
// regression_test.go — same right-column filter narrows/hides contract, on
// the live tui.New() root path) and TestQA_Copy_Detail_CopiesFieldValue
// (qa_copy_test.go — same "c copies the active field's value" contract, on
// the live tui.New() root path).
//
// TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter below still drives
// DetailModel.Update() directly (not .View()/.CopyContent) — out of this
// item's literal scope; kept as-is and flagged as a residual gap.

func TestPreview_RightColumnTabFocus_SkipsDimRowsOnEnter(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	// tg=0 (dim), asg=2 (available), others dim.
	for _, msg := range []messages.RelatedCheckResult{
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
	nav, ok := msg.(messages.RelatedNavigate)
	if !ok {
		t.Fatalf("expected RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "asg" {
		t.Errorf("focused right column should skip dim rows and land on asg; got target %q", nav.TargetType)
	}
}

func TestPreview_RightColumnScroll_KeepsDeepCursorRowVisible(t *testing.T) {
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })

	defs := make([]resource.RelatedDef, 0, 20)
	for i := 1; i <= 20; i++ {
		target := fmt.Sprintf("t%02d", i)
		defs = append(defs, resource.RelatedDef{
			TargetType:  target,
			DisplayName: fmt.Sprintf("Type %02d", i),
			Checker:     resource.NoopChecker,
		})
	}
	resource.SetRelatedForTest("ec2", defs)

	m := newPreviewDemoModel(t, 120, 8)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
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
	m, _ = previewApplyMsg(m, messages.Navigate{
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
