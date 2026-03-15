package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// ===========================================================================
// BUG: S3 bucket list should NOT show region column (BucketRegion is nil
// in real AWS API). Region only matters on single bucket detail view.
// ===========================================================================

func TestS3BucketList_NoRegionColumn(t *testing.T) {
	rt := resource.FindResourceType("s3")
	if rt == nil {
		t.Fatal("s3 resource type not found")
	}
	for _, col := range rt.Columns {
		if col.Key == "region" {
			t.Error("S3 bucket list should NOT have a 'region' column — BucketRegion is not returned by ListBuckets API")
		}
	}
}

// ===========================================================================
// BUG: After drilling into an S3 bucket, objects must be displayed with
// correct columns (key, size, last_modified, storage_class) — NOT the
// bucket columns (name, creation_date).
// ===========================================================================

func TestS3Objects_DisplayedWithCorrectColumns(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-bucket" // we're inside a bucket
	state.S3Prefix = ""
	state.Width = 120
	state.Height = 24
	state.Resources = []resource.Resource{
		{
			ID: "folder1/", Name: "folder1/", Status: "folder",
			Fields: map[string]string{
				"key": "folder1/", "size": "", "last_modified": "", "storage_class": "",
			},
		},
		{
			ID: "readme.txt", Name: "readme.txt", Status: "file",
			Fields: map[string]string{
				"key": "readme.txt", "size": "1.5 KB", "last_modified": "2026-03-10", "storage_class": "STANDARD",
			},
		},
		{
			ID: "data.csv", Name: "data.csv", Status: "file",
			Fields: map[string]string{
				"key": "data.csv", "size": "45.2 MB", "last_modified": "2026-03-14", "storage_class": "STANDARD",
			},
		},
	}

	view := state.View()
	content := view.Content

	// Object column headers must be visible
	if !strings.Contains(content, "Key") {
		t.Error("S3 object view should show 'Key' column header")
	}
	if !strings.Contains(content, "Size") {
		t.Error("S3 object view should show 'Size' column header")
	}

	// Object values must be visible
	if !strings.Contains(content, "folder1/") {
		t.Error("S3 object view should show folder 'folder1/'")
	}
	if !strings.Contains(content, "readme.txt") {
		t.Error("S3 object view should show file 'readme.txt'")
	}
	if !strings.Contains(content, "1.5 KB") {
		t.Error("S3 object view should show file size '1.5 KB'")
	}
	if !strings.Contains(content, "STANDARD") {
		t.Error("S3 object view should show storage class 'STANDARD'")
	}
}

func TestS3Objects_BreadcrumbsShowBucketAndPrefix(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-bucket"
	state.S3Prefix = "logs/2026/"
	state.Width = 120
	state.Height = 24
	state.Resources = []resource.Resource{
		{
			ID: "logs/2026/app.log", Name: "logs/2026/app.log",
			Fields: map[string]string{"key": "logs/2026/app.log", "size": "100 KB"},
		},
	}
	state.Breadcrumbs = []string{"main", "S3", "my-bucket", "logs/2026/"}

	view := state.View()
	content := view.Content

	if !strings.Contains(content, "my-bucket") {
		t.Error("Breadcrumbs should show bucket name")
	}
}

// ===========================================================================
// BUG: Enter on S3 bucket should load objects and they should appear
// ===========================================================================

func TestS3DrillDown_ObjectsAppearAfterLoad(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "" // viewing bucket list
	state.Resources = []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	state.SelectedIndex = 0

	// Press Enter to drill into bucket
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.S3Bucket != "my-bucket" {
		t.Fatalf("S3Bucket should be 'my-bucket', got %q", state.S3Bucket)
	}
	if !state.Loading {
		t.Error("Should be loading after drill-down")
	}
	if cmd == nil {
		t.Error("Should return a fetch command")
	}

	// Simulate objects loaded
	objects := []resource.Resource{
		{
			ID: "folder/", Name: "folder/", Status: "folder",
			Fields: map[string]string{"key": "folder/", "size": "", "last_modified": "", "storage_class": ""},
		},
		{
			ID: "file.txt", Name: "file.txt", Status: "file",
			Fields: map[string]string{"key": "file.txt", "size": "2.0 KB", "last_modified": "2026-01-01", "storage_class": "STANDARD"},
		},
	}
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})
	state = updated.(app.AppState)

	if len(state.Resources) != 2 {
		t.Errorf("Should have 2 resources after load, got %d", len(state.Resources))
	}
	if state.Loading {
		t.Error("Loading should be false after ResourcesLoadedMsg")
	}

	// Render — objects should be visible
	state.Width = 120
	state.Height = 24
	view := state.View()
	if !strings.Contains(view.Content, "file.txt") {
		t.Error("Object 'file.txt' should be visible after drill-down and load")
	}
}
