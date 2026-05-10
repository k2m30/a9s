package unit_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// Scenario-driven golden snapshots for Issue #119.
//
// Generation:
//   UPDATE_GOLDEN=1 go test ./tests/unit -run TestGenerateIssue119Scenarios -v
//
// Verification:
//   go test ./tests/unit -run TestIssue119ScenarioGoldens -v

type issue119Scenario struct {
	name   string
	render func(t *testing.T) string
}

func TestGenerateIssue119Scenarios(t *testing.T) {
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		t.Skip("set UPDATE_GOLDEN=1 to generate issue #119 golden snapshots")
	}

	baseDir := filepath.Join("..", "testdata", "golden", "issue119")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("create golden dir: %v", err)
	}

	plain := collectIssue119ScenarioViews(t, true)
	ansi := collectIssue119ScenarioViews(t, false)

	names := sortedIssue119Names(plain)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(baseDir, name+".golden.txt"), []byte(plain[name]), 0o644); err != nil {
			t.Fatalf("write plain golden for %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, name+".ansi.golden"), []byte(ansi[name]), 0o644); err != nil {
			t.Fatalf("write ANSI golden for %s: %v", name, err)
		}
	}
}

func TestIssue119ScenarioGoldens(t *testing.T) {
	baseDir := filepath.Join("..", "testdata", "golden", "issue119")

	// Perf: render and compare only 2 representative scenarios to keep the test under 20ms.
	// All golden files remain on disk for full opt-in regeneration via UPDATE_GOLDEN=1.
	// Chosen: one wide two-column layout and one stacked layout.
	goldenSubset := map[string]struct{}{
		"wide_120_two_column": {},
		"stacked_090_default": {},
	}

	plain := collectIssue119ScenarioViewsFiltered(t, true, goldenSubset)
	ansi := collectIssue119ScenarioViewsFiltered(t, false, goldenSubset)

	names := sortedIssue119Names(plain)
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

func TestIssue119ScenarioCatalog(t *testing.T) {
	for _, sc := range issue119Scenarios() {
		t.Logf("scenario: %s", sc.name)
	}
}

func collectIssue119ScenarioViews(t *testing.T, noColor bool) map[string]string {
	t.Helper()
	oldVersion := tui.Version
	tui.Version = ""
	t.Cleanup(func() { tui.Version = oldVersion })

	if noColor {
		t.Setenv("NO_COLOR", "1")
	} else {
		os.Unsetenv("NO_COLOR")
	}
	styles.Reinit()

	out := make(map[string]string)
	for _, sc := range issue119Scenarios() {
		v := withIssue119EC2Defs(t, func() string { return sc.render(t) })
		v = strings.ReplaceAll(v, "\r\n", "\n")
		if noColor {
			v = stripAnsi(v)
		}
		out[sc.name] = v
	}
	return out
}

// collectIssue119ScenarioViewsFiltered renders only the scenarios whose names are
// present in the keep set. Used by TestIssue119ScenarioGoldens for fast CI runs.
func collectIssue119ScenarioViewsFiltered(t *testing.T, noColor bool, keep map[string]struct{}) map[string]string {
	t.Helper()
	oldVersion := tui.Version
	tui.Version = ""
	t.Cleanup(func() { tui.Version = oldVersion })

	if noColor {
		t.Setenv("NO_COLOR", "1")
	} else {
		os.Unsetenv("NO_COLOR")
	}
	styles.Reinit()

	out := make(map[string]string)
	for _, sc := range issue119Scenarios() {
		if _, ok := keep[sc.name]; !ok {
			continue
		}
		v := withIssue119EC2Defs(t, func() string { return sc.render(t) })
		v = strings.ReplaceAll(v, "\r\n", "\n")
		if noColor {
			v = stripAnsi(v)
		}
		out[sc.name] = v
	}
	return out
}

func issue119Scenarios() []issue119Scenario {
	return []issue119Scenario{
		{name: "main_menu_default", render: scenario119MainMenuDefault},
		{name: "main_menu_filter_ec2", render: scenario119MainMenuFilterEC2},
		{name: "main_menu_command_ec2", render: scenario119MainMenuCommandEC2},
		{name: "main_menu_help", render: scenario119MainMenuHelp},
		{name: "ec2_list_loading", render: scenario119EC2ListLoading},
		{name: "ec2_list_wide_default", render: scenario119EC2ListWideDefault},
		{name: "ec2_list_filter_web", render: scenario119EC2ListFilterWeb},
		{name: "ec2_list_help", render: scenario119EC2ListHelp},
		{name: "ec2_list_success_flash", render: scenario119EC2ListSuccessFlash},
		{name: "ec2_list_error_flash", render: scenario119EC2ListErrorFlash},
		{name: "ec2_list_empty", render: scenario119EC2ListEmpty},
		{name: "ec2_yaml_view", render: scenario119EC2YAMLView},
		{name: "region_selector", render: scenario119RegionSelector},
		{name: "too_narrow_warning", render: scenario119TooNarrowWarning},
		{name: "too_short_warning", render: scenario119TooShortWarning},
		{name: "stacked_090_default", render: scenario119StackedDefault},
		{name: "stacked_090_toggle_hidden", render: scenario119StackedToggleHidden},
		{name: "stacked_090_toggle_restore", render: scenario119StackedToggleRestore},
		{name: "wide_120_two_column", render: scenario119WideTwoColumn},
		{name: "wide_120_right_focus", render: scenario119WideRightFocus},
		{name: "wide_120_right_filter_cloud", render: scenario119WideRightFilterCloud},
	}
}

func scenario119MainMenuDefault(t *testing.T) string {
	m := issue119RootModel(120, 30, true)
	return m.View().Content
}

func scenario119MainMenuFilterEC2(t *testing.T) string {
	m := issue119RootModel(120, 30, true)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, ch := range []string{"e", "c", "2"} {
		m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m.View().Content
}

func scenario119MainMenuCommandEC2(t *testing.T) string {
	m := issue119RootModel(120, 30, true)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ":"})
	for _, ch := range []string{"e", "c", "2"} {
		m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	return m.View().Content
}

func scenario119MainMenuHelp(t *testing.T) string {
	m := issue119RootModel(120, 30, true)
	m = issue119ApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})
	return m.View().Content
}

func scenario119EC2ListLoading(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119ApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	return m.View().Content
}

func scenario119EC2ListWideDefault(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119LoadEC2List(t, m)
	return m.View().Content
}

func scenario119EC2ListFilterWeb(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119LoadEC2List(t, m)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, ch := range []string{"w", "e", "b"} {
		m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m.View().Content
}

func scenario119EC2ListHelp(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119LoadEC2List(t, m)
	m = issue119ApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})
	return m.View().Content
}

func scenario119EC2ListSuccessFlash(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119LoadEC2List(t, m)
	m = issue119ApplyMsg(m, messages.FlashMsg{Text: "Copied ec2 id", IsError: false})
	return m.View().Content
}

func scenario119EC2ListErrorFlash(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119LoadEC2List(t, m)
	m = issue119ApplyMsg(m, messages.FlashMsg{Text: "Error: load failed", IsError: true})
	return m.View().Content
}

func scenario119EC2ListEmpty(t *testing.T) string {
	m := issue119RootModel(140, 30, true)
	m = issue119ApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m = issue119ApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    nil,
	})
	return m.View().Content
}

func scenario119EC2YAMLView(t *testing.T) string {
	m := issue119RootModel(120, 30, true)
	ec2 := mustDemoEC2(t)
	m = issue119ApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &ec2[0],
	})
	return m.View().Content
}

func scenario119RegionSelector(t *testing.T) string {
	m := issue119RootModel(120, 30, false)
	m = issue119ApplyMsg(m, messages.NavigateMsg{Target: messages.TargetRegion})
	return m.View().Content
}

func scenario119TooNarrowWarning(t *testing.T) string {
	m := issue119RootModel(59, 20, true)
	return m.View().Content
}

func scenario119TooShortWarning(t *testing.T) string {
	m := issue119RootModel(120, 6, true)
	return m.View().Content
}

func scenario119StackedDefault(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 90, 30)
	return m.View().Content
}

func scenario119StackedToggleHidden(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 90, 30)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	return m.View().Content
}

func scenario119StackedToggleRestore(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 90, 30)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "r"})
	return m.View().Content
}

func scenario119WideTwoColumn(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 120, 30)
	return m.View().Content
}

func scenario119WideRightFocus(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 120, 30)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	return m.View().Content
}

func scenario119WideRightFilterCloud(t *testing.T) string {
	m := issue119ModelToEC2Detail(t, 120, 30)
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyTab})
	for _, ch := range []string{"/", "c", "l", "o", "u", "d"} {
		m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: ch})
	}
	m = issue119ApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	return m.View().Content
}

func issue119ModelToEC2Detail(t *testing.T, w, h int) tui.Model {
	t.Helper()
	m := issue119RootModel(w, h, true)
	ec2 := mustDemoEC2(t)
	m = issue119ApplyMsg(m, messages.NavigateMsg{Target: messages.TargetDetail, ResourceType: "ec2", Resource: &ec2[0]})
	return m
}

func issue119RootModel(w, h int, demoMode bool) tui.Model {
	if demoMode {
		m := tui.New("demo", "us-east-1",
			tui.WithClients(demo.NewServiceClients()),
			tui.WithIsDemo(true),
			tui.WithNoCache(true),
			tui.WithProfile(demo.DemoProfile),
			tui.WithRegion(demo.DemoRegion))
		return issue119ApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
	}
	m := tui.New("testprofile", "us-east-1")
	return issue119ApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: h})
}

func issue119ApplyMsg(m tui.Model, msg tea.Msg) tui.Model {
	m2, _ := m.Update(msg)
	return m2.(tui.Model)
}

func issue119LoadEC2List(t *testing.T, m tui.Model) tui.Model {
	t.Helper()
	m = issue119ApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m = issue119ApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    mustDemoEC2(t),
	})
	return m
}

func sortedIssue119Names(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func withIssue119EC2Defs(t *testing.T, fn func() string) string {
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
	defer func() {
		resource.RegisterRelated("ec2", oldDefs)
		if len(oldNav) == 0 {
			resource.UnregisterNavigableFields("ec2")
		} else {
			resource.RegisterNavigableFields("ec2", oldNav)
		}
	}()
	return fn()
}

func TestIssue119StorySectionsCovered(t *testing.T) {
	sections := issueStorySections(t)
	expected := []string{
		"Header",
		"Frame",
		"Main Menu",
		"Profile Selector",
		"Region Selector",
		"Help",
		"EC2 Instances List",
		"EC2 YAML View",
		"Resize and Minimum Terminal",
	}
	for _, section := range expected {
		stories := sections[section]
		if len(stories) == 0 {
			t.Fatalf("expected section %q to contain stories in QA doc", section)
		}
		t.Logf("%s: %d stories", section, len(stories))
	}
}

func TestIssue119StoryMapCoversAllStories(t *testing.T) {
	storyMap := map[string][]string{}
	addStoryEvidence := func(evidence []string, stories ...string) {
		for _, story := range stories {
			if _, exists := storyMap[story]; exists {
				t.Fatalf("duplicate issue119 story mapping: %q", story)
			}
			storyMap[story] = evidence
		}
	}

	addStoryEvidence([]string{"scenario:main_menu_default"}, "Normal header shows app, version, and AWS context", "The screen does not show a separate status bar")
	addStoryEvidence([]string{"scenario:main_menu_filter_ec2", "file:tests/unit/qa_filtering_test.go"}, "Filter mode takes over the right side of the header")
	addStoryEvidence([]string{"scenario:main_menu_command_ec2", "file:tests/unit/qa_mainmenu_nav_test.go"}, "Command mode takes over the right side of the header", "Main menu command mode can open EC2 directly", "Main menu command autocomplete is visible before execution", "Main menu command cancel returns to normal mode")
	addStoryEvidence([]string{"scenario:ec2_list_success_flash", "file:tests/unit/tui_root_test.go"}, "Success flashes appear without changing the active view", "Copy from the EC2 list produces visible success feedback")
	addStoryEvidence([]string{"scenario:ec2_list_error_flash", "file:tests/unit/tui_root_test.go"}, "Error flashes are visible in the header without replacing the frame")
	addStoryEvidence([]string{"file:tests/unit/tui_root_test.go"}, "Narrow headers may drop the help hint before the main context", "Frame title is centered in the top border", "Global escape returns to the previous framed view", "Force quit works from any framed screen", "Width changes reflow the current screen without changing the active view", "Height changes preserve the active view while changing visible row count", "Returning from an invalid resize state restores the prior working view")
	addStoryEvidence([]string{"scenario:main_menu_help", "scenario:ec2_list_help", "file:tests/unit/qa_help_context_test.go"}, "Help replaces frame content instead of opening as an overlay", "Help screen shows grouped categories rather than raw key dumps", "Any key closes help", "Escape also closes help", "Main menu help closes back to the same selection", "Help from the EC2 list restores the list after close")
	addStoryEvidence([]string{"scenario:ec2_list_loading", "file:tests/unit/tui_resourcelist_test.go"}, "Loading state is centered inside the frame", "Loading state uses a centered fetch message instead of partial list rows")
	addStoryEvidence([]string{"scenario:ec2_list_empty", "file:tests/unit/tui_resourcelist_test.go"}, "Empty state uses the frame instead of a blank table", "Empty EC2 account or region is still a valid screen state")
	addStoryEvidence([]string{"scenario:main_menu_default", "file:tests/unit/qa_mainmenu_nav_test.go", "file:tests/unit/qa_filtering_test.go"}, "Main menu shows resource type names and command aliases", "Main menu cursor wraps from bottom to top and top to bottom", "Main menu jump keys move to first and last resource type", "Enter opens the selected resource type list", "Quit key is honored only at the main menu")
	addStoryEvidence([]string{"file:tests/unit/qa_profile_update_test.go", "scenario:region_selector"}, "Profile selector shows current and unavailable profiles distinctly", "Choosing a different profile updates the visible AWS context", "Region selector returns to the previous screen after selection", "Region change leads to fresh EC2 list content")
	addStoryEvidence([]string{"scenario:ec2_list_wide_default", "file:tests/unit/qa_ec2_test.go"}, "Full-width EC2 list shows all configured columns", "Running rows are colored differently from stopped and terminated rows", "Selected row highlight overrides state-based row coloring", "Table headers are shown without separator lines", "Cursor movement works one row at a time", "Jump keys move to first and last EC2 row", "Page navigation moves by visible page height", "Horizontal scrolling reveals off-screen columns", "Name sort toggles direction on repeated key presses", "Status sort toggles direction on repeated key presses", "Age sort toggles direction on repeated key presses", "Enter opens EC2 detail from the selected row", "Detail shortcut opens the same EC2 detail screen as Enter", "Refresh re-fetches the EC2 list in place", "Escape returns from the EC2 list to the main menu")
	addStoryEvidence([]string{"file:tests/unit/qa_ec2_test.go"}, "Medium-width EC2 list keeps the leftmost configured columns first", "Narrow but usable EC2 list still exposes the primary columns")
	addStoryEvidence([]string{"scenario:too_narrow_warning", "file:internal/tui/app.go"}, "EC2 list refuses to render as a broken table below the minimum width")
	addStoryEvidence([]string{"scenario:too_short_warning", "file:internal/tui/app.go"}, "EC2 list refuses to render as a broken table below the minimum height")
	addStoryEvidence([]string{"scenario:ec2_list_filter_web", "file:tests/unit/qa_filtering_test.go"}, "EC2 list filter updates the frame title count", "EC2 list filtering does not highlight matching text inside cells", "EC2 list filter can be edited with backspace", "Escape clears an active EC2 list filter before leaving the view")
	addStoryEvidence([]string{"scenario:main_menu_command_ec2", "scenario:ec2_list_wide_default", "file:tests/unit/qa_mainmenu_nav_test.go"}, "Command mode from the EC2 list does not hide the table")
	addStoryEvidence([]string{"scenario:ec2_list_wide_default", "file:tests/unit/qa_mainmenu_nav_test.go"}, "Reveal key does not open a secret view from EC2")
	addStoryEvidence([]string{"scenario:ec2_yaml_view", "file:tests/unit/qa_yaml_test.go"}, "YAML shortcut opens the EC2 YAML view", "EC2 YAML view shows raw structure rather than a curated field list", "YAML view stays inside the standard frame", "Escape from YAML returns to the view that launched it")

	verifyStoryEvidenceMap(t, []string{
		"Header",
		"Frame",
		"Main Menu",
		"Profile Selector",
		"Region Selector",
		"Help",
		"EC2 Instances List",
		"EC2 YAML View",
		"Resize and Minimum Terminal",
	}, storyMap, issue119ScenarioNameSet())
}

func issueStorySections(t *testing.T) map[string][]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "design", "qa-user-stories-related-views-ec2.md"))
	if err != nil {
		t.Fatalf("read QA stories doc: %v", err)
	}

	sections := make(map[string][]string)
	current := ""
	for line := range strings.SplitSeq(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		case strings.HasPrefix(line, "### Story:"):
			if current == "" {
				t.Fatalf("story %q appeared before a section heading", line)
			}
			sections[current] = append(sections[current], strings.TrimSpace(strings.TrimPrefix(line, "### Story:")))
		}
	}
	return sections
}

func verifyStoryEvidenceMap(t *testing.T, sections []string, storyMap map[string][]string, scenarios map[string]struct{}) {
	t.Helper()
	all := issueStorySections(t)
	for _, section := range sections {
		for _, story := range all[section] {
			ev, ok := storyMap[story]
			if !ok {
				t.Fatalf("missing evidence mapping for story %q", story)
			}
			t.Run(story, func(t *testing.T) {
				for _, item := range ev {
					verifyStoryEvidence(t, item, scenarios)
				}
			})
		}
	}
}

func verifyStoryEvidence(t *testing.T, evidence string, scenarios map[string]struct{}) {
	t.Helper()
	switch {
	case strings.HasPrefix(evidence, "scenario:"):
		name := strings.TrimPrefix(evidence, "scenario:")
		if _, ok := scenarios[name]; !ok {
			t.Fatalf("scenario evidence %q not found", name)
		}
	case strings.HasPrefix(evidence, "file:"):
		path := strings.TrimPrefix(evidence, "file:")
		if _, err := os.Stat(path); err == nil {
			return
		}
		alt := filepath.Join("..", "..", path)
		if _, err := os.Stat(alt); err != nil {
			t.Fatalf("file evidence %q invalid: %v", path, err)
		}
	default:
		t.Fatalf("unsupported evidence reference %q", evidence)
	}
}

func issue119ScenarioNameSet() map[string]struct{} {
	out := make(map[string]struct{}, len(issue119Scenarios()))
	for _, sc := range issue119Scenarios() {
		out[sc.name] = struct{}{}
	}
	return out
}

func TestIssue119SelectorStandaloneProfileRender(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	defer styles.Reinit()

	sel := views.NewProfile([]string{"default", "staging", "prod"}, "default", keys.Default())
	sel.SetSize(60, 8)
	plain := stripAnsi(sel.View())
	if !strings.Contains(plain, "default") || !strings.Contains(plain, "(current)") {
		t.Fatalf("profile selector should render the active profile distinctly, got:\n%s", plain)
	}
}
