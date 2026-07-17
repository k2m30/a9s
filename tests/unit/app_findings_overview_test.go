// app_findings_overview_test.go — RED pins for Controller.FindingsOverview()
// (#461): the cross-type findings rollup that unions Wave-1 row findings
// (RowStore rows' .Findings, reached via messages.AvailabilityChecked →
// Controller.Handle → runtime.Core.ObserveRows) with Wave-2 enrichment
// findings (Controller.enrichmentStore, reached ONLY via
// Controller.ApplyEnrichmentState — confirmed the single production writer,
// internal/tui/views/resourcelist.go:839; messages.EnrichmentChecked routed
// through Controller.Handle does not touch enrichmentStore at all).
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// newFindingsOverviewController builds a minimal hermetic Controller for
// directly driving AvailabilityChecked/ApplyEnrichmentState without a live
// or demo AWS connection. Routes through the blessed newTestControllerAndCore
// helper (app_patch_cache_intents_test.go) instead of calling app.New
// directly, per the construction-discipline gate
// (qa_controller_construction_discipline_test.go) — that helper already
// pairs t.TempDir() + t.Cleanup(c.Close) in the race-safe order.
func newFindingsOverviewController(t *testing.T) (*runtime.Core, *app.Controller) {
	t.Helper()
	ctrl, core := newTestControllerAndCore(t)
	return core, ctrl
}

// mkFindingResource builds a realistic resource.Resource carrying the given
// findings, matching real fetcher output shape (ID/Name/Type/Fields set,
// Findings populated directly as AvailabilityChecked.Resources carries them).
func mkFindingResource(id, name, typ string, findings ...domain.Finding) resource.Resource {
	return resource.Resource{
		ID:       id,
		Name:     name,
		Type:     typ,
		Fields:   map[string]string{},
		Findings: findings,
	}
}

// handleAvailability feeds a messages.AvailabilityChecked for resType through
// ctrl.Handle, stamped with the controller's current AvailabilityGen so it is
// not dropped as stale (AvailabilityChecked.AcceptZeroGen() is false).
func handleAvailability(t *testing.T, core *runtime.Core, ctrl *app.Controller, resType string, resources []resource.Resource, truncated bool) {
	t.Helper()
	ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: resType,
		HasResources: len(resources) > 0,
		Count:        len(resources),
		Truncated:    truncated,
		Resources:    resources,
		Gen:          core.AvailabilityGen(),
	})
}

// findGroup returns the FindingsGroup with the given code, or nil.
func findGroup(groups []app.FindingsGroup, code domain.FindingCode) *app.FindingsGroup {
	for i := range groups {
		if groups[i].Code == code {
			return &groups[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// TestFindingsOverview_Table
// ---------------------------------------------------------------------------

func TestFindingsOverview_Table(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, core *runtime.Core, ctrl *app.Controller)
		check func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals)
	}{
		{
			name: "single_type_two_resources_one_broken_code",
			setup: func(t *testing.T, core *runtime.Core, ctrl *app.Controller) {
				finding := domain.Finding{
					Code:     "ec2.impaired",
					Phrase:   "instance status check failed",
					Severity: domain.SevBroken,
					Source:   "wave1",
				}
				resources := []resource.Resource{
					mkFindingResource("i-0a1b2c3d4e500001", "prod-web-01", "ec2", finding),
					mkFindingResource("i-0a1b2c3d4e500002", "prod-web-02", "ec2", finding),
				}
				handleAvailability(t, core, ctrl, "ec2", resources, false)
			},
			check: func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals) {
				if len(groups) != 1 {
					t.Fatalf("len(groups) = %d, want 1: %+v", len(groups), groups)
				}
				g := groups[0]
				if g.Code != "ec2.impaired" {
					t.Errorf("groups[0].Code = %q, want %q", g.Code, "ec2.impaired")
				}
				if g.Count != 2 {
					t.Errorf("groups[0].Count = %d, want 2", g.Count)
				}
				if g.Severity != domain.SevBroken {
					t.Errorf("groups[0].Severity = %v, want SevBroken", g.Severity)
				}
				if len(g.Types) != 1 || g.Types[0] != "ec2" {
					t.Errorf("groups[0].Types = %v, want [ec2]", g.Types)
				}
				if len(g.SampleIDs) == 0 || len(g.SampleIDs) > 5 {
					t.Errorf("groups[0].SampleIDs = %v, want 1-5 entries", g.SampleIDs)
				}
				if g.Truncated {
					t.Error("groups[0].Truncated = true, want false")
				}
				if totals.Open != 2 {
					t.Errorf("totals.Open = %d, want 2", totals.Open)
				}
				if totals.Errors != 2 {
					t.Errorf("totals.Errors = %d, want 2", totals.Errors)
				}
				if totals.Warnings != 0 {
					t.Errorf("totals.Warnings = %d, want 0", totals.Warnings)
				}
				if totals.OpenGroups != 1 {
					t.Errorf("totals.OpenGroups = %d, want 1", totals.OpenGroups)
				}
			},
		},
		{
			name: "same_resource_same_code_wave1_and_wave2_counts_once",
			setup: func(t *testing.T, core *runtime.Core, ctrl *app.Controller) {
				wave1Finding := domain.Finding{
					Code:     "rds.storage.low",
					Phrase:   "storage below threshold",
					Severity: domain.SevWarn,
					Source:   "wave1",
				}
				resources := []resource.Resource{
					mkFindingResource("rds-prod-01", "orders-prod-db", "rds", wave1Finding),
				}
				handleAvailability(t, core, ctrl, "rds", resources, false)

				wave2Findings := map[string][]domain.Finding{
					"rds-prod-01": {{
						Code:     "rds.storage.low",
						Phrase:   "storage below threshold",
						Severity: domain.SevWarn,
						Source:   "wave2:rds",
					}},
				}
				ctrl.ApplyEnrichmentState("rds", 1, false, wave2Findings, nil)
			},
			check: func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals) {
				g := findGroup(groups, "rds.storage.low")
				if g == nil {
					t.Fatalf("no group for rds.storage.low: %+v", groups)
				}
				if g.Count != 1 {
					t.Errorf("groups[rds.storage.low].Count = %d, want 1 (resource counted once across wave-1+wave-2)", g.Count)
				}
				if totals.Open != 1 {
					t.Errorf("totals.Open = %d, want 1", totals.Open)
				}
			},
		},
		{
			name: "sort_severity_desc_count_desc_code_asc",
			setup: func(t *testing.T, core *runtime.Core, ctrl *app.Controller) {
				terminated := domain.Finding{Code: "ec2.terminated", Phrase: "instance terminated unexpectedly", Severity: domain.SevBroken, Source: "wave1"}
				warnCPU := domain.Finding{Code: "ec2.warn.cpu", Phrase: "sustained high CPU", Severity: domain.SevWarn, Source: "wave1"}
				warnDisk := domain.Finding{Code: "ec2.warn.disk", Phrase: "disk usage high", Severity: domain.SevWarn, Source: "wave1"}
				warnMem := domain.Finding{Code: "ec2.warn.mem", Phrase: "memory pressure high", Severity: domain.SevWarn, Source: "wave1"}
				resources := []resource.Resource{
					mkFindingResource("i-0f0000000000a001", "app-a", "ec2", terminated),
					mkFindingResource("i-0f0000000000a002", "app-b", "ec2", warnCPU),
					mkFindingResource("i-0f0000000000a003", "app-c", "ec2", warnCPU),
					mkFindingResource("i-0f0000000000a004", "app-d", "ec2", warnCPU),
					mkFindingResource("i-0f0000000000a005", "app-e", "ec2", warnDisk),
					mkFindingResource("i-0f0000000000a006", "app-f", "ec2", warnMem),
				}
				handleAvailability(t, core, ctrl, "ec2", resources, false)
			},
			check: func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals) {
				if len(groups) != 4 {
					t.Fatalf("len(groups) = %d, want 4: %+v", len(groups), groups)
				}
				wantOrder := []domain.FindingCode{"ec2.terminated", "ec2.warn.cpu", "ec2.warn.disk", "ec2.warn.mem"}
				for i, want := range wantOrder {
					if groups[i].Code != want {
						t.Errorf("groups[%d].Code = %q, want %q (severity desc, count desc, code asc order)", i, groups[i].Code, want)
					}
				}
				if totals.OpenGroups != 4 {
					t.Errorf("totals.OpenGroups = %d, want 4", totals.OpenGroups)
				}
			},
		},
		{
			name: "warn_only_vs_mixed_resource_errors_classification",
			setup: func(t *testing.T, core *runtime.Core, ctrl *app.Controller) {
				warnOnly := domain.Finding{Code: "lambda.warn.throttle", Phrase: "throttling detected", Severity: domain.SevWarn, Source: "wave1"}
				brokenAlso := domain.Finding{Code: "lambda.broken.timeout", Phrase: "invocations timing out", Severity: domain.SevBroken, Source: "wave1"}
				resources := []resource.Resource{
					mkFindingResource("orders-fn-warn", "orders-fn-warn", "lambda", warnOnly),
					mkFindingResource("orders-fn-broken", "orders-fn-broken", "lambda", warnOnly, brokenAlso),
				}
				handleAvailability(t, core, ctrl, "lambda", resources, false)
			},
			check: func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals) {
				warnGroup := findGroup(groups, "lambda.warn.throttle")
				if warnGroup == nil {
					t.Fatalf("no group for lambda.warn.throttle: %+v", groups)
				}
				if warnGroup.Count != 2 {
					t.Errorf("lambda.warn.throttle group Count = %d, want 2 (both resources carry this finding)", warnGroup.Count)
				}
				brokenGroup := findGroup(groups, "lambda.broken.timeout")
				if brokenGroup == nil {
					t.Fatalf("no group for lambda.broken.timeout: %+v", groups)
				}
				if brokenGroup.Count != 1 {
					t.Errorf("lambda.broken.timeout group Count = %d, want 1", brokenGroup.Count)
				}
				if totals.Open != 2 {
					t.Errorf("totals.Open = %d, want 2", totals.Open)
				}
				if totals.Errors != 1 {
					t.Errorf("totals.Errors = %d, want 1 (only the resource whose WORST finding is SevBroken)", totals.Errors)
				}
				if totals.Warnings != 1 {
					t.Errorf("totals.Warnings = %d, want 1 (the mixed resource's worst is SevBroken, so it must not also land in Warnings)", totals.Warnings)
				}
				if totals.OpenGroups != 2 {
					t.Errorf("totals.OpenGroups = %d, want 2", totals.OpenGroups)
				}
			},
		},
		{
			name: "truncated_wave2_propagates_to_group_and_totals",
			setup: func(t *testing.T, core *runtime.Core, ctrl *app.Controller) {
				wave2Findings := map[string][]domain.Finding{
					"arn:aws:s3:::a9s-orders-prod-logs": {{
						Code:     "s3.public_access",
						Phrase:   "public access not blocked",
						Severity: domain.SevWarn,
						Source:   "wave2:s3",
					}},
				}
				ctrl.ApplyEnrichmentState("s3", 1, true, wave2Findings, nil)
			},
			check: func(t *testing.T, groups []app.FindingsGroup, totals app.FindingsTotals) {
				g := findGroup(groups, "s3.public_access")
				if g == nil {
					t.Fatalf("no group for s3.public_access: %+v", groups)
				}
				if !g.Truncated {
					t.Error("groups[s3.public_access].Truncated = false, want true (wave-2 enrichment reported truncated)")
				}
				if !totals.Truncated {
					t.Error("totals.Truncated = false, want true")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			core, ctrl := newFindingsOverviewController(t)
			tc.setup(t, core, ctrl)
			groups, totals := ctrl.FindingsOverview()
			tc.check(t, groups, totals)
		})
	}
}

// ---------------------------------------------------------------------------
// TestFindingsOverview_Wave2CarryAcrossRefetch
// ---------------------------------------------------------------------------

// TestFindingsOverview_Wave2CarryAcrossRefetch pins the #461 no-flicker
// acceptance: enrichment findings recorded via ApplyEnrichmentState live in
// Controller.enrichmentStore, a store entirely separate from RowStore. A
// fresh Wave-1 refetch for the same type (same resource IDs, no wave-1
// findings on the refreshed rows) must not make the wave-2-sourced group
// disappear or shrink — enrichmentStore is untouched by
// messages.AvailabilityChecked handling.
func TestFindingsOverview_Wave2CarryAcrossRefetch(t *testing.T) {
	core, ctrl := newFindingsOverviewController(t)

	const resID = "i-0fedcba987654321"
	const resName = "critical-app-01"

	wave2Findings := map[string][]domain.Finding{
		resID: {{
			Code:     "ec2.cpu.sustained_high",
			Phrase:   "sustained high CPU",
			Severity: domain.SevWarn,
			Source:   "wave2:ec2",
		}},
	}
	ctrl.ApplyEnrichmentState("ec2", 1, false, wave2Findings, nil)

	groups, _ := ctrl.FindingsOverview()
	before := findGroup(groups, "ec2.cpu.sustained_high")
	if before == nil {
		t.Fatalf("precondition failed: no group for ec2.cpu.sustained_high after ApplyEnrichmentState: %+v", groups)
	}
	if before.Count != 1 {
		t.Fatalf("precondition failed: ec2.cpu.sustained_high Count = %d, want 1", before.Count)
	}

	// Fresh Wave-1 rows for the same type/ID, carrying no wave-1 findings of
	// their own — exactly what a routine availability refresh looks like.
	handleAvailability(t, core, ctrl, "ec2", []resource.Resource{
		mkFindingResource(resID, resName, "ec2"),
	}, false)

	groups, totals := ctrl.FindingsOverview()
	after := findGroup(groups, "ec2.cpu.sustained_high")
	if after == nil {
		t.Fatalf("ec2.cpu.sustained_high group disappeared after Wave-1 refetch (no-flicker acceptance broken): %+v", groups)
	}
	if after.Count != 1 {
		t.Errorf("ec2.cpu.sustained_high Count after refetch = %d, want 1 (unchanged)", after.Count)
	}
	if totals.Open != 1 {
		t.Errorf("totals.Open after refetch = %d, want 1", totals.Open)
	}
}

// ---------------------------------------------------------------------------
// TestFindingsOverview_DemoSweep_NonEmptyStable
// ---------------------------------------------------------------------------

// newDemoSweepController boots a demo Controller exactly as core/web's
// newSession does for a demo session (core/web/construct.go): pre-supplied
// demo clients, synchronous ClientsReady handshake, DrainSync running the
// full availability + Wave-2 enrichment sweep to completion before
// returning. Built on top of the blessed newTestControllerAndCore helper
// (app_patch_cache_intents_test.go) rather than calling app.New directly —
// its Profile/Region ("demo"/"us-east-1") already match demo.DemoProfile/
// demo.DemoRegion, and TypesTotal/TypesWithoutRules are derived from the
// menu's own type walk (menuAllItems), not the types slice passed to
// runtime.New, so the nil types there is not a gap for this test.
func newDemoSweepController(t *testing.T) (*runtime.Core, *app.Controller) {
	t.Helper()
	ctrl, core := newTestControllerAndCore(t)
	core.SetPreSuppliedClients(demo.NewServiceClients())
	core.SetNoCache(true)
	core.SetIsDemo(true)
	ctrl.SetUIMode("web")

	pre := core.PreSuppliedClients()
	intents, tasks := core.HandleClientsReady(runtime.ClientsReadyEvent{
		Clients: pre, Region: core.Region(), Gen: core.ConnectGen(), StackDepth: 1,
	})
	ctrl.ApplyIntents(intents)
	app.DrainSync(ctrl, tasks)

	return core, ctrl
}

func TestFindingsOverview_DemoSweep_NonEmptyStable(t *testing.T) {
	_, ctrl := newDemoSweepController(t)

	groups1, totals1 := ctrl.FindingsOverview()
	if len(groups1) == 0 {
		t.Fatal("FindingsOverview() after a full demo sweep returned 0 groups; demo fixtures carry real Sev findings (ec2, ecs, eks, cfn, kinesis, secrets)")
	}
	if totals1.TypesTotal == 0 {
		t.Error("totals.TypesTotal = 0, want > 0 (catalog types shown on the menu)")
	}
	if totals1.Resources == 0 {
		t.Error("totals.Resources = 0, want > 0 (sum of known availability counts after a full sweep)")
	}

	for _, g := range groups1 {
		if g.Code == "" {
			t.Errorf("group has empty Code: %+v", g)
		}
		if g.Phrase == "" {
			t.Errorf("group %q has empty Phrase", g.Code)
		}
		if g.Count < 1 {
			t.Errorf("group %q Count = %d, want >= 1", g.Code, g.Count)
		}
		if len(g.SampleIDs) > 5 {
			t.Errorf("group %q SampleIDs has %d entries, want <= 5", g.Code, len(g.SampleIDs))
		}
	}

	groups2, totals2 := ctrl.FindingsOverview()
	if len(groups2) != len(groups1) {
		t.Fatalf("FindingsOverview() called twice returned different group counts: %d vs %d — must be stable/idempotent", len(groups1), len(groups2))
	}
	for i := range groups1 {
		if groups1[i].Code != groups2[i].Code ||
			groups1[i].Phrase != groups2[i].Phrase ||
			groups1[i].Severity != groups2[i].Severity ||
			groups1[i].Count != groups2[i].Count ||
			groups1[i].Truncated != groups2[i].Truncated {
			t.Errorf("groups[%d] changed between calls: %+v vs %+v", i, groups1[i], groups2[i])
		}
	}
	if totals1 != totals2 {
		t.Errorf("FindingsTotals changed between calls: %+v vs %+v — must be stable/idempotent", totals1, totals2)
	}
}

// ---------------------------------------------------------------------------
// TestFindingsOverview_TypesWithoutRules
// ---------------------------------------------------------------------------

// TestFindingsOverview_TypesWithoutRules pins totals.TypesWithoutRules
// against the same formula core/runtime/scan_status.go already uses
// (availabilityOutcome, scan_status.go:101): a type has no rules when it
// declares zero FindingDefs AND has no registered Wave-2 issue enricher.
// Every currently-registered catalog type carries at least one of the two
// (want == 0 today) — this still pins the counting formula itself, so a
// regression in either operand (miscounting Findings, or a broken
// HasIssueEnricher lookup) is caught the moment a type's coverage actually
// changes, without this test needing to fabricate a catalog entry.
func TestFindingsOverview_TypesWithoutRules(t *testing.T) {
	core, ctrl := newFindingsOverviewController(t)

	want := 0
	for _, td := range resource.AllResourceTypes() {
		if len(td.Findings) == 0 && !core.HasIssueEnricher(td.ShortName) {
			want++
		}
	}

	_, totals := ctrl.FindingsOverview()
	if totals.TypesWithoutRules != want {
		t.Errorf("totals.TypesWithoutRules = %d, want %d (types with len(Findings)==0 and !HasIssueEnricher)", totals.TypesWithoutRules, want)
	}
}
