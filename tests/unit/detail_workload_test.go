package unit

// detail_workload_test.go — workload indivisibility for every Controller
// entry point that begins a detail operation (#261 boundary-sealing wave):
// Core.DetailOperationTasks is deleted; core/app.beginDetailWorkloadLocked
// is now the single builder every such entry point routes through, returning
// the COMPLETE workload (enrich + related, where both are registered) as one
// slice callers append wholesale — no per-half selective discard. The related
// half is omitted only when a cache replay populated the panel in the same
// call (D6: no re-fan-out over cached data).
//
// Fixture: "ec2" (resource.GetDetailEnricher + resource.GetRelated both
// non-empty — 1 detail enricher, 19 related defs, verified empirically) is
// the source type for every test. "cfn" (EC2's related target on the
// CloudFormation stack pivot) is itself both enricher- and related-registered,
// used by the openRelatedDetail test.
//
// Every entry point is driven through the exported Controller/Apply surface
// (app.Controller.Apply / .Handle) — never by calling the internal builder
// directly — so a regression where one entry point's integration with the
// shared builder breaks (wrong args, dropped return value, wrong gate) is
// caught the same way a real caller would trigger it.

import (
	"errors"
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

const workloadSrcType = "ec2"

// workloadRes builds a minimal ec2 resource fixture for these tests — only
// ID/Type matter, since none of the entry points under test read any other
// field before dispatching.
func workloadRes(id string) resource.Resource {
	return resource.Resource{ID: id, Type: workloadSrcType}
}

// workloadRelatedDefByTarget returns the ec2 RelatedDef whose TargetType
// matches target, and its index in resource.GetRelated("ec2") (the same
// order initDetailRelatedRows seeds ds.RelatedRows in, and so the same index
// visibleRelatedRowAt/ActionRelatedSelect's Arg addresses when no filter is
// active). t.Fatal's if not found — every caller of this helper depends on
// the def actually existing.
func workloadRelatedDefByTarget(t *testing.T, target string) (resource.RelatedDef, int) {
	t.Helper()
	defs := resource.GetRelated(workloadSrcType)
	for i, d := range defs {
		if d.TargetType == target {
			return d, i
		}
	}
	t.Fatalf("resource.GetRelated(%q) has no def targeting %q", workloadSrcType, target)
	return resource.RelatedDef{}, -1
}

// findTaskKind returns the first task in tasks whose Key.Kind matches kind,
// or nil.
func findTaskKind(tasks []runtime.TaskRequest, kind runtime.TaskKind) *runtime.TaskRequest {
	for i := range tasks {
		if tasks[i].Key.Kind == kind {
			return &tasks[i]
		}
	}
	return nil
}

// taskKindsOf returns the TaskKind of every task, for failure messages.
func taskKindsOf(tasks []runtime.TaskRequest) []runtime.TaskKind {
	kinds := make([]runtime.TaskKind, len(tasks))
	for i, t := range tasks {
		kinds[i] = t.Key.Kind
	}
	return kinds
}

// assertCompleteWorkload asserts tasks contains exactly one KindEnrichDetail
// and one KindRelatedCheck task, both carrying the same non-zero
// runtime.TaskOpID — the workload-indivisibility contract every entry point
// under test must satisfy when the source type registers both an enricher
// and related defs.
func assertCompleteWorkload(t *testing.T, tasks []runtime.TaskRequest) {
	t.Helper()
	enrich := findTaskKind(tasks, runtime.KindEnrichDetail)
	related := findTaskKind(tasks, runtime.KindRelatedCheck)
	if enrich == nil {
		t.Errorf("tasks missing KindEnrichDetail; got %v", taskKindsOf(tasks))
	}
	if related == nil {
		t.Errorf("tasks missing KindRelatedCheck; got %v", taskKindsOf(tasks))
	}
	if enrich == nil || related == nil {
		return
	}
	enrichOp := runtime.TaskOpID(enrich.Payload)
	relatedOp := runtime.TaskOpID(related.Payload)
	if enrichOp == 0 {
		t.Error("KindEnrichDetail task's TaskOpID is 0, want a real (non-zero) DetailOperation ID")
	}
	if relatedOp == 0 {
		t.Error("KindRelatedCheck task's TaskOpID is 0, want a real (non-zero) DetailOperation ID")
	}
	if enrichOp != relatedOp {
		t.Errorf("enrich op %d != related op %d — both halves of one workload must share the same DetailOperation ID", enrichOp, relatedOp)
	}
}

// openWorkloadDetail opens an ec2 list, loads one resource, and selects it —
// the shared setup every test below builds on. Returns the tasks dispatched
// by the detail-open itself (already asserted complete by
// TestDetailWorkload_DetailOpen_BothTasksSameOp; callers of this helper care
// about the tasks their OWN action produces afterward).
func openWorkloadDetail(t *testing.T, c *app.Controller, id string) []runtime.TaskRequest {
	t.Helper()
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	return tasks
}

// TestDetailWorkload_DetailOpen_BothTasksSameOp covers "detail open":
// selecting a row from an open list (ActionSelect → openSelectedListDetail →
// HandleNavigate(NavigateTargetDetail) → applyNavResult's NavigateKindPushDetail
// case).
func TestDetailWorkload_DetailOpen_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	tasks := openWorkloadDetail(t, c, "i-workload0000001")
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_YAMLDirectOpen_BothTasksSameOp covers "YAML/JSON direct
// open" (YAML half): ActionOpenYAML from a selected list row, never having
// opened the plain detail view first. Related-check dispatch is NOT gated
// on the YAML view actually showing a related panel — its result still
// write-throughs RelatedCache for the detail screen underneath / a later
// plain-detail open of the same resource (navigate.go's own doc comment on
// this call site).
func TestDetailWorkload_YAMLDirectOpen_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes("i-workload0000002")}, nil, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenYAML})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_JSONDirectOpen_BothTasksSameOp covers "YAML/JSON direct
// open" (JSON half) — see TestDetailWorkload_YAMLDirectOpen_BothTasksSameOp.
func TestDetailWorkload_JSONDirectOpen_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes("i-workload0000003")}, nil, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenJSON})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_BackToDetail_BothTasksSameOp covers "Back-to-detail":
// open a detail, push JSON on top of it (selectedResourceForAction reads the
// detail's own resource), then Esc back to the detail — handleActionBack's
// re-dispatch on any revealed detail screen.
func TestDetailWorkload_BackToDetail_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	openWorkloadDetail(t, c, "i-workload0000004")
	c.Apply(app.Action{Kind: app.ActionOpenJSON}) // push JSON on top of the detail

	_, tasks := c.Apply(app.Action{Kind: app.ActionBack})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_RelatedRowRetry_BothTasksSameOp covers "related-row
// retry": a focused RelatedUnknown row (blank, actionable, no count/IDs/
// filter) resolves IN PLACE on Enter/click — resource.RelatedEnter returns
// RelatedEnterResolveInPlace — re-dispatching the source's own workload
// rather than navigating. Driven via ActionRelatedSelect (the web click
// path, actions_nav.go's handleActionRelatedSelect); the keyboard
// equivalent (handleActionSelect's related-focus branch) is the identical
// shape and not independently covered here.
func TestDetailWorkload_RelatedRowRetry_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	const id = "i-workload0000005"
	openWorkloadDetail(t, c, id)

	def0, idx0 := workloadRelatedDefByTarget(t, "cfn")
	c.ApplyDetailRelatedResultForResource(workloadSrcType, id, def0.DisplayName, def0.TargetType,
		domain.RelatedUnknown, 0, false, "", false, nil, nil)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx0)})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_RelatedRowRetry_ForceRelated_CompleteButStaleCache_StillDispatchesRelatedCheck
// pins #261's forceRelated pin (item c's companion): a resolve-in-place
// retry (ActionRelatedSelect on a RelatedUnknown row) must still dispatch
// KindRelatedCheck even when relatedCacheCoverage would otherwise read the
// cache as COMPLETE (every def already has an entry, seeded stale here) —
// forceRelated's RelatedCacheDelete is deliberately NOT made redundant by
// PatchRelatedCache's idempotent-per-def replace (item c): a duplicate-free
// cache can still be fully populated and stale, and only the delete forces
// relatedCacheCoverage to read "incomplete" again.
func TestDetailWorkload_RelatedRowRetry_ForceRelated_CompleteButStaleCache_StillDispatchesRelatedCheck(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000015"
	openWorkloadDetail(t, c, id)

	def0, idx0 := workloadRelatedDefByTarget(t, "cfn")

	// Seed COMPLETE coverage for every ec2 def (including "cfn" itself) with
	// stale values — relatedCacheCoverage alone would read this as complete
	// and suppress KindRelatedCheck (item b), regardless of item c's
	// duplicate-free replace fix.
	defs := resource.GetRelated(workloadSrcType)
	results := make([]runtime.RelatedCacheResult, 0, len(defs))
	for _, d := range defs {
		results = append(results, runtime.RelatedCacheResult{
			DefDisplayName: d.DisplayName,
			Result:         resource.RelatedCheckResult{TargetType: d.TargetType, State: domain.RelatedResolved, Count: 3},
		})
	}
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), results)

	// Put the retried row into RelatedUnknown focus — the resolve-in-place
	// trigger (resource.RelatedEnter returns RelatedEnterResolveInPlace).
	c.ApplyDetailRelatedResultForResource(workloadSrcType, id, def0.DisplayName, def0.TargetType,
		domain.RelatedUnknown, 0, false, "", false, nil, nil)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx0)})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related == nil {
		t.Errorf("KindRelatedCheck task absent on a forceRelated resolve-in-place retry against a COMPLETE (but stale) related cache; want still dispatched (forceRelated's delete must not be made redundant by the duplicate-free replace fix). tasks: %v", taskKindsOf(tasks))
	}
}

// TestDetailWorkload_DetailCtrlR_BothTasksSameOp covers "detail Ctrl+R":
// ActionRefresh while a detail screen is on top always deletes the related
// cache first, so this is never the cache-replay case — both tasks must be
// present unconditionally.
func TestDetailWorkload_DetailCtrlR_BothTasksSameOp(t *testing.T) {
	c := newTestController(t)
	openWorkloadDetail(t, c, "i-workload0000006")

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_OpenRelatedDetail_BothTasksSameOp covers
// "openRelatedDetail": a single-RelatedID cache hit on a related row (State
// RelatedResolved, exactly one ResourceID, target already cached) resolves
// to NavigationKindDetail — applyRelatedNavResult's case for it calls
// core/app's openRelatedDetail directly. Uses ec2's "cfn" pivot (cfn is
// itself both enricher- and related-registered) as the target so the
// pushed detail's own workload is asserted complete.
func TestDetailWorkload_OpenRelatedDetail_BothTasksSameOp(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const srcID = "i-workload0000007"
	const targetID = "stack-workload0000001"
	openWorkloadDetail(t, c, srcID)

	// The target stack must already be RowStore-resident for
	// HandleRelatedNavigate's cache snapshot (sourced from session.RowStore)
	// to resolve a cache hit (NavigationKindDetail) instead of dispatching a
	// fetch. c.ApplyResourcesLoaded is a no-op here — it only writes through
	// when a matching list screen is on top (core/app/list_body.go's `ls !=
	// nil` gate), and the top screen at this point is ec2's detail, not a
	// cfn list — so this seeds the Core's RowStore directly instead.
	core.ObserveRows("cfn", []resource.Resource{{ID: targetID, Type: "cfn"}}, nil, session.OriginFetch, false)

	def0, idx0 := workloadRelatedDefByTarget(t, "cfn")
	c.ApplyDetailRelatedResultForResource(workloadSrcType, srcID, def0.DisplayName, def0.TargetType,
		domain.RelatedResolved, 1, false, "", false, []string{targetID}, nil)

	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx0)})
	assertCompleteWorkload(t, tasks)
}

// TestDetailWorkload_CacheReplay_CompleteCoverage_RelatedOmitted_EnrichPresentFreshOp
// covers the replay carve-out's COMPLETE-coverage half: a fresh detail open
// for a resource whose related cache already holds an entry for EVERY def
// resource.GetRelated(ec2) registers must NOT dispatch a new
// KindRelatedCheck (the cached rows are replayed directly into the panel
// instead — D6), while KindEnrichDetail must still be present, carrying a
// genuinely fresh (non-zero) op ID. Partial coverage (some but not all defs
// cached) is a DIFFERENT contract — see
// TestDetailWorkload_CacheReplay_PartialCoverage_RelatedStillDispatched.
func TestDetailWorkload_CacheReplay_CompleteCoverage_RelatedOmitted_EnrichPresentFreshOp(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000008"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	if len(defs) < 2 {
		t.Fatalf("resource.GetRelated(%q) has only %d defs, want several for a meaningful complete-coverage fixture", workloadSrcType, len(defs))
	}
	results := make([]runtime.RelatedCacheResult, 0, len(defs))
	for _, d := range defs {
		results = append(results, runtime.RelatedCacheResult{
			DefDisplayName: d.DisplayName,
			Result:         resource.RelatedCheckResult{TargetType: d.TargetType, State: domain.RelatedResolved, Count: 3},
		})
	}
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), results)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related != nil {
		t.Errorf("KindRelatedCheck task present on a COMPLETE cache-replay detail open; want omitted (D6: no re-fan-out over cached data). tasks: %v", taskKindsOf(tasks))
	}
	enrich := findTaskKind(tasks, runtime.KindEnrichDetail)
	if enrich == nil {
		t.Fatalf("KindEnrichDetail task missing on a cache-replay detail open; tasks: %v", taskKindsOf(tasks))
	}
	if op := runtime.TaskOpID(enrich.Payload); op == 0 {
		t.Error("KindEnrichDetail task's TaskOpID is 0 on a cache-replay open, want a fresh non-zero DetailOperation ID")
	}

	snap := c.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("Snapshot().Body.Detail is nil after a cache-replay detail open")
	}
	def0 := defs[0]
	var found bool
	for _, block := range snap.Body.Detail.Related {
		if block.Name == def0.DisplayName {
			found = true
			if block.State != domain.RelatedResolved || block.Count != 3 {
				t.Errorf("related block %q = {State:%v Count:%d}, want {State:RelatedResolved Count:3} replayed from cache", def0.DisplayName, block.State, block.Count)
			}
		}
	}
	if !found {
		t.Errorf("related panel has no block named %q — cache replay did not populate it", def0.DisplayName)
	}
}

// TestDetailWorkload_CacheReplay_PartialCoverage_RelatedStillDispatched
// covers the replay carve-out's PARTIAL-coverage half (#261 Codex P1): a
// related cache covering only SOME of ec2's registered defs must still
// merge every cached entry it has into the panel (render what you know), but
// must NOT suppress KindRelatedCheck — the still-uncached defs' rows would
// otherwise be stranded in Loading forever, since BeginDetailOperation has
// already invalidated whatever was still running for them under any prior
// operation ID.
func TestDetailWorkload_CacheReplay_PartialCoverage_RelatedStillDispatched(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000010"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	if len(defs) < 2 {
		t.Fatalf("resource.GetRelated(%q) has only %d defs, want at least 2 so a strict subset is possible", workloadSrcType, len(defs))
	}
	def0 := defs[0]
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), []runtime.RelatedCacheResult{
		{DefDisplayName: def0.DisplayName, Result: resource.RelatedCheckResult{TargetType: def0.TargetType, State: domain.RelatedResolved, Count: 3}},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	related := findTaskKind(tasks, runtime.KindRelatedCheck)
	if related == nil {
		t.Errorf("KindRelatedCheck task absent on a PARTIAL cache-replay detail open (%d/%d defs cached); want still dispatched so the uncached defs are not stranded in Loading. tasks: %v", 1, len(defs), taskKindsOf(tasks))
	}
	enrich := findTaskKind(tasks, runtime.KindEnrichDetail)
	if enrich == nil {
		t.Fatalf("KindEnrichDetail task missing on a cache-replay detail open; tasks: %v", taskKindsOf(tasks))
	}

	snap := c.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("Snapshot().Body.Detail is nil after a cache-replay detail open")
	}
	var found bool
	for _, block := range snap.Body.Detail.Related {
		if block.Name == def0.DisplayName {
			found = true
			if block.State != domain.RelatedResolved || block.Count != 3 {
				t.Errorf("related block %q = {State:%v Count:%d}, want {State:RelatedResolved Count:3} eagerly merged from the partial cache", def0.DisplayName, block.State, block.Count)
			}
		}
	}
	if !found {
		t.Errorf("related panel has no block named %q — partial cache replay did not merge the entry it DOES have", def0.DisplayName)
	}
}

// ---------------------------------------------------------------------------
// YAML/JSON direct-open cache-replay suppression (#261 Codex-flagged
// regression, item b): beginDetailWorkloadLocked's suppression decision
// (relatedCacheCoverage) is screen-independent — a YAML/JSON-only open has
// no detail panel to merge into, but must still suppress KindRelatedCheck on
// complete coverage exactly like the plain-detail path above
// (TestDetailWorkload_CacheReplay_CompleteCoverage_.../_PartialCoverage_...).
// Before the fix, replayRelatedCache bailed out (incomplete) whenever
// topDetailState() was nil — always true under a YAML/JSON screen — so
// every such open re-ran the ENTIRE related fan-out against AWS regardless
// of cache completeness.
// ---------------------------------------------------------------------------

func TestDetailWorkload_YAMLOpen_CompleteCoverage_RelatedOmitted(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000011"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	results := make([]runtime.RelatedCacheResult, 0, len(defs))
	for _, d := range defs {
		results = append(results, runtime.RelatedCacheResult{
			DefDisplayName: d.DisplayName,
			Result:         resource.RelatedCheckResult{TargetType: d.TargetType, State: domain.RelatedResolved, Count: 3},
		})
	}
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), results)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenYAML})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related != nil {
		t.Errorf("KindRelatedCheck task present on a YAML open with COMPLETE related-cache coverage; want omitted. tasks: %v", taskKindsOf(tasks))
	}
	if enrich := findTaskKind(tasks, runtime.KindEnrichDetail); enrich == nil {
		t.Fatalf("KindEnrichDetail task missing on a YAML open; tasks: %v", taskKindsOf(tasks))
	}
}

func TestDetailWorkload_YAMLOpen_PartialCoverage_RelatedStillDispatched(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000012"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	if len(defs) < 2 {
		t.Fatalf("resource.GetRelated(%q) has only %d defs, want at least 2 so a strict subset is possible", workloadSrcType, len(defs))
	}
	def0 := defs[0]
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), []runtime.RelatedCacheResult{
		{DefDisplayName: def0.DisplayName, Result: resource.RelatedCheckResult{TargetType: def0.TargetType, State: domain.RelatedResolved, Count: 3}},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenYAML})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related == nil {
		t.Errorf("KindRelatedCheck task absent on a YAML open with PARTIAL related-cache coverage (%d/%d defs cached); want still dispatched. tasks: %v", 1, len(defs), taskKindsOf(tasks))
	}
}

func TestDetailWorkload_JSONOpen_CompleteCoverage_RelatedOmitted(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000013"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	results := make([]runtime.RelatedCacheResult, 0, len(defs))
	for _, d := range defs {
		results = append(results, runtime.RelatedCacheResult{
			DefDisplayName: d.DisplayName,
			Result:         resource.RelatedCheckResult{TargetType: d.TargetType, State: domain.RelatedResolved, Count: 3},
		})
	}
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), results)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenJSON})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related != nil {
		t.Errorf("KindRelatedCheck task present on a JSON open with COMPLETE related-cache coverage; want omitted. tasks: %v", taskKindsOf(tasks))
	}
	if enrich := findTaskKind(tasks, runtime.KindEnrichDetail); enrich == nil {
		t.Fatalf("KindEnrichDetail task missing on a JSON open; tasks: %v", taskKindsOf(tasks))
	}
}

func TestDetailWorkload_JSONOpen_PartialCoverage_RelatedStillDispatched(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000014"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	defs := resource.GetRelated(workloadSrcType)
	if len(defs) < 2 {
		t.Fatalf("resource.GetRelated(%q) has only %d defs, want at least 2 so a strict subset is possible", workloadSrcType, len(defs))
	}
	def0 := defs[0]
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), []runtime.RelatedCacheResult{
		{DefDisplayName: def0.DisplayName, Result: resource.RelatedCheckResult{TargetType: def0.TargetType, State: domain.RelatedResolved, Count: 3}},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenJSON})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related == nil {
		t.Errorf("KindRelatedCheck task absent on a JSON open with PARTIAL related-cache coverage (%d/%d defs cached); want still dispatched. tasks: %v", 1, len(defs), taskKindsOf(tasks))
	}
}

// TestDetailWorkload_RelatedPanelToggle asserts ActionToggleRelated's two
// halves: toggling the panel OFF is a pure visibility flip (no tasks — there
// is nothing to fetch for a hidden panel), and toggling it back ON dispatches
// the complete detail workload, mirroring the TUI's handleToggleRelated so a
// panel opened after an earlier failure self-heals identically on both lanes.
// The related half rides the builder's cache replay: with an unpopulated
// cache (this harness folds no results) the related task must be present.
func TestDetailWorkload_RelatedPanelToggle(t *testing.T) {
	c := newTestController(t)
	openWorkloadDetail(t, c, "i-workload0000009")

	_, offTasks := c.Apply(app.Action{Kind: app.ActionToggleRelated})
	if len(offTasks) != 0 {
		t.Errorf("toggle OFF returned %v tasks, want none (hiding the panel fetches nothing)", taskKindsOf(offTasks))
	}

	_, onTasks := c.Apply(app.Action{Kind: app.ActionToggleRelated})
	assertCompleteWorkload(t, onTasks)
}

// ---------------------------------------------------------------------------
// Sticky-refresh contract (boundary-sealing wave): an explicit Ctrl+R
// refresh is a demand on the RESOURCE, not on the one DetailOperation that
// happened to carry it (core/app/navigate.go's beginDetailWorkloadLocked doc
// comment). A non-refresh successor operation for the same resource,
// beginning before the refresh's own enrichment result has folded, inherits
// SkipCache=true from session.PendingDetailRefresh — otherwise a
// panel-toggle or related-row retry fired mid-refresh would silently
// downgrade back to cached (pre-refresh) enrichment. The pending demand is
// cleared only by foldEnrichDetailResultLocked's success path (an error
// fold returns before reaching the clear), and by Session.Rotate().
// ---------------------------------------------------------------------------

// mintSuccessorWorkload seeds a fresh RelatedUnknown row for id's "cfn"
// related def and fires ActionRelatedSelect on it — resource.RelatedEnter
// resolves a blank Unknown row to RelatedEnterResolveInPlace, so this both
// begins a brand-new, non-refresh (refresh=false) DetailOperation AND
// immediately re-dispatches its complete workload, the vehicle every test
// below uses to mint a "successor" operation after an initial Ctrl+R. Forcing
// the row back to Unknown at the start of every call makes it safe to call
// repeatedly in one test regardless of what a prior call or fold left the
// row's state at.
func mintSuccessorWorkload(t *testing.T, c *app.Controller, id string) []runtime.TaskRequest {
	t.Helper()
	def0, idx0 := workloadRelatedDefByTarget(t, "cfn")
	c.ApplyDetailRelatedResultForResource(workloadSrcType, id, def0.DisplayName, def0.TargetType,
		domain.RelatedUnknown, 0, false, "", false, nil, nil)
	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx0)})
	return tasks
}

// enrichSkipCache extracts DetailCtx.SkipCache from tasks' KindEnrichDetail
// task, failing the test if the task or its DetailCtx is absent — every
// workload in this suite is for "ec2", which always registers a detail
// enricher.
func enrichSkipCache(t *testing.T, tasks []runtime.TaskRequest) bool {
	t.Helper()
	enrich := findTaskKind(tasks, runtime.KindEnrichDetail)
	if enrich == nil {
		t.Fatalf("tasks missing KindEnrichDetail; got %v", taskKindsOf(tasks))
	}
	payload, ok := enrich.Payload.(runtime.EnrichDetailPayload)
	if !ok {
		t.Fatalf("KindEnrichDetail Payload = %T, want runtime.EnrichDetailPayload", enrich.Payload)
	}
	if payload.DetailCtx == nil {
		t.Fatal("EnrichDetailPayload.DetailCtx is nil")
	}
	return payload.DetailCtx.SkipCache
}

// TestStickyRefresh_SuccessorInheritsSkipCacheWhilePending covers (i): Ctrl+R
// begins a refresh operation N (its own enrich task's SkipCache is true, the
// existing per-op contract); a successor operation N+1 minted before N's
// enrichment result folds (mintSuccessorWorkload's resolve-in-place retry)
// must ALSO carry SkipCache=true — inherited from the still-pending refresh,
// not the successor's own refresh=false.
func TestStickyRefresh_SuccessorInheritsSkipCacheWhilePending(t *testing.T) {
	c := newTestController(t)
	const id = "i-sticky0000001"
	openWorkloadDetail(t, c, id)

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if !enrichSkipCache(t, refreshTasks) {
		t.Fatal("precondition failed: Ctrl+R's own enrich task must carry SkipCache=true")
	}

	successorTasks := mintSuccessorWorkload(t, c, id)
	if !enrichSkipCache(t, successorTasks) {
		t.Error("successor operation's DetailCtx.SkipCache = false, want true — it must inherit the still-pending refresh demand")
	}
}

// TestStickyRefresh_SuccessfulFoldAtOpGreaterOrEqualClearsIt covers (ii): once
// an EnrichDetailResult for this resource folds successfully (Err == nil) at
// an operation ID >= the refresh op, the NEXT non-refresh workload for the
// same resource must have SkipCache=false again — the pending demand is
// satisfied, not permanently sticky.
func TestStickyRefresh_SuccessfulFoldAtOpGreaterOrEqualClearsIt(t *testing.T) {
	c := newTestController(t)
	const id = "i-sticky0000002"
	openWorkloadDetail(t, c, id)

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	refreshOp := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if refreshOp == 0 {
		t.Fatal("precondition failed: refresh enrich task's TaskOpID is 0")
	}

	c.Handle(messages.EnrichDetailResult{
		ResourceType: workloadSrcType,
		ResourceID:   id,
		// EnrichedRes replaces ds.Resource wholesale on a successful fold
		// (applyDetailEnrichmentForResourceLocked) — must carry the same
		// identity as the resource under test, or the detail's Resource.ID
		// goes blank and mintSuccessorWorkload's
		// ApplyDetailRelatedResultForResource match silently no-ops.
		EnrichedRes: workloadRes(id),
		OperationID: refreshOp,
		Err:         nil,
	})

	successorTasks := mintSuccessorWorkload(t, c, id)
	if enrichSkipCache(t, successorTasks) {
		t.Error("successor operation's DetailCtx.SkipCache = true after a successful enrich fold at op >= the refresh op, want false (cleared)")
	}
}

// TestStickyRefresh_ErrorFoldDoesNotClearIt covers (iii): an EnrichDetailResult
// fold with a non-nil Err must NOT clear the pending refresh — the next
// successor workload still inherits SkipCache=true, since the refresh demand
// was never actually satisfied (foldEnrichDetailResultLocked returns before
// reaching the clear when msg.Err != nil).
func TestStickyRefresh_ErrorFoldDoesNotClearIt(t *testing.T) {
	c := newTestController(t)
	const id = "i-sticky0000003"
	openWorkloadDetail(t, c, id)

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	refreshOp := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if refreshOp == 0 {
		t.Fatal("precondition failed: refresh enrich task's TaskOpID is 0")
	}

	c.Handle(messages.EnrichDetailResult{
		ResourceType: workloadSrcType,
		ResourceID:   id,
		OperationID:  refreshOp,
		Err:          errors.New("enrich boom"),
	})

	successorTasks := mintSuccessorWorkload(t, c, id)
	if !enrichSkipCache(t, successorTasks) {
		t.Error("successor operation's DetailCtx.SkipCache = false after an ERROR enrich fold, want true — an error fold must never clear the pending refresh")
	}
}

// TestStickyRefresh_DifferentResourceUnaffected covers (iv): resource A's
// pending refresh must never leak onto a DIFFERENT resource B's (same type,
// different ID) workload — the pending-refresh map is keyed by the full
// resource identity (runtime.RelatedCacheKey: type+ID), not by type alone.
func TestStickyRefresh_DifferentResourceUnaffected(t *testing.T) {
	c := newTestController(t)
	const idA = "i-sticky0000004"
	const idB = "i-sticky0000005"

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(idA), workloadRes(idB)}, nil, false)

	c.Apply(app.Action{Kind: app.ActionSelect}) // opens A (cursor 0)
	_, refreshTasksA := c.Apply(app.Action{Kind: app.ActionRefresh})
	if !enrichSkipCache(t, refreshTasksA) {
		t.Fatal("precondition failed: A's own Ctrl+R enrich task must carry SkipCache=true")
	}

	c.Apply(app.Action{Kind: app.ActionBack})     // back to the list (A, B)
	c.Apply(app.Action{Kind: app.ActionMoveDown}) // cursor -> B
	_, openTasksB := c.Apply(app.Action{Kind: app.ActionSelect})

	if enrichSkipCache(t, openTasksB) {
		t.Error("resource B's fresh (non-refresh) detail open has DetailCtx.SkipCache = true, want false — A's pending refresh must not leak onto a different resource")
	}
}

// TestStickyRefresh_RotateClearsIt covers (v): Session.Rotate() (profile/
// region switch) must clear every recorded pending refresh — a successor
// workload for the same resource after Rotate must not carry SkipCache=true
// forward into the new session.
func TestStickyRefresh_RotateClearsIt(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-sticky0000006"
	openWorkloadDetail(t, c, id)

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if !enrichSkipCache(t, refreshTasks) {
		t.Fatal("precondition failed: Ctrl+R's own enrich task must carry SkipCache=true")
	}

	core.Session().Rotate()

	successorTasks := mintSuccessorWorkload(t, c, id)
	if enrichSkipCache(t, successorTasks) {
		t.Error("successor operation's DetailCtx.SkipCache = true after Session.Rotate(), want false — Rotate must clear all pending-refresh state")
	}
}

// TestStickyRefresh_EnricherlessTypeDoesNotArmIt pins the arming precondition:
// only a workload that actually carries enrichment records a pending refresh.
// SkipCache is the record's sole consumer and only an EnrichDetailResult
// retires it (Core.HandleEnrichDetailResult), so arming it for one of the many
// types with no registered detail enricher would strand an entry no completion
// can ever clear — a per-refreshed-resource leak living until Rotate.
func TestStickyRefresh_EnricherlessTypeDoesNotArmIt(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const rt, id = "vpc", "vpc-0a1b2c3d4e5f60001"
	if resource.GetDetailEnricher(rt) != nil {
		t.Fatalf("fixture invalid: %q now registers a detail enricher — pick a type without one", rt)
	}

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: rt})
	c.ApplyResourcesLoaded(rt, []resource.Resource{{ID: id, Type: rt}}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if findTaskKind(tasks, runtime.KindEnrichDetail) != nil {
		t.Fatalf("fixture invalid: %q dispatched an enrich task; got %v", rt, taskKindsOf(tasks))
	}
	if _, armed := core.PendingDetailRefreshGet(runtime.RelatedCacheKey(rt, id)); armed {
		t.Error("refresh on an enricher-less type armed the pending-refresh record — nothing can ever clear it")
	}
}
