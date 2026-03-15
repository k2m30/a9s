package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// BUG 1: Filter text duplicated — shown in both title area and status bar.
// Should only be in the status bar.
func TestFilterText_NotDuplicatedInTitle(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 24
	state.FilterMode = true
	state.Filter = "prod"
	state.Resources = []resource.Resource{
		{ID: "i-1", Name: "prod-web", Fields: map[string]string{"instance_id": "i-1", "name": "prod-web"}},
		{ID: "i-2", Name: "dev-api", Fields: map[string]string{"instance_id": "i-2", "name": "dev-api"}},
	}
	state.FilteredResources = []resource.Resource{state.Resources[0]}

	view := state.View()

	// Count occurrences of "prod" — should appear in status bar and data,
	// but NOT in the title line as "filter: prod"
	if strings.Contains(view.Content, "filter: prod") {
		t.Error("Title should NOT show 'filter: prod' — filter info belongs only in the status bar")
	}
}

// BUG 2: Total View output must fit within terminal height.
// Otherwise status bar is pushed off screen.
func TestView_FitsWithinTerminalHeight(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 20

	// 50 resources — more than terminal height
	resources := make([]resource.Resource, 50)
	for i := range resources {
		resources[i] = resource.Resource{
			ID: "i-" + string(rune('a'+i%26)), Name: "server",
			Fields: map[string]string{"instance_id": "i-x", "name": "server"},
		}
	}
	state.Resources = resources

	view := state.View()
	lines := strings.Split(view.Content, "\n")

	// Remove trailing empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > state.Height {
		t.Errorf("View output has %d lines but terminal height is %d — status bar will be off screen",
			len(lines), state.Height)
	}
}

func TestView_FitsWithinTerminalHeight_WithFilter(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 20
	state.FilterMode = true
	state.Filter = "server"

	resources := make([]resource.Resource, 50)
	for i := range resources {
		resources[i] = resource.Resource{
			ID: "i-x", Name: "server",
			Fields: map[string]string{"instance_id": "i-x", "name": "server"},
		}
	}
	state.Resources = resources
	state.FilteredResources = resources

	view := state.View()
	lines := strings.Split(view.Content, "\n")

	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > state.Height {
		t.Errorf("View with filter has %d lines but terminal height is %d",
			len(lines), state.Height)
	}
}
