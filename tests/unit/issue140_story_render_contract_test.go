package unit_test

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// Issue #140 / docs/qa/ec2-related-navigation-stories.md
// Render-contract style coverage for key EC2 QA stories.

func TestIssue140_Story_EC2_001_InitialDetailRenderContract(t *testing.T) {
	d := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(d.View())
	lines := strings.Split(plain, "\n")
	if len(lines) == 0 {
		t.Fatal("empty detail view")
	}

	if !strings.Contains(lines[0], "InstanceId:") {
		t.Fatalf("EC2-001: first row must be InstanceId; got: %q", lines[0])
	}

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m = m2.(tui.Model)
	ec2 := mustDemoEC2(t)
	m2, _ = m.Update(messages.NavigateMsg{Target: messages.TargetDetail, ResourceType: "ec2", Resource: &ec2[0]})
	m = m2.(tui.Model)
	frame := stripAnsi(m.View().Content)

	if !strings.Contains(frame, "detail --") {
		t.Fatalf("EC2-001: frame title must include 'detail --'; got:\n%s", frame)
	}
	if !strings.Contains(frame, ec2[0].ID) {
		t.Fatalf("EC2-001: frame title must include id %q; got:\n%s", ec2[0].ID, frame)
	}
}

func TestIssue140_Story_EC2_017_UnderlineVisibilityOnNavigableRow(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(styles.Reinit)
	withIssue140EC2RelatedDefs(t)

	d := makePreviewEC2Detail(t, 120, 35)
	viewBefore := d.View()

	lineBefore := findLineContaining(viewBefore, "VpcId:")
	if lineBefore == "" {
		t.Fatalf("EC2-017: could not find VpcId row before selection\n%s", stripAnsi(viewBefore))
	}
	if !strings.Contains(lineBefore, "\x1b[4") {
		t.Fatalf("EC2-017: VpcId should be underlined when not selected; line=%q", lineBefore)
	}

	// Move until the selected row is VpcId.
	foundSelectedVpc := false
	for range 80 {
		sel := findSelectedLine(d.View())
		if strings.Contains(sel, "VpcId:") {
			foundSelectedVpc = true
			break
		}
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}
	if !foundSelectedVpc {
		t.Fatalf("EC2-017: could not move selection to VpcId row\n%s", stripAnsi(d.View()))
	}
	viewSelected := d.View()
	lineSelected := findSelectedLine(viewSelected)
	if lineSelected == "" {
		t.Fatalf("EC2-017: could not find VpcId row when selected\n%s", stripAnsi(viewSelected))
	}
	if !strings.Contains(lineSelected, "VpcId:") {
		t.Fatalf("EC2-017: selected line should be VpcId row; line=%q", lineSelected)
	}
	if strings.Contains(lineSelected, "\x1b[4;") || strings.Contains(lineSelected, "\x1b[4m") {
		t.Fatalf("EC2-017: underline should disappear when row is selected; line=%q", lineSelected)
	}
}

func TestIssue140_Story_EC2_018_RightColumnTypeSetContract(t *testing.T) {
	withIssue140EC2RelatedDefs(t)
	d := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(d.View())

	mustContain := []string{
		"RELATED",
		"Target Groups",
		"Auto Scaling Groups",
		"CloudWatch Alarms",
		"EKS Node Groups",
		"CloudFormation Stacks",
		"Elastic Beanstalk",
		"EBS Snapshots",
		"Elastic IPs",
		"CloudTrail Events",
	}
	for _, want := range mustContain {
		if !strings.Contains(plain, want) {
			t.Fatalf("EC2-018: expected right column to include %q; got:\n%s", want, plain)
		}
	}

	mustNotContain := []string{"VpcId", "SubnetId", "SecurityGroups", "ImageId"}
	for _, bad := range mustNotContain {
		if strings.Contains(plain, bad+" (") {
			t.Fatalf("EC2-018: right column must not include forward field relationship %q; got:\n%s", bad, plain)
		}
	}
}

func TestIssue140_Story_EC2_020_CountsRenderAsResultsArrive(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	d = deliverRelatedResult(d, "asg", 1)
	d = deliverRelatedResult(d, "alarm", 2)
	d = deliverRelatedResult(d, "tg", 0)
	plain := stripAnsi(d.View())

	if !strings.Contains(plain, "Auto Scaling Groups (1)") {
		t.Fatalf("EC2-020: expected 'Auto Scaling Groups (1)' after count update; got:\n%s", plain)
	}
	if !strings.Contains(plain, "CloudWatch Alarms (2)") {
		t.Fatalf("EC2-020: expected 'CloudWatch Alarms (2)' after count update; got:\n%s", plain)
	}
	if !strings.Contains(plain, "Target Groups (0)") {
		t.Fatalf("EC2-020: expected zero-count row marker '(0)' for Target Groups; got:\n%s", plain)
	}
}

func TestIssue140_Story_EC2_023_ToggleRightColumnRenderContract(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	before := stripAnsi(d.View())
	if !strings.Contains(before, "RELATED") {
		t.Fatalf("EC2-023: precondition failed, expected RELATED panel visible; got:\n%s", before)
	}

	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	hidden := stripAnsi(d.View())
	if strings.Contains(hidden, "RELATED") {
		t.Fatalf("EC2-023: first r press should hide right column; got:\n%s", hidden)
	}

	d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "r"})
	restored := stripAnsi(d.View())
	if !strings.Contains(restored, "RELATED") {
		t.Fatalf("EC2-023: pressing r again should restore right column; got:\n%s", restored)
	}
}

func TestIssue140_Story_EC2_021_TabFocusMovesToFirstAvailableRightRow(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	// Make first row dim/unavailable and second row available.
	d = deliverRelatedResult(d, "tg", 0)
	d = deliverRelatedResult(d, "asg", 1)
	d = deliverRelatedResult(d, "alarm", 0)
	d = deliverRelatedResult(d, "cfn", 0)

	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("EC2-021: pressing Enter on right-focused column should emit RelatedNavigateMsg")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EC2-021: expected RelatedNavigateMsg after Enter on right column, got %T", msg)
	}
	if nav.TargetType != "asg" {
		t.Fatalf("EC2-021: right-column focus should land on first available row (asg), got %q", nav.TargetType)
	}
}

func TestIssue140_Story_EC2_033_DimRowsAreSkippedInRightColumn(t *testing.T) {
	d, cleanup := ec2StoryDetail(t, 120, 30, true)
	defer cleanup()

	// Only alarm row is actionable; dim rows should be skipped by right-column cursor.
	d = deliverRelatedResult(d, "tg", 0)
	d = deliverRelatedResult(d, "asg", 0)
	d = deliverRelatedResult(d, "alarm", 2)
	d = deliverRelatedResult(d, "cfn", 0)

	d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("EC2-033: Enter on right column should navigate to first non-dim row")
	}
	msg := cmd()
	nav, ok := msg.(messages.RelatedNavigateMsg)
	if !ok {
		t.Fatalf("EC2-033: expected RelatedNavigateMsg, got %T", msg)
	}
	if nav.TargetType != "alarm" {
		t.Fatalf("EC2-033: cursor should skip dim rows and navigate to alarm, got %q", nav.TargetType)
	}
}

func TestIssue140_Story_EC2_029_FilteredAlarmListTitleAndScope(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = m2.(tui.Model)

	alarms := []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Status: "alarm"},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Status: "ok"},
		{ID: "unrelated-alarm", Name: "unrelated-alarm", Status: "ok"},
	}
	m2, _ = m.Update(messages.ResourcesLoadedMsg{ResourceType: "alarm", Resources: alarms})
	m = m2.(tui.Model)

	source := resource.Resource{ID: "i-0a1b2c3d4e5f60001", Name: "web-prod-01"}
	m2, _ = m.Update(messages.RelatedNavigateMsg{
		TargetType:     "alarm",
		SourceType:     "ec2",
		SourceResource: source,
		RelatedIDs:     []string{"web-prod-cpu-high", "web-prod-status-check"},
	})
	m = m2.(tui.Model)
	plain := stripAnsi(m.View().Content)

	if !strings.Contains(plain, "alarms(2)") {
		t.Fatalf("EC2-029: filtered alarm list title must include count=2; got:\n%s", plain)
	}
	if !strings.Contains(plain, source.ID) || !strings.Contains(plain, source.Name) {
		t.Fatalf("EC2-029: list title must include source context '%s (%s)'; got:\n%s", source.ID, source.Name, plain)
	}
	if !strings.Contains(plain, "web-prod-cpu-high") || !strings.Contains(plain, "web-prod-status-check") {
		t.Fatalf("EC2-029: expected both related alarms in filtered list; got:\n%s", plain)
	}
	if strings.Contains(plain, "unrelated-alarm") {
		t.Fatalf("EC2-029: filtered list must not include unrelated alarms; got:\n%s", plain)
	}
}

func withIssue140EC2RelatedDefs(t *testing.T) {
	t.Helper()
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	oldNav := append([]resource.NavigableField(nil), resource.GetActiveNavigableFields("ec2")...)
	resource.RegisterRelated("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: noopChecker},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: noopChecker},
		{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: noopChecker},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: noopChecker},
		{TargetType: "eb", DisplayName: "Elastic Beanstalk", Checker: noopChecker},
		{TargetType: "eip", DisplayName: "Elastic IPs", Checker: noopChecker},
		{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: noopChecker},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: noopChecker},
	})
	resource.RegisterNavigableFields("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
		{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
	})
	t.Cleanup(func() {
		resource.RegisterRelated("ec2", oldDefs)
		if len(oldNav) == 0 {
			resource.UnregisterNavigableFields("ec2")
		} else {
			resource.RegisterNavigableFields("ec2", oldNav)
		}
	})
}

func findLineContaining(view, needle string) string {
	for ln := range strings.SplitSeq(view, "\n") {
		if strings.Contains(ln, needle) {
			return ln
		}
	}
	return ""
}

func findSelectedLine(view string) string {
	for ln := range strings.SplitSeq(view, "\n") {
		if strings.Contains(ln, "\x1b[48;") {
			return ln
		}
	}
	return ""
}
