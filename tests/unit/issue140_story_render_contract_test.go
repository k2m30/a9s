package unit_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// Issue #140 / docs/qa/ec2-related-navigation-stories.md
// Render-contract style coverage for key EC2 QA stories.
//
// TestIssue140_Story_EC2_001_InitialDetailRenderContract and
// TestIssue140_Story_EC2_020_CountsRenderAsResultsArrive deleted (round 5,
// specs/022-codebase-cleanup, DetailModel core cleanup): both duplicate
// issue140_scenarios_golden_test.go's CI-verified golden scenarios
// ec2_001_initial_detail / ec2_020_counts_arrived on the live tui.New() root
// path — same fixture, same assertions, same resource IDs/counts.

func TestIssue140_Story_EC2_017_UnderlineVisibilityOnNavigableRow(t *testing.T) {
	tuitest.ForceColor(t)
	withIssue140EC2RelatedDefs(t)

	c := makePreviewEC2Detail(t, 120, 35)
	viewBefore := previewDetailView(t, c, 120, 35)

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
		sel := findSelectedLine(previewDetailView(t, c, 120, 35))
		if strings.Contains(sel, "VpcId:") {
			foundSelectedVpc = true
			break
		}
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	if !foundSelectedVpc {
		t.Fatalf("EC2-017: could not move selection to VpcId row\n%s", stripAnsi(previewDetailView(t, c, 120, 35)))
	}
	viewSelected := previewDetailView(t, c, 120, 35)
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
	c := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(previewDetailView(t, c, 120, 35))

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

// TestIssue140_Story_EC2_023_ToggleRightColumnRenderContract drives the real
// root tui.Model "r" keypress (not raw Controller.Apply(ActionToggleRelated)):
// the TUI adapter syncs the renderer's auto-show state into the controller
// before dispatching the toggle, a step a bare Controller.Apply call skips.
func TestIssue140_Story_EC2_023_ToggleRightColumnRenderContract(t *testing.T) {
	m := newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	before := previewView(m)
	if !strings.Contains(before, "RELATED") {
		t.Fatalf("EC2-023: precondition failed, expected RELATED panel visible; got:\n%s", before)
	}

	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	hidden := previewView(m)
	if strings.Contains(hidden, "RELATED") {
		t.Fatalf("EC2-023: first r press should hide right column; got:\n%s", hidden)
	}

	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	restored := previewView(m)
	if !strings.Contains(restored, "RELATED") {
		t.Fatalf("EC2-023: pressing r again should restore right column; got:\n%s", restored)
	}
}

// issue140SetupRightColumnFocus registers a 4-def "tg/asg/alarm/cfn" related
// set for ec2, opens the detail screen for previewEC2Resource, delivers the
// given counts, then presses Tab (right-column focus) and Enter — the live
// replacement for the retired views.NewDetail(...).Update(RelatedCheckResult)
// .Update(KeyTab).Update(KeyEnter) chain shared by EC2-021/EC2-033.
func issue140SetupRightColumnFocus(t *testing.T, counts map[string]int) (m tui.Model, cmd tea.Cmd) {
	t.Helper()
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	t.Cleanup(func() { resource.SetRelatedForTest("ec2", oldDefs) })
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: resource.NoopCheckerForTest},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: resource.NoopCheckerForTest},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: resource.NoopCheckerForTest},
	})

	m = newPreviewDemoModel(t, 120, 30)
	ec2Res := previewEC2Resource()
	m, _ = previewApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})
	// SourceResourceID and OperationID must be set — a compliant adapter
	// drops any RelatedCheckResult missing the source ID or carrying a
	// stale operation id. activeOp is the live DetailOperation the Navigate
	// above just began (read via the Core accessor rather than hardcoding a
	// value — no refresh/profile/region switch happens between Navigate and
	// this loop, so every result below is current relative to it).
	// ResourceIDs must also match Count — a real RelatedChecker always
	// returns exactly Count IDs; a Count>0 result with no IDs is a data
	// shape production never produces, and downstream
	// (runtime_adapter_related.go's NavigationKindFilteredList branch) keys
	// its title-suffix/pendingFilter wiring off len(RelatedIDs), not Count
	// alone.
	activeOp := m.Core().ActiveDetailOp()
	for _, target := range []string{"tg", "asg", "alarm", "cfn"} {
		ids := make([]string, counts[target])
		for i := range ids {
			ids[i] = fmt.Sprintf("%s-%d", target, i)
		}
		m, _ = previewApplyMsg(m, messages.RelatedCheckResult{
			ResourceType:     "ec2",
			SourceResourceID: ec2Res.ID,
			OperationID:      activeOp,
			Result:           resource.KnownRelated(target, ids, false),
		})
	}

	m, _ = previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m, cmd = previewApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m, cmd
}

func TestIssue140_Story_EC2_021_TabFocusMovesToFirstAvailableRightRow(t *testing.T) {
	// Make first row dim/unavailable and second row available.
	m, cmd := issue140SetupRightColumnFocus(t, map[string]int{"tg": 0, "asg": 1, "alarm": 0, "cfn": 0})
	if cmd == nil {
		t.Fatalf("EC2-021: pressing Enter on right-focused column should dispatch a navigation command")
	}
	// The Enter cmd resolves to messages.RelatedNavigate, which itself
	// dispatches the demo fetch as a further tea.Cmd — one more
	// previewApplyMsg round-trip resolves it (tui.Model.Update's own
	// tea.BatchMsg case, internal/tui/app.go, drains every sub-command) so
	// the title's TitleSuffix breadcrumb is actually rendered (buildListFrameTitle
	// returns the bare type name while ls.Loading is still true).
	m, cmd = previewApplyMsg(m, cmd())
	if cmd != nil {
		m, _ = previewApplyMsg(m, cmd())
	}

	view := previewView(m)
	// A bare Contains(view, "asg"/asgRT.ListTitle) can pass without real
	// navigation (the EC2 detail's own RELATED panel already renders the
	// asg row's display name). Require the source-scoped title breadcrumb
	// runtime.RelatedTitleSuffix produces — unique to the navigated-to
	// screen — plus proof the EC2 detail's RELATED panel is actually gone.
	wantSuffix := runtime.RelatedTitleSuffix(previewEC2Resource())
	if !strings.Contains(view, wantSuffix) {
		t.Errorf("EC2-021: right-column focus should land on first available row (asg), showing title suffix %q; got:\n%s", wantSuffix, view)
	}
	if strings.Contains(view, "RELATED") {
		t.Errorf("EC2-021: navigating to asg should leave the EC2 detail's RELATED panel behind; got:\n%s", view)
	}
	// The suffix + RELATED-gone checks above prove SOME navigation happened;
	// they don't prove it landed on asg specifically rather than another
	// child type sharing the same suffix format. Require the destination's
	// own frame-title type marker too (asg has no catalog ListTitle override,
	// core/aws/catalog_compute.go, so buildListFrameTitle falls back to
	// ShortName "asg").
	if wantMarker := relatedNavListTitleMarker(t, "asg"); !strings.Contains(view, wantMarker) {
		t.Errorf("EC2-021: expected destination frame-title marker %q for asg; got:\n%s", wantMarker, view)
	}
}

func TestIssue140_Story_EC2_033_DimRowsAreSkippedInRightColumn(t *testing.T) {
	// Only alarm row is actionable; dim rows should be skipped by right-column cursor.
	m, cmd := issue140SetupRightColumnFocus(t, map[string]int{"tg": 0, "asg": 0, "alarm": 2, "cfn": 0})
	if cmd == nil {
		t.Fatalf("EC2-033: Enter on right column should navigate to first non-dim row")
	}
	// See EC2-021 above: one more drain round-trip is needed for the
	// TitleSuffix breadcrumb to actually render (not just "Loading...").
	m, cmd = previewApplyMsg(m, cmd())
	if cmd != nil {
		m, _ = previewApplyMsg(m, cmd())
	}

	view := previewView(m)
	// Same collision risk as EC2-021 above: require the source-scoped title
	// breadcrumb plus proof the RELATED panel is gone, not a bare name match.
	wantSuffix := runtime.RelatedTitleSuffix(previewEC2Resource())
	if !strings.Contains(view, wantSuffix) {
		t.Errorf("EC2-033: cursor should skip dim rows and land on alarm, showing title suffix %q; got:\n%s", wantSuffix, view)
	}
	if strings.Contains(view, "RELATED") {
		t.Errorf("EC2-033: navigating to alarm should leave the EC2 detail's RELATED panel behind; got:\n%s", view)
	}
	// Destination-type proof, same reasoning as EC2-021: the suffix format
	// is identical for every child type, so also require alarm's own
	// frame-title marker (alarm DOES have a catalog ListTitle override,
	// core/aws/catalog_monitoring.go: ListTitle "alarms").
	if wantMarker := relatedNavListTitleMarker(t, "alarm"); !strings.Contains(view, wantMarker) {
		t.Errorf("EC2-033: expected destination frame-title marker %q for alarm; got:\n%s", wantMarker, view)
	}
}

// relatedNavListTitleMarker returns the frame-title type marker
// buildListFrameTitle (core/app/list_body.go) renders for typeName: its
// catalog ListTitle override when set, else its bare ShortName — the same
// fallback production uses, so the returned marker matches what a real
// related navigation to typeName actually shows regardless of whether a
// ListTitle override exists.
func relatedNavListTitleMarker(t *testing.T, typeName string) string {
	t.Helper()
	td := resource.FindResourceType(typeName)
	if td == nil {
		t.Fatalf("resource type %q not registered", typeName)
	}
	marker := td.ShortName
	if td.ListTitle != "" {
		marker = td.ListTitle
	}
	return marker + "("
}

func TestIssue140_Story_EC2_029_FilteredAlarmListTitleAndScope(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = m2.(tui.Model)

	alarms := []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Fields: map[string]string{"status": "alarm"}},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Fields: map[string]string{"status": "ok"}},
		{ID: "unrelated-alarm", Name: "unrelated-alarm", Fields: map[string]string{"status": "ok"}},
	}
	m2, _ = m.Update(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "alarm", Resources: alarms})
	m = m2.(tui.Model)

	source := resource.Resource{ID: "i-0a1b2c3d4e5f60001", Name: "web-prod-01"}
	m2, _ = m.Update(messages.RelatedNavigate{
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
	resource.SetRelatedForTest("ec2", []resource.RelatedDef{
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
	resource.SetNavigableFieldsForTest("ec2", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "SubnetId", TargetType: "subnet"},
		{FieldPath: "ImageId", TargetType: "ami"},
		{FieldPath: "SecurityGroups.GroupId", TargetType: "sg"},
	})
	t.Cleanup(func() {
		resource.SetRelatedForTest("ec2", oldDefs)
		if len(oldNav) == 0 {
			resource.CleanupNavigableFieldsForTest("ec2")
		} else {
			resource.SetNavigableFieldsForTest("ec2", oldNav)
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
