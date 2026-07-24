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
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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

// TestDetailWorkload_CacheReplay_RelatedOmitted_EnrichPresentFreshOp covers
// the replay carve-out: a fresh detail open for a resource whose related
// cache is already populated must NOT dispatch a new KindRelatedCheck (the
// cached rows are replayed directly into the panel instead — D6), while
// KindEnrichDetail must still be present, carrying a genuinely fresh
// (non-zero) op ID.
func TestDetailWorkload_CacheReplay_RelatedOmitted_EnrichPresentFreshOp(t *testing.T) {
	c, core := newDetailParityHeadlessController(t)
	const id = "i-workload0000008"
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: workloadSrcType})
	c.ApplyResourcesLoaded(workloadSrcType, []resource.Resource{workloadRes(id)}, nil, false)

	def0 := resource.GetRelated(workloadSrcType)[0]
	core.RelatedCacheSet(runtime.RelatedCacheKey(workloadSrcType, id), []runtime.RelatedCacheResult{
		{DefDisplayName: def0.DisplayName, Result: resource.RelatedCheckResult{TargetType: def0.TargetType, State: domain.RelatedResolved, Count: 3}},
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related != nil {
		t.Errorf("KindRelatedCheck task present on a cache-replay detail open; want omitted (D6: no re-fan-out over cached data). tasks: %v", taskKindsOf(tasks))
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
