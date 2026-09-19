package unit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// prevRootModel is the tui.Model (if any) newRootSizedModel handed back on
// its previous call in this binary, kept so the next call can flush its
// headless controller (CloseController, nil-safe) before abandoning its
// config directory.
var prevRootModel *tui.Model

// newRootSizedModel creates a model with a size set so View() actually
// renders.
//
// Every call sets a fresh, unique A9S_CONFIG_FOLDER via os.Setenv (there is
// no *testing.T to call t.Setenv with): callers share the
// "testprofile"/"us-east-1" pair, so a shared directory would leak one
// test's persisted list rows into another's fresh model. A caller that needs
// the model to read pre-seeded files (e.g. a themes/ directory) calls this
// FIRST, then seeds the directory this call resolved
// (os.Getenv("A9S_CONFIG_FOLDER") after it returns); see
// TestStackSync_SelectorFlow / TestStackSync_DoublePopGuard in
// tui_stack_sync_test.go.
//
// Each call also closes the previous call's headless controller before
// abandoning its directory: a queued availability-cache write resolves
// A9S_CONFIG_FOLDER at write time, so a writer left running could land in a
// later test's t.TempDir() and fail its RemoveAll cleanup
// (app_availsave_tempdir_cleanup_race_test.go).
func newRootSizedModel() tui.Model {
	if prevRootModel != nil {
		prevRootModel.CloseController()
		prevRootModel = nil
	}
	if dir, err := os.MkdirTemp("", "a9s-roottest-config-*"); err == nil {
		os.Setenv("A9S_CONFIG_FOLDER", dir) //nolint:errcheck // best-effort per-call isolation, not test-critical
	}
	m := tuitest.Sized("testprofile", "us-east-1")
	prevRootModel = &m
	return m
}

func rootApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	return tuitest.Step(m, msg)
}

func rootViewContent(m tui.Model) string {
	return tuitest.Render(m)
}

func rootKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func rootSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

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
	m := newBlessedModel(t, "default", "us-east-1")
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

func TestRootHandleNavigate_ResourceList(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
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
	m, _ = rootApplyMsg(m, messages.Navigate{
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
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "yaml") {
		t.Errorf("after navigate to YAML, frame title should contain 'yaml', got: %s", plain)
	}
}

func TestRootHandleNavigate_JSON_QQuits(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-abc123",
		Name: "my-instance",
		Fields: map[string]string{
			"State": "running",
		},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		Resource:     res,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetJSON,
		Resource:     res,
		ResourceType: "ec2",
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("q"))
	if cmd == nil {
		t.Fatal("q on JSON view should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q on JSON view should emit tea.QuitMsg, got %T", msg)
	}
}

func TestRootHandleNavigate_Help(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
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

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target: messages.TargetRegion,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "aws-regions") {
		t.Errorf("after navigate to region, frame title should contain 'aws-regions', got: %s", plain)
	}
}

func TestRootPopView_ReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetHelp})

	m, _ = rootApplyMsg(m, messages.PopView{})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after pop, should be back at main menu with 'resource-types', got: %s", plain)
	}
}

func TestRootPopView_CannotPopLastView(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.PopView{})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "resource-types") {
		t.Errorf("should still show main menu after pop on single view, got: %s", plain)
	}
}

func TestRootExecuteCommand_ResourceType(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "ec2" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Error("executeCommand('ec2') should return a command (NavigateMsg)")
	}
}

func TestRootExecuteCommand_Quit(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	m, _ = rootApplyMsg(m, rootKeyPress("q"))

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("executeCommand('q') should return a quit command")
	}
}

func TestRootExecuteCommand_UnknownCommand(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

	for _, r := range "nonsense" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("executeCommand with unknown command should return a command for FlashMsg")
	}
}

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

	m, _ = rootApplyMsg(m, rootKeyPress("/"))

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

	m, _ = rootApplyMsg(m, rootKeyPress(":"))

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

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Copied!", IsError: false})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "Copied!") {
		t.Errorf("after FlashMsg, header should contain 'Copied!', got: %s", plain)
	}
}

func TestRootFetchResources_NilClients(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	// m.clients is nil (no AWS connection)

	_, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	if cmd == nil {
		t.Error("navigating to resource list with nil clients should still return a command")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.APIError:
		// expected
	case messages.ResourcesLoaded:
		t.Error("with nil clients, should not return ResourcesLoadedMsg")
	default:
		// Could be a batch cmd, that's also OK
	}
}

func TestRootNavigateAndPopRoundTrip(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Navigate: MainMenu -> ResourceList -> Detail -> pop -> pop
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	res := &resource.Resource{ID: "my-bucket", Name: "my-bucket"}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, messages.PopView{})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3") {
		t.Errorf("after first pop, should be at s3 resource list, got: %s", plain)
	}

	m, _ = rootApplyMsg(m, messages.PopView{})
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
	m := newBlessedModel(t, "test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 40, Height: 24})
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even when terminal is too narrow")
	}
}

func TestRoot_View_AltScreenOnMinHeight(t *testing.T) {
	m := newBlessedModel(t, "test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 5})
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even when terminal is too short")
	}
}

func TestRoot_View_AltScreenOnZeroWidth(t *testing.T) {
	m := newBlessedModel(t, "test", "us-east-1")
	// No WindowSizeMsg sent, width is 0
	v := m.View()
	if !v.AltScreen {
		t.Error("View() must set AltScreen=true even before terminal size is known")
	}
}

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
	if len(lines) != 40 {
		t.Errorf("expected 40 lines for height 40, got %d", len(lines))
	}
}

func TestRoot_FilterMode_WorksOnMainMenu(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '/'})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'e', Text: "e"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'c', Text: "c"})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: '2', Text: "2"})
	content := rootViewContent(m)
	if !strings.Contains(content, "EC2") {
		t.Error("filter on main menu should show EC2 match")
	}
	if !strings.Contains(content, "/ec2") {
		t.Error("header should show active filter text /ec2")
	}
}

func TestRoot_MainMenu_SelectedRowSingleLine(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	lines := strings.Split(content, "\n")
	// Verify that each resource type name appears on exactly one line (no wrapping).
	sampleNames := []string{
		"EC2 Instances",
		"ECS Services",
		"Lambda Functions",
		"EKS Clusters",
		"EKS Node Groups",
		"Load Balancers",
		"Security Groups",
		"VPCs",
		"DB Instances",
		"S3 Buckets",
	}
	resourceCount := 0
	for _, line := range lines {
		plain := stripANSI(line)
		for _, name := range sampleNames {
			if strings.Contains(plain, name) {
				resourceCount++
				break
			}
		}
	}
	if resourceCount != len(sampleNames) {
		t.Errorf("expected %d resource type lines (one per type), got %d — rows may be wrapping", len(sampleNames), resourceCount)
	}
}

func TestRoot_S3_EnterBucketShowsObjects(t *testing.T) {
	m := newRootSizedModel()
	m, cmd := rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "s3"})
	// The fetch cmd is not run; rows are loaded below.
	_ = cmd
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "s3", Resources: buckets})
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	content := rootViewContent(m)
	plain := stripANSI(content)
	if strings.Contains(plain, "my-bucket") && !strings.Contains(plain, "my-bucket yaml") {
		// Good — we're in an objects view with the bucket name in the title
	} else {
		t.Errorf("Enter on S3 bucket should show objects for my-bucket, got: %s", plain[:min(200, len(plain))])
	}
	if strings.Contains(plain, "No detail") || strings.Contains(plain, "Initializing") {
		t.Error("Enter on S3 bucket should drill into objects list, not show detail/yaml view")
	}
}

func TestRoot_S3_EscapeFromObjectsReturnsToBuckets(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "s3", Resources: buckets, Provenance: messages.FetchProvenanceCanonicalList})

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	content := rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "s3(1)") && !strings.Contains(plain, "my-bucket") {
		t.Errorf("Escape from objects should return to bucket list, got: %s", plain[:min(200, len(plain))])
	}
}

func TestRoot_EnterChildView_UnknownChildType(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// Send EnterChildViewMsg with a child type that is not registered
	_, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     "nonexistent_type",
		ParentContext: map[string]string{"id": "test"},
		DisplayName:   "test",
	})

	if cmd == nil {
		t.Fatal("EnterChildViewMsg with unknown child type should return a command")
	}

	msg := cmd()
	flashMsg, ok := msg.(messages.Flash)
	if !ok {
		t.Fatalf("expected FlashMsg, got %T", msg)
	}
	if !flashMsg.IsError {
		t.Error("FlashMsg.IsError should be true for unknown child type")
	}
	if !strings.Contains(flashMsg.Text, "unknown child type") {
		t.Errorf("FlashMsg.Text should contain 'unknown child type', got %q", flashMsg.Text)
	}
}

func TestRoot_EnterChildView_NilClients(t *testing.T) {
	tui.Version = "0.6.0"
	// Register a temporary child type so handleEnterChildView passes the lookup
	testChildType := "test_nil_clients_child"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Nil Clients Child",
		ShortName: testChildType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(testChildType, func(_ context.Context, clients any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		// This should not be reached if clients are nil — the model checks first
		return resource.FetchResult{}, nil
	})
	defer resource.CleanupChildTypeForTest(testChildType)
	defer resource.CleanupPaginatedChildForTest(testChildType)

	// Create model WITHOUT demo mode and WITHOUT clients (clients == nil)
	m := newRootSizedModel()

	// Send EnterChildViewMsg — handleEnterChildView will push a view and call fetchChildResources
	_, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     testChildType,
		ParentContext: map[string]string{"bucket": "test"},
		DisplayName:   "test",
	})

	if cmd == nil {
		t.Fatal("EnterChildViewMsg should return a command (batch of init + fetch)")
	}

	// Extract the APIErrorMsg from the batch — fetchChildResources should detect nil clients
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.APIError)
		return ok
	})

	apiErr, ok := msg.(messages.APIError)
	if !ok {
		t.Fatalf("expected APIErrorMsg from nil clients fetch, got %T", msg)
	}
	if !strings.Contains(apiErr.Err.Error(), "not initialized") {
		t.Errorf("APIErrorMsg.Err should contain 'not initialized', got %q", apiErr.Err.Error())
	}
}

func TestRoot_EnterChildView_NilParentContext(t *testing.T) {
	tui.Version = "0.6.0"
	testChildType := "test_nil_ctx"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Nil Ctx",
		ShortName: testChildType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(testChildType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	defer resource.CleanupChildTypeForTest(testChildType)
	defer resource.CleanupPaginatedChildForTest(testChildType)

	// Create model in demo mode so we don't need real AWS clients
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	m, cmd := rootApplyMsg(m, messages.EnterChildView{
		ChildType:     testChildType,
		ParentContext: nil,
		DisplayName:   "test",
	})

	content := rootViewContent(m)
	if content == "" {
		t.Error("View() should return non-empty content after entering child view")
	}

	if cmd != nil {
		_ = cmd() //nolint:ineffassign,staticcheck // verifying no panic on execution
	}
}

// longAWSError is a realistic AWS AccessDeniedException message (~250 chars)
// that overflows the header.
const longAWSError = "User: arn:aws:iam::123456789012:user/test-user@example.com is not authorized to perform: kms:DescribeKey on resource: arn:aws:kms:eu-central-1:123456789012:key/abcdef01-2345-6789-abcd-ef0123456789 because no resource-based policy allows the kms:DescribeKey action"

func TestRoot_View_LongErrorNoLineExceedsWidth(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "kms",
		Err:          fmt.Errorf("%s", longAWSError),
	})

	// Get the view WITHOUT a long error for baseline comparison
	baselineModel := newRootSizedModel()
	baselineContent := rootViewContent(baselineModel)
	baselineLines := len(strings.Split(baselineContent, "\n"))

	content := rootViewContent(m)
	errorLines := len(strings.Split(content, "\n"))

	// The long error must not cause additional lines compared to the baseline.
	// When the header wraps, it adds extra lines to the total output.
	if errorLines != baselineLines {
		t.Errorf("long error should not add extra lines: baseline=%d, with error=%d — header is wrapping", baselineLines, errorLines)
	}
}

func TestRoot_View_LongErrorStillOneLine(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "kms",
		Err:          fmt.Errorf("%s", longAWSError),
	})

	content := rootViewContent(m)
	lines := strings.Split(content, "\n")
	// Terminal height is 40, so View() must produce exactly 40 lines.
	// If the header wraps due to a long error, extra lines push content off-screen.
	if len(lines) != 40 {
		t.Errorf("expected exactly 40 lines (terminal height), got %d — long error likely caused header to wrap", len(lines))
	}
}

func TestRoot_View_ErrorTruncatedInHeader(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "kms",
		Err:          fmt.Errorf("%s", longAWSError),
	})

	content := rootViewContent(m)
	// The header must occupy exactly 1 line. Count how many lines before the
	// first frame border character (top-left corner "┌") to detect wrapping.
	lines := strings.Split(content, "\n")
	headerLineCount := 0
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "\u250c") { // ┌ = top-left frame corner
			break
		}
		headerLineCount++
	}
	if headerLineCount != 1 {
		t.Errorf("header should occupy exactly 1 line, but occupies %d lines — long error text is causing the header to wrap", headerLineCount)
	}
}
