package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Helpers: navigate to EC2 list, load fixture data, wide terminal
// ===========================================================================

// newEC2ListModel creates a root model navigated to the EC2 list with
// fixtureEC2Instances loaded. Terminal size is 160x40 to show all columns.
//
// Note on column rendering: fixture data has RawStruct=nil, so config-driven
// column paths (InstanceId, PrivateIpAddress, etc.) fall back to title-matching
// against Fields keys. Columns whose title matches a Fields key exactly
// (case-insensitive) will render: Name, State, Type. Other columns (Instance ID,
// Private IP, Public IP, Launch Time) will be empty because title-matching
// cannot bridge the space-vs-underscore difference.
func newEC2ListModel(t *testing.T) tui.Model {
	t.Helper()
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    fixtureEC2Instances(),
	})
	return m
}

// ===========================================================================
// A. EC2 Instance List View
// ===========================================================================

// A.1 Column Layout

func TestQA_EC2_A1_1_ListColumns_AllSixPresent(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	expected := []string{"Instance ID", "State", "Type", "Private IP", "Public IP", "Launch Time"}
	for _, col := range expected {
		if !strings.Contains(plain, col) {
			t.Errorf("A.1.1: EC2 list should contain column header %q", col)
		}
	}
}

func TestQA_EC2_A1_5_StateColumnData(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	// State column title-matches Fields key "state" -- data renders
	if !strings.Contains(plain, "running") {
		t.Error("A.1.5: list should contain 'running' state")
	}
	if !strings.Contains(plain, "terminated") {
		t.Error("A.1.5: list should contain 'terminated' state")
	}
}

func TestQA_EC2_A1_6_TypeColumnData(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	// Type column title-matches Fields key "type" -- data renders
	expectedTypes := []string{"g4dn.xlarge", "t3.large", "t3.xlarge"}
	for _, et := range expectedTypes {
		if !strings.Contains(plain, et) {
			t.Errorf("A.1.6: list should contain instance type %q", et)
		}
	}
}

func TestQA_EC2_A1_NameColumnData(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	// Name column title-matches Fields key "name" -- data renders
	expectedNames := []string{"VPN", "kafka", "monitoring", "apps-on-demand", "apps"}
	for _, name := range expectedNames {
		if !strings.Contains(plain, name) {
			t.Errorf("A.1: list should contain instance name %q", name)
		}
	}
}

func TestQA_EC2_A1_12_NoPipeSeparators(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	lines := strings.Split(plain, "\n")
	for _, line := range lines {
		// Skip border lines
		if strings.ContainsAny(line, "\u250c\u2510\u2514\u2518\u2500") {
			continue
		}
		trimmed := strings.Trim(line, "\u2502 ")
		if strings.Contains(trimmed, " | ") {
			t.Error("A.1.12: columns should not have pipe separators")
		}
	}
}

// A.2 Frame and Title

func TestQA_EC2_A2_1_FrameTitleShowsResourceTypeAndCount(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(6)") {
		t.Errorf("A.2.1: frame title should show 'ec2(6)', got: %s", plain)
	}
}

func TestQA_EC2_A2_3_FrameUsesBoxDrawingCharacters(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "\u250c") {
		t.Error("A.2.3: frame should contain top-left corner")
	}
	if !strings.Contains(plain, "\u2518") {
		t.Error("A.2.3: frame should contain bottom-right corner")
	}
	if !strings.Contains(plain, "\u2502") {
		t.Error("A.2.3: frame should contain side border")
	}
}

// A.3 Header Bar

func TestQA_EC2_A3_1_HeaderShowsAppIdentityAndContext(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "a9s") {
		t.Error("A.3.1: header should contain 'a9s'")
	}
	if !strings.Contains(plain, "v0.6.0") {
		t.Error("A.3.1: header should contain version")
	}
	if !strings.Contains(plain, "testprofile:us-east-1") {
		t.Error("A.3.1: header should contain profile:region")
	}
}

func TestQA_EC2_A3_2_HeaderShowsHelpHint(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "? for help") {
		t.Error("A.3.2: header should contain '? for help'")
	}
}

// A.4 Status Coloring

func TestQA_EC2_A4_StatusColoring_RunningRowHasANSI(t *testing.T) {
	m := newEC2ListModel(t)
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")

	// Find a line containing "VPN" (running instance, renders via name column)
	var vpnLine string
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "VPN") {
			vpnLine = line
			break
		}
	}
	if vpnLine == "" {
		t.Fatal("A.4.1: could not find VPN instance row")
	}

	// Running rows should have ANSI coloring (green)
	if !strings.Contains(vpnLine, "\x1b[") {
		t.Error("A.4.1: running instance row should have ANSI color codes")
	}
}

func TestQA_EC2_A4_StatusColoring_StoppedRowHasANSI(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	stoppedInstance := resource.Resource{
		ID:     "i-stopped123",
		Name:   "stopped-instance",
		Status: "stopped",
		Fields: map[string]string{
			"name":  "stopped-instance",
			"state": "stopped",
			"type":  "t3.micro",
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{stoppedInstance},
	})

	content := rootViewContent(m)
	plain := stripANSI(content)

	if !strings.Contains(plain, "stopped") {
		t.Error("A.4.2: list should show stopped state")
	}
	if !strings.Contains(content, "\x1b[") {
		t.Error("A.4.2: stopped row should have ANSI color codes")
	}
}

func TestQA_EC2_A4_StatusColoring_TerminatedRowHasANSI(t *testing.T) {
	m := newEC2ListModel(t)
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")

	// The terminated instance has name "apps" and state "terminated"
	var terminatedLine string
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "terminated") {
			terminatedLine = line
			break
		}
	}
	if terminatedLine == "" {
		t.Fatal("A.4.4: could not find a line with 'terminated' state")
	}
	if !strings.Contains(terminatedLine, "\x1b[") {
		t.Error("A.4.4: terminated instance row should have ANSI color codes (dim)")
	}
}

// A.5 Row Selection

func TestQA_EC2_A5_5_FirstRowSelected(t *testing.T) {
	m := newEC2ListModel(t)
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")

	// First data row should have the selected row style (blue background).
	// Find a line containing "g4dn.xlarge" (first instance's type)
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "g4dn.xlarge") {
			// This line should have ANSI styling for selection
			if !strings.Contains(line, "\x1b[") {
				t.Error("A.5.5: first row should have ANSI styling for selection")
			}
			return
		}
	}
	t.Error("A.5.5: could not find first instance (g4dn.xlarge) in output")
}

// A.6 Navigation

func TestQA_EC2_A6_1_DownMovesSelectionDown(t *testing.T) {
	m := newEC2ListModel(t)

	// Press j (down)
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	plain := stripANSI(rootViewContent(m))

	// After moving down, second instance (VPN) data should still be visible
	if !strings.Contains(plain, "VPN") {
		t.Error("A.6.1: after pressing j, VPN instance should be visible")
	}
}

func TestQA_EC2_A6_3_TopJumpsToFirstRow(t *testing.T) {
	m := newEC2ListModel(t)

	// Move down a few times
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	// Press g (go to top)
	m, _ = rootApplyMsg(m, rootKeyPress("g"))

	plain := stripANSI(rootViewContent(m))

	// First instance (g4dn.xlarge) should be visible
	if !strings.Contains(plain, "g4dn.xlarge") {
		t.Error("A.6.3: after pressing g, first instance should be visible")
	}
}

func TestQA_EC2_A6_4_BottomJumpsToLastRow(t *testing.T) {
	m := newEC2ListModel(t)

	// Press G (go to bottom)
	m, _ = rootApplyMsg(m, rootKeyPress("G"))

	plain := stripANSI(rootViewContent(m))

	// Last instance (terminated "apps") should be visible
	if !strings.Contains(plain, "terminated") {
		t.Error("A.6.4: after pressing G, last instance (terminated) should be visible")
	}
}

// A.7 Sort

func TestQA_EC2_A7_1_SortByNameAscending(t *testing.T) {
	m := newEC2ListModel(t)

	// Press N to sort by name ascending
	m, _ = rootApplyMsg(m, rootKeyPress("N"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "\u2191") {
		t.Error("A.7.1: sort ascending should show up-arrow indicator")
	}
}

func TestQA_EC2_A7_2_SortByNameDescending(t *testing.T) {
	m := newEC2ListModel(t)

	// Press N twice to toggle to descending
	m, _ = rootApplyMsg(m, rootKeyPress("N"))
	m, _ = rootApplyMsg(m, rootKeyPress("N"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "\u2193") {
		t.Error("A.7.2: sort descending should show down-arrow indicator")
	}
}

func TestQA_EC2_A7_3_SortByIDAscending(t *testing.T) {
	m := newEC2ListModel(t)

	// Press I to sort by ID ascending
	m, _ = rootApplyMsg(m, rootKeyPress("I"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Instance ID\u2191") {
		t.Error("A.7.3: Instance ID column header should show ascending indicator")
	}
}

func TestQA_EC2_A7_5_SortByAgeAscending(t *testing.T) {
	m := newEC2ListModel(t)

	// Press A to sort by age ascending
	m, _ = rootApplyMsg(m, rootKeyPress("A"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Launch Time\u2191") {
		t.Error("A.7.5: Launch Time column header should show ascending indicator")
	}
}

func TestQA_EC2_A7_7_SortIndicatorExactlyOneColumn(t *testing.T) {
	m := newEC2ListModel(t)

	// Sort by name first, then switch to ID
	m, _ = rootApplyMsg(m, rootKeyPress("N"))
	m, _ = rootApplyMsg(m, rootKeyPress("I"))

	plain := stripANSI(rootViewContent(m))

	arrowCount := strings.Count(plain, "\u2191") + strings.Count(plain, "\u2193")
	if arrowCount != 1 {
		t.Errorf("A.7.7: exactly one sort indicator expected, got %d", arrowCount)
	}
}

// A.8 Filter

func TestQA_EC2_A8_1_FilterModeActivates(t *testing.T) {
	m := newEC2ListModel(t)

	// Press / to enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "/") {
		t.Error("A.8.1: filter mode should show / in header")
	}
}

func TestQA_EC2_A8_2_FilterNarrowsResults(t *testing.T) {
	m := newEC2ListModel(t)

	// Filter by "VPN"
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "VPN" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	// Filter should narrow to 1 result; check the frame title
	if !strings.Contains(plain, "ec2(1/6)") {
		t.Errorf("A.8.2: filter by VPN should narrow to 1/6, got: %s", plain[:min(300, len(plain))])
	}
	// VPN name should be visible in the filtered row
	if !strings.Contains(plain, "VPN") {
		t.Error("A.8.2: filter should show VPN instance name")
	}
}

func TestQA_EC2_A8_3_FilterTitleShowsFilteredCount(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "VPN" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(1/6)") {
		t.Errorf("A.8.3: frame title should show 'ec2(1/6)', got: %s", plain)
	}
}

func TestQA_EC2_A8_5_FilterByState(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "terminated" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	// Should narrow to 1 terminated instance
	if !strings.Contains(plain, "ec2(1/6)") {
		t.Errorf("A.8.5: filter by 'terminated' should narrow to 1/6, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_EC2_A8_6_FilterByInstanceType(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "t3.xlarge" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	// Should find 1 instance (apps-on-demand)
	if !strings.Contains(plain, "ec2(1/6)") {
		t.Errorf("A.8.6: filter by 't3.xlarge' should narrow to 1/6, got: %s", plain[:min(300, len(plain))])
	}
	if !strings.Contains(plain, "apps-on-demand") {
		t.Error("A.8.6: filtered result should show apps-on-demand name")
	}
}

func TestQA_EC2_A8_7_FilterByIPAddress(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "10.0.48" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	// Two instances have IPs starting with 10.0.48
	if !strings.Contains(plain, "ec2(2/6)") {
		t.Errorf("A.8.7: filter by '10.0.48' should narrow to 2/6, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_EC2_A8_8_FilterCaseInsensitive(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "RUNNING" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	// Should match running instances (5 running in fixture)
	if !strings.Contains(plain, "ec2(5/6)") {
		t.Errorf("A.8.8: case-insensitive filter 'RUNNING' should narrow to 5/6, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_EC2_A8_9_FilterNoMatchesEmptyTable(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "zzzznotexist" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(0/6)") {
		t.Errorf("A.8.9: no matches should show 'ec2(0/6)', got: %s", plain)
	}
}

func TestQA_EC2_A8_11_EscClearsFilter(t *testing.T) {
	m := newEC2ListModel(t)

	// Enter filter and type something
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "VPN" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Press Escape to clear filter
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(6)") {
		t.Errorf("A.8.11: after Esc, frame title should revert to 'ec2(6)', got: %s", plain)
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("A.8.11: after Esc, header should revert to '? for help'")
	}
}

// A.9 Command Mode

func TestQA_EC2_A9_1_CommandModeActivates(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, ":") {
		t.Error("A.9.1: command mode should show : in header")
	}
}

func TestQA_EC2_A9_5_EscCancelsCommandMode(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("s"))
	m, _ = rootApplyMsg(m, rootKeyPress("3"))

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(6)") {
		t.Error("A.9.5: Esc during command mode should stay on EC2 list")
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("A.9.5: after canceling command mode, header should show '? for help'")
	}
}

// A.10 Actions from List

func TestQA_EC2_A10_1_EnterOpensDetailView(t *testing.T) {
	m := newEC2ListModel(t)

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))

	// First instance has no Name, so frame title shows the ID
	if !strings.Contains(plain, "i-0aaa111111111111a") {
		t.Errorf("A.10.1: Enter should open detail view for first instance, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_EC2_A10_3_YOpensYAMLView(t *testing.T) {
	m := newEC2ListModel(t)

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("A.10.3: pressing y should open YAML view, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_EC2_A10_4_CopyCopiesInstanceID(t *testing.T) {
	m := newEC2ListModel(t)

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("A.10.4: pressing c should return a command for copy")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("A.10.4: expected FlashMsg, got %T", msg)
	}
	// The flash should reference the instance ID (copy success or clipboard error)
	if !flash.IsError && !strings.Contains(flash.Text, "i-0aaa111111111111a") {
		t.Errorf("A.10.4: flash should mention instance ID, got: %s", flash.Text)
	}
}

func TestQA_EC2_A10_6_EscReturnsToMainMenu(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("A.10.6: Esc should return to main menu, got: %s", plain[:min(200, len(plain))])
	}
}

// A.11 Edge Cases: Missing Data

func TestQA_EC2_A11_1_InstanceWithNoPublicIP(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	// kafka instance has no public IP but should still be visible (has name "kafka")
	if !strings.Contains(plain, "kafka") {
		t.Error("A.11.1: instance with no public IP should still appear in list")
	}
}

func TestQA_EC2_A11_2_InstanceWithNoNameTag(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	// First instance has Name="" -- its row should appear with its type
	if !strings.Contains(plain, "g4dn.xlarge") {
		t.Error("A.11.2: instance with no Name should still appear with its type")
	}
}

func TestQA_EC2_A11_3_TerminatedInstancesAppearInList(t *testing.T) {
	m := newEC2ListModel(t)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "terminated") {
		t.Error("A.11.3: terminated instance should appear in the list")
	}
}

// A.12 Empty and Error States

func TestQA_EC2_A12_1_EmptyInstanceList(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(0)") {
		t.Errorf("A.12.1: empty list should show 'ec2(0)', got: %s", plain[:min(200, len(plain))])
	}
}

// A.13 Loading State

func TestQA_EC2_A13_1_LoadingState(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	// Don't send ResourcesLoadedMsg -- loading state

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "Loading") {
		t.Errorf("A.13.1: before data loads, should show Loading indicator, got: %s", plain[:min(200, len(plain))])
	}
}

// A.14 Responsive Behavior

func TestQA_EC2_A14_1_TerminalTooNarrow(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 50, Height: 40})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "narrow") {
		t.Errorf("A.14.1: terminal < 60 should show narrow error, got: %s", plain)
	}
}

func TestQA_EC2_A14_5_TerminalTooShort(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 5})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "short") {
		t.Errorf("A.14.5: terminal < 7 lines should show short error, got: %s", plain)
	}
}

// ===========================================================================
// B. EC2 Detail View
// ===========================================================================

func newEC2DetailModel(t *testing.T, r resource.Resource) tui.Model {
	t.Helper()
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &r,
	})
	return m
}

func TestQA_EC2_B1_1_DetailViewOpensWithName(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2DetailModel(t, instances[1]) // VPN

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "VPN") {
		t.Errorf("B.1.1: detail frame title should contain 'VPN', got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_B1_1_DetailViewFallsBackToID(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2DetailModel(t, instances[0]) // No name

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "i-0aaa111111111111a") {
		t.Errorf("B.1.1: detail frame title should fall back to ID when no name, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_B2_DetailFieldsDisplayed(t *testing.T) {
	instances := fixtureEC2Instances()
	inst := instances[1] // VPN
	m := newEC2DetailModel(t, inst)

	plain := stripANSI(rootViewContent(m))

	// Detail renders from Fields map (sorted alphabetically)
	expectedValues := []string{
		"i-0bbb222222222222b",
		"running",
		"t3.large",
		"10.0.48.175",
		"203.0.113.10",
	}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("B.2: detail view should contain value %q", val)
		}
	}
}

func TestQA_EC2_B3_1_DetailKeysStyledWithANSI(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2DetailModel(t, instances[1])

	content := rootViewContent(m)

	if !strings.Contains(content, "\x1b[") {
		t.Error("B.3.1: detail keys should have ANSI color styling")
	}
}

func TestQA_EC2_B8_1_YKeyFromDetailOpensYAML(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2DetailModel(t, instances[1])

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("B.8.1: pressing y from detail should open YAML view, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_B8_3_EscFromDetailReturnsToList(t *testing.T) {
	m := newEC2ListModel(t)

	// Enter detail view
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Esc back
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(6)") {
		t.Errorf("B.8.3: Esc from detail should return to EC2 list, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_B9_1_DetailWithNoPublicIP(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2DetailModel(t, instances[2]) // kafka, no public IP

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "kafka") {
		t.Error("B.9.1: detail should display instance with no public IP")
	}
}

func TestQA_EC2_Detail_AllFieldsFromFixture(t *testing.T) {
	instances := fixtureEC2Instances()
	for _, inst := range instances {
		t.Run(inst.ID, func(t *testing.T) {
			m := newEC2DetailModel(t, inst)
			plain := stripANSI(rootViewContent(m))

			for key, val := range inst.Fields {
				if val == "" {
					continue
				}
				if !strings.Contains(plain, key) && !strings.Contains(plain, val) {
					t.Errorf("detail view for %s should contain field %q or value %q", inst.ID, key, val)
				}
			}
		})
	}
}

// ===========================================================================
// C. EC2 YAML View
// ===========================================================================

func newEC2YAMLModel(t *testing.T, r resource.Resource) tui.Model {
	t.Helper()
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: &r,
	})
	return m
}

func TestQA_EC2_C1_1_YAMLViewOpens(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[1])

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("C.1.1: YAML view frame title should contain 'yaml', got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_C1_2_YAMLFrameTitleShowsName(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[1])

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "VPN yaml") {
		t.Errorf("C.1.2: YAML frame title should contain 'VPN yaml', got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_C2_YAMLContentContainsFields(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[1])

	plain := stripANSI(rootViewContent(m))

	expectedKeys := []string{"instance_id", "state", "type", "private_ip", "public_ip", "launch_time"}
	for _, key := range expectedKeys {
		if !strings.Contains(plain, key) {
			t.Errorf("C.2: YAML view should contain key %q", key)
		}
	}
}

func TestQA_EC2_C2_YAMLContentContainsValues(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[1])

	plain := stripANSI(rootViewContent(m))

	expectedValues := []string{"i-0bbb222222222222b", "running", "t3.large", "10.0.48.175", "203.0.113.10"}
	for _, val := range expectedValues {
		if !strings.Contains(plain, val) {
			t.Errorf("C.2: YAML view should contain value %q", val)
		}
	}
}

func TestQA_EC2_C3_SyntaxColoring_HasANSI(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[1])

	content := rootViewContent(m)

	if !strings.Contains(content, "\x1b[") {
		t.Error("C.3: YAML content should have ANSI color codes for syntax coloring")
	}

	// Count lines with ANSI to confirm coloring is widespread
	lines := strings.Split(content, "\n")
	ansiCount := 0
	for _, line := range lines {
		if strings.Contains(line, "\x1b[") {
			ansiCount++
		}
	}
	if ansiCount < 3 {
		t.Error("C.3: YAML view should have syntax coloring on multiple lines")
	}
}

func TestQA_EC2_C3_SyntaxColoring_KeysVsValues(t *testing.T) {
	// Verify coloring through a YAML model with known data.
	r := resource.Resource{
		ID:     "i-colortest",
		Name:   "colortest",
		Status: "running",
		Fields: map[string]string{
			"instance_id": "i-colortest",
			"state":       "running",
		},
	}
	m := newEC2YAMLModel(t, r)
	content := rootViewContent(m)

	lines := strings.Split(content, "\n")
	coloredKeyLines := 0
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "instance_id:") || strings.Contains(plain, "state:") {
			if strings.Contains(line, "\x1b[") {
				coloredKeyLines++
			}
		}
	}
	if coloredKeyLines == 0 {
		t.Error("C.3: YAML key-value lines should have ANSI syntax coloring")
	}
}

func TestQA_EC2_C6_1_EscFromYAMLReturnsToList(t *testing.T) {
	m := newEC2ListModel(t)

	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2(6)") {
		t.Errorf("C.6.1: Esc from YAML should return to EC2 list, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_C7_2_YAMLWithNoPublicIP(t *testing.T) {
	instances := fixtureEC2Instances()
	m := newEC2YAMLModel(t, instances[2]) // kafka, no public IP

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Error("C.7.2: YAML view should render for instance with no public IP")
	}
	if !strings.Contains(plain, "kafka") {
		t.Error("C.7.2: YAML view should contain instance name")
	}
}

func TestQA_EC2_YAML_AllFixtureInstances(t *testing.T) {
	instances := fixtureEC2Instances()
	for _, inst := range instances {
		t.Run(inst.ID, func(t *testing.T) {
			m := newEC2YAMLModel(t, inst)
			plain := stripANSI(rootViewContent(m))

			if !strings.Contains(plain, "yaml") {
				t.Errorf("YAML view for %s should contain 'yaml' in frame title", inst.ID)
			}
		})
	}
}

func TestQA_EC2_YAML_FieldsMapRendersCorrectly(t *testing.T) {
	r := resource.Resource{
		ID:     "i-test123",
		Name:   "test-instance",
		Status: "running",
		Fields: map[string]string{
			"instance_id": "i-test123",
			"state":       "running",
			"type":        "t3.medium",
			"private_ip":  "10.0.0.1",
			"public_ip":   "54.1.2.3",
			"launch_time": "2026-01-01T00:00:00Z",
		},
	}

	m := newEC2YAMLModel(t, r)
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "i-test123") {
		t.Error("YAML view should contain instance_id value")
	}
	if !strings.Contains(plain, "t3.medium") {
		t.Error("YAML view should contain type value")
	}
}

// ===========================================================================
// D. Cross-View Navigation Flows
// ===========================================================================

func TestQA_EC2_D1_FullNavigationStack(t *testing.T) {
	tui.Version = "0.6.0"
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 160, Height: 40})

	// Start at main menu
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatal("D.1: should start at main menu")
	}

	// Navigate to EC2 list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    fixtureEC2Instances(),
	})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(6)") {
		t.Fatal("D.1: should be at EC2 list")
	}

	// Enter detail view
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	plain = stripANSI(rootViewContent(m))
	// Detail view should show fields
	if strings.Contains(plain, "ec2(6)") {
		t.Fatal("D.1: should have left the list view")
	}

	// Enter YAML view from detail
	m, cmd = rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatal("D.1: should be at YAML view")
	}

	// Esc back to detail
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "yaml") {
		t.Fatal("D.1: Esc from YAML should return to detail")
	}

	// Esc back to list
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(6)") {
		t.Fatal("D.1: Esc from detail should return to EC2 list")
	}

	// Esc back to main menu
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatal("D.1: Esc from EC2 list should return to main menu")
	}
}

func TestQA_EC2_D2_ListToYAMLAndBack(t *testing.T) {
	m := newEC2ListModel(t)

	// y from list goes to YAML
	m, cmd := rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatal("D.2: y from list should go to YAML view")
	}

	// Esc should return to list, not detail
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(6)") {
		t.Errorf("D.2: Esc from YAML should return to list, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_D4_SelectDifferentInstancesThenDetail(t *testing.T) {
	m := newEC2ListModel(t)

	// Open detail for first instance
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	// First instance has no name -- title is the ID
	if !strings.Contains(plain, "i-0aaa111111111111a") {
		t.Fatal("D.4: detail should show first instance")
	}

	// Go back to list
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Move to second instance (VPN)
	m, _ = rootApplyMsg(m, rootKeyPress("j"))

	// Open detail
	m, cmd = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "VPN") {
		t.Errorf("D.4: detail should show VPN instance, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_EC2_D8_DetailToYAMLAndBackToDetail(t *testing.T) {
	m := newEC2ListModel(t)

	// Enter detail view
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Go to YAML from detail
	m, cmd = rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Fatal("D.8: should be in YAML view")
	}

	// Esc should return to detail, not list
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	plain = stripANSI(rootViewContent(m))

	if strings.Contains(plain, "ec2(6)") {
		t.Error("D.8: Esc from YAML should return to detail, not list")
	}
	if strings.Contains(plain, "yaml") {
		t.Error("D.8: Esc from YAML should not stay on YAML")
	}
}

// ===========================================================================
// E. FilterResources unit tests (direct function test)
// ===========================================================================

func TestQA_EC2_FilterResources_ByInstanceID(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("i-0aaa", instances)
	if len(result) != 1 {
		t.Errorf("filter by 'i-0aaa' should return 1 instance, got %d", len(result))
	}
	if len(result) == 1 && result[0].ID != "i-0aaa111111111111a" {
		t.Errorf("filtered instance should be i-0aaa111111111111a, got %s", result[0].ID)
	}
}

func TestQA_EC2_FilterResources_ByStatus(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("terminated", instances)
	if len(result) != 1 {
		t.Errorf("filter by 'terminated' should return 1 instance, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_ByType(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("g4dn", instances)
	if len(result) != 1 {
		t.Errorf("filter by 'g4dn' should return 1 instance, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_ByIP(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("10.0.48", instances)
	if len(result) != 2 {
		t.Errorf("filter by '10.0.48' should return 2 instances, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_CaseInsensitive(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("RUNNING", instances)
	runningCount := 0
	for _, inst := range instances {
		if inst.Status == "running" {
			runningCount++
		}
	}
	if len(result) != runningCount {
		t.Errorf("case-insensitive filter 'RUNNING' should return %d instances, got %d", runningCount, len(result))
	}
}

func TestQA_EC2_FilterResources_NoMatch(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("zzzznotexist", instances)
	if len(result) != 0 {
		t.Errorf("filter with no match should return 0, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_EmptyQuery(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("", instances)
	if len(result) != len(instances) {
		t.Errorf("empty filter should return all %d instances, got %d", len(instances), len(result))
	}
}

func TestQA_EC2_FilterResources_ByName(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("kafka", instances)
	if len(result) != 1 {
		t.Errorf("filter by 'kafka' should return 1 instance, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_ByPublicIP(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("203.0.113.20", instances)
	if len(result) != 1 {
		t.Errorf("filter by '203.0.113.20' should return 1 instance, got %d", len(result))
	}
}

func TestQA_EC2_FilterResources_ByLaunchTime(t *testing.T) {
	instances := fixtureEC2Instances()
	result := views.FilterResources("2026-03-17", instances)
	if len(result) != 1 {
		t.Errorf("filter by '2026-03-17' should return 1 instance, got %d", len(result))
	}
}

// ===========================================================================
// Flash message
// ===========================================================================

func TestQA_EC2_FlashMsgAfterCopy(t *testing.T) {
	m := newEC2ListModel(t)

	m, _ = rootApplyMsg(m, messages.FlashMsg{Text: "Copied!", IsError: false})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Copied!") {
		t.Error("flash message 'Copied!' should appear in header after copy")
	}
}
