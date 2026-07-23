package unit

// app_restart_availability_sweep_test.go — coverage for
// Controller.RestartAvailabilitySweep (core/app/actions_list.go): the neutral
// main-menu refresh bundle handleActionRefresh's menu branch now calls — bump
// AvailabilityGen + EnrichmentGen, reset the enrichment maps, strip
// wave2-sourced findings from every retained type's rows
// (Core.ClearAllWave2Findings, the cross-type twin of the deleted TUI
// clearAllWave2), reset the probe maps, clear the menu's cached
// availability/issue state (MenuClearAvailabilityIntent), and clear the
// swept-pair latch — returning the single TaskKindLoadAvailCache task that
// chains the sweep through the drain loop. No-op (nil tasks, gens unchanged)
// in no-cache mode.
//
// Transplanted from ref/detail-enrichment-261-attempt1 (git show); adapted
// away from attempt-1's dead core.SnapshotCache() (no such method in v2) to
// core.AnyLaneResources(rt), the current RowStore accessor for a type's
// cached rows regardless of lane.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestApply_ActionRefresh_MenuScreen_BumpsGensClearsMenuAndStripsWave2 drives
// Apply(ActionRefresh) on the menu screen (the default/root screen of a
// freshly-constructed Controller) and asserts the full mutation bundle:
// the TaskKindLoadAvailCache task is returned, both gens advance, the
// menu's cached availability/issue state is cleared through the exported
// GetMenuAvailability/GetMenuTruncated/GetMenuIssueCounts readers, and a
// wave2-sourced finding is stripped from a seeded type's rows while its
// wave1 finding survives (reusing the wave-3 RefreshListEnrichment idiom).
func TestApply_ActionRefresh_MenuScreen_BumpsGensClearsMenuAndStripsWave2(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 5, Truncated: true},
		runtime.PatchMenu{ResourceType: "ec2", Issues: 2},
	})
	if got := c.GetMenuAvailability(); got["ec2"] != 5 {
		t.Fatalf("precondition failed: GetMenuAvailability()[\"ec2\"] = %d, want 5", got["ec2"])
	}
	if got := c.GetMenuTruncated(); !got["ec2"] {
		t.Fatalf("precondition failed: GetMenuTruncated()[\"ec2\"] = %v, want true", got["ec2"])
	}
	if got := c.GetMenuIssueCounts(); got["ec2"] != 2 {
		t.Fatalf("precondition failed: GetMenuIssueCounts()[\"ec2\"] = %d, want 2", got["ec2"])
	}

	const rt = "test-menu-refresh-src"
	wave1 := domain.Finding{Code: "wave1-code", Phrase: "wave1 issue", Severity: domain.SevWarn, Source: "wave1"}
	wave2 := domain.Finding{Code: "wave2-code", Phrase: "wave2 issue", Severity: domain.SevBroken, Source: "wave2:" + rt}
	res := resource.Resource{ID: "res-001", Findings: []domain.Finding{wave1, wave2}}
	core.ObserveRows(rt, []resource.Resource{res}, nil, session.OriginFetch, false)

	genBefore := core.AvailabilityGen()
	enrichGenBefore := core.EnrichmentGen()

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})

	var loadTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.TaskKindLoadAvailCache {
			loadTask = &tasks[i]
		}
	}
	if loadTask == nil {
		t.Fatalf("Apply(ActionRefresh) on the menu returned no TaskKindLoadAvailCache task; tasks: %+v", tasks)
	}

	if got := core.AvailabilityGen(); got != genBefore+1 {
		t.Errorf("AvailabilityGen() = %d, want %d", got, genBefore+1)
	}
	if got := core.EnrichmentGen(); got != enrichGenBefore+1 {
		t.Errorf("EnrichmentGen() = %d, want %d", got, enrichGenBefore+1)
	}

	if got := c.GetMenuAvailability(); len(got) != 0 {
		t.Errorf("GetMenuAvailability() = %v after refresh, want cleared", got)
	}
	if got := c.GetMenuTruncated(); len(got) != 0 {
		t.Errorf("GetMenuTruncated() = %v after refresh, want cleared", got)
	}
	if got := c.GetMenuIssueCounts(); len(got) != 0 {
		t.Errorf("GetMenuIssueCounts() = %v after refresh, want cleared", got)
	}

	rows := core.AnyLaneResources(rt)
	if len(rows) != 1 {
		t.Fatalf("rows for %q after refresh = %d, want 1", rt, len(rows))
	}
	gotFindings := rows[0].Findings
	if len(gotFindings) != 1 || gotFindings[0].Source != "wave1" {
		t.Errorf("Findings after refresh = %+v, want exactly the surviving wave1 finding (wave2-sourced findings must be stripped)", gotFindings)
	}
}

// TestApply_ActionRefresh_MenuScreen_NoCacheMode_NilTasksGensUnchanged covers
// the no-cache short-circuit: RestartAvailabilitySweep must return nil
// (no TaskKindLoadAvailCache, no gen bump) when the session runs in
// no-cache mode.
func TestApply_ActionRefresh_MenuScreen_NoCacheMode_NilTasksGensUnchanged(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	core.SetNoCache(true)

	genBefore := core.AvailabilityGen()
	enrichGenBefore := core.EnrichmentGen()

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})

	if tasks != nil {
		t.Errorf("Apply(ActionRefresh) on the menu in no-cache mode returned %+v, want nil tasks", tasks)
	}
	if got := core.AvailabilityGen(); got != genBefore {
		t.Errorf("AvailabilityGen() = %d, want unchanged %d", got, genBefore)
	}
	if got := core.EnrichmentGen(); got != enrichGenBefore {
		t.Errorf("EnrichmentGen() = %d, want unchanged %d", got, enrichGenBefore)
	}
}
