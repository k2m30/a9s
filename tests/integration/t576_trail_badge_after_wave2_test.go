//go:build integration

package integration

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// The trail's three GetTrailStatus signals are Broken Wave-2 findings, so
// they bump the menu badge once the Wave-2 sweep has answered: the demo's
// not-logging, delivery-error and stale-delivery trails and its public log
// bucket trail join the three Wave-1 issue rows.
func TestT576Dev_TrailBadgeCountsStatusSignalsAfterWave2(t *testing.T) {
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t576 Init")
	var cmd tea.Cmd
	m, cmd = fullIntegrationApplyMsg(m, initMsg)
	availMsg := fullIntegrationExtractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityPrefetched)
		return ok
	})
	var wave2Cmd tea.Cmd
	m, wave2Cmd = fullIntegrationApplyMsg(m, availMsg)
	m = drainWave2Enrichment(t, m, wave2Cmd)

	scenario := &fullIntegrationScenario{t: t, model: m}
	got, ok := renderedMenuCounts(t, scenario.currentView())["trail"]
	if !ok {
		t.Fatal("the demo menu shows no count for trail")
	}
	if got.issues != 7 {
		t.Errorf("trail after the Wave-2 sweep: menu shows (%d) issues:%d, want issues:7", got.rows, got.issues)
	}
}
