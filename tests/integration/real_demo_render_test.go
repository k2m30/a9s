//go:build integration

package integration

// real_demo_render_test.go — drives the EXACT production startup path that
// `./a9s --demo` uses, renders a few dbi detail views, and dumps the rendered
// frame to the test log. Phase 8.4 "user-observable visual sanity".
//
// No scenario-harness abstractions. If the dumped frame lacks the Issues
// section, the bug is real and reproducible — users WILL see this.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	demofixtures "github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func TestRealDemo_DBIDetailShowsIssues(t *testing.T) {
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})

	// Init returns the exact same cmd that main.go feeds to the tea runtime.
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "demo Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)

	// Drain every message produced by the ClientsReady → Availability →
	// Enrichment chain. Walk all yielded messages recursively.
	m = drainAll(t, m, cmd)

	// Navigate to the dbi list via the same NavigateMsg a user keypress would produce.
	m, cmd = fullIntegrationApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "dbi",
	})
	m = drainAll(t, m, cmd)

	// Fetch a concrete dbi resource from the clients (production path — same as
	// the app's own lookups).
	targets := []struct {
		id           string
		mustContain  []string
		mustBeAfter  string
	}{
		// Attention entries capitalize the first letter for presentation.
		{
			id:          demofixtures.WarnDbiMultiID,
			mustContain: []string{"No automated backups", "Publicly accessible", "Unencrypted storage"},
			mustBeAfter: "Attention",
		},
		{
			id:          "db-public-no-encryption",
			mustContain: []string{"No automated backups", "Publicly accessible", "Unencrypted storage", "Deletion protection off"},
			mustBeAfter: "Attention",
		},
		{
			id:          demofixtures.WarnDbiPublicMaintID,
			mustContain: []string{"Publicly accessible"},
			mustBeAfter: "Attention",
		},
	}

	for _, tc := range targets {
		tc := tc
		t.Run(tc.id, func(t *testing.T) {
			res := fullIntegrationMustFindResourceByID(t, clients, "dbi", tc.id)
			m2, cmd := fullIntegrationApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetDetail,
				ResourceType: "dbi",
				Resource:     &res,
			})
			m2 = drainAll(t, m2, cmd)

			view := fullIntegrationStripANSI(fullIntegrationViewContent(m2))
			t.Logf("\n--- rendered detail for %s ---\n%s\n--- end ---", tc.id, view)

			// Check Attention section header is present.
			hdrIdx := findAttentionHeaderLine(view)
			if hdrIdx < 0 {
				t.Fatalf("Attention section header NOT FOUND in rendered detail for %s. This is the bug the user sees.", tc.id)
			}

			// Every expected phrase must appear AFTER the header.
			lines := strings.Split(view, "\n")
			for _, phrase := range tc.mustContain {
				found := -1
				for i := hdrIdx + 1; i < len(lines); i++ {
					if strings.Contains(lines[i], phrase) {
						found = i
						break
					}
				}
				if found < 0 {
					t.Fatalf("phrase %q NOT FOUND in rendered detail for %s (Attention header at line %d)", phrase, tc.id, hdrIdx)
				}
			}
		})
	}
}

// drainAll walks every message produced by cmd (and its returned cmds,
// recursively) and applies each to the model. Mirrors the tea runtime behavior
// without the scenario-harness `shouldDrainFollowups` filter.
func drainAll(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	for _, msg := range fullIntegrationCollectCmdMessages(cmd) {
		var next tea.Cmd
		m, next = fullIntegrationApplyMsg(m, msg)
		if next != nil {
			m = drainAll(t, m, next)
		}
	}
	return m
}

// findAttentionHeaderLine returns the index of the first line whose content
// (stripped of box-drawing `│` and whitespace) is "Attention" or starts with
// "Attention (" (the header includes a count). Returns -1 when the Attention
// section header is absent.
func findAttentionHeaderLine(view string) int {
	for i, line := range strings.Split(view, "\n") {
		parts := strings.Split(line, "│")
		for _, p := range parts {
			cell := strings.TrimSpace(p)
			if cell == "Attention" || strings.HasPrefix(cell, "Attention (") {
				return i
			}
		}
	}
	return -1
}

// Ensure the resource package is referenced (avoid unused-import when the
// Resource type is only used indirectly via fullIntegration* helpers).
var _ = resource.Resource{}
