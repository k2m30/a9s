package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// viewsDirs returns the standard views directory path for tests in tests/unit/.
var viewsDirs = []string{"../../.a9s/views"}

// S3 folders are navigable; Enter on one drills into the prefix.
func TestBug_S3_EnterOnFolder_NavigatesIntoPrefix(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "s3", Resources: buckets})
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	objects := []resource.Resource{
		{ID: "enterprise/", Name: "enterprise/", Fields: map[string]string{
			"key": "enterprise/", "size": "", "last_modified": "", "storage_class": "", "kind": "folder",
		}},
		{ID: "readme.txt", Name: "readme.txt", Fields: map[string]string{
			"key": "readme.txt", "size": "1024", "last_modified": "2025-01-01", "storage_class": "STANDARD", "kind": "file",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceChild, ResourceType: "s3_objects", Resources: objects})
	_, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on folder should return a command to navigate into prefix")
	}
	msg := cmd()
	if childMsg, ok := msg.(messages.EnterChildView); ok {
		if childMsg.ChildType != "s3_objects" {
			t.Errorf("EnterChildViewMsg.ChildType should be 's3_objects', got %q", childMsg.ChildType)
		}
	} else if nav, ok := msg.(messages.Navigate); ok && nav.Target == messages.TargetDetail {
		t.Error("Enter on S3 folder must navigate into prefix, not show detail view")
	} else {
		t.Errorf("Enter on S3 folder should send EnterChildViewMsg, got %T", msg)
	}
}

// d on an S3 bucket opens its detail instead of entering the bucket.
func TestBug_S3_DKeyOnBucket_ShowsDetail(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "s3"})
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "s3", Resources: buckets})
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'd'})
	if cmd == nil {
		t.Fatal("d key should return a command")
	}
	msg := cmd()
	if _, ok := msg.(messages.EnterChildView); ok {
		t.Error("d key on S3 bucket must show detail view, not drill into bucket")
	}
	if nav, ok := msg.(messages.Navigate); ok {
		if nav.Target != messages.TargetDetail {
			t.Errorf("d key should navigate to detail, got target %v", nav.Target)
		}
	}
}

// The detail view uses the ViewDef of the resource's own type.
func TestBug_Detail_UsesCorrectViewDefForResourceType(t *testing.T) {
	cfg, err := config.LoadFromDirs(viewsDirs)
	if err != nil {
		t.Skipf("views dir not found: %v", err)
	}

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
		RawStruct: raw,
		Fields:    map[string]string{"instance_id": instID, "name": "web-server", "state": "running"},
	}

	tui.Version = "test"
	m := newBlessedModel(t, "test", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: []resource.Resource{res}})
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: 'd'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}

	content := rootViewContent(m)
	plain := stripANSI(content)

	if !strings.Contains(plain, "InstanceId") {
		t.Errorf("EC2 detail must show InstanceId from EC2 ViewDef, got:\n%s", plain[:min(500, len(plain))])
	}
	if !strings.Contains(plain, "InstanceType") {
		t.Error("EC2 detail must show InstanceType from EC2 ViewDef")
	}
	if strings.Contains(plain, "Tags") && !strings.Contains(plain, "InstanceId") {
		t.Error("EC2 detail shows only Tags — wrong ViewDef selected from config")
	}

	_ = cfg
}

func TestBug_S3Object_DetailShowsAllConfiguredFields(t *testing.T) {
	cfg, err := config.LoadFromDirs(viewsDirs)
	if err != nil {
		t.Skipf("views dir not found: %v", err)
	}

	vd := config.GetViewDef(cfg, "s3_objects")
	if len(vd.Detail) == 0 {
		t.Fatal("s3_objects detail config is empty — .a9s/views/s3_objects.yaml must have a detail section")
	}

	t.Logf("s3_objects detail paths: %v", vd.Detail)

	// s3types.Object has: Key, Size, LastModified, StorageClass, ETag, Owner
	// NOT "Name" — that's a bucket field
	for _, df := range vd.Detail {
		if df.String() == "Name" {
			t.Error("s3_objects detail config has path 'Name' but s3types.Object has 'Key', not 'Name' — this will extract nothing")
		}
	}
}
