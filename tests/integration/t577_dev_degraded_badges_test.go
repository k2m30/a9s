//go:build integration

package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// A degraded row is a Warning row the badge already counts, so a check left
// unrun on it cannot make the count short: after the Wave-2 sweep, no type
// whose demo list carries degraded rows shows its issue badge as a lower
// bound. The demo's ng list is the exception for another reason: ListNodegroups
// is refused for one cluster (fixtures.EKS DeniedNodegroupListClusters), so the
// stored list lacks that cluster's node groups and its count is a lower bound.
func TestT577Dev_DegradedRowsNeverOpenTheBadge(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "probe Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	var wave2Cmd tea.Cmd
	m, wave2Cmd = fullIntegrationApplyMsg(m, availMsg)
	m = drainWave2Enrichment(t, m, wave2Cmd)
	view := (&fullIntegrationScenario{t: t, model: m}).currentView()
	for _, short := range []string{"sqs", "cb", "ecs", "ecs-svc", "ecs-task", "kms", "athena", "eks", "ng", "ddb", "mwaa", "transfer", "opensearch", "lt"} {
		rt := resource.FindResourceType(short)
		wantLowerBound := short == "ng"
		found := false
		for _, line := range strings.Split(view, "\n") {
			if !strings.Contains(line, rt.Name+" (") {
				continue
			}
			found = true
			if _, badge, ok := strings.Cut(line, " issues:"); !ok || strings.Contains(strings.Fields(badge)[0], "+") != wantLowerBound {
				t.Errorf("%s after the sweep (lower bound wanted %v): %s", short, wantLowerBound, strings.TrimSpace(line))
			}
		}
		if !found {
			t.Errorf("the demo menu shows no line for %s", short)
		}
	}
}
