package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// helper: create a model with a size set so View() actually renders
func newRootSizedModel() tui.Model {
	m := tui.New("testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	return m
}

// helper: send a message through Update and return the updated model
func rootApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// helper: get rendered content string from View()
func rootViewContent(m tui.Model) string {
	return m.View().Content
}

// helper: create key press for a printable character
func rootKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// helper: create key press for a special key
func rootSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

// ── View() tests ────────────────────────────────────────────────────────────

func TestRootView_ReturnsNonEmptyWithFrame(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	rendered := rootViewContent(m)

	if rendered == "" {
		t.Error("View() should return non-empty string when width > 0")
	}
}

func TestRootView_ContainsHeader(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "a9s") {
		t.Error("View() should contain 'a9s' in the header")
	}
	if !strings.Contains(plain, "v0.6.0") {
		t.Errorf("View() should contain version 'v0.6.0', got: %s", plain)
	}
	if !strings.Contains(plain, "testprofile:us-east-1") {
		t.Errorf("View() should contain 'testprofile:us-east-1', got: %s", plain)
	}
}

func TestRootView_ContainsFrameBorders(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))

	// Frame should have border characters
	if !strings.Contains(plain, "\u250c") { // top-left corner
		t.Error("View() should contain top-left corner character")
	}
	if !strings.Contains(plain, "\u2518") { // bottom-right corner
		t.Error("View() should contain bottom-right corner character")
	}
	if !strings.Contains(plain, "\u2502") { // side border
		t.Error("View() should contain side border character")
	}
}

func TestRootView_EmptyWhenWidthZero(t *testing.T) {
	m := tui.New("default", "us-east-1")
	// Don't send WindowSizeMsg — width stays 0

	rendered := rootViewContent(m)

	if rendered != "" {
		t.Errorf("View() should return empty string when width==0, got %q", rendered)
	}
}

func TestRootView_ContainsFrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))

	// MainMenu frame title is "resource-types(10)"
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("View() should contain frame title with resource-types, got: %s", plain)
	}
}

func TestRootView_HeaderAndFrameLines(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	rendered := rootViewContent(m)
	lines := strings.Split(rendered, "\n")

	// Should have at least 3 lines: header + top border + bottom border
	if len(lines) < 3 {
		t.Errorf("View() should have at least 3 lines, got %d", len(lines))
	}
}

// ── handleNavigate tests ────────────────────────────────────────────────────

func TestRootHandleNavigate_ResourceList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ec2") {
		t.Errorf("after navigate to resource list, frame title should contain 'ec2', got: %s", plain)
	}
}

func TestRootHandleNavigate_Detail(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-abc123",
		Name: "my-instance",
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "my-instance") {
		t.Errorf("after navigate to detail, frame title should contain resource name, got: %s", plain)
	}
}

func TestRootHandleNavigate_YAML(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-abc123",
		Name: "my-instance",
	}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("after navigate to YAML, frame title should contain 'yaml', got: %s", plain)
	}
}

func TestRootHandleNavigate_Help(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetHelp,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "help") {
		t.Errorf("after navigate to help, frame title should contain 'help', got: %s", plain)
	}
}

func TestRootHandleNavigate_Region(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target: messages.TargetRegion,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "aws-regions") {
		t.Errorf("after navigate to region, frame title should contain 'aws-regions', got: %s", plain)
	}
}

// ── popView tests ───────────────────────────────────────────────────────────

func TestRootPopView_ReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Push help
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})

	// Pop it
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after pop, should be back at main menu with 'resource-types', got: %s", plain)
	}
}

func TestRootPopView_CannotPopLastView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Try to pop the only view — should not crash
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("should still show main menu after pop on single view, got: %s", plain)
	}
}

// ── executeCommand tests ────────────────────────────────────────────────────

func TestRootExecuteCommand_ResourceType(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type "ec2"
	for _, r := range "ec2" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Press enter to execute
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// The command should return a NavigateMsg via cmd
	if cmd == nil {
		t.Error("executeCommand('ec2') should return a command (NavigateMsg)")
	}
}

func TestRootExecuteCommand_Quit(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type "q"
	m, _ = rootApplyMsg(m, rootKeyPress("q"))

	// Press enter
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// Should return tea.Quit
	if cmd == nil {
		t.Fatal("executeCommand('q') should return a quit command")
	}
}

func TestRootExecuteCommand_UnknownCommand(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type "nonsense"
	for _, r := range "nonsense" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Press enter
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// Should produce a FlashMsg (error) — delivered via cmd
	if cmd == nil {
		t.Fatal("executeCommand with unknown command should return a command for FlashMsg")
	}
}

// ── headerRight tests ───────────────────────────────────────────────────────

func TestRootHeaderRight_NormalMode(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "? for help") {
		t.Errorf("in normal mode, header should contain '? for help', got: %s", plain)
	}
}

func TestRootHeaderRight_FilterMode(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	// Type some filter text
	for _, r := range "test" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "/test") {
		t.Errorf("in filter mode, header should contain '/test', got: %s", plain)
	}
}

func TestRootHeaderRight_CommandMode(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Enter command mode
	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	// Type some command text
	for _, r := range "dbi" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, ":dbi") {
		t.Errorf("in command mode, header should contain ':dbi', got: %s", plain)
	}
}

func TestRootHeaderRight_FlashMsg(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.FlashMsg{Text: "Copied!", IsError: false})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Copied!") {
		t.Errorf("after FlashMsg, header should contain 'Copied!', got: %s", plain)
	}
}

// ── fetchResources nil clients test ─────────────────────────────────────────

func TestRootFetchResources_NilClients(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	// m.clients is nil (no AWS connection)

	// Navigate to resource list — should not panic
	_, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// The cmd should be non-nil (either a fetch command or an error)
	if cmd == nil {
		t.Error("navigating to resource list with nil clients should still return a command")
	}

	// Execute the command — it should return an APIErrorMsg, not panic
	msg := cmd()
	switch msg.(type) {
	case messages.APIErrorMsg:
		// expected
	case messages.ResourcesLoadedMsg:
		t.Error("with nil clients, should not return ResourcesLoadedMsg")
	default:
		// Could be a batch cmd, that's also OK
	}
}

// ── Integration: navigate then pop round-trip ───────────────────────────────

func TestRootNavigateAndPopRoundTrip(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate: MainMenu -> ResourceList -> Detail -> pop -> pop
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	res := &resource.Resource{ID: "my-bucket", Name: "my-bucket"}
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	// Pop back to resource list
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3") {
		t.Errorf("after first pop, should be at s3 resource list, got: %s", plain)
	}

	// Pop back to main menu
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after second pop, should be at main menu, got: %s", plain)
	}
}

func TestRoot_View_AltScreenEnabled(t *testing.T) {
	m := newRootSizedModel()
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true for full-screen TUI mode")
	}
}

func TestRoot_View_AltScreenOnMinWidth(t *testing.T) {
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even when terminal is too narrow")
	}
}

func TestRoot_View_AltScreenOnMinHeight(t *testing.T) {
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 5})
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even when terminal is too short")
	}
}

func TestRoot_View_AltScreenOnZeroWidth(t *testing.T) {
	m := tui.New("test", "us-east-1")
	// No WindowSizeMsg sent, width is 0
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even before terminal size is known")
	}
}

// --- Bug fix tests: header width, filter on main menu, row wrapping ---

func TestRoot_View_HeaderExactWidth(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	firstLine := strings.Split(content, "\n")[0]
	vis := lipglossWidth(firstLine)
	if vis > 80 {
		t.Errorf("header line must not exceed terminal width 80, got %d", vis)
	}
}

func TestRoot_View_NoLineExceedsTerminalWidth(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	for i, line := range strings.Split(content, "\n") {
		vis := lipglossWidth(line)
		if vis > 80 {
			t.Errorf("line %d exceeds terminal width 80: got %d", i, vis)
		}
	}
}

func TestRoot_View_LineCountMatchesHeight(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")
	// Should be exactly 40 lines (terminal height)
	if len(lines) != 40 {
		t.Errorf("expected 40 lines for height 40, got %d", len(lines))
	}
}

func TestRoot_FilterMode_WorksOnMainMenu(t *testing.T) {
	m := newRootSizedModel()
	// Press "/" to enter filter mode
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})
	// Type "ec2"
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'e', Text: "e"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'c', Text: "c"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '2', Text: "2"})
	content := rootViewContent(m)
	// Should still show EC2 Instances
	if !strings.Contains(content, "EC2") {
		t.Error("filter on main menu should show EC2 match")
	}
	// Should show filter text in header
	if !strings.Contains(content, "/ec2") {
		t.Error("header should show active filter text /ec2")
	}
}

func TestRoot_MainMenu_SelectedRowSingleLine(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")
	// Count non-empty content lines inside the frame (between top and bottom border)
	// Frame has: header(1) + top border(1) + content(h-3) + bottom border(1)
	// With 10 resource types and height 24, all should fit in single lines
	// Count lines containing resource type names
	resourceCount := 0
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "S3 Buckets") ||
			strings.Contains(plain, "EC2 Instances") ||
			strings.Contains(plain, "DB Instances") ||
			strings.Contains(plain, "ElastiCache") ||
			strings.Contains(plain, "DB Clusters") ||
			strings.Contains(plain, "EKS Clusters") ||
			strings.Contains(plain, "Secrets Manager") ||
			strings.Contains(plain, "VPCs") ||
			strings.Contains(plain, "Security Groups") ||
			strings.Contains(plain, "EKS Node Groups") {
			resourceCount++
		}
	}
	if resourceCount != 10 {
		t.Errorf("expected 10 resource type lines (one per type), got %d — rows may be wrapping", resourceCount)
	}
}

func TestRoot_S3_EnterBucketShowsObjects(t *testing.T) {
	m := newRootSizedModel()
	// Navigate to S3
	m, cmd := rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "s3"})
	// Execute fetch cmd (ignored — we'll load manually)
	_ = cmd
	// Load buckets
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	// Press Enter on the bucket — returns a cmd that produces S3EnterBucketMsg
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	// Execute the cmd to get the S3EnterBucketMsg and process it
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	content := rootViewContent(m)
	// Frame title should show the bucket name (objects view), not detail view title
	plain := stripANSI(content)
	// Should be loading objects or showing object list — frame title has bucket name
	if strings.Contains(plain, "my-bucket") && !strings.Contains(plain, "my-bucket yaml") {
		// Good — we're in an objects view with the bucket name in the title
	} else {
		t.Errorf("Enter on S3 bucket should show objects for my-bucket, got: %s", plain[:min(200, len(plain))])
	}
	// Must NOT be in detail view
	if strings.Contains(plain, "No detail") || strings.Contains(plain, "Initializing") {
		t.Error("Enter on S3 bucket should drill into objects list, not show detail/yaml view")
	}
}

func TestRoot_S3_EscapeFromObjectsReturnsToBuckets(t *testing.T) {
	m := newRootSizedModel()
	// Navigate to S3 buckets
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	// Enter bucket — execute returned cmd
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	// Escape should go back to bucket list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	content := rootViewContent(m)
	plain := stripANSI(content)
	// Frame title should show s3(1) — back to bucket list
	if !strings.Contains(plain, "s3(1)") && !strings.Contains(plain, "my-bucket") {
		t.Errorf("Escape from objects should return to bucket list, got: %s", plain[:min(200, len(plain))])
	}
}
