package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// Test full navigation round-trip for every resource type:
// MainMenu → :command → resource list → Enter (detail) → Esc → resource list → Esc → MainMenu

func makeResource(id string) resource.Resource {
	return resource.Resource{
		ID: id, Name: id, Status: "active",
		Fields: map[string]string{
			"instance_id": id, "name": id, "state": "active",
			"type": "t3.medium", "private_ip": "10.0.0.1",
			"public_ip": "54.1.2.3", "launch_time": "2026-01-01",
			"db_identifier": id, "engine": "mysql", "engine_version": "8.0",
			"status": "available", "class": "db.m5.large", "endpoint": "x.rds.amazonaws.com",
			"multi_az": "Yes",
			"cluster_id": id, "node_type": "cache.m5.large", "nodes": "3",
			"cluster_name": id, "version": "1.28", "platform_version": "eks.1",
			"secret_name": id, "description": "test", "last_accessed": "2026-01-01",
			"last_changed": "2026-01-01", "rotation_enabled": "No",
			"key": id, "size": "1 KB", "last_modified": "2026-01-01", "storage_class": "STANDARD",
		},
		DetailData: map[string]string{"ID": id, "Name": id},
		RawJSON:    `{"id":"` + id + `"}`,
	}
}

func testResourceRoundTrip(t *testing.T, resourceType, command string) {
	t.Helper()

	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 120
	state.Height = 24

	// Step 1: Execute command
	state.CommandMode = true
	state.CommandText = command
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("[%s] after :%s expected ResourceListView, got %d", resourceType, command, state.CurrentView)
	}
	if state.CurrentResourceType != resourceType {
		t.Fatalf("[%s] expected resource type %q, got %q", resourceType, resourceType, state.CurrentResourceType)
	}

	// Step 2: Load resources
	resources := []resource.Resource{makeResource(resourceType + "-001"), makeResource(resourceType + "-002")}
	updated, _ = state.Update(app.ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources})
	state = updated.(app.AppState)

	if len(state.Resources) != 2 {
		t.Fatalf("[%s] expected 2 resources, got %d", resourceType, len(state.Resources))
	}

	// Step 3: Enter on resource → detail view
	state.SelectedIndex = 0
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentView != app.DetailView {
		t.Fatalf("[%s] Enter should open DetailView, got %d", resourceType, state.CurrentView)
	}

	// Step 4: Esc → back to resource list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("[%s] Esc from detail should return to ResourceListView, got %d", resourceType, state.CurrentView)
	}
	if state.CurrentResourceType != resourceType {
		t.Errorf("[%s] should still be on resource type %q, got %q", resourceType, resourceType, state.CurrentResourceType)
	}

	// Step 5: Esc → back to main menu
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.MainMenuView {
		t.Errorf("[%s] Esc from resource list should return to MainMenuView, got %d", resourceType, state.CurrentView)
	}
}

func TestNavigationRoundTrip_EC2(t *testing.T) {
	testResourceRoundTrip(t, "ec2", "ec2")
}

func TestNavigationRoundTrip_RDS(t *testing.T) {
	testResourceRoundTrip(t, "rds", "rds")
}

func TestNavigationRoundTrip_Redis(t *testing.T) {
	testResourceRoundTrip(t, "redis", "redis")
}

func TestNavigationRoundTrip_DocDB(t *testing.T) {
	testResourceRoundTrip(t, "docdb", "docdb")
}

func TestNavigationRoundTrip_EKS(t *testing.T) {
	testResourceRoundTrip(t, "eks", "eks")
}

func TestNavigationRoundTrip_Secrets(t *testing.T) {
	testResourceRoundTrip(t, "secrets", "secrets")
}

// Also test d key (describe) round-trip, not just Enter
func testDescribeRoundTrip(t *testing.T, resourceType, command string) {
	t.Helper()

	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 120
	state.Height = 24

	// Navigate to resource type
	state.CommandMode = true
	state.CommandText = command
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	// Load resources
	updated, _ = state.Update(app.ResourcesLoadedMsg{
		ResourceType: resourceType,
		Resources:    []resource.Resource{makeResource(resourceType + "-001")},
	})
	state = updated.(app.AppState)

	// d → detail
	state.SelectedIndex = 0
	updated, _ = state.Update(tea.KeyPressMsg{Code: -1, Text: "d"})
	state = updated.(app.AppState)

	if state.CurrentView != app.DetailView {
		t.Fatalf("[%s] d should open DetailView, got %d", resourceType, state.CurrentView)
	}

	// Esc → back to list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("[%s] Esc from detail should return to ResourceListView, got %d", resourceType, state.CurrentView)
	}

	// Esc → main menu
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.MainMenuView {
		t.Errorf("[%s] Esc from list should return to MainMenuView, got %d", resourceType, state.CurrentView)
	}
}

func TestDescribeRoundTrip_EC2(t *testing.T)     { testDescribeRoundTrip(t, "ec2", "ec2") }
func TestDescribeRoundTrip_RDS(t *testing.T)     { testDescribeRoundTrip(t, "rds", "rds") }
func TestDescribeRoundTrip_Redis(t *testing.T)   { testDescribeRoundTrip(t, "redis", "redis") }
func TestDescribeRoundTrip_DocDB(t *testing.T)   { testDescribeRoundTrip(t, "docdb", "docdb") }
func TestDescribeRoundTrip_EKS(t *testing.T)     { testDescribeRoundTrip(t, "eks", "eks") }
func TestDescribeRoundTrip_Secrets(t *testing.T) { testDescribeRoundTrip(t, "secrets", "secrets") }

// Test y (JSON view) round-trip
func testJSONRoundTrip(t *testing.T, resourceType, command string) {
	t.Helper()

	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 120
	state.Height = 24

	state.CommandMode = true
	state.CommandText = command
	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	updated, _ = state.Update(app.ResourcesLoadedMsg{
		ResourceType: resourceType,
		Resources:    []resource.Resource{makeResource(resourceType + "-001")},
	})
	state = updated.(app.AppState)

	// y → JSON
	state.SelectedIndex = 0
	updated, _ = state.Update(tea.KeyPressMsg{Code: -1, Text: "y"})
	state = updated.(app.AppState)

	if state.CurrentView != app.JSONView {
		t.Fatalf("[%s] y should open JSONView, got %d", resourceType, state.CurrentView)
	}

	// Esc → back to list
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.ResourceListView {
		t.Fatalf("[%s] Esc from JSON should return to ResourceListView, got %d", resourceType, state.CurrentView)
	}

	// Esc → main menu
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView != app.MainMenuView {
		t.Errorf("[%s] Esc from list should return to MainMenuView, got %d", resourceType, state.CurrentView)
	}
}

func TestJSONRoundTrip_EC2(t *testing.T)     { testJSONRoundTrip(t, "ec2", "ec2") }
func TestJSONRoundTrip_RDS(t *testing.T)     { testJSONRoundTrip(t, "rds", "rds") }
func TestJSONRoundTrip_Redis(t *testing.T)   { testJSONRoundTrip(t, "redis", "redis") }
func TestJSONRoundTrip_DocDB(t *testing.T)   { testJSONRoundTrip(t, "docdb", "docdb") }
func TestJSONRoundTrip_EKS(t *testing.T)     { testJSONRoundTrip(t, "eks", "eks") }
func TestJSONRoundTrip_Secrets(t *testing.T) { testJSONRoundTrip(t, "secrets", "secrets") }
