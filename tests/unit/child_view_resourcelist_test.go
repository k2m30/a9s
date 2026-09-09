package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// handleChildKey — data-driven Enter key routing
// ===========================================================================

func TestHandleChildKey_EnterOnS3Bucket_ProducesEnterChildViewMsg(t *testing.T) {
	// Create an S3 bucket list with Children defined
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
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
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load buckets
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket", "creation_date": "2025-01-01"}},
	}, nil, false)

	// Press Enter
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 bucket with Children should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("Expected EnterChildViewMsg, got %T", msg)
	}
	if childMsg.ChildType != "s3_objects" {
		t.Errorf("ChildType = %q, want %q", childMsg.ChildType, "s3_objects")
	}
	if childMsg.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("ParentContext[bucket] = %q, want %q", childMsg.ParentContext["bucket"], "my-bucket")
	}
	if childMsg.DisplayName != "my-bucket" {
		t.Errorf("DisplayName = %q, want %q", childMsg.DisplayName, "my-bucket")
	}
}

func TestHandleChildKey_EnterOnR53Zone_ProducesEnterChildViewMsg(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "Route 53 Hosted Zones",
		ShortName: "r53",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 36},
			{Key: "zone_id", Title: "Zone ID", Width: 30},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "r53_records",
			Key:            "enter",
			ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
			DisplayNameKey: "zone_name",
		}},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("r53", []resource.Resource{
		{ID: "/hostedzone/ZTEST", Name: "example.com.", Fields: map[string]string{"zone_id": "/hostedzone/ZTEST", "name": "example.com."}},
	}, nil, false)

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on R53 zone with Children should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("Expected EnterChildViewMsg, got %T", msg)
	}
	if childMsg.ChildType != "r53_records" {
		t.Errorf("ChildType = %q, want %q", childMsg.ChildType, "r53_records")
	}
	if childMsg.ParentContext["zone_id"] != "/hostedzone/ZTEST" {
		t.Errorf("ParentContext[zone_id] = %q, want %q", childMsg.ParentContext["zone_id"], "/hostedzone/ZTEST")
	}
	if childMsg.ParentContext["zone_name"] != "example.com." {
		t.Errorf("ParentContext[zone_name] = %q, want %q", childMsg.ParentContext["zone_name"], "example.com.")
	}
	if childMsg.DisplayName != "example.com." {
		t.Errorf("DisplayName = %q, want %q", childMsg.DisplayName, "example.com.")
	}
}

func TestHandleChildKey_DrillConditionFalse_FallsThrough(t *testing.T) {
	// S3 object list with drill condition: only folders
	td := resource.ResourceTypeDef{
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
	k := keys.Default()
	ctrl := newChildListViewCtrl(t, td)
	m := views.NewChildResourceList(td, map[string]string{"bucket": "b1"}, "b1", nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a file (not a folder)
	ctrl.ApplyResourcesLoaded("s3_objects", []resource.Resource{
		{ID: "data/file.txt", Name: "data/file.txt", Fields: map[string]string{"status": "file", "key": "data/file.txt"}},
	}, nil, false)

	// Press Enter — should fall through to detail view (not child drill)
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 file should still produce a command (detail view)")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("Expected NavigateMsg for file (drill condition false), got %T", msg)
	}
	if navMsg.Target != messages.TargetDetail {
		t.Errorf("Target = %d, want TargetDetail (%d)", navMsg.Target, messages.TargetDetail)
	}
}

func TestHandleChildKey_DrillConditionTrue_ProducesChildMsg(t *testing.T) {
	td := resource.ResourceTypeDef{
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
	k := keys.Default()
	ctrl := newChildListViewCtrl(t, td)
	m := views.NewChildResourceList(td, map[string]string{"bucket": "b1"}, "b1", nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a folder
	ctrl.ApplyResourcesLoaded("s3_objects", []resource.Resource{
		{ID: "data/", Name: "data/", Fields: map[string]string{"status": "folder", "key": "data/"}},
	}, nil, false)

	// Press Enter — should produce EnterChildViewMsg
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 folder should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("Expected EnterChildViewMsg for folder, got %T", msg)
	}
	if childMsg.ChildType != "s3_objects" {
		t.Errorf("ChildType = %q, want %q", childMsg.ChildType, "s3_objects")
	}
	if childMsg.ParentContext["prefix"] != "data/" {
		t.Errorf("ParentContext[prefix] = %q, want %q", childMsg.ParentContext["prefix"], "data/")
	}
	if childMsg.ParentContext["bucket"] != "b1" {
		t.Errorf("ParentContext[bucket] = %q, want %q (from @parent.bucket)", childMsg.ParentContext["bucket"], "b1")
	}
}

func TestHandleChildKey_NoChildren_DefaultsToDetail(t *testing.T) {
	// EC2 has no Children — Enter should go to detail
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20},
		},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-123", Name: "web-1", Fields: map[string]string{"status": "running", "instance_id": "i-123"}},
	}, nil, false)

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on EC2 instance should produce detail navigation command")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("Expected NavigateMsg for EC2, got %T", msg)
	}
	if navMsg.Target != messages.TargetDetail {
		t.Errorf("Target = %d, want TargetDetail (%d)", navMsg.Target, messages.TargetDetail)
	}
}

// ===========================================================================
// buildChildContext — resolves context keys
// ===========================================================================

func TestBuildChildContext_ID(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "ID"},
			DisplayNameKey: "bucket",
		}},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "test-bucket", Name: "test-bucket", Fields: map[string]string{}},
	}, nil, false)

	// Press Enter, verify context
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildView)

	if childMsg.ParentContext["bucket"] != "test-bucket" {
		t.Errorf("Context for 'ID' source should resolve to resource ID, got %q", childMsg.ParentContext["bucket"])
	}
}

func TestBuildChildContext_Name(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "R53 Zones",
		ShortName: "r53",
		Children: []resource.ChildViewDef{{
			ChildType:      "r53_records",
			Key:            "enter",
			ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
			DisplayNameKey: "zone_name",
		}},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("r53", []resource.Resource{
		{ID: "/hostedzone/Z1", Name: "test.com.", Fields: map[string]string{}},
	}, nil, false)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildView)

	if childMsg.ParentContext["zone_name"] != "test.com." {
		t.Errorf("Context for 'Name' source should resolve to resource Name, got %q", childMsg.ParentContext["zone_name"])
	}
}

func TestBuildChildContext_AtParent(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r resource.Resource) bool { return r.Fields["status"] == "folder" },
		}},
	}
	k := keys.Default()
	parentCtx := map[string]string{"bucket": "my-bucket"}
	ctrl := newChildListViewCtrl(t, td)
	m := views.NewChildResourceList(td, parentCtx, "my-bucket", nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("s3_objects", []resource.Resource{
		{ID: "folder1/", Name: "folder1/", Fields: map[string]string{"status": "folder", "key": "folder1/"}},
	}, nil, false)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildView)

	if childMsg.ParentContext["bucket"] != "my-bucket" {
		t.Errorf("@parent.bucket should resolve to parent context value, got %q", childMsg.ParentContext["bucket"])
	}
	if childMsg.ParentContext["prefix"] != "folder1/" {
		t.Errorf("ID should resolve to resource ID, got %q", childMsg.ParentContext["prefix"])
	}
}

func TestBuildChildContext_FieldsKey(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "Test",
		ShortName: "test",
		Columns:   []resource.Column{{Key: "custom_field", Title: "Custom", Width: 20}},
		Children: []resource.ChildViewDef{{
			ChildType:      "test_child",
			Key:            "enter",
			ContextKeys:    map[string]string{"custom": "custom_field"},
			DisplayNameKey: "custom",
		}},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("test", []resource.Resource{
		{ID: "1", Name: "one", Fields: map[string]string{"custom_field": "custom-value"}},
	}, nil, false)

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildView)

	if childMsg.ParentContext["custom"] != "custom-value" {
		t.Errorf("Fields key should resolve to resource Fields value, got %q", childMsg.ParentContext["custom"])
	}
}

// ===========================================================================
// handleChildKey for non-"enter" keys (e.g., "e" for events)
// ===========================================================================

func TestHandleChildKey_NonEnterKey_EventsKey(t *testing.T) {
	// Create a resource type with a child bound to the "e" key (events)
	td := resource.ResourceTypeDef{
		Name:      "Test Parent",
		ShortName: "test_parent_events",
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
			{Key: "name", Title: "Name", Width: 30},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "test_events",
			Key:            "e",
			ContextKeys:    map[string]string{"parent_id": "ID"},
			DisplayNameKey: "parent_id",
		}},
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a resource
	ctrl.ApplyResourcesLoaded("test_parent_events", []resource.Resource{
		{ID: "res-123", Name: "my-resource", Fields: map[string]string{"status": "active", "id": "res-123", "name": "my-resource"}},
	}, nil, false)

	// Press "e" key — triggers keys.Events which calls handleChildKey("e", ...)
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Fatal("pressing 'e' on resource with Children[Key='e'] should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildView)
	if !ok {
		t.Fatalf("expected EnterChildViewMsg from 'e' key, got %T", msg)
	}
	if childMsg.ChildType != "test_events" {
		t.Errorf("ChildType = %q, want %q", childMsg.ChildType, "test_events")
	}
	if childMsg.ParentContext["parent_id"] != "res-123" {
		t.Errorf("ParentContext[parent_id] = %q, want %q", childMsg.ParentContext["parent_id"], "res-123")
	}
	if childMsg.DisplayName != "res-123" {
		t.Errorf("DisplayName = %q, want %q", childMsg.DisplayName, "res-123")
	}
}

func TestHandleChildKey_NonEnterKey_NoChildDefined(t *testing.T) {
	// Create a resource type with NO children for the "e" key
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2_no_events",
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20},
		},
		// No Children defined — pressing "e" should be a no-op
	}
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 20)
	m, _ = m.Init()

	ctrl.ApplyResourcesLoaded("ec2_no_events", []resource.Resource{
		{ID: "i-123", Name: "web-1", Fields: map[string]string{"status": "running", "instance_id": "i-123"}},
	}, nil, false)

	// Press "e" key — no child defined, should return nil cmd (no-op)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd != nil {
		t.Error("pressing 'e' on resource type with no events child should return nil cmd")
	}
}
