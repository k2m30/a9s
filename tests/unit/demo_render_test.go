package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	demo "github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// TestDemoRender_ListViewShowsData verifies demo fixtures actually render
// visible data through the real view pipeline with production views.yaml.
// This is the TDD test that should have existed from the start.
func TestDemoRender_ListViewShowsData(t *testing.T) {
	cfg, _ := config.LoadFrom([]string{".a9s/views.yaml"})

	tests := []struct {
		shortName string
		expectIn  []string // substrings that MUST appear in rendered output
	}{
		{"ec2", []string{"web-prod-01", "running", "t3.large"}},
		{"s3", []string{"data-pipeline-logs", "webapp-assets-prod"}},
		{"lambda", []string{"api-gateway-authorizer", "nodejs20.x"}},
		{"dbi", []string{"prod-api-primary", "available", "aurora-post"}},
	}

	for _, tt := range tests {
		t.Run(tt.shortName, func(t *testing.T) {
			resources, ok := demo.GetResources(tt.shortName)
			if !ok {
				t.Fatalf("no demo data for %s", tt.shortName)
			}

			td := resource.FindResourceType(tt.shortName)
			if td == nil {
				t.Fatalf("resource type %s not found", tt.shortName)
			}

			k := keys.Default()
			m := views.NewResourceList(*td, cfg, k)
			m.SetSize(120, 30)
			m, _ = m.Init()
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: tt.shortName,
				Resources:    resources,
			})

			output := m.View()
			plain := stripANSI(output)

			for _, expect := range tt.expectIn {
				if !strings.Contains(plain, expect) {
					t.Errorf("[%s] expected %q in rendered output, not found.\nOutput:\n%s",
						tt.shortName, expect, plain)
				}
			}
		})
	}
}
