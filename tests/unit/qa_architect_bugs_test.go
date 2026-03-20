package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// --- Bug 1: S3 folders should be navigable, not show detail ---

func TestBug_S3_EnterOnFolder_NavigatesIntoPrefix(t *testing.T) {
	m := newRootSizedModel()
	// Navigate to S3 buckets
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	// Enter bucket
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	// Load objects including a folder
	objects := []resource.Resource{
		{ID: "enterprise/", Name: "enterprise/", Status: "folder", Fields: map[string]string{
			"key": "enterprise/", "size": "", "last_modified": "", "storage_class": "",
		}},
		{ID: "readme.txt", Name: "readme.txt", Status: "", Fields: map[string]string{
			"key": "readme.txt", "size": "1024", "last_modified": "2025-01-01", "storage_class": "STANDARD",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})
	// Press Enter on the folder — should navigate into prefix, NOT show detail
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on folder should return a command to navigate into prefix")
	}
	msg := cmd()
	// Should be S3NavigatePrefixMsg, not NavigateMsg{TargetDetail}
	if _, ok := msg.(messages.S3NavigatePrefixMsg); !ok {
		if nav, ok := msg.(messages.NavigateMsg); ok && nav.Target == messages.TargetDetail {
			t.Error("Enter on S3 folder must navigate into prefix, not show detail view")
		} else {
			t.Errorf("Enter on S3 folder should send S3NavigatePrefixMsg, got %T", msg)
		}
	}
}

// --- Bug 2: d key on S3 bucket should show detail, not enter bucket ---

func TestBug_S3_DKeyOnBucket_ShowsDetail(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	// Press d (describe) — should show detail, NOT enter bucket
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'd'})
	if cmd == nil {
		t.Fatal("d key should return a command")
	}
	msg := cmd()
	// Should be NavigateMsg{TargetDetail}, NOT S3EnterBucketMsg
	if _, ok := msg.(messages.S3EnterBucketMsg); ok {
		t.Error("d key on S3 bucket must show detail view, not drill into bucket")
	}
	if nav, ok := msg.(messages.NavigateMsg); ok {
		if nav.Target != messages.TargetDetail {
			t.Errorf("d key should navigate to detail, got target %v", nav.Target)
		}
	}
}

// --- Bug 3+4: Detail view must use correct ViewDef for the resource type ---

func TestBug_Detail_UsesCorrectViewDefForResourceType(t *testing.T) {
	// Load the FULL config (all 8 resource types) — same as production
	cfg, err := config.LoadFrom([]string{"/Users/k2m30/projects/a9s/.a9s/views.yaml"})
	if err != nil {
		t.Skipf("views.yaml not found: %v", err)
	}

	// Create an EC2-like resource with RawStruct that has Tags
	type fakeEC2 struct {
		InstanceId       *string
		InstanceType     *string
		PrivateIpAddress *string
		Tags             []struct {
			Key   *string
			Value *string
		}
	}
	instID := "i-test123"
	instType := "t3.micro"
	privIP := "10.0.1.42"
	tagKey := "Name"
	tagVal := "web-server"
	raw := fakeEC2{
		InstanceId:       &instID,
		InstanceType:     &instType,
		PrivateIpAddress: &privIP,
		Tags: []struct {
			Key   *string
			Value *string
		}{{Key: &tagKey, Value: &tagVal}},
	}

	res := resource.Resource{
		ID:        instID,
		Name:      "web-server",
		Status:    "running",
		RawStruct: raw,
		Fields:    map[string]string{"instance_id": instID, "name": "web-server", "state": "running"},
	}

	tui.Version = "test"
	m := tui.New("test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Navigate to EC2, load resources, then open detail
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: []resource.Resource{res}})
	// Open detail via d key
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'd'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	content := rootViewContent(m)
	plain := stripANSI(content)

	// The detail view should show EC2-specific fields, not random fields from another ViewDef
	// With the full config, the bug causes it to pick a random ViewDef (e.g., secrets or s3)
	// which would NOT contain "InstanceId" or "InstanceType"
	if !strings.Contains(plain, "InstanceId") {
		t.Errorf("EC2 detail must show InstanceId from EC2 ViewDef, got:\n%s", plain[:min(500, len(plain))])
	}
	if !strings.Contains(plain, "InstanceType") {
		t.Error("EC2 detail must show InstanceType from EC2 ViewDef")
	}
	// Must NOT show only Tags
	if strings.Contains(plain, "Tags") && !strings.Contains(plain, "InstanceId") {
		t.Error("EC2 detail shows only Tags — wrong ViewDef selected from config")
	}

	_ = cfg // ensure cfg is used
}

func TestBug_S3Object_DetailShowsAllConfiguredFields(t *testing.T) {
	cfg, err := config.LoadFrom([]string{"/Users/k2m30/projects/a9s/.a9s/views.yaml"})
	if err != nil {
		t.Skipf("views.yaml not found: %v", err)
	}

	// views.yaml s3_objects detail has: Key, Size, LastModified, StorageClass, ETag
	// (or whatever is configured — we check that at least 3 fields show)
	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.Detail) == 0 {
		t.Skip("no s3_objects detail config")
	}

	t.Logf("s3_objects detail paths: %v", vd.Detail)

	// Check that the configured paths actually match s3types.Object field names
	// s3types.Object has: Key, Size, LastModified, StorageClass, ETag, Owner
	// NOT "Name" — that's a bucket field
	for _, path := range vd.Detail {
		if path == "Name" {
			t.Error("s3_objects detail config has path 'Name' but s3types.Object has 'Key', not 'Name' — this will extract nothing")
		}
	}
}
