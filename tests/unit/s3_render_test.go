package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

func TestS3BucketList_RenderShowsBucketNames(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "" // bucket list, not inside a bucket
	state.Width = 120
	state.Height = 40
	state.SelectedIndex = 0

	// Create 5 buckets with realistic data
	state.Resources = []resource.Resource{
		{ID: "my-app-bucket", Name: "my-app-bucket", Fields: map[string]string{"name": "my-app-bucket", "creation_date": "2025-01-15"}},
		{ID: "logs-bucket", Name: "logs-bucket", Fields: map[string]string{"name": "logs-bucket", "creation_date": "2025-02-20"}},
		{ID: "terraform-state", Name: "terraform-state", Fields: map[string]string{"name": "terraform-state", "creation_date": "2024-06-01"}},
		{ID: "backups", Name: "backups", Fields: map[string]string{"name": "backups", "creation_date": "2024-01-10"}},
		{ID: "data-lake", Name: "data-lake", Fields: map[string]string{"name": "data-lake", "creation_date": "2025-03-01"}},
	}

	// Trigger breadcrumb update via ResourcesLoadedMsg
	model, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    state.Resources,
	})
	state = model.(app.AppState)

	view := state.View()
	content := view.Content

	// Every bucket name must appear in the rendered output
	for _, r := range state.Resources {
		if !strings.Contains(content, r.Name) {
			t.Errorf("Bucket %q should be visible in rendered output", r.Name)
		}
	}

	// Column headers
	if !strings.Contains(content, "Bucket Name") {
		t.Error("Should show 'Bucket Name' column header")
	}
	if !strings.Contains(content, "Creation Date") {
		t.Error("Should show 'Creation Date' column header")
	}

	// Count in breadcrumbs (Bug 14: count shown in breadcrumbs, not title)
	if !strings.Contains(content, "(5)") {
		t.Error("Should show '(5)' count in breadcrumbs")
	}
}

func TestS3BucketList_RenderWithZeroHeight(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = ""
	state.Width = 120
	state.Height = 0 // before WindowSizeMsg
	state.Resources = []resource.Resource{
		{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"name": "bucket-a", "creation_date": "2025-01-01"}},
	}

	view := state.View()
	// Should still show at least some content, not crash
	if !strings.Contains(view.Content, "bucket-a") {
		t.Error("Even with Height=0, bucket name should be visible (min 3 rows)")
	}
}

func TestEC2List_RenderShowsInstanceData(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 160
	state.Height = 40
	state.SelectedIndex = 0
	state.Resources = []resource.Resource{
		{
			ID: "i-abc123", Name: "web-server", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-abc123", "name": "web-server", "state": "running",
				"type": "t3.medium", "private_ip": "10.0.1.1",
				"public_ip": "54.1.2.3", "launch_time": "2026-01-15",
			},
		},
	}

	view := state.View()
	content := view.Content

	if !strings.Contains(content, "i-abc123") {
		t.Error("Instance ID should be visible")
	}
	if !strings.Contains(content, "web-server") {
		t.Error("Instance name should be visible")
	}
	if !strings.Contains(content, "running") {
		t.Error("Instance state should be visible")
	}

	// Check all column headers from EC2 definition
	rt := resource.FindResourceType("ec2")
	for _, col := range rt.Columns {
		if !strings.Contains(content, col.Title) {
			t.Errorf("EC2 column header %q should be visible", col.Title)
		}
	}
}
