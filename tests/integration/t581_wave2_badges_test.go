//go:build integration

package integration

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ECR scan on push, launch template IMDSv1 and EBS encryption, and the
// Transfer security policy are judged on the Wave-2 sweep, from reads the
// fetcher does not make. Their findings are Warning, which colours the row
// but does not bump the menu badge, so the settled badge counts the Wave-1
// issue rows plus the Wave-2 Broken findings, and each witness row still
// names its Wave-2 cause in the list once the sweep has answered.
func TestT581Dev_Wave2WitnessesAfterTheSweep(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t581 Init")
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
	counts := renderedMenuCounts(t, scenario.currentView())
	for short, want := range map[string]int{"ecr": 3, "lt": 3, "transfer": 9} {
		got, ok := counts[short]
		if !ok {
			t.Errorf("the demo menu shows no count for %s", short)
			continue
		}
		if got.issues != want {
			t.Errorf("%s after the Wave-2 sweep: menu shows (%d) issues:%d, want issues:%d", short, got.rows, got.issues, want)
		}
	}

	for _, w := range []struct{ short, row, status string }{
		{"ecr", demofixtures.ECRScanOnPushOff, "pushed images not scanned"},
		{"lt", "warn-lt-unencrypted", "EBS encryption disabled"},
		{"lt", "warn-lt-imdsv1", "IMDSv1 allowed"},
		{"transfer", demofixtures.WarnTransferLegacyPolicyID, "legacy security policy"},
	} {
		scenario.OpenList(w.short)
		scenario.ExpectRowStatusEquals(w.row, w.status)
	}
}
