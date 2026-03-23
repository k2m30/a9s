package unit

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ===========================================================================
// Step 1: ChildViewDef struct and child type/fetcher registries
// ===========================================================================

func TestChildViewDef_Fields(t *testing.T) {
	def := resource.ChildViewDef{
		ChildType:      "s3_objects",
		Key:            "enter",
		ContextKeys:    map[string]string{"bucket": "ID"},
		DisplayNameKey: "bucket",
		DrillCondition: func(r resource.Resource) bool { return r.Status == "folder" },
	}

	if def.ChildType != "s3_objects" {
		t.Errorf("ChildType = %q, want %q", def.ChildType, "s3_objects")
	}
	if def.Key != "enter" {
		t.Errorf("Key = %q, want %q", def.Key, "enter")
	}
	if def.ContextKeys["bucket"] != "ID" {
		t.Errorf("ContextKeys[bucket] = %q, want %q", def.ContextKeys["bucket"], "ID")
	}
	if def.DisplayNameKey != "bucket" {
		t.Errorf("DisplayNameKey = %q, want %q", def.DisplayNameKey, "bucket")
	}

	// DrillCondition should be non-nil and work
	folder := resource.Resource{Status: "folder"}
	file := resource.Resource{Status: "file"}
	if !def.DrillCondition(folder) {
		t.Error("DrillCondition should return true for folder")
	}
	if def.DrillCondition(file) {
		t.Error("DrillCondition should return false for file")
	}
}

func TestChildViewDef_NilDrillCondition(t *testing.T) {
	var def resource.ChildViewDef

	// DrillCondition nil means no filtering — always drill
	if def.DrillCondition != nil {
		t.Error("DrillCondition should be nil when not set")
	}
}

func TestResourceTypeDef_Children(t *testing.T) {
	td := resource.ResourceTypeDef{
		Children: []resource.ChildViewDef{
			{
				ChildType:      "s3_objects",
				Key:            "enter",
				ContextKeys:    map[string]string{"bucket": "ID"},
				DisplayNameKey: "bucket",
			},
		},
	}

	if len(td.Children) != 1 {
		t.Fatalf("Children length = %d, want 1", len(td.Children))
	}
	if td.Children[0].ChildType != "s3_objects" {
		t.Errorf("Children[0].ChildType = %q, want %q", td.Children[0].ChildType, "s3_objects")
	}
}

// ===========================================================================
// Child type registry
// ===========================================================================

func TestRegisterChildType(t *testing.T) {
	childDef := resource.ResourceTypeDef{
		Name:      "Test Child",
		ShortName: "test_child",
		Columns:   []resource.Column{{Key: "name", Title: "Name", Width: 30}},
	}
	resource.RegisterChildType(childDef)
	defer resource.UnregisterChildType("test_child")

	got := resource.GetChildType("test_child")
	if got == nil {
		t.Fatal("GetChildType returned nil for registered child type")
	}
	if got.Name != "Test Child" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Child")
	}
	if got.ShortName != "test_child" {
		t.Errorf("ShortName = %q, want %q", got.ShortName, "test_child")
	}
}

func TestGetChildType_NotRegistered(t *testing.T) {
	got := resource.GetChildType("nonexistent_child")
	if got != nil {
		t.Errorf("GetChildType should return nil for unregistered type, got %v", got)
	}
}

func TestUnregisterChildType(t *testing.T) {
	resource.RegisterChildType(resource.ResourceTypeDef{
		Name:      "Temp Child",
		ShortName: "temp_child",
	})
	resource.UnregisterChildType("temp_child")

	got := resource.GetChildType("temp_child")
	if got != nil {
		t.Error("GetChildType should return nil after UnregisterChildType")
	}
}

// ===========================================================================
// Child fetcher registry
// ===========================================================================

func TestRegisterChildFetcher(t *testing.T) {
	called := false
	resource.RegisterChildFetcher("test_child_fetch", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		called = true
		return []resource.Resource{{ID: "test-1", Name: "Test"}}, nil
	})
	defer resource.UnregisterChildFetcher("test_child_fetch")

	fetcher := resource.GetChildFetcher("test_child_fetch")
	if fetcher == nil {
		t.Fatal("GetChildFetcher returned nil for registered fetcher")
	}

	resources, err := fetcher(context.Background(), nil, resource.ParentContext{"bucket": "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("fetcher was not called")
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if resources[0].ID != "test-1" {
		t.Errorf("resources[0].ID = %q, want %q", resources[0].ID, "test-1")
	}
}

func TestGetChildFetcher_NotRegistered(t *testing.T) {
	got := resource.GetChildFetcher("nonexistent_fetcher")
	if got != nil {
		t.Errorf("GetChildFetcher should return nil for unregistered fetcher, got non-nil")
	}
}

func TestUnregisterChildFetcher(t *testing.T) {
	resource.RegisterChildFetcher("temp_fetcher", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		return nil, nil
	})
	resource.UnregisterChildFetcher("temp_fetcher")

	got := resource.GetChildFetcher("temp_fetcher")
	if got != nil {
		t.Error("GetChildFetcher should return nil after UnregisterChildFetcher")
	}
}

func TestParentContext_Type(t *testing.T) {
	pc := resource.ParentContext{
		"bucket": "my-bucket",
		"prefix": "data/",
	}

	if pc["bucket"] != "my-bucket" {
		t.Errorf("ParentContext[bucket] = %q, want %q", pc["bucket"], "my-bucket")
	}
	if pc["prefix"] != "data/" {
		t.Errorf("ParentContext[prefix] = %q, want %q", pc["prefix"], "data/")
	}
}

// ===========================================================================
// Multiple child types on one parent
// ===========================================================================

func TestResourceTypeDef_MultipleChildren(t *testing.T) {
	td := resource.ResourceTypeDef{
		Children: []resource.ChildViewDef{
			{ChildType: "eks_nodes", Key: "enter"},
			{ChildType: "eks_events", Key: "e"},
			{ChildType: "eks_logs", Key: "L"},
		},
	}

	if len(td.Children) != 3 {
		t.Fatalf("Children length = %d, want 3", len(td.Children))
	}
	keys := []string{"enter", "e", "L"}
	for i, expected := range keys {
		if td.Children[i].Key != expected {
			t.Errorf("Children[%d].Key = %q, want %q", i, td.Children[i].Key, expected)
		}
	}
}

// ===========================================================================
// Child fetcher receives ParentContext correctly
// ===========================================================================

func TestChildFetcher_ReceivesParentContext(t *testing.T) {
	var receivedCtx resource.ParentContext
	resource.RegisterChildFetcher("ctx_test", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
		receivedCtx = parentCtx
		return nil, nil
	})
	defer resource.UnregisterChildFetcher("ctx_test")

	fetcher := resource.GetChildFetcher("ctx_test")
	expectedCtx := resource.ParentContext{
		"zone_id":   "/hostedzone/Z123",
		"zone_name": "example.com.",
	}
	_, _ = fetcher(context.Background(), nil, expectedCtx)

	if receivedCtx["zone_id"] != "/hostedzone/Z123" {
		t.Errorf("parentCtx[zone_id] = %q, want %q", receivedCtx["zone_id"], "/hostedzone/Z123")
	}
	if receivedCtx["zone_name"] != "example.com." {
		t.Errorf("parentCtx[zone_name] = %q, want %q", receivedCtx["zone_name"], "example.com.")
	}
}
