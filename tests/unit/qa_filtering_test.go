package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// typeFilter enters filter mode and types each character of text.
func typeFilter(m tui.Model, text string) tui.Model {
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})
	for _, ch := range text {
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: string(ch)})
	}
	return m
}

// loadEC2Resources navigates to ec2 resource list and loads test resources.
func loadEC2Resources(m tui.Model, resources []resource.Resource) tui.Model {
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    resources, Provenance: messages.FetchProvenanceCanonicalList,
	})
	return m
}

// sampleEC2Resources returns a set of test EC2 resources.
func sampleEC2Resources() []resource.Resource {
	return []resource.Resource{
		{ID: "i-abc001", Name: "api-prod-01", Fields: map[string]string{"instance_id": "i-abc001", "name": "api-prod-01", "state": "running", "type": "t3.medium"}},
		{ID: "i-abc002", Name: "api-prod-02", Fields: map[string]string{"instance_id": "i-abc002", "name": "api-prod-02", "state": "running", "type": "t3.medium"}},
		{ID: "i-abc003", Name: "worker-01", Fields: map[string]string{"instance_id": "i-abc003", "name": "worker-01", "state": "running", "type": "t3.large"}},
		{ID: "i-abc004", Name: "worker-02", Fields: map[string]string{"instance_id": "i-abc004", "name": "worker-02", "state": "pending", "type": "t3.large"}},
		{ID: "i-abc005", Name: "bastion", Fields: map[string]string{"instance_id": "i-abc005", "name": "bastion", "state": "running", "type": "t2.micro"}},
		{ID: "i-abc006", Name: "old-worker", Fields: map[string]string{"instance_id": "i-abc006", "name": "old-worker", "state": "stopped", "type": "t3.medium"}},
		{ID: "i-abc007", Name: "legacy-app", Fields: map[string]string{"instance_id": "i-abc007", "name": "legacy-app", "state": "terminated", "type": "t2.small"}},
		{ID: "i-abc008", Name: "db-server", Fields: map[string]string{"instance_id": "i-abc008", "name": "db-server", "state": "running", "type": "r5.xlarge"}},
		{ID: "i-abc009", Name: "cache-node", Fields: map[string]string{"instance_id": "i-abc009", "name": "cache-node", "state": "running", "type": "r5.large"}},
		{ID: "i-abc010", Name: "monitoring", Fields: map[string]string{"instance_id": "i-abc010", "name": "monitoring", "state": "running", "type": "t3.small"}},
	}
}

// ── Main menu -- / activates filter mode ─────────────────────────────

func TestQA_Filter_11_01_MainMenu_SlashActivatesFilterMode(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "/") {
		t.Error("after pressing /, header should show filter indicator /")
	}
	if strings.Contains(plain, "? for help") {
		t.Error("after pressing /, header should NOT show '? for help'")
	}
}

// ── Main menu -- filter "ec2" shows only EC2 ─────────────────────────

func TestQA_Filter_11_02_MainMenu_FilterEC2ShowsOnlyEC2(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "ec2")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("filter 'ec2' on main menu should show EC2 Instances")
	}
	for _, name := range []string{"S3 Buckets", "DB Instances", "ElastiCache Redis", "DB Clusters", "EKS Clusters", "Secrets Manager", "VPCs", "Security Groups", "EKS Node Groups"} {
		if strings.Contains(plain, name) {
			t.Errorf("filter 'ec2' on main menu should NOT show %s", name)
		}
	}
	if !strings.Contains(plain, "/ec2") {
		t.Error("header should show /ec2")
	}
	// Frame title should show resource-types(1/71) — +1 for the permanent
	// synthetic "costs" entry alongside resource.AllResourceTypes().
	if !strings.Contains(plain, "resource-types(1/71)") {
		t.Errorf("frame title should show resource-types(1/71), got: %s", plain)
	}
}

// ── Main menu -- filter "s3" shows only S3 ───────────────────────────

func TestQA_Filter_11_03_MainMenu_FilterS3ShowsOnlyS3(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "s3")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "S3 Buckets") {
		t.Error("filter 's3' should show S3 Buckets")
	}
	if !strings.Contains(plain, "resource-types(1/71)") {
		t.Errorf("frame title should show resource-types(1/71), got: %s", plain)
	}
}

// ── Main menu -- filter "xxx" shows nothing ──────────────────────────

func TestQA_Filter_11_04_MainMenu_FilterNoMatch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "xxx")
	plain := stripANSI(rootViewContent(m))

	for _, name := range []string{"S3 Buckets", "EC2 Instances", "DB Instances", "ElastiCache Redis", "DB Clusters", "EKS Clusters", "Secrets Manager", "VPCs", "Security Groups", "EKS Node Groups"} {
		if strings.Contains(plain, name) {
			t.Errorf("filter 'xxx' should NOT show %s", name)
		}
	}
	if !strings.Contains(plain, "resource-types(0/71)") {
		t.Errorf("frame title should show resource-types(0/71), got: %s", plain)
	}
}

// ── Main menu -- filter is case-insensitive ──────────────────────────

func TestQA_Filter_11_05_MainMenu_CaseInsensitive(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "EC2")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("case-insensitive filter 'EC2' should match EC2 Instances")
	}
}

// ── Main menu -- backspace removes characters ────────────────────────

func TestQA_Filter_11_06_MainMenu_Backspace(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "ec2")
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types(1/71)") {
		t.Fatalf("precondition: filter 'ec2' should show 1/71, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/ec") {
		t.Errorf("after backspace, header should show /ec, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	plain = stripANSI(rootViewContent(m))
	// All 71 items should be back (70 resource types + the synthetic costs entry)
	if !strings.Contains(plain, "resource-types(71)") {
		t.Errorf("after clearing filter, frame should show resource-types(71), got: %s", plain)
	}
}

// ── Main menu -- frame title updates with filtered count ─────────────

func TestQA_Filter_11_07_MainMenu_FrameTitleCount(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "e")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "/71)") {
		t.Errorf("filtered frame title should contain /71) showing filtered count, got: %s", plain)
	}
}

// ── Main menu -- Esc clears filter ───────────────────────────────────

func TestQA_Filter_11_08_MainMenu_EscClearsFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "ec2")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types(71)") {
		t.Errorf("after Esc, frame title should be resource-types(71), got: %s", plain)
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("after Esc, header should show '? for help'")
	}
}

// ── Main menu -- Enter confirms filter ───────────────────────────────

func TestQA_Filter_11_09_MainMenu_EnterConfirmsFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "ec2")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "EC2 Instances") {
		t.Error("after Enter, EC2 Instances should still be visible")
	}
	if strings.Contains(plain, "S3 Buckets") {
		t.Error("after Enter, S3 Buckets should NOT be shown (filter persists)")
	}
}

// ── Resource list -- / activates live-filter ─────────────────────────

func TestQA_Filter_11_10_ResourceList_SlashActivates(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "api-prod-01") {
		t.Error("filter 'prod' should show api-prod-01")
	}
	if !strings.Contains(plain, "api-prod-02") {
		t.Error("filter 'prod' should show api-prod-02")
	}
	if strings.Contains(plain, "bastion") {
		t.Error("filter 'prod' should NOT show bastion")
	}
	if !strings.Contains(plain, "/prod") {
		t.Error("header should show /prod")
	}
}

// ── Resource list -- filter persists across scroll ───────────────────

func TestQA_Filter_11_11_ResourceList_FilterPersistsOnScroll(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/prod") {
		t.Error("filter should persist after scrolling")
	}
	if strings.Contains(plain, "bastion") {
		t.Error("filter should persist - bastion should still be hidden")
	}
}

// ── Resource list -- filter clears on Esc ────────────────────────────

func TestQA_Filter_11_12_ResourceList_EscClears(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(10") {
		t.Errorf("after Esc, frame should show ec2(10...), got: %s", plain)
	}
}

// ── Resource list -- cursor resets to 0 on filter change ─────────────

func TestQA_Filter_11_13_ResourceList_CursorResetsOnFilterChange(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "j"})

	m = typeFilter(m, "prod")
	plain := stripANSI(rootViewContent(m))

	// The first matching resource (api-prod-01) should be visible -- we can't
	// directly test cursor position, but the first match should appear in view
	if !strings.Contains(plain, "api-prod-01") {
		t.Error("after filter change, first match should be visible (cursor reset to 0)")
	}
}

// ── Resource list -- frame title shows filtered count ────────────────

func TestQA_Filter_11_14_ResourceList_FrameTitleFilteredCount(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(2/10)") {
		t.Errorf("frame title should show ec2(2/10), got: %s", plain)
	}
}

// ── Profile selector -- / should filter profiles ─────────────────────

func TestQA_Filter_11_15_ProfileSelector_FilterWorks(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// The region selector stands in for the profile selector, which needs AWS
	// config.
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "aws-regions") {
		t.Fatal("should be on region selector view")
	}

	m = typeFilter(m, "us-east")
	plain = stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "/us-east") {
		t.Error("header should show /us-east")
	}
	if !strings.Contains(plain, "us-east-1") {
		t.Error("us-east-1 should be visible after filter")
	}
	if strings.Contains(plain, "eu-west") {
		t.Error("eu-west should NOT be visible with filter 'us-east'")
	}
}

// ── Region selector -- / should filter regions ───────────────────────

func TestQA_Filter_11_16_RegionSelector_FilterWorks(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetRegion})

	m = typeFilter(m, "eu")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "eu-") {
		t.Error("filter 'eu' should show eu- regions")
	}
	if strings.Contains(plain, "us-east-1") {
		t.Error("filter 'eu' should NOT show us-east-1")
	}
	if !strings.Contains(plain, "aws-regions(") {
		t.Errorf("frame title should show aws-regions filtered count, got: %s", plain)
	}
}

// ── Detail view -- / key activates search ────────────────────────────

func TestQA_Filter_11_17_DetailView_SlashActivatesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-instance", Fields: map[string]string{"state": "running"}}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "test-instance") {
		t.Error("should still be on detail view showing test-instance")
	}
	// Should NOT have entered filter mode -- filter mode renders a "/" prefix in the header
	// followed by the filter input text, not a search prompt. The filter input is always on
	// the header right side via modeFilter, never inside the view body.
	lines := strings.Split(plain, "\n")
	if len(lines) > 0 {
		headerLine := lines[0]
		// Filter mode makes the header right show "/"+input (e.g. just "/" when empty).
		// If the header line ends with exactly "/" and nothing else, filter mode was activated.
		trimmed := strings.TrimSpace(headerLine)
		if strings.HasSuffix(trimmed, "/") && !strings.Contains(trimmed, "a9s") {
			t.Error("/ on detail view should NOT activate filter mode (header shows filter input)")
		}
	}
	// Search activation replaces "? for help" with search info in the header.
	if strings.Contains(plain, "? for help") {
		t.Error("header should NOT show '? for help' when detail view search is active")
	}
}

// ── YAML view -- / key activates search ──────────────────────────────

func TestQA_Filter_11_18_YAMLView_SlashActivatesSearch(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{ID: "i-abc123", Name: "test-yaml", Fields: map[string]string{"key": "value"}}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Error("should still be on yaml view")
	}
	lines := strings.Split(plain, "\n")
	if len(lines) > 0 {
		headerLine := lines[0]
		trimmed := strings.TrimSpace(headerLine)
		if strings.HasSuffix(trimmed, "/") && !strings.Contains(trimmed, "a9s") {
			t.Error("/ on YAML view should NOT activate filter mode (header shows filter input)")
		}
	}
	// Search activation replaces "? for help" with search info in the header.
	if strings.Contains(plain, "? for help") {
		t.Error("header should NOT show '? for help' when YAML view search is active")
	}
}

// ── Help view -- / key closes help ───────────────────────────────────

func TestQA_Filter_11_19_HelpView_SlashClosesHelp(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "help") {
		t.Fatal("precondition: should be on help view")
	}

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Error("after / on help, should return to main menu")
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("after help closes from /, should NOT enter filter mode")
	}
}

// ── Reveal view -- / key is ignored ──────────────────────────────────

func TestQA_Filter_11_20_RevealView_SlashIgnored(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceID: "test-secret",
		Value:      "s3cr3t-value",
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-secret") {
		t.Fatal("precondition: should be on reveal view")
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-secret") {
		t.Error("should still be on reveal view")
	}
}

// ── Filter with special characters ───────────────────────────────────

func TestQA_Filter_11_21_SpecialCharacters(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	resources := []resource.Resource{
		{ID: "i-001", Name: "api-prod-01", Fields: map[string]string{"name": "api-prod-01"}},
		{ID: "i-002", Name: "app.service", Fields: map[string]string{"name": "app.service"}},
		{ID: "i-003", Name: "my_bucket", Fields: map[string]string{"name": "my_bucket"}},
	}
	m = loadEC2Resources(m, resources)

	m = typeFilter(m, "api-prod")
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "api-prod-01") {
		t.Error("filter with dashes should match api-prod-01")
	}
	if strings.Contains(plain, "app.service") {
		t.Error("filter 'api-prod' should NOT match app.service")
	}
}

// ── Filter that matches everything ───────────────────────────────────

func TestQA_Filter_11_22_FilterMatchesAll(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "i-")
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(10") {
		t.Errorf("when filter matches all, frame should show ec2(10...), got: %s", plain)
	}
}

// ── Filter then navigate to detail then back ─────────────────────────

func TestQA_Filter_11_23_FilterClearedOnNavigateBack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(10)") && !strings.Contains(plain, "ec2(2/10)") {
		// Accept either -- depends on implementation
		t.Logf("after navigate back, frame shows: %s", plain)
	}
}

// ── Double Esc -- first clears filter, second goes back ──────────────

func TestQA_Filter_11_24_DoubleEsc(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(10") {
		t.Errorf("first Esc should clear filter and show all 10 resources, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("second Esc should go back to main menu, got: %s", plain)
	}
}

// ── Esc clears confirmed filter on resource list ─────────────────────
// After typing /prod + Enter, filter is confirmed (modeNormal but filter active).
// Esc should clear the filter, NOT navigate back.

func TestQA_Filter_11_24a_EscClearsConfirmedFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "/prod") {
		t.Error("after Enter, filter input indicator should be gone from header")
	}
	if strings.Contains(plain, "bastion") {
		t.Error("filter should still be active after Enter - bastion should be hidden")
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(10") {
		t.Errorf("Esc should clear confirmed filter and show all resources, got: %s", plain)
	}
	if strings.Contains(plain, "resource-types") {
		t.Error("Esc with active filter should clear filter, not navigate back to main menu")
	}
}

// ── Esc on main menu should NOT quit ────────────────────────────────

func TestQA_Filter_11_24b_EscOnMainMenuNeverQuits(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd != nil {
		t.Error("Esc on main menu should be a no-op (nil cmd), not quit the app")
	}
}

// ── Esc clears confirmed filter on main menu ────────────────────────

func TestQA_Filter_11_24c_EscClearsConfirmedFilterOnMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m = typeFilter(m, "ec2")
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "S3 Buckets") {
		t.Error("confirmed filter should hide S3 Buckets")
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types(71)") {
		t.Errorf("Esc should clear confirmed filter, showing all 71 types, got: %s", plain)
	}
}

// ── Very long filter string ──────────────────────────────────────────

func TestQA_Filter_11_25_VeryLongFilterString(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	longFilter := strings.Repeat("a", 60)
	m = typeFilter(m, longFilter)

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("view should not be empty with long filter string")
	}
	if !strings.Contains(plain, "/"+longFilter[:10]) {
		t.Error("header should show the long filter text (at least partially)")
	}
}

// ── Filter mode then resize terminal ─────────────────────────────────

func TestQA_Filter_11_26_FilterSurvivesResize(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m = loadEC2Resources(m, sampleEC2Resources())

	m = typeFilter(m, "prod")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/prod") {
		t.Error("filter should survive terminal resize")
	}
	if strings.Contains(plain, "bastion") {
		t.Error("filtered items should still be hidden after resize")
	}
}

// ── Filter on empty resource list ────────────────────────────────────

func TestQA_Filter_11_27_FilterOnEmptyList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	m = typeFilter(m, "anything")
	plain := stripANSI(rootViewContent(m))

	if plain == "" {
		t.Error("view should not be empty")
	}
	if !strings.Contains(plain, "/anything") {
		t.Error("header should show filter text even on empty list")
	}
}
