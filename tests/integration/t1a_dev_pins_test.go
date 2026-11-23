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

// The state-table witnesses added to the demo move the Wave-1 pins of their
// types. Once the Wave-2 sweep has answered, the menu shows the settled count
// for each of those types, and every witness row names its state in the list.
func TestT1aDev_StateWitnessesAfterTheSweep(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	clients := demo.NewServiceClients()
	m := tui.New(demo.DemoProfile, demo.DemoRegion, tui.WithClients(clients), tui.WithNoCache(true))
	t.Cleanup(m.CloseController)
	m, _ = fullIntegrationApplyMsg(m, tea.WindowSizeMsg{Width: 240, Height: 220})
	initMsg := fullIntegrationRequireCmdMsg(t, m.Init(), "t1a Init")
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
		"cfn":        {19, 8},
		"dbi":        {59, 36},
		"dbc":        {24, 22},
		"ddb":        {14, 11},
		"efs":        {13, 10},
		"ebs":        {11, 7},
		"eb":         {12, 5},
		"kms":        {23, 11},
		"lambda":     {43, 18},
		"redshift":   {36, 30},
		"vpc":        {9, 2},
		"opensearch": {14, 11},
		"ecs-task":   {21, 11},
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
		{"cfn", demofixtures.CFNDeletingStack, "delete in progress"},
		{"cfn", demofixtures.CFNDeleteFailedStack, "delete failed"},
		{"cfn", demofixtures.CFNTerminationProtectionOff, "update rollback failed"},
		{"dbi", demofixtures.DBIDeleting, "deleting"},
		{"dbc", demofixtures.DBCDeleting, "deleting"},
		{"ddb", demofixtures.GlobalOrdersReplicaID, "replication not authorized"},
		{"efs", demofixtures.DimEFSDeletedID, "deleted"},
		{"ebs", demofixtures.EBSDeletedVolumeID, "deleted"},
		{"eb", "acme-api-linking", "linking to"},
		{"eb", "acme-reports-grey", "health: grey"},
		{"kms", demofixtures.KMSCreatingReplicaKeyID, "creating in progress"},
		{"lambda", demofixtures.LambdaDeactivatingName, "deactivating"},
		{"lambda", demofixtures.LambdaDeactivatedName, "deactivated"},
		{"lambda", demofixtures.LambdaActiveNonInvocableName, "active but not invocable"},
		{"lambda", demofixtures.LambdaDeletingName, "deleting"},
		{"redshift", demofixtures.RedshiftFinalSnapshotID, "final snapshot"},
		{"redshift", demofixtures.RedshiftPrepForResizeID, "resize"},
		{"redshift", demofixtures.RedshiftResizeCleanupID, "resize"},
		{"redshift", demofixtures.RedshiftCancellingResizeID, "resize"},
		{"redshift", demofixtures.RedshiftRotatingKeysID, "rotating"},
		{"redshift", demofixtures.RedshiftUpdatingHSMID, "hardware security module"},
		{"vpc", demofixtures.VPCDeletingID, "deleting"},
		{"opensearch", demofixtures.CreatingDomain, "creating"},
		{"ecs-task", demofixtures.ECSTaskDeletedID, "deleted"},
	} {
		scenario.OpenList(w.short)
		scenario.ExpectRowStatusEquals(w.row, w.status)
	}
}
