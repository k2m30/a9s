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

// s3BucketTypeDef returns the S3 bucket type definition (matches resource.FindResourceType("s3")).
func s3BucketTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Aliases:   []string{"s3", "buckets"},
		Columns: []resource.Column{
			{Key: "name", Title: "Bucket Name", Width: 40},
			{Key: "creation_date", Title: "Creation Date", Width: 22},
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
		Resources:    fixtureS3Buckets(), Provenance: messages.FetchProvenanceCanonicalList,
	})
	return m
}

// s3LoadedObjectModel creates a root TUI model navigated to S3 -> bucket -> objects loaded.
func s3LoadedObjectModel() tui.Model {
	m := s3LoadedBucketModel()
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceChild,
		ResourceType: "s3_objects",
		Resources:    fixtureS3Objects(),
	})
	return m
}

// s3RLBucketModel creates a standalone ResourceListModel for S3 buckets with data loaded.
func s3RLBucketModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	td := s3BucketTypeDef()
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()
	ctrl.ApplyResourcesLoaded("s3", fixtureS3Buckets(), nil, false)
	return m
}

// s3RLObjectModel creates a standalone ResourceListModel for S3 objects inside a bucket.
func s3RLObjectModel(t *testing.T, bucket string) views.ResourceListModel {
	t.Helper()
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
	ctrl := newChildListViewCtrl(t, childDef)
	m := views.NewChildResourceList(childDef, map[string]string{"bucket": bucket}, bucket, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()
	ctrl.ApplyResourcesLoaded("s3_objects", fixtureS3Objects(), nil, false)
	return m
}

// s3KeyPress creates a tea.KeyPressMsg for a printable character.
func s3KeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func TestQA_S3_A8_1_EnterOnBucket_SendsEnterChildViewMsg(t *testing.T) {
	m := s3RLBucketModel(t)

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
	m := s3RLBucketModel(t)

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
	if _, ok := msg.(messages.EnterChildView); !ok {
		t.Errorf("Enter on S3 bucket should produce EnterChildViewMsg, got %T", msg)
	}
}

func TestQA_S3_A13_Escape_ReturnsToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("Escape from S3 bucket list should return to main menu, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_S3_B14_1_Escape_FromObjectsReturnsToBuckets(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("Escape from object list should return to bucket list with s3(5), got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_B14_1_Escape_FromObjects_DoesNotReturnToMainMenu(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "resource-types") {
		t.Error("single Escape from object list should NOT go to main menu; should go to bucket list")
	}
}

func TestQA_S3_C1_BucketDetail_ViaDetailCommand(t *testing.T) {
	m := s3RLBucketModel(t)

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

func TestQA_S3_C2_ObjectDetail_EnterSendsDetail(t *testing.T) {
	m := s3RLObjectModel(t, "test-app-state")

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

func TestQA_S3_C3_ObjectDetail_FrameTitle(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "terraform") {
		t.Errorf("detail view should show object name in frame title, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_C3_DetailView_EscapeReturnsToObjectList(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedObjectModel()

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("Escape from detail should return to object list, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_D2_1_FullFlowStack(t *testing.T) {
	tui.Version = "0.6.0"
	m := newRootSizedModel()

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Fatalf("should start at main menu, got: %s", plain[:min(200, len(plain))])
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    fixtureS3Buckets(), Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Fatalf("should be at S3 bucket list with s3(5), got: %s", plain[:min(200, len(plain))])
	}

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3_objects",
		Resources:    fixtureS3Objects(), Provenance: messages.FetchProvenanceChild,
	})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Fatalf("should be at object list for test-app-state, got: %s", plain[:min(300, len(plain))])
	}

	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "terraform") {
		t.Fatalf("should be at detail view for terraform.tfstate, got: %s", plain[:min(300, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("after escape from detail, should be at object list, got: %s", plain[:min(300, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("after escape from objects, should be at bucket list s3(5), got: %s", plain[:min(300, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "resource-types") {
		t.Errorf("after escape from bucket list, should be at main menu, got: %s", plain[:min(300, len(plain))])
	}
}

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

func TestQA_S3_FilterMode_ViaRootModel(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("d"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "/cdn") {
		t.Errorf("header should show active filter '/cdn', got: %s", plain[:min(200, len(plain))])
	}
	if !strings.Contains(plain, "cdn") {
		t.Error("filter 'cdn' should show cdn buckets")
	}
	if !strings.Contains(plain, "2/5") {
		t.Errorf("frame title should show 2/5 for cdn filter, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_S3_FilterMode_EscapeClearsFilter(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("d"))
	m, _ = rootApplyMsg(m, rootKeyPress("n"))

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("escape from filter should restore all buckets, got: %s", plain[:min(200, len(plain))])
	}
	if !strings.Contains(plain, "? for help") {
		t.Errorf("header should revert to '? for help', got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_S3_CommandMode_NavigateToEC2(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	m, _ = rootApplyMsg(m, rootKeyPress("e"))
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	m, _ = rootApplyMsg(m, rootKeyPress("2"))
	_, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	if cmd == nil {
		t.Fatal("command 'ec2' should produce a command")
	}
}

func TestQA_S3_YAML_FromObjectList(t *testing.T) {
	m := s3RLObjectModel(t, "test-app-state")

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

func TestQA_S3_D2_2_BucketYAMLRoundTrip(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

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

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "s3(5)") {
		t.Errorf("escape from YAML should return to bucket list, got: %s", plain[:min(200, len(plain))])
	}
}

func TestQA_S3_EnterChildViewMsg_CreatesObjectListView(t *testing.T) {
	tui.Version = "0.6.0"
	m := s3LoadedBucketModel()

	m, _ = rootApplyMsg(m, messages.EnterChildView{
		ChildType:     "s3_objects",
		ParentContext: map[string]string{"bucket": "test-app-state"},
		DisplayName:   "test-app-state",
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "test-app-state") {
		t.Errorf("EnterChildViewMsg should create object list view with bucket name, got: %s", plain[:min(300, len(plain))])
	}
}

func TestQA_S3_D1_HeaderConsistency(t *testing.T) {
	tui.Version = "0.6.0"

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
