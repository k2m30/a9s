package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// QA S3 helpers
// ===========================================================================

// s3BucketTypeDef returns the S3 bucket type definition (matches resource.FindResourceType("s3")).
func s3BucketTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Aliases:   []string{"s3", "buckets"},
		Columns: []resource.Column{
			{Key: "name", Title: "Bucket Name", Width: 40, Sortable: true},
			{Key: "creation_date", Title: "Creation Date", Width: 22, Sortable: true},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "ID"},
			DisplayNameKey: "bucket",
		}},
	}
}

// s3LoadedBucketModel creates a root TUI model navigated to S3 with buckets loaded.
func s3LoadedBucketModel() tui.Model {
	tui.Version = "0.6.0"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    fixtureS3Buckets(),
	})
	return m
}

// s3LoadedObjectModel creates a root TUI model navigated to S3 -> bucket -> objects loaded.
func s3LoadedObjectModel() tui.Model {
	m := s3LoadedBucketModel()
	// Press Enter to drill into first bucket
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	// Load objects (child list type is s3_objects)
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3_objects",
		Resources:    fixtureS3Objects(),
	})
	return m
}

// s3RLBucketModel creates a standalone ResourceListModel for S3 buckets with data loaded.
func s3RLBucketModel() views.ResourceListModel {
	td := s3BucketTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    fixtureS3Buckets(),
	})
	return m
}

// s3RLObjectModel creates a standalone ResourceListModel for S3 objects inside a bucket.
func s3RLObjectModel(bucket string) views.ResourceListModel {
	k := keys.Default()
	childDef := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r resource.Resource) bool { return r.Fields["status"] == "folder" },
		}},
	}
	m := views.NewChildResourceList(childDef, map[string]string{"bucket": bucket}, bucket, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoaded{
		ResourceType: "s3_objects",
		Resources:    fixtureS3Objects(),
	})
	return m
}

// s3KeyPress creates a tea.KeyPressMsg for a printable character.
func s3KeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ===========================================================================
// A. S3 Bucket List View
// ===========================================================================

// A.8 Enter Key (Drill Into Bucket)

func TestQA_S3_A8_1_EnterOnBucket_SendsEnterChildViewMsg(t *testing.T) {
	m := s3RLBucketModel()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 bucket should produce a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("Enter on S3 bucket should produce EnterChildViewMsg, got %T", msg)
	}

	expected := fixtureS3Buckets()[0].ID
	if childMsg.ParentContext["bucket"] != expected {
		t.Errorf("EnterChildViewMsg.ParentContext[bucket] should be %q, got %q", expected, childMsg.ParentContext["bucket"])
	}
	if childMsg.ChildType != "s3_objects" {
		t.Errorf("EnterChildViewMsg.ChildType should be 's3_objects', got %q", childMsg.ChildType)
	}
}

func TestQA_S3_A8_2_EnterOnBucket_DoesNotSendTargetDetail(t *testing.T) {
	m := s3RLBucketModel()

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 bucket should produce a command")
	}

	msg := cmd()
	if nav, ok := msg.(messages.Navigate); ok {
		if nav.Target == messages.TargetDetail {
			t.Error("Enter on S3 bucket must NOT send TargetDetail NavigateMsg (it should drill into objects)")
		}
	}
	// Verify it's EnterChildViewMsg
	if _, ok := msg.(messages.EnterChildView); !ok {
		t.Errorf("Enter on S3 bucket should produce EnterChildViewMsg, got %T", msg)
	}
}

// A.13 Escape returns to main menu

func TestQA_S3_A13_Escape_ReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Press Escape to go back to main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("Escape from S3 bucket list should return to main menu, got: %s", plain[:min(200, len(plain))])
	}
}

// ===========================================================================
// B. S3 Object List View
// ===========================================================================

// B.14 Escape (Back to Bucket List)

func TestQA_S3_B14_1_Escape_FromObjectsReturnsToBuckets(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	// Press Escape to go back to bucket list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	// Should be back at bucket list, showing s3(5)
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("Escape from object list should return to bucket list with s3(5), got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_B14_1_Escape_FromObjects_DoesNotReturnToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	// Press Escape once -- should go to bucket list, not main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "resource-types") {
		t.Error("single Escape from object list should NOT go to main menu; should go to bucket list")
	}
}

// ===========================================================================
// C. S3 Detail View
// ===========================================================================

// C.1 Bucket Detail (via d from bucket list — d always opens detail view)

func TestQA_S3_C1_BucketDetail_ViaDetailCommand(t *testing.T) {
	m := s3RLBucketModel()

	// The 'd' key always opens the detail view (never drills into S3).
	_, cmd := m.Update(s3KeyPress("d"))
	if cmd == nil {
		t.Fatal("d key on S3 bucket should produce a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("d on S3 bucket should produce NavigateMsg for detail, got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("d on S3 bucket should navigate to detail, got target: %d", nav.Target)
	}
}

// C.2 Object Detail (via Enter or d from object list)

func TestQA_S3_C2_ObjectDetail_EnterSendsDetail(t *testing.T) {
	m := s3RLObjectModel("test-app-state")

	// For objects inside a bucket (s3_objects), Enter sends TargetDetail
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 object should produce a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("Enter on S3 object should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetDetail {
		t.Errorf("Enter on S3 object should target Detail view, got target: %d", nav.Target)
	}
	if nav.Resource == nil {
		t.Fatal("NavigateMsg.Resource should not be nil")
	}
	if nav.Resource.ID != "dev/terraform.tfstate" {
		t.Errorf("detail resource ID should be 'dev/terraform.tfstate', got: %q", nav.Resource.ID)
	}
}

// C.3 Detail View Navigation -- tested via root model

func TestQA_S3_C3_ObjectDetail_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	// Press Enter to go to detail of the first object
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	// The detail view frame title should show the object key/name
	if !strings.Contains(plain, "terraform") {
		t.Errorf("detail view should show object name in frame title, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_C3_DetailView_EscapeReturnsToObjectList(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	// Go to detail
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	// Escape from detail
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	// Should be back at object list with the bucket name visible
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("Escape from detail should return to object list, got: %s", plain[:min(300, len(plain))])
	}
}

// ===========================================================================
// D. Cross-Cutting / Full Flow Tests
// ===========================================================================

// D.2 View Stack: Main Menu -> S3 Bucket List -> Object List -> Detail -> Escape chain

func TestQA_S3_D2_1_FullFlowStack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// 1. Verify we start at main menu
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatalf("should start at main menu, got: %s", plain[:min(200, len(plain))])
	}

	// 2. Navigate to S3 bucket list
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    fixtureS3Buckets(),
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Fatalf("should be at S3 bucket list with s3(5), got: %s", plain[:min(200, len(plain))])
	}

	// 3. Enter bucket -> object list
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3_objects",
		Resources:    fixtureS3Objects(),
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Fatalf("should be at object list for test-app-state, got: %s", plain[:min(300, len(plain))])
	}

	// 4. Enter object -> detail view
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "terraform") {
		t.Fatalf("should be at detail view for terraform.tfstate, got: %s", plain[:min(300, len(plain))])
	}

	// 5. Escape from detail -> back to object list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("after escape from detail, should be at object list, got: %s", plain[:min(300, len(plain))])
	}

	// 6. Escape from object list -> back to bucket list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("after escape from objects, should be at bucket list s3(5), got: %s", plain[:min(300, len(plain))])
	}

	// 7. Escape from bucket list -> back to main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after escape from bucket list, should be at main menu, got: %s", plain[:min(300, len(plain))])
	}
}

// Test the main menu -> S3 entry point

func TestQA_S3_MainMenu_ToS3Selection(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	// EC2 is the first item in the main menu, press Enter
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Enter on main menu should produce a command")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("Enter on main menu should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetResourceList {
		t.Errorf("should navigate to TargetResourceList, got: %d", nav.Target)
	}
	if nav.ResourceType != "ec2" {
		t.Errorf("first menu item should be EC2, got: %q", nav.ResourceType)
	}
}

// Test filter mode works on S3 bucket list via root model

func TestQA_S3_FilterMode_ViaRootModel(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Enter filter mode with "/"
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	// Type "cdn"
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("d"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	plain := stripANSI(rootViewContent(m))
	// Header should show filter text
	if !strings.Contains(plain, "/cdn") {
		t.Errorf("header should show active filter '/cdn', got: %s", plain[:min(200, len(plain))])
	}
	// Should show filtered buckets (cdn-cloudfront and cdn-test)
	if !strings.Contains(plain, "cdn") {
		t.Error("filter 'cdn' should show cdn buckets")
	}
	// Frame title should show filtered count
	if !strings.Contains(plain, "2/5") {
		t.Errorf("frame title should show 2/5 for cdn filter, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_S3_FilterMode_EscapeClearsFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("d"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	// Escape from filter mode
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	// Should show all 5 buckets again
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("escape from filter should restore all buckets, got: %s", plain[:min(200, len(plain))])
	}
	// Header should revert to "? for help"
	if !strings.Contains(plain, "? for help") {
		t.Errorf("header should revert to '? for help', got: %s", plain[:min(200, len(plain))])
	}
}

// Test command mode from S3 bucket list

func TestQA_S3_CommandMode_NavigateToEC2(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Enter command mode with ":"
	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	// Type "ec2"
	m, _ = rootApplyMsg(m, rootKeyPress("e"))
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("2"))
	// Press Enter to execute
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("command 'ec2' should produce a command")
	}
}

// Test YAML view from S3 object list

func TestQA_S3_YAML_FromObjectList(t *testing.T) {
	m := s3RLObjectModel("test-app-state")

	_, cmd := m.Update(s3KeyPress("y"))
	if cmd == nil {
		t.Fatal("y key on S3 object should produce a command for YAML view")
	}

	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("y key should produce NavigateMsg, got %T", msg)
	}
	if nav.Target != messages.TargetYAML {
		t.Errorf("y key should target YAML view, got: %d", nav.Target)
	}
	if nav.Resource == nil {
		t.Fatal("YAML NavigateMsg.Resource should not be nil")
	}
}

// Test copy returns the selected resource ID (clipboard not tested, just the data)

// Test that S3 bucket list has exactly 2 columns (Bucket Name, Creation Date)

func TestQA_S3_BucketList_ExactlyTwoColumns(t *testing.T) {
	rt := resource.FindResourceType("s3")
	if rt == nil {
		t.Fatal("resource type 's3' not found")
	}
	if len(rt.Columns) != 2 {
		t.Errorf("S3 bucket list should have exactly 2 columns, got %d", len(rt.Columns))
	}
	if rt.Columns[0].Title != "Bucket Name" {
		t.Errorf("first column should be 'Bucket Name', got %q", rt.Columns[0].Title)
	}
	if rt.Columns[1].Title != "Creation Date" {
		t.Errorf("second column should be 'Creation Date', got %q", rt.Columns[1].Title)
	}
}

// Test that S3 object list has the expected columns

func TestQA_S3_ObjectList_ExpectedColumns(t *testing.T) {
	cols := resource.S3ObjectColumns()
	expectedTitles := []string{"Key", "Size", "Last Modified", "Storage Class"}

	if len(cols) != len(expectedTitles) {
		t.Fatalf("S3 object columns count: expected %d, got %d", len(expectedTitles), len(cols))
	}
	for i, expected := range expectedTitles {
		if cols[i].Title != expected {
			t.Errorf("column %d title: expected %q, got %q", i, expected, cols[i].Title)
		}
	}
}

// Test that S3 bucket list ResourceType() returns "s3"

// Test that S3 object list ResourceType() returns "s3_objects"

// Test that horizontal scroll works in object list

// Test no separator line below column headers

// Test S3 bucket list view includes all fixture buckets in rendered output

// Test full flow: main menu -> S3 -> YAML view -> escape chain

func TestQA_S3_D2_2_BucketYAMLRoundTrip(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Open YAML view for first bucket
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, rootKeyPress("y"))
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "yaml") {
		t.Errorf("should be in YAML view, got: %s", plain[:min(200, len(plain))])
	}

	// Escape from YAML -> back to bucket list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("escape from YAML should return to bucket list, got: %s", plain[:min(200, len(plain))])
	}
}

// Test the EnterChildViewMsg for S3 is processed correctly by root model

func TestQA_S3_EnterChildViewMsg_CreatesObjectListView(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	// Simulate EnterChildViewMsg for S3 objects
	m, _ = rootApplyMsg(m, messages.EnterChildView{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "test-app-state"},
		DisplayName:   "test-app-state",
	})

	plain := stripANSI(rootViewContent(m))
	// Should show loading state for the bucket or the bucket name in the frame
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("EnterChildViewMsg should create object list view with bucket name, got: %s", plain[:min(300, len(plain))])
	}
}

// Test header consistency across S3 views

func TestQA_S3_D1_HeaderConsistency(t *testing.T) {
	tui.Version = "0.6.0"

	// Test header in bucket list
	m := s3LoadedBucketModel()
	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "a9s") {
		t.Error("S3 bucket list should show 'a9s' in header")
	}
	if !strings.Contains(plain, "v0.6.0") {
		t.Error("S3 bucket list should show version in header")
	}
	if !strings.Contains(plain, "testprofile:us-east-1") {
		t.Error("S3 bucket list should show profile:region in header")
	}
	if !strings.Contains(plain, "? for help") {
		t.Error("S3 bucket list should show '? for help' in header")
	}
}
