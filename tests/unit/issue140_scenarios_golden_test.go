package unit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// Scenario-driven golden snapshots for Issue #140.
//
// Generation:
//   UPDATE_GOLDEN=1 go test ./tests/unit -run TestGenerateIssue140Scenarios -v
//
// Verification:
//   go test ./tests/unit -run TestIssue140ScenarioGoldens -v

type issue140Scenario struct {
	name   string
	render func(t *testing.T) string
}

func TestGenerateIssue140Scenarios(t *testing.T) {
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		t.Skip("set UPDATE_GOLDEN=1 to generate issue #140 golden snapshots")
	}

	baseDir := filepath.Join("..", "testdata", "golden", "issue140")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("create golden dir: %v", err)
	}

	plain := collectIssue140ScenarioViews(t, true)
	ansi := collectIssue140ScenarioViews(t, false)

	names := sortedScenarioNames(plain)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(baseDir, name+".golden.txt"), []byte(plain[name]), 0o644); err != nil {
			t.Fatalf("write plain golden for %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, name+".ansi.golden"), []byte(ansi[name]), 0o644); err != nil {
			t.Fatalf("write ANSI golden for %s: %v", name, err)
		}
	}
}

func TestIssue140ScenarioGoldens(t *testing.T) {
	baseDir := filepath.Join("..", "testdata", "golden", "issue140")

	plain := collectIssue140ScenarioViews(t, true)
	ansi := collectIssue140ScenarioViews(t, false)

	names := sortedScenarioNames(plain)
	for _, name := range names {
		plainPath := filepath.Join(baseDir, name+".golden.txt")
		ansiPath := filepath.Join(baseDir, name+".ansi.golden")

		expectedPlain, err := os.ReadFile(plainPath)
		if err != nil {
			t.Fatalf("read %s: %v (generate with UPDATE_GOLDEN=1)", plainPath, err)
		}
		expectedANSI, err := os.ReadFile(ansiPath)
		if err != nil {
			t.Fatalf("read %s: %v (generate with UPDATE_GOLDEN=1)", ansiPath, err)
		}

		actualPlain := strings.ReplaceAll(plain[name], "\r\n", "\n")
		goldenPlain := strings.ReplaceAll(string(expectedPlain), "\r\n", "\n")
		if goldenPlain != actualPlain {
			t.Fatalf("plain golden mismatch for scenario %s\n--- expected ---\n%s\n--- actual ---\n%s", name, goldenPlain, actualPlain)
		}
		actualANSI := strings.ReplaceAll(ansi[name], "\r\n", "\n")
		goldenANSI := strings.ReplaceAll(string(expectedANSI), "\r\n", "\n")
		if goldenANSI != actualANSI {
			t.Fatalf("ANSI golden mismatch for scenario %s\n--- expected ---\n%s\n--- actual ---\n%s", name, goldenANSI, actualANSI)
		}
	}
}

func collectIssue140ScenarioViews(t *testing.T, noColor bool) map[string]string {
	t.Helper()
	oldVersion := tui.Version
	tui.Version = ""
	t.Cleanup(func() { tui.Version = oldVersion })

	if noColor {
		t.Setenv("NO_COLOR", "1")
	} else {
		t.Setenv("NO_COLOR", "")
	}
	styles.Reinit()

	out := make(map[string]string)
	for _, sc := range issue140Scenarios() {
		v := withScenarioEC2Defs(t, func() string { return sc.render(t) })
		v = strings.ReplaceAll(v, "\r\n", "\n")
		if noColor {
			v = stripAnsi(v)
		}
		out[sc.name] = v
	}
	return out
}

func issue140Scenarios() []issue140Scenario {
	return []issue140Scenario{
		{name: "ec2_001_initial_detail", render: scenarioEC2001InitialDetail},
		{name: "ec2_019_related_loading", render: scenarioEC2019RelatedLoading},
		{name: "ec2_017_vpcid_selected", render: scenarioEC2017VpcIDSelected},
		{name: "ec2_018_right_column_types", render: scenarioEC2018RightColumnTypes},
		{name: "ec2_020_counts_arrived", render: scenarioEC2020CountsArrived},
		{name: "ec2_021_right_focus_after_tab", render: scenarioEC2021RightFocusAfterTab},
		{name: "ec2_023_right_hidden_after_toggle", render: scenarioEC2023RightHiddenAfterToggle},
		{name: "ec2_025_right_filter_live_cloud", render: scenarioEC2025RightFilterLiveCloud},
		{name: "ec2_028_right_filter_cleared", render: scenarioEC2028RightFilterCleared},
		{name: "ec2_029_filtered_alarms_list", render: scenarioEC2029FilteredAlarmsList},
		{name: "ec2_033_only_alarm_available_focus", render: scenarioEC2033OnlyAlarmAvailableFocus},
		{name: "ec2_034_cloudtrail_last", render: scenarioEC2034CloudTrailLast},
	}
}

func scenarioEC2001InitialDetail(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	return m.View().Content
}

func scenarioEC2019RelatedLoading(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	return m.View().Content
}

func scenarioEC2017VpcIDSelected(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	for i := 0; i < 7; i++ {
		m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	}
	return m.View().Content
}

func scenarioEC2018RightColumnTypes(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	return m.View().Content
}

func scenarioEC2020CountsArrived(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "asg", Count: 1}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "alarm", Count: 2}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "tg", Count: 0}})
	return m.View().Content
}

func scenarioEC2021RightFocusAfterTab(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "tg", Count: 0}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "asg", Count: 1}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "alarm", Count: 0}})
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m.View().Content
}

func scenarioEC2023RightHiddenAfterToggle(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	return m.View().Content
}

func scenarioEC2025RightFilterLiveCloud(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	for _, ch := range []string{"/", "c", "l", "o", "u", "d"} {
		m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	return m.View().Content
}

func scenarioEC2028RightFilterCleared(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	for _, ch := range []string{"/", "c", "l", "o", "u", "d"} {
		m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	return m.View().Content
}

func scenarioEC2029FilteredAlarmsList(t *testing.T) string {
	m := issue140DemoModel(t, 120, 30)
	source := mustDemoEC2(t)[0]
	m = issue140ApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "alarm", Resources: []resource.Resource{
		{ID: "web-prod-cpu-high", Name: "web-prod-cpu-high", Status: "alarm"},
		{ID: "web-prod-status-check", Name: "web-prod-status-check", Status: "ok"},
		{ID: "unrelated-alarm", Name: "unrelated-alarm", Status: "ok"},
	}})
	m = issue140ApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType:     "alarm",
		SourceType:     "ec2",
		SourceResource: source,
		RelatedIDs:     []string{"web-prod-cpu-high", "web-prod-status-check"},
	})
	return m.View().Content
}

func scenarioEC2033OnlyAlarmAvailableFocus(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "tg", Count: 0}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "asg", Count: 0}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "alarm", Count: 2}})
	m = issue140ApplyMsg(m, messages.RelatedCheckResultMsg{ResourceType: "ec2", Result: resource.RelatedCheckResult{TargetType: "cfn", Count: 0}})
	m = issue140ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m.View().Content
}

func scenarioEC2034CloudTrailLast(t *testing.T) string {
	m := issue140DemoModel(t, 120, 35)
	m = issue140NavigateToEC2Detail(t, m)
	return m.View().Content
}

func issue140DemoModel(t *testing.T, w, h int) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m2.(tui.Model)
}

func issue140NavigateToEC2Detail(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	ec2 := mustDemoEC2(t)
	m2, _ := m.Update(messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})
	return m2.(tui.Model)
}

func issue140ApplyMsg(m tui.Model, msg tea.Msg) tui.Model {
	m2, _ := m.Update(msg)
	return m2.(tui.Model)
}

func sortedScenarioNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func TestIssue140ScenarioCatalog(t *testing.T) {
	// Human-readable, test output only (no assertions).
	for _, sc := range issue140Scenarios() {
		t.Log(fmt.Sprintf("scenario: %s", sc.name))
	}
}

func withScenarioEC2Defs(t *testing.T, fn func() string) string {
	t.Helper()
	oldDefs := append([]resource.RelatedDef(nil), resource.GetRelated("ec2")...)
	oldNav := append([]resource.NavigableField(nil), resource.GetNavigableFields("ec2")...)
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
	defer func() {
		resource.RegisterRelated("ec2", oldDefs)
		resource.RegisterNavigableFields("ec2", oldNav)
	}()
	return fn()
}

func TestIssue140StorySectionsCovered(t *testing.T) {
	expected := []string{
		"EC2 Detail",
		"EC2 Related Types",
		"EC2 Detail Stacked Layout",
		"Related Results Lists",
		"Destination Detail Views",
		"Cache, Refresh, and Background Update",
	}
	sections := issueStorySections(t)
	for _, section := range expected {
		stories := sections[section]
		if len(stories) == 0 {
			t.Fatalf("expected section %q to contain stories in QA doc", section)
		}
		t.Logf("%s: %d stories", section, len(stories))
	}
}

func TestIssue140StoryMapCoversAllStories(t *testing.T) {
	storyMap := map[string][]string{}
	addStoryEvidence := func(evidence []string, stories ...string) {
		for _, story := range stories {
			if _, exists := storyMap[story]; exists {
				t.Fatalf("duplicate issue140 story mapping: %q", story)
			}
			storyMap[story] = evidence
		}
	}

	addStoryEvidence([]string{"scenario:ec2_001_initial_detail", "file:tests/unit/left_column_preview_regressions_test.go", "file:tests/unit/issue140_story_render_contract_test.go"}, "Wide terminals show EC2 detail and related resources side by side", "EC2 detail shows the configured curated field set instead of raw YAML", "Section headers and nested fields are visibly structured", "Long detail values wrap instead of forcing horizontal detail scrolling", "Updated detail flow no longer depends on a word-wrap toggle", "Pressing `w` does not introduce a separate wrap mode in the updated detail screen")
	addStoryEvidence([]string{"scenario:ec2_021_right_focus_after_tab", "file:tests/unit/detail_focus_test.go"}, "Focus indicator changes with the active detail column", "Tab switches focus between detail and related columns", "Shift-Tab also flips focus between the two visible columns", "H and L switch focus instead of horizontally scrolling the detail view")
	addStoryEvidence([]string{"file:tests/unit/ec2_stories_cursor_enter_test.go"}, "Left-column cursor moves row by row across both plain and navigable fields", "Left-column jump keys go to the first and last detail rows", "Detail paging works on the focused column", "Enter on a plain non-navigable detail row does not leave the view", "Enter on VpcId opens the VPC detail screen", "Enter on SubnetId opens the subnet detail screen", "Enter on a security group ID opens the security group detail screen", "Enter on ImageId opens the AMI detail screen", "Enter on an attached EBS volume ID opens the EBS volume detail screen", "Enter on a network interface ID opens the ENI detail screen")
	addStoryEvidence([]string{"scenario:ec2_017_vpcid_selected", "file:tests/unit/issue140_story_render_contract_test.go", "file:tests/unit/detail_rendering_spec007_test.go"}, "Navigable EC2 field values are visibly different from plain values", "Selected navigable fields use row selection instead of underline")
	addStoryEvidence([]string{"file:tests/unit/qa_search_views_test.go", "file:tests/unit/issue119_scenarios_golden_test.go"}, "Left-column search uses the header and highlights matching detail rows", "Search match indicator is visible in the left detail column", "Search next and previous keys only apply to left-column search", "Search highlighting outranks the navigable underline cue", "Left-column search persists internally when focus moves away", "Escape cancels detail search input before clearing search results or leaving the view", "Escape clears existing search results before popping EC2 detail")
	addStoryEvidence([]string{"file:tests/unit/ec2_stories_rightcol_misc_test.go", "file:tests/unit/qa_copy_test.go"}, "Copy from the left detail column copies the active field value", "YAML shortcut works from EC2 detail regardless of column focus", "Detail help reflects the two-column interaction model")

	addStoryEvidence([]string{"scenario:ec2_019_related_loading", "scenario:ec2_018_right_column_types", "file:tests/unit/ec2_stories_rightcol_misc_test.go"}, "Related types are visible by default when EC2 detail opens", "Related rows start dim and become active as availability is discovered", "Available related rows without a cheap count remain selectable without a number", "Background check failures are silent on screen", "CloudTrail row is always visible and sorted last", "CloudTrail row does not show an inline count")
	addStoryEvidence([]string{"scenario:ec2_020_counts_arrived", "file:tests/unit/issue140_story_render_contract_test.go"}, "Available related rows may show counts when the count is known")
	addStoryEvidence([]string{"scenario:ec2_033_only_alarm_available_focus", "file:tests/unit/issue140_story_render_contract_test.go"}, "Unavailable related rows remain dim and cannot be selected", "Right-column cursor only lands on active rows", "Right-column jump keys land on the first and last active related row")
	addStoryEvidence([]string{"scenario:ec2_025_right_filter_live_cloud", "file:tests/unit/issue119_scenarios_golden_test.go"}, "Right-column filter narrows visible related type names live", "Right-column filtering can still show matching dim rows", "Right-column filter state survives a focus switch")
	addStoryEvidence([]string{"scenario:ec2_028_right_filter_cleared", "file:tests/unit/issue119_scenarios_golden_test.go"}, "Escape clears right-column filtering before leaving EC2 detail")
	addStoryEvidence([]string{"file:tests/unit/ec2_stories_rightcol_misc_test.go"}, "Copy from the right column copies the related type label", "Related column can be hidden and the left detail uses the full width", "Tab has no effect while the related column is hidden", "Toggling related back on restores the related pane", "Right-column overflow shows a visible scroll indicator", "Refresh resets the visible related-state resolution and starts over")
	addStoryEvidence([]string{"file:tests/unit/related_navigate_count_spec008_test.go", "file:tests/unit/ec2_stories_nav_chains_test.go"}, "Enter on a single-result related type goes directly to detail", "Enter on a multi-result related type opens a result list", "CloudTrail enter currently gives a visible placeholder response")

	addStoryEvidence([]string{"file:tests/unit/issue119_scenarios_golden_test.go", "file:tests/unit/issue119_140_regressions_test.go"}, "Medium-width terminals stack related content below detail content", "Focus switching still works in stacked mode", "Resize across the side-by-side threshold preserves detail state")

	addStoryEvidence([]string{"scenario:ec2_029_filtered_alarms_list", "file:tests/unit/related_navigate_count_spec008_test.go"}, "CloudWatch Alarms list shows alarm summary columns after EC2 related navigation", "Related-result lists reuse standard list interactions", "Related-result list fetch failure is shown as visible screen feedback")
	addStoryEvidence([]string{"file:tests/unit/related_navigate_count_spec008_test.go", "file:tests/unit/ec2_stories_nav_chains_test.go"}, "Target Groups list shows target-group columns after EC2 related navigation", "Auto Scaling Groups list shows ASG summary columns after EC2 related navigation", "CloudFormation Stacks list shows stack summary columns after EC2 related navigation", "EKS Node Groups list shows node group summary columns after EC2 related navigation", "Elastic Beanstalk environments list shows environment summary columns after EC2 related navigation", "EBS Snapshots list shows snapshot summary columns after EC2 related navigation", "Elastic IP list shows EIP summary columns after EC2 related navigation", "CloudWatch Log Groups list shows log-group summary columns after EC2 related navigation", "Route 53 records list shows record summary columns after EC2 related navigation")

	addStoryEvidence([]string{"file:tests/unit/qa_detail_ec2_family_test.go", "file:tests/unit/ec2_stories_nav_chains_test.go"}, "VPC detail shows the configured VPC detail fields", "Subnet detail shows the configured subnet detail fields", "Security group detail shows the configured security-group detail fields", "EIP detail shows the configured Elastic IP detail fields", "ENI detail shows the configured network-interface detail fields")
	addStoryEvidence([]string{"file:tests/unit/qa_detail_v220_test.go", "file:tests/unit/related_navigate_count_spec008_test.go"}, "AMI detail shows the configured image detail fields", "EBS volume detail shows the configured volume detail fields", "EBS snapshot detail shows the configured snapshot detail fields", "Log group detail shows the configured log-group detail fields", "Route 53 record detail shows the configured record detail fields")
	addStoryEvidence([]string{"file:tests/unit/qa_detail_services_test.go", "file:tests/unit/ec2_stories_nav_chains_test.go"}, "Auto Scaling Group detail shows the configured ASG detail fields", "Alarm detail shows the configured alarm detail fields", "CloudFormation stack detail shows the configured stack detail fields", "Node group detail shows the configured EKS node-group detail fields", "Elastic Beanstalk detail shows the configured environment detail fields", "CloudTrail event detail shows the configured event detail fields")

	addStoryEvidence([]string{"file:tests/unit/aws_cfn_resources_test.go", "file:tests/unit/qa_child_pagination_test.go", "file:tests/unit/child_view_resourcelist_test.go"}, "In CloudFormation stack context, uppercase R opens stack resources", "Lowercase r still belongs to related toggle in detail context", "CloudFormation stack resources list shows the configured columns")

	addStoryEvidence([]string{"file:tests/unit/rightcolumn_test.go", "file:tests/unit/ec2_stories_rightcol_misc_test.go"}, "Reopening the same EC2 detail in the same session can show already-known related availability immediately", "Switching to another EC2 instance does not leak old related-state updates onto the new screen", "Region or profile changes clear visible related-state assumptions", "Refresh on a related-result list re-fetches that result view in place", "Background related checking is silent and does not use a spinner per row")

	verifyStoryEvidenceMap(t, []string{
		"EC2 Detail",
		"EC2 Related Types",
		"EC2 Detail Stacked Layout",
		"Related Results Lists",
		"Destination Detail Views",
		"CloudFormation Stack Resources",
		"Cache, Refresh, and Background Update",
	}, storyMap, issue140ScenarioNameSet())
}

func issue140ScenarioNameSet() map[string]struct{} {
	out := make(map[string]struct{}, len(issue140Scenarios()))
	for _, sc := range issue140Scenarios() {
		out[sc.name] = struct{}{}
	}
	return out
}
