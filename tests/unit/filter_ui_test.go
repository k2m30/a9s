package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// BUG: Pressing / activates filter mode but there's no visual feedback.
// The status bar should show "/" prompt immediately, like k9s does.
// When typing, it should show "/text" with match count.

func TestFilterMode_StatusBarShowsSlashPrompt(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 24
	state.Resources = []resource.Resource{
		{ID: "i-1", Name: "web", Fields: map[string]string{"instance_id": "i-1", "name": "web"}},
	}

	// Press /
	updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "/"})
	state = updated.(app.AppState)

	if !state.FilterMode {
		t.Fatal("/ should activate FilterMode")
	}

	view := state.View()

	// Status bar must show "/" to indicate filter mode is active
	// Split by newlines, last non-empty line is the status bar
	lines := strings.Split(view.Content, "\n")
	statusBar := ""
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			statusBar = lines[i]
			break
		}
	}

	if !strings.Contains(statusBar, "/") {
		t.Errorf("Status bar should show '/' prompt in filter mode, got: %q", statusBar)
	}
}

func TestFilterMode_StatusBarShowsTypedText(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 24
	state.Resources = []resource.Resource{
		{ID: "i-1", Name: "prod-web", Fields: map[string]string{"instance_id": "i-1", "name": "prod-web"}},
		{ID: "i-2", Name: "dev-api", Fields: map[string]string{"instance_id": "i-2", "name": "dev-api"}},
	}

	// Activate filter and type "prod"
	state.FilterMode = true
	for _, ch := range "prod" {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		state = updated.(app.AppState)
	}

	view := state.View()

	// Status bar must show "/prod"
	if !strings.Contains(view.Content, "/prod") {
		t.Error("Status bar should show '/prod' while typing filter")
	}
}

func TestFilterMode_StatusBarShowsMatchCount(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 24
	state.Resources = []resource.Resource{
		{ID: "i-1", Name: "prod-web", Fields: map[string]string{"name": "prod-web"}},
		{ID: "i-2", Name: "dev-api", Fields: map[string]string{"name": "dev-api"}},
		{ID: "i-3", Name: "prod-db", Fields: map[string]string{"name": "prod-db"}},
	}

	// Activate filter and type "prod"
	state.FilterMode = true
	for _, ch := range "prod" {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: string(ch)})
		state = updated.(app.AppState)
	}

	view := state.View()

	// Should show match count like "/prod (2/3)"
	if !strings.Contains(view.Content, "2") {
		t.Error("Status bar should show match count (2 matches)")
	}
}
