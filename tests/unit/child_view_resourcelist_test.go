package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// Step 4: NewChildResourceList constructor
// ===========================================================================

func TestNewChildResourceList_S3Objects(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
		Children: []resource.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "@parent.bucket", "prefix": "ID"},
			DisplayNameKey: "bucket",
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
	}
	parentCtx := map[string]string{"bucket": "test-bucket"}
	k := keys.Default()

	m := views.NewChildResourceList(childDef, parentCtx, "test-bucket", nil, k)
	m.SetSize(120, 20)

	if m.ResourceType() != "s3_objects" {
		t.Errorf("ResourceType() = %q, want %q", m.ResourceType(), "s3_objects")
	}
	if m.FrameTitle() != "test-bucket" {
		t.Errorf("FrameTitle() = %q, want %q", m.FrameTitle(), "test-bucket")
	}
	if m.ParentContext()["bucket"] != "test-bucket" {
		t.Errorf("ParentContext()[bucket] = %q, want %q", m.ParentContext()["bucket"], "test-bucket")
	}
}

func TestNewChildResourceList_R53Records(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "R53 Records",
		ShortName: "r53_records",
		Columns:   resource.R53RecordColumns(),
	}
	parentCtx := map[string]string{
		"zone_id":   "/hostedzone/ZTEST",
		"zone_name": "example.com.",
	}
	k := keys.Default()

	m := views.NewChildResourceList(childDef, parentCtx, "example.com.", nil, k)
	m.SetSize(120, 20)

	if m.ResourceType() != "r53_records" {
		t.Errorf("ResourceType() = %q, want %q", m.ResourceType(), "r53_records")
	}
	if m.FrameTitle() != "example.com." {
		t.Errorf("FrameTitle() = %q, want %q", m.FrameTitle(), "example.com.")
	}
	if m.ParentContext()["zone_id"] != "/hostedzone/ZTEST" {
		t.Errorf("ParentContext()[zone_id] = %q, want %q", m.ParentContext()["zone_id"], "/hostedzone/ZTEST")
	}
}

func TestNewChildResourceList_Loading(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
	}
	k := keys.Default()

	m := views.NewChildResourceList(childDef, map[string]string{"bucket": "b1"}, "b1", nil, k)
	m.SetSize(120, 20)

	// During loading, view should show spinner
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty during loading")
	}
}

// ===========================================================================
// handleChildKey — data-driven Enter key routing
// ===========================================================================

func TestHandleChildKey_EnterOnS3Bucket_ProducesEnterChildViewMsg(t *testing.T) {
	// Create an S3 bucket list with Children defined
	td := resource.ResourceTypeDef{
		Name:      "S3 Buckets",
		ShortName: "s3",
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
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load buckets
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "my-bucket", Name: "my-bucket", Status: "", Fields: map[string]string{"name": "my-bucket", "creation_date": "2025-01-01"}},
		},
	})

	// Press Enter
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 bucket with Children should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildViewMsg)
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
			{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "r53_records",
			Key:            "enter",
			ContextKeys:    map[string]string{"zone_id": "ID", "zone_name": "Name"},
			DisplayNameKey: "zone_name",
		}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources: []resource.Resource{
			{ID: "/hostedzone/ZTEST", Name: "example.com.", Status: "", Fields: map[string]string{"zone_id": "/hostedzone/ZTEST", "name": "example.com."}},
		},
	})

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on R53 zone with Children should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildViewMsg)
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
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
	}
	k := keys.Default()
	m := views.NewChildResourceList(td, map[string]string{"bucket": "b1"}, "b1", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a file (not a folder)
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3_objects",
		Resources: []resource.Resource{
			{ID: "data/file.txt", Name: "data/file.txt", Status: "file", Fields: map[string]string{"key": "data/file.txt"}},
		},
	})

	// Press Enter — should fall through to detail view (not child drill)
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 file should still produce a command (detail view)")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.NavigateMsg)
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
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
	}
	k := keys.Default()
	m := views.NewChildResourceList(td, map[string]string{"bucket": "b1"}, "b1", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a folder
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3_objects",
		Resources: []resource.Resource{
			{ID: "data/", Name: "data/", Status: "folder", Fields: map[string]string{"key": "data/"}},
		},
	})

	// Press Enter — should produce EnterChildViewMsg
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on S3 folder should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildViewMsg)
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
			{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-123", Name: "web-1", Status: "running", Fields: map[string]string{"instance_id": "i-123"}},
		},
	})

	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on EC2 instance should produce detail navigation command")
	}

	msg := cmd()
	navMsg, ok := msg.(messages.NavigateMsg)
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
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "test-bucket", Name: "test-bucket", Fields: map[string]string{}},
		},
	})

	// Press Enter, verify context
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildViewMsg)

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
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "r53",
		Resources: []resource.Resource{
			{ID: "/hostedzone/Z1", Name: "test.com.", Fields: map[string]string{}},
		},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildViewMsg)

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
			DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
		}},
	}
	k := keys.Default()
	parentCtx := map[string]string{"bucket": "my-bucket"}
	m := views.NewChildResourceList(td, parentCtx, "my-bucket", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3_objects",
		Resources: []resource.Resource{
			{ID: "folder1/", Name: "folder1/", Status: "folder", Fields: map[string]string{"key": "folder1/"}},
		},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildViewMsg)

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
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "test",
		Resources: []resource.Resource{
			{ID: "1", Name: "one", Fields: map[string]string{"custom_field": "custom-value"}},
		},
	})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	childMsg := msg.(messages.EnterChildViewMsg)

	if childMsg.ParentContext["custom"] != "custom-value" {
		t.Errorf("Fields key should resolve to resource Fields value, got %q", childMsg.ParentContext["custom"])
	}
}

// ===========================================================================
// FrameTitle with displayName
// ===========================================================================

func TestChildResourceList_FrameTitle_DisplayName(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
	}
	k := keys.Default()

	m := views.NewChildResourceList(childDef, map[string]string{"bucket": "my-bucket"}, "my-bucket", nil, k)
	m.SetSize(120, 20)

	title := m.FrameTitle()
	if title != "my-bucket" {
		t.Errorf("FrameTitle() = %q, want %q", title, "my-bucket")
	}
}

func TestChildResourceList_FrameTitle_WithCount(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "S3 Objects",
		ShortName: "s3_objects",
		Columns:   resource.S3ObjectColumns(),
	}
	k := keys.Default()

	m := views.NewChildResourceList(childDef, map[string]string{"bucket": "b1"}, "b1", nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3_objects",
		Resources: []resource.Resource{
			{ID: "file1.txt", Name: "file1.txt", Status: "file", Fields: map[string]string{"key": "file1.txt"}},
			{ID: "file2.txt", Name: "file2.txt", Status: "file", Fields: map[string]string{"key": "file2.txt"}},
		},
	})

	title := m.FrameTitle()
	if title != "b1(2)" {
		t.Errorf("FrameTitle() = %q, want %q", title, "b1(2)")
	}
}

// ===========================================================================
// ParentContext accessor
// ===========================================================================

func TestParentContext_Accessor(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "R53 Records",
		ShortName: "r53_records",
		Columns:   resource.R53RecordColumns(),
	}
	k := keys.Default()

	parentCtx := map[string]string{"zone_id": "Z123", "zone_name": "test.com."}
	m := views.NewChildResourceList(childDef, parentCtx, "test.com.", nil, k)

	got := m.ParentContext()
	if got["zone_id"] != "Z123" {
		t.Errorf("ParentContext()[zone_id] = %q, want %q", got["zone_id"], "Z123")
	}
	if got["zone_name"] != "test.com." {
		t.Errorf("ParentContext()[zone_name] = %q, want %q", got["zone_name"], "test.com.")
	}
}

func TestParentContext_NilForTopLevel(t *testing.T) {
	td := resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Columns:   []resource.Column{{Key: "instance_id", Title: "ID", Width: 20}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)

	got := m.ParentContext()
	if got != nil {
		t.Errorf("ParentContext() should be nil for top-level resource list, got %v", got)
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
			{Key: "id", Title: "ID", Width: 20, Sortable: true},
			{Key: "name", Title: "Name", Width: 30, Sortable: true},
		},
		Children: []resource.ChildViewDef{{
			ChildType:      "test_events",
			Key:            "e",
			ContextKeys:    map[string]string{"parent_id": "ID"},
			DisplayNameKey: "parent_id",
		}},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	// Load a resource
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "test_parent_events",
		Resources: []resource.Resource{
			{ID: "res-123", Name: "my-resource", Status: "active", Fields: map[string]string{"id": "res-123", "name": "my-resource"}},
		},
	})

	// Press "e" key — triggers keys.Events which calls handleChildKey("e", ...)
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd == nil {
		t.Fatal("pressing 'e' on resource with Children[Key='e'] should return a command")
	}

	msg := cmd()
	childMsg, ok := msg.(messages.EnterChildViewMsg)
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
			{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
		},
		// No Children defined — pressing "e" should be a no-op
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2_no_events",
		Resources: []resource.Resource{
			{ID: "i-123", Name: "web-1", Status: "running", Fields: map[string]string{"instance_id": "i-123"}},
		},
	})

	// Press "e" key — no child defined, should return nil cmd (no-op)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if cmd != nil {
		t.Error("pressing 'e' on resource type with no events child should return nil cmd")
	}
}
