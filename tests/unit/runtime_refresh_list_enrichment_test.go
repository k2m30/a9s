package unit

// runtime_refresh_list_enrichment_test.go — coverage for
// Core.RefreshListEnrichment (core/runtime/handlers_resources.go): the
// neutral list-refresh enrichment-rerun bundle handleActionRefresh's list
// branch (core/app/actions_list.go) now calls — canonicalize the resource
// type, strip wave2-sourced findings from its cached rows, bump the per-type
// enrichment gen, clear the EnrichmentRan latch and the truncated-ID set —
// returning the bumped token (0 for a type with no registered issue
// enricher) that rides onto the refetch's FetchResourcesPayload.TypeGen so
// the resulting ResourcesLoaded can trigger a fresh enrichment probe.
//
// Transplanted from ref/detail-enrichment-261-attempt1 (git show); adapted:
//   - core.SnapshotCache()[rt] (no such method in v2) -> core.AnyLaneResources(rt).
//   - Dropped TestApply_ActionRefresh_ListScreen_FetchTask_AdmittedOverInFlightOriginalFetch
//     entirely and stripped the Generation/ReplaceInFlight assertions from the
//     TypeGen pin below: TaskRequest carries no Generation/ReplaceInFlight
//     fields in v2 and runtime.ShouldAdmitTask does not exist — that
//     same-key-admission concern is being redesigned as op-aware admission at
//     the core/web dispatch layer instead (see the coordinator's item E), not
//     as a per-TaskRequest field on the Controller/runtime side.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestRefreshListEnrichment_EnricherRegisteredType_TokenAdvancesTruncatedClearedWave2Stripped
// covers the enricher-registered-type axis: the token advances on each call,
// EnrichmentTruncatedIDs is cleared (seeded via the real production path —
// Core.HandleEvent(messages.EnrichmentChecked), not a raw field poke), and
// wave2-sourced findings are stripped from the type's cached rows while a
// wave1 finding on the same resource survives.
//
// EnrichmentRan has no exported reader anywhere in core/runtime, core/app,
// or internal/tui (it is write-only: set true in handleEnrichmentChecked,
// otherwise only ever deleted or wholesale-reset) — dropping that sub-check
// per the ses-rule-set-store precedent from the prior round.
func TestRefreshListEnrichment_EnricherRegisteredType_TokenAdvancesTruncatedClearedWave2Stripped(t *testing.T) {
	const rt = "test-refresh-enrich-src"
	awsclient.SetWave2EnricherForTest(t, rt, awsclient.IssueEnricher{Fn: awsclient.InFetcherWave2Sentinel, Priority: 100})

	c := newExecutorCore(t)

	wave1 := domain.Finding{Code: "wave1-code", Phrase: "wave1 issue", Severity: domain.SevWarn, Source: "wave1"}
	wave2 := domain.Finding{Code: "wave2-code", Phrase: "wave2 issue", Severity: domain.SevBroken, Source: "wave2:" + rt}
	res := resource.Resource{ID: "res-001", Findings: []domain.Finding{wave1, wave2}}
	c.ObserveRows(rt, []resource.Resource{res}, nil, session.OriginFetch, false)

	c.HandleEvent(messages.EnrichmentChecked{
		ResourceType: rt,
		TruncatedIDs: map[string]bool{"res-001": true},
	})
	if got := c.EnrichmentTruncatedIDs(rt); len(got) == 0 {
		t.Fatalf("precondition failed: EnrichmentTruncatedIDs(%q) empty after seeding via HandleEvent, got %v", rt, got)
	}

	genBefore := c.EnrichmentTypeGen(rt)

	tok1 := c.RefreshListEnrichment(rt)
	if tok1 == 0 {
		t.Fatal("RefreshListEnrichment on an enricher-registered type returned 0, want a non-zero bumped token")
	}
	if tok1 != genBefore+1 {
		t.Errorf("token = %d, want %d (genBefore+1)", tok1, genBefore+1)
	}
	if got := c.EnrichmentTypeGen(rt); got != tok1 {
		t.Errorf("EnrichmentTypeGen(%q) = %d after refresh, want it to match the returned token %d", rt, got, tok1)
	}

	tok2 := c.RefreshListEnrichment(rt)
	if tok2 != tok1+1 {
		t.Errorf("second RefreshListEnrichment call token = %d, want %d (advances again on each call)", tok2, tok1+1)
	}

	if got := c.EnrichmentTruncatedIDs(rt); len(got) != 0 {
		t.Errorf("EnrichmentTruncatedIDs(%q) = %v after refresh, want cleared", rt, got)
	}

	rows := c.AnyLaneResources(rt)
	if len(rows) != 1 {
		t.Fatalf("rows for %q after refresh = %d, want 1", rt, len(rows))
	}
	got := rows[0].Findings
	if len(got) != 1 || got[0].Source != "wave1" {
		t.Errorf("Findings after RefreshListEnrichment = %+v, want exactly the surviving wave1 finding (wave2-sourced findings must be stripped)", got)
	}
}

// TestRefreshListEnrichment_NonEnricherType_ReturnsZeroNoMutation covers the
// non-enricher-type axis: RefreshListEnrichment must no-op entirely (0
// returned, gen unchanged, rows untouched) rather than bumping/stripping for
// a type with no registered Wave2 enricher.
func TestRefreshListEnrichment_NonEnricherType_ReturnsZeroNoMutation(t *testing.T) {
	const rt = "test-refresh-no-enrich-src"
	c := newExecutorCore(t)

	res := resource.Resource{ID: "res-002", Findings: []domain.Finding{{Code: "wave2-code", Source: "wave2:" + rt}}}
	c.ObserveRows(rt, []resource.Resource{res}, nil, session.OriginFetch, false)

	genBefore := c.EnrichmentTypeGen(rt)

	tok := c.RefreshListEnrichment(rt)
	if tok != 0 {
		t.Errorf("RefreshListEnrichment on a non-enricher type returned %d, want 0", tok)
	}
	if got := c.EnrichmentTypeGen(rt); got != genBefore {
		t.Errorf("EnrichmentTypeGen(%q) = %d, want unchanged %d (non-enricher type must not bump)", rt, got, genBefore)
	}

	rows := c.AnyLaneResources(rt)
	if len(rows) != 1 || len(rows[0].Findings) != 1 || rows[0].Findings[0].Source != "wave2:"+rt {
		t.Errorf("rows for %q mutated by RefreshListEnrichment on a non-enricher type: %+v", rt, rows)
	}
}

// TestRefreshListEnrichment_AliasInput_CanonicalTypeGenBumped covers the
// alias axis using a real catalog alias ("instances" -> ShortName "ec2",
// core/aws/catalog_compute.go): RefreshListEnrichment must canonicalize
// BEFORE bumping — the method's own doc comment flags bumping under the
// alias key as a real historical hazard (it would never match the
// canonical-keyed read HandleResourcesLoaded's rerun-match check performs).
func TestRefreshListEnrichment_AliasInput_CanonicalTypeGenBumped(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, "ec2", awsclient.IssueEnricher{Fn: awsclient.InFetcherWave2Sentinel, Priority: 100})

	c := newExecutorCore(t)
	genBefore := c.EnrichmentTypeGen("ec2")

	tok := c.RefreshListEnrichment("instances")
	if tok == 0 {
		t.Fatal(`RefreshListEnrichment("instances") returned 0, want a non-zero bumped token (ec2 has a registered enricher)`)
	}
	if tok != genBefore+1 {
		t.Errorf("token = %d, want %d", tok, genBefore+1)
	}
	if got := c.EnrichmentTypeGen("ec2"); got != tok {
		t.Errorf(`EnrichmentTypeGen("ec2") = %d, want it to match the bumped token %d — the alias input must resolve to the canonical ShortName before bumping`, got, tok)
	}
	if got := c.EnrichmentTypeGen("instances"); got != 0 {
		t.Errorf(`EnrichmentTypeGen("instances") = %d, want 0 — the alias itself must never be used as a gen-map key`, got)
	}
}

// TestApply_ActionRefresh_ListScreen_FetchTaskCarriesBumpedTypeGen is the
// neutral end-to-end pin: on an open list of an enricher-registered type,
// Apply(ActionRefresh) must return a KindFetchResources task whose
// FetchResourcesPayload.TypeGen matches the token RefreshListEnrichment just
// bumped — the wiring gap class this whole method exists to close (the
// mutation happening without the token reaching the refetch task, or vice
// versa).
func TestApply_ActionRefresh_ListScreen_FetchTaskCarriesBumpedTypeGen(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, "ec2", awsclient.IssueEnricher{Fn: awsclient.InFetcherWave2Sentinel, Priority: 100})

	c, core := newDetailParityHeadlessController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", []resource.Resource{{ID: "i-e2e0000000000001"}}, nil, false)

	pre := c.Snapshot()
	if pre.Body.List == nil || len(pre.Body.List.Rows) != 1 {
		t.Fatalf("precondition failed: ec2 list = %+v, want 1 row", pre.Body.List)
	}

	genBefore := core.EnrichmentTypeGen("ec2")

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})

	var fetchTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchResources {
			fetchTask = &tasks[i]
		}
	}
	if fetchTask == nil {
		t.Fatalf("Apply(ActionRefresh) on an open list returned no KindFetchResources task; tasks: %+v", tasks)
	}
	payload, ok := fetchTask.Payload.(runtime.FetchResourcesPayload)
	if !ok {
		t.Fatalf("KindFetchResources task Payload = %T, want runtime.FetchResourcesPayload", fetchTask.Payload)
	}
	wantGen := genBefore + 1
	if payload.TypeGen != wantGen {
		t.Errorf("FetchResourcesPayload.TypeGen = %d, want %d (the bumped token)", payload.TypeGen, wantGen)
	}
	if got := core.EnrichmentTypeGen("ec2"); got != payload.TypeGen {
		t.Errorf(`EnrichmentTypeGen("ec2") = %d after refresh, want it to match the task's TypeGen %d`, got, payload.TypeGen)
	}
}
