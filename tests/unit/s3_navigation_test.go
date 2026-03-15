package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// BUG: S3 bucket → file → Enter (detail) → Esc → Esc should return to
// bucket list, not main menu. The navigation history must track each
// S3 drill-down step so Esc unwinds correctly:
//   MainMenu → BucketList → ObjectList → DetailView
//   Esc from Detail → ObjectList
//   Esc from ObjectList → BucketList
//   Esc from BucketList → MainMenu

func TestS3Navigation_EscFromDetailGoesToObjectList(t *testing.T) {
	// Start: viewing objects inside a bucket
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-bucket"
	state.S3Prefix = ""
	state.Width = 80
	state.Height = 24
	state.Resources = []resource.Resource{
		{
			ID: "readme.txt", Name: "readme.txt", Status: "file",
			Fields:     map[string]string{"key": "readme.txt", "size": "1 KB"},
			DetailData: map[string]string{"Key": "readme.txt", "Size": "1 KB"},
		},
	}
	state.SelectedIndex = 0

	// Enter on file → should open detail view
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentView != app.DetailView {
		t.Fatalf("Enter on S3 file should open DetailView, got view %d", state.CurrentView)
	}

	// Esc from detail → should return to object list (still inside bucket)
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("Esc from DetailView should return to ResourceListView, got view %d", state.CurrentView)
	}
	if state.S3Bucket != "my-bucket" {
		t.Errorf("Should still be inside bucket 'my-bucket', got %q", state.S3Bucket)
	}
}

func TestS3Navigation_EscFromObjectListGoesToBucketList(t *testing.T) {
	// Simulate full navigation: MainMenu → :s3 → Enter bucket → Esc
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 80
	state.Height = 24

	// Navigate to S3 via command
	state.CommandMode = true
	state.CommandText = "s3"
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("Expected ResourceListView after :s3, got %d", state.CurrentView)
	}
	if state.CurrentResourceType != "s3" {
		t.Fatalf("Expected resource type 's3', got %q", state.CurrentResourceType)
	}

	// Simulate buckets loaded
	buckets := []resource.Resource{
		{ID: "my-bucket", Name: "my-bucket", Fields: map[string]string{"name": "my-bucket"}},
	}
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	state = updated.(app.AppState)

	// Enter on bucket → drill down
	state.SelectedIndex = 0
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.S3Bucket != "my-bucket" {
		t.Fatalf("S3Bucket should be 'my-bucket', got %q", state.S3Bucket)
	}
	if cmd == nil {
		t.Fatal("Should return fetch command for S3 objects")
	}

	// Simulate objects loaded
	objects := []resource.Resource{
		{
			ID: "file.txt", Name: "file.txt", Status: "file",
			Fields:     map[string]string{"key": "file.txt", "size": "2 KB"},
			DetailData: map[string]string{"Key": "file.txt"},
		},
	}
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})
	state = updated.(app.AppState)

	// Esc from object list → should go back to bucket list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("Esc from object list should stay in ResourceListView (bucket list), got view %d", state.CurrentView)
	}
	if state.S3Bucket != "" {
		t.Errorf("S3Bucket should be empty (back to bucket list), got %q", state.S3Bucket)
	}
}

func TestS3Navigation_EscFromObjectsRestoresBucketData(t *testing.T) {
	// BUG: After bucket list → drill into bucket → esc back to bucket list,
	// the Resources must contain the original BUCKETS, not the objects
	// from inside the bucket. Otherwise names are blank (wrong Fields keys)
	// and count is wrong.
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = ""
	state.Width = 120
	state.Height = 24

	// Original bucket list
	buckets := []resource.Resource{
		{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"name": "bucket-a", "creation_date": "2025-01-01"}},
		{ID: "bucket-b", Name: "bucket-b", Fields: map[string]string{"name": "bucket-b", "creation_date": "2025-02-01"}},
		{ID: "bucket-c", Name: "bucket-c", Fields: map[string]string{"name": "bucket-c", "creation_date": "2025-03-01"}},
	}
	state.Resources = buckets

	// Push current view (bucket list) and drill into bucket-a
	state.SelectedIndex = 0
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	// Simulate objects loaded (replaces Resources)
	objects := []resource.Resource{
		{ID: "file1.txt", Name: "file1.txt", Status: "file", Fields: map[string]string{"key": "file1.txt", "size": "1 KB"}},
		{ID: "file2.txt", Name: "file2.txt", Status: "file", Fields: map[string]string{"key": "file2.txt", "size": "2 KB"}},
	}
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: objects})
	state = updated.(app.AppState)

	if len(state.Resources) != 2 {
		t.Fatalf("Inside bucket should have 2 objects, got %d", len(state.Resources))
	}

	// Esc back to bucket list — triggers re-fetch
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.S3Bucket != "" {
		t.Fatalf("S3Bucket should be empty after esc, got %q", state.S3Bucket)
	}
	if !state.Loading {
		t.Error("Should be loading (re-fetching bucket list)")
	}
	if cmd == nil {
		t.Fatal("Should return a fetch command to reload buckets")
	}

	// Simulate bucket list re-loaded
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: "s3", Resources: buckets})
	state = updated.(app.AppState)

	// Now resources should be the original buckets
	if len(state.Resources) != 3 {
		t.Errorf("After re-fetch, should have 3 buckets, got %d", len(state.Resources))
	}

	// Bucket names should be visible in rendered output
	view := state.View()
	if !strings.Contains(view.Content, "bucket-a") {
		t.Error("Bucket 'bucket-a' should be visible after returning to bucket list")
	}
	if !strings.Contains(view.Content, "bucket-b") {
		t.Error("Bucket 'bucket-b' should be visible after returning to bucket list")
	}
}

func TestS3Navigation_FullRoundTrip(t *testing.T) {
	// MainMenu → :s3 → bucket list → Enter bucket → object list →
	// Enter file → detail → Esc → object list → Esc → bucket list → Esc → MainMenu
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 80
	state.Height = 24

	// Step 1: :s3
	state.CommandMode = true
	state.CommandText = "s3"
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	// Load buckets
	updated, _ = state.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"name": "bucket-a"}},
		},
	})
	state = updated.(app.AppState)

	// Step 2: Enter bucket
	state.SelectedIndex = 0
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	// Load objects
	updated, _ = state.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources: []resource.Resource{
			{
				ID: "doc.pdf", Name: "doc.pdf", Status: "file",
				Fields:     map[string]string{"key": "doc.pdf", "size": "5 MB"},
				DetailData: map[string]string{"Key": "doc.pdf", "Size": "5 MB"},
			},
		},
	})
	state = updated.(app.AppState)

	// Step 3: Enter file → detail
	state.SelectedIndex = 0
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)
	if state.CurrentView != app.DetailView {
		t.Fatalf("Step 3: expected DetailView, got %d", state.CurrentView)
	}

	// Step 4: Esc → back to object list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)
	if state.CurrentView != app.ResourceListView {
		t.Fatalf("Step 4: expected ResourceListView (objects), got %d", state.CurrentView)
	}
	if state.S3Bucket != "bucket-a" {
		t.Errorf("Step 4: should still be in bucket 'bucket-a', got %q", state.S3Bucket)
	}

	// Step 5: Esc → back to bucket list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)
	if state.CurrentView != app.ResourceListView {
		t.Fatalf("Step 5: expected ResourceListView (buckets), got %d", state.CurrentView)
	}
	if state.S3Bucket != "" {
		t.Errorf("Step 5: S3Bucket should be empty (bucket list), got %q", state.S3Bucket)
	}

	// Step 6: Esc → back to main menu
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)
	if state.CurrentView != app.MainMenuView {
		t.Errorf("Step 6: expected MainMenuView, got %d", state.CurrentView)
	}
}
