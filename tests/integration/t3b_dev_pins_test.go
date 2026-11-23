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

// Once the Wave-2 sweep has answered, the menu shows the settled count of each
// type that carries a policy witness, every cross-account witness names its
// share in the list, and a policy nobody could read reads "not inspected" on
// its row: as the mark on a Wave 2 row, as the "<noun> policy not inspected"
// warning on a Wave 1 row.
func TestT3BDev_PolicyWitnessesAfterTheSweep(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t3b Init")
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
	for short, want := range map[string]struct{ rows, issues int }{
		"sqs":          {12, 3},
		"sns":          {13, 1},
		"ecr":          {14, 3},
		"codeartifact": {5, 1},
		"role":         {56, 5},
		"opensearch":   {15, 12},
		"secrets":      {39, 21},
		"vpce":         {13, 9},
	} {
		got, ok := counts[short]
		if !ok {
			t.Errorf("the demo menu shows no count for %s", short)
			continue
		}
		if got.rows != want.rows || got.issues != want.issues {
			t.Errorf("%s after the Wave-2 sweep: menu shows (%d) issues:%d, want (%d) issues:%d", short, got.rows, got.issues, want.rows, want.issues)
		}
	}

	for _, w := range []struct{ short, row, status string }{
		{"kms", demofixtures.KMSCrossAccountPolicy, "key policy grants another account"},
		{"sqs", demofixtures.SQSCrossAccountPolicy, "queue policy grants another account"},
		{"sns", demofixtures.SNSCrossAccountPolicyARN, "topic policy grants another account"},
		{"lambda", demofixtures.LambdaCrossAccountPolicy, "resource policy grants another account"},
		{"ecr", demofixtures.ECRCrossAccountPolicy, "repository policy grants another account"},
		{"efs", demofixtures.EFSCrossAccountPolicy, "file system policy grants another account"},
		{"codeartifact", demofixtures.CodeArtifactCrossAccountPolicy, "repository policy grants another account"},
		{"opensearch", demofixtures.OpenSearchUnreadablePolicy, "access policy not inspected"},
		{"role", demofixtures.RoleUnreadableTrust, "trust policy not inspected"},
		{"vpce", demofixtures.VPCEUnreadablePolicy, "endpoint policy not inspected"},
		{"secrets", demofixtures.SecretUnreadablePolicy, "not inspected"},
		{"ecr", demofixtures.ECRPolicyMissing, "not inspected"},
	} {
		scenario.OpenList(w.short)
		scenario.ExpectRowStatusEquals(w.row, w.status)
	}
}
