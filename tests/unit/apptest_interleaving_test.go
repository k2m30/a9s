package unit

// apptest_interleaving_test.go — the payoff half of GitHub issue #488 item 1:
// drives core/app/apptest's deterministic interleaving harness against the
// REAL detail-view production path (Controller.Apply/ExecuteOne), rather than
// exercising the harness types in isolation.
//
// Fixture choice: "sfn" (not "ec2", the usual detail_workload_test.go
// fixture) — it registers both a detail enricher (enrichSfn) and related
// defs (checkSFNRole/checkSFNKMS/checkSFNLambda) that all call the SAME
// coalesceable DescribeStateMachine operation under the same DetailOperation,
// and it is one of the four services core/aws/coalesce.go wires a
// ledger-capable coalescing decorator for (core/aws/call_ledger.go) — the
// only fixture that lets apptest.NoDuplicateAWSCalls check something real.
//
// Action-set cut for scope (a): the full historical action space is open
// detail / open YAML / open JSON / Ctrl+R refresh / toggle related / Back /
// related-row retry. Kept: open (mandatory entry), refresh, toggle-related,
// open-yaml — refresh/toggle/text-screen-opens are where every review-round
// defect actually lived. Cut: open-json (identical dispatch path to open-yaml
// through beginDetailWorkloadLocked — see that method's own doc comment: the
// cache-replay suppression decision is screen-independent, so JSON adds a
// second search-tree multiplier for zero additional fold/cache coverage),
// Back and related-row retry (both explicitly lower-yield per the dispatch;
// both also reduce to the same beginDetailWorkloadLocked(forceRelated=true)
// call already exercised structurally by refresh). This keeps the search at
// depth 4 with real branching at 3 of the 4 steps.
//
// The two invariants that need a live *app.Controller/*awsclient.CallLedger
// reference (LatchCleared, NoDuplicateAWSCalls) aren't expressible through
// Check's (vs, pending, events) signature directly; NewController stashes the
// current replay's controller/ledger into closure-captured variables that
// Check reads. Safe ONLY because Explorer.Explore's search is a pure
// single-goroutine depth-first recursion (core/app/apptest/explorer.go) — the
// variables are never read from a stale replay.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/app/apptest"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/trace"
)

const sfnInterleavingType = "sfn"

// sfnFixtureResource builds a minimal "sfn" resource carrying the "arn" field
// checkSFNRole/checkSFNKMS/checkSFNLambda and sfnDescribe all key on.
// RawStruct is deliberately left nil: enrichDetail's own documented
// contract (core/aws/detail_enrich_engine.go) treats a nil RawStruct as a
// silent, error-free no-op — sufficient for every test below except the
// Explorer, which only needs the RELATED checkers' real coalescing, not the
// enricher's.
func sfnFixtureResource(id string) resource.Resource {
	return resource.Resource{
		ID:   id,
		Type: sfnInterleavingType,
		Fields: map[string]string{
			"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:" + id,
		},
	}
}

// completeTaskForOp re-scans sched.Pending() fresh and completes the first
// task of kind whose runtime.TaskOpID matches op — robust against Complete's
// own index-shifting, unlike tracking a Pending() index across calls.
func completeTaskForOp(sched *apptest.Scheduler, kind runtime.TaskKind, op domain.Gen) error {
	for i, task := range sched.Pending() {
		if task.Key.Kind == kind && runtime.TaskOpID(task.Payload) == op {
			return sched.Complete(i)
		}
	}
	return errors.New("apptest_interleaving_test: no pending task of the requested kind/op")
}

// ---------------------------------------------------------------------------
// Scope (a) — permanent exploration test
// ---------------------------------------------------------------------------

func sfnInterleavingActions() []apptest.Action {
	const id = "sfn-explorer-0000001"
	return []apptest.Action{
		{
			Name: "open(sfn/explorer-1)",
			Run: func(_ context.Context, ctrl *app.Controller) []runtime.TaskRequest {
				ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
				ctrl.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
				_, tasks := ctrl.Apply(app.Action{Kind: app.ActionSelect})
				return tasks
			},
		},
		{
			Name: "refresh",
			Run: func(_ context.Context, ctrl *app.Controller) []runtime.TaskRequest {
				_, tasks := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
				return tasks
			},
		},
		{
			Name: "toggle-related",
			Run: func(_ context.Context, ctrl *app.Controller) []runtime.TaskRequest {
				_, tasks := ctrl.Apply(app.Action{Kind: app.ActionToggleRelated})
				return tasks
			},
		},
		{
			Name: "open-yaml",
			Run: func(_ context.Context, ctrl *app.Controller) []runtime.TaskRequest {
				_, tasks := ctrl.Apply(app.Action{Kind: app.ActionOpenYAML})
				return tasks
			},
		},
	}
}

// TestApptestExplorer_DetailViewActionSpace_AllInvariantsHold is the
// permanent regression harness for GitHub issue #488's "action B lands while
// action A is in flight" defect class: every legal interleaving of
// open→refresh→toggle-related→open-yaml against the completion order of the
// enrich/related tasks those actions spawn must satisfy all five apptest
// invariants.
func TestApptestExplorer_DetailViewActionSpace_AllInvariantsHold(t *testing.T) {
	const id = "sfn-explorer-0000001"

	var lastCtrl *app.Controller
	var lastLedger *awsclient.CallLedger

	explorer := &apptest.Explorer{
		NewController: func() *app.Controller {
			c, core := newDetailParityHeadlessController(t)
			fake := &coalesceSfnFake{}
			ledger := awsclient.NewCallLedger()
			core.Session().Clients = &awsclient.ServiceClients{
				SFN: awsclient.NewCoalescingSFNWithLedger(fake, ledger),
			}
			lastCtrl = c
			lastLedger = ledger
			return c
		},
		Actions: sfnInterleavingActions(),
		Trace:   true,
		Check: func(vs app.ViewState, pending []runtime.TaskRequest, events []trace.Event) error {
			if err := apptest.NoOrphanLoadingRelated(vs, pending); err != nil {
				return err
			}
			if err := apptest.MonotonicDetailFold(events); err != nil {
				return err
			}
			if err := apptest.MonotonicCacheWrites(events); err != nil {
				return err
			}
			if err := apptest.NoDuplicateAWSCalls(lastLedger); err != nil {
				return err
			}
			// LatchCleared's contract ("cleared by quiescence") only makes
			// sense once nothing is still outstanding — checking it while a
			// refresh's own enrich is still pending would be a false
			// positive on a merely-not-yet-folded (not stuck) latch.
			if len(pending) == 0 {
				if err := apptest.LatchCleared(lastCtrl, runtime.RelatedCacheKey(sfnInterleavingType, id)); err != nil {
					return err
				}
			}
			return nil
		},
	}

	start := time.Now()
	failure := explorer.Explore(context.Background())
	elapsed := time.Since(start)
	t.Logf("Explore() wall time: %s", elapsed)

	if failure != nil {
		t.Errorf("interleaving invariant violated — replay: %s", failure)
	}
}

// ---------------------------------------------------------------------------
// Scope (c) — the harness's own failure output must be usable
// ---------------------------------------------------------------------------

// TestApptestExplorer_FailureReplay_IsHumanReadable drives a deliberately-
// violating Check (always errors) through a tiny two-action Explorer and
// asserts Explore's returned Failure carries a Replay string a human could
// actually follow and reproduce, plus an Error() that surfaces the
// underlying cause.
func TestApptestExplorer_FailureReplay_IsHumanReadable(t *testing.T) {
	wantErr := "deliberate violation for harness usability check"
	explorer := &apptest.Explorer{
		NewController: func() *app.Controller {
			c, _ := newDetailParityHeadlessController(t)
			return c
		},
		Actions: sfnInterleavingActions()[:1], // open only — keep the tree to one node
		Check: func(app.ViewState, []runtime.TaskRequest, []trace.Event) error {
			return errors.New(wantErr)
		},
	}

	failure := explorer.Explore(context.Background())
	if failure == nil {
		t.Fatal("expected a *Failure from a Check that always errors, got nil")
	}
	if failure.Replay == "" {
		t.Error("Failure.Replay is empty — nothing for a human to reproduce the violation from")
	}
	if !strings.Contains(failure.Replay, "open(") {
		t.Errorf("Failure.Replay = %q, want it to contain the applied action's own readable name", failure.Replay)
	}
	if !strings.Contains(failure.Error(), wantErr) {
		t.Errorf("Failure.Error() = %q, want it to contain the underlying Check error %q", failure.Error(), wantErr)
	}
	if !strings.Contains(failure.Error(), failure.Replay) {
		t.Errorf("Failure.Error() = %q, want it to contain the Replay string %q", failure.Error(), failure.Replay)
	}
}

// ---------------------------------------------------------------------------
// Scope (b) — named regression pins for known historical interleaving bugs
// ---------------------------------------------------------------------------

// TestApptestScheduler_StickyRefreshSupersededByPanelToggle_LatchOnlyClearsOnGenuineFreshFold
// pins the historical "sticky refresh" defect via its PANEL-TOGGLE vehicle
// specifically (detail_workload_test.go's existing TestStickyRefresh_* suite
// only exercises the related-row-retry vehicle): Ctrl+R arms
// session.PendingDetailRefresh; before its own enrichment folds, toggling the
// related panel off then back on begins a fresh, non-refresh, non-forced
// successor operation for the SAME resource (core/app/detail_cursor.go's
// ActionToggleRelated case) — that successor must still inherit
// SkipCache=true, and the latch must clear only once a genuinely fresh
// enrichment fold lands, never silently downgrading back to cached data.
func TestApptestScheduler_StickyRefreshSupersededByPanelToggle_LatchOnlyClearsOnGenuineFreshFold(t *testing.T) {
	const id = "sfn-sticky-0000001"
	ctx := context.Background()
	c, _ := newDetailParityHeadlessController(t)
	sched := apptest.NewScheduler(ctx, c)

	// open(sfn/sticky-1) → op1, drained to quiescence.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, openTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	sched.Submit(openTasks...)
	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the initial detail open: %v", err)
	}

	// refresh → op2, arms PendingDetailRefresh unconditionally at Apply time.
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	sched.Submit(refreshTasks...)
	refreshOp := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if refreshOp == 0 {
		t.Fatal("precondition failed: refresh's enrich task TaskOpID is 0")
	}

	// Complete ONLY op2's related-check, deliberately leaving its enrich
	// pending — the refresh's own enrichment has NOT yet folded.
	if err := completeTaskForOp(sched, runtime.KindRelatedCheck, refreshOp); err != nil {
		t.Fatalf("completing refresh's related-check: %v", err)
	}

	// toggle-related OFF then ON — the panel-toggle successor vehicle.
	_, offTasks := c.Apply(app.Action{Kind: app.ActionToggleRelated})
	if len(offTasks) != 0 {
		t.Fatalf("toggle OFF returned %v tasks, want none", taskKindsOf(offTasks))
	}
	_, onTasks := c.Apply(app.Action{Kind: app.ActionToggleRelated})
	sched.Submit(onTasks...)

	if !enrichSkipCache(t, onTasks) {
		t.Error("panel-toggle-ON's enrich task has SkipCache=false — the still-pending refresh demand was silently downgraded to cached enrichment")
	}

	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the toggle-ON workload (plus the leftover stale op2 enrich): %v", err)
	}

	if err := apptest.LatchCleared(c, runtime.RelatedCacheKey(sfnInterleavingType, id)); err != nil {
		t.Error(err)
	}
}

// TestApptestScheduler_PartialRelatedCache_StillDispatchesCheck_NoRowStrandedInLoading
// pins #261's Codex-flagged "partial related cache" defect
// (core/app/navigate.go's relatedCacheCoverage doc comment): a related cache
// covering only SOME of "sfn"'s registered defs — as if a prior operation
// completed some checks but never finished the rest before this operation
// began — must still dispatch KindRelatedCheck, or the still-uncached defs'
// rows are stranded in Loading forever (BeginDetailOperation has already
// invalidated whatever was running for them under any prior operation).
func TestApptestScheduler_PartialRelatedCache_StillDispatchesCheck_NoRowStrandedInLoading(t *testing.T) {
	const id = "sfn-partial-0000001"
	ctx := context.Background()
	c, core := newDetailParityHeadlessController(t)
	sched := apptest.NewScheduler(ctx, c)

	defs := resource.GetRelated(sfnInterleavingType)
	if len(defs) < 2 {
		t.Fatalf("resource.GetRelated(%q) has only %d defs, want at least 2 for a meaningful partial-coverage fixture", sfnInterleavingType, len(defs))
	}
	def0 := defs[0]
	core.RelatedCacheSet(runtime.RelatedCacheKey(sfnInterleavingType, id), []runtime.RelatedCacheResult{
		{DefDisplayName: def0.DisplayName, Result: resource.RelatedCheckResult{TargetType: def0.TargetType, State: domain.RelatedResolved, Count: 3}},
	})

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	sched.Submit(tasks...)

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related == nil {
		t.Fatalf("a PARTIAL related cache (1/%d defs) suppressed KindRelatedCheck; want still dispatched so the uncached defs are not stranded in Loading. tasks: %v", len(defs), taskKindsOf(tasks))
	}

	// Mid-flight: the related-check is still pending — no row may show
	// Loading without it.
	if err := apptest.NoOrphanLoadingRelated(c.Snapshot(), sched.Pending()); err != nil {
		t.Error(err)
	}

	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the related-check fan-out: %v", err)
	}

	// Quiescent: nothing pending, so nothing may still show Loading.
	if err := apptest.NoOrphanLoadingRelated(c.Snapshot(), sched.Pending()); err != nil {
		t.Error(err)
	}
}

// TestApptestScheduler_StaleEnrichResult_LandingAfterNewer_IsRejectedNotAccepted
// pins the ordering-inversion defect class the whole harness exists for: a
// fresh refresh operation (op2) begins before the original open's (op1)
// enrichment ever folds; op2's enrich result is completed FIRST (as a truly
// concurrent system would deliver it), op1's stale enrich arrives SECOND —
// op1's late fold must be rejected (superseded), never regressing the
// visible detail back to older enrichment, and MonotonicDetailFold must hold
// across the whole accepted-fold history.
func TestApptestScheduler_StaleEnrichResult_LandingAfterNewer_IsRejectedNotAccepted(t *testing.T) {
	const id = "sfn-stale-0000001"
	ctx := context.Background()
	c, _ := newDetailParityHeadlessController(t)
	sched := apptest.NewScheduler(ctx, c)
	rec, restore := apptest.StartTraceRecorder()
	defer restore()

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, openTasks := c.Apply(app.Action{Kind: app.ActionSelect}) // op1
	sched.Submit(openTasks...)
	op1 := runtime.TaskOpID(findTaskKind(openTasks, runtime.KindEnrichDetail).Payload)
	if op1 == 0 {
		t.Fatal("precondition failed: op1's enrich task TaskOpID is 0")
	}

	// A fresh refresh — op2 — begins before op1's enrichment ever folds.
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	sched.Submit(refreshTasks...)
	op2 := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if op2 == 0 || op2 == op1 {
		t.Fatalf("precondition failed: op2 = %d, want a fresh non-zero op distinct from op1 = %d", op2, op1)
	}

	// The NEWER operation's result arrives first.
	if err := completeTaskForOp(sched, runtime.KindEnrichDetail, op2); err != nil {
		t.Fatalf("completing op2's enrich: %v", err)
	}
	// The STALE (older) operation's result arrives late.
	if err := completeTaskForOp(sched, runtime.KindEnrichDetail, op1); err != nil {
		t.Fatalf("completing op1's stale enrich: %v", err)
	}

	var acceptedOps, rejectedOps []domain.Gen
	for _, ev := range rec.Events() {
		if ev.Kind != trace.KindFold || ev.EventType != "EnrichDetailResult" {
			continue
		}
		if ev.Accepted {
			acceptedOps = append(acceptedOps, domain.Gen(ev.OperationID))
		} else {
			rejectedOps = append(rejectedOps, domain.Gen(ev.OperationID))
		}
	}
	if len(acceptedOps) != 1 || acceptedOps[0] != op2 {
		t.Errorf("accepted EnrichDetailResult folds = %v, want exactly [%d] (only the newer operation)", acceptedOps, op2)
	}
	if len(rejectedOps) != 1 || rejectedOps[0] != op1 {
		t.Errorf("rejected EnrichDetailResult folds = %v, want exactly [%d] (the stale, late-arriving operation)", rejectedOps, op1)
	}
	if err := apptest.MonotonicDetailFold(rec.Events()); err != nil {
		t.Error(err)
	}
}

// TestApptestScheduler_ByIDPlaceholder_UnrelatedResourcesLoaded_NeverTouchesOutstandingOwnFetch
// pins the by-ID auto-open placeholder regression
// (app_autoopen_single_detail_fallback_test.go's
// TestApply_AutoOpenSingleDetail_UnrelatedResourcesLoaded_NoStubNoDetailOpen
// covers the same contract via a hand-built Handle call; this pins it via a
// REAL Scheduler-executed task instead): an "ami" by-ID placeholder pending
// its own fetch must not react at all to an unrelated type's
// ResourcesLoaded — delivered here as a genuinely executed KindFetchResources
// task for "ec2" (demo fixtures), not a synthetic event literal.
func TestApptestScheduler_ByIDPlaceholder_UnrelatedResourcesLoaded_NeverTouchesOutstandingOwnFetch(t *testing.T) {
	const targetID = "ami-0scheduler0000001"
	ctx := context.Background()
	c, core := newDetailParityHeadlessController(t)
	core.SetIsDemo(true)
	core.SetPreSuppliedClients(demo.NewServiceClients())
	core.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // side-effect only, mirrors newExecutorCore
		Clients:    nil,
		Gen:        core.ConnectGen(),
		StackDepth: 1,
	})
	sched := apptest.NewScheduler(ctx, c)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	// The "ami" placeholder's own fetch is deliberately never resolved in
	// this test — exactly the outstanding window autoOpenSingleDetail's
	// unrelated-type guard exists for.

	sched.Submit(runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "ec2"}})
	if err := sched.Complete(0); err != nil {
		t.Fatalf("completing the unrelated ec2 fetch task: %v", err)
	}

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v after an unrelated type's ResourcesLoaded landed, want BodyKindList — the ami placeholder pending its OWN fetch must be untouched", snap.Body.Kind)
	}
	if snap.Body.List == nil || len(snap.Body.List.Rows) != 0 {
		t.Errorf("Body.List.Rows = %+v, want still empty — an unrelated type's fetch must never resolve a different type's placeholder", snap.Body.List.Rows)
	}

	// A real ec2 ResourcesLoaded cascades into its own follow-up (its
	// TaskKindProbeEnrich issue-enrichment probe) — draining that unrelated
	// cascade to genuine quiescence must still leave the "ami" placeholder
	// exactly as untouched as the single-task snapshot above.
	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the unrelated ec2 cascade: %v", err)
	}
	snap = c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v after draining the unrelated ec2 cascade to quiescence, want BodyKindList — still untouched", snap.Body.Kind)
	}
	if snap.Body.List == nil || len(snap.Body.List.Rows) != 0 {
		t.Errorf("Body.List.Rows = %+v after draining the unrelated ec2 cascade, want still empty", snap.Body.List.Rows)
	}
}
