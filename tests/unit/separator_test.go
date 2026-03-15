package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"
)

// BUG: Separator line uses Unicode box-drawing character ─ (U+2500) which
// renders as diamonds in some terminals. Must use plain ASCII dash "-".

func TestSeparator_UsesASCIIDash(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 200
	state.Height = 24
	state.Resources = []resource.Resource{
		{ID: "i-1", Name: "test", Fields: map[string]string{
			"instance_id": "i-1", "name": "test", "state": "running",
			"type": "t3.medium", "private_ip": "10.0.0.1",
			"public_ip": "54.1.2.3", "launch_time": "2026-01-01",
		}},
	}

	view := state.View()

	if strings.Contains(view.Content, "─") {
		t.Error("Separator must use ASCII dash '-', not Unicode box-drawing '─' (U+2500)")
	}
	if !strings.Contains(view.Content, "---") {
		t.Error("Separator should contain '---' (ASCII dashes)")
	}
}
