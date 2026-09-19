package unit

// Fixture choice: "sfn" (not "ec2", the usual detail_workload_test.go
// fixture) — it registers both a detail enricher (enrichSfn) and related
// defs (checkSFNRole/checkSFNKMS/checkSFNLambda) that all call the SAME
// coalesceable DescribeStateMachine operation under the same DetailOperation,
// and it is one of the four services core/aws/coalesce.go wires a
// ledger-capable coalescing decorator for (core/aws/call_ledger.go) — the
// only fixture that lets apptest.NoDuplicateAWSCalls check something real.
//
// Action set: of open detail / open YAML / open JSON / Ctrl+R refresh /
// toggle related / Back / related-row retry, the search keeps open
// (mandatory entry), refresh, toggle-related and open-yaml. open-json shares
// open-yaml's dispatch path through beginDetailWorkloadLocked (see that
// method's own doc comment: the cache-replay suppression decision is
// screen-independent), so it would add a second search-tree multiplier for
// zero additional fold/cache coverage; Back and related-row retry both
// reduce to the same beginDetailWorkloadLocked(forceRelated=true) call
// refresh already exercises structurally. This keeps the search at depth 4
// with real branching at 3 of the 4 steps.
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
	"fmt"
	"strings"
	"testing"
	"time"

	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

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
// checkSFNRole/checkSFNKMS/checkSFNLambda and sfnDescribe all key on, PLUS a
// real RawStruct (sfntypes.StateMachineListItem with StateMachineArn set) so
// enrichSfn's own id/fetch actually run instead of short-circuiting.
//
// RawStruct must be set: with it nil, enrichment returns
// awsclient.ErrDetailEnrichSkipped, which never clears the latch.
func sfnFixtureResource(id string) resource.Resource {
	arn := "arn:aws:states:us-east-1:123456789012:stateMachine:" + id
	return resource.Resource{
		ID:   id,
		Type: sfnInterleavingType,
		Fields: map[string]string{
			"arn": arn,
		},
		RawStruct: sfntypes.StateMachineListItem{StateMachineArn: &arn},
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

func TestApptestExplorer_DetailViewActionSpace_AllInvariantsHold(t *testing.T) {
	const id = "sfn-explorer-0000001"

	var lastCtrl *app.Controller
	var lastLedger *awsclient.CallLedger
	var lastFake *coalesceSfnFake

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
			lastFake = fake
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
				// Without a real RawStruct every enrich is an ErrDetailEnrichSkipped no-op
				// that never clears the latch. "open" is every replay's first action, so by
				// quiescence its enrich has attempted DescribeStateMachine.
				if lastFake.describeCalls.Load() == 0 {
					return fmt.Errorf("apptest: quiescent but the coalescing SFN fake's DescribeStateMachine never fired — sfnFixtureResource's RawStruct must carry a real StateMachineArn so enrichSfn genuinely completes")
				}
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

// Ctrl+R arms session.PendingDetailRefresh; toggling the related panel off and
// on before the refresh's enrichment folds begins a non-refresh successor
// operation for the same resource. That successor inherits SkipCache=true, and
// the latch clears only once a fresh enrichment fold lands.
func TestApptestScheduler_StickyRefreshSupersededByPanelToggle_LatchOnlyClearsOnGenuineFreshFold(t *testing.T) {
	const id = "sfn-sticky-0000001"
	ctx := context.Background()
	c, core := newDetailParityHeadlessController(t)
	fake := &coalesceSfnFake{}
	core.Session().Clients = &awsclient.ServiceClients{SFN: fake}
	sched := apptest.NewScheduler(ctx, c)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, openTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	sched.Submit(openTasks...)
	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the initial detail open: %v", err)
	}

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	sched.Submit(refreshTasks...)
	refreshOp := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if refreshOp == 0 {
		t.Fatal("precondition failed: refresh's enrich task TaskOpID is 0")
	}

	if err := completeTaskForOp(sched, runtime.KindRelatedCheck, refreshOp); err != nil {
		t.Fatalf("completing refresh's related-check: %v", err)
	}

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

	// Without a real RawStruct every enrich is an ErrDetailEnrichSkipped no-op, so
	// a zero call count means LatchCleared below checks nothing.
	if got := fake.describeCalls.Load(); got == 0 {
		t.Fatal("fake.describeCalls == 0 — DescribeStateMachine never fired; sfnFixtureResource's RawStruct must carry a real StateMachineArn so enrichSfn genuinely completes instead of skipping")
	}

	if err := apptest.LatchCleared(c, runtime.RelatedCacheKey(sfnInterleavingType, id)); err != nil {
		t.Error(err)
	}
}

// A related cache covering only some of sfn's defs must still dispatch
// KindRelatedCheck: BeginDetailOperation has invalidated whatever ran for the
// uncached defs, so their rows would stay Loading.
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
		{DefDisplayName: def0.DisplayName, Result: resource.KnownRelated(def0.TargetType, []string{"partial-cache-1", "partial-cache-2", "partial-cache-3"}, false)},
	})

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	sched.Submit(tasks...)

	if related := findTaskKind(tasks, runtime.KindRelatedCheck); related == nil {
		t.Fatalf("a PARTIAL related cache (1/%d defs) suppressed KindRelatedCheck; want still dispatched so the uncached defs are not stranded in Loading. tasks: %v", len(defs), taskKindsOf(tasks))
	}

	if err := apptest.NoOrphanLoadingRelated(c.Snapshot(), sched.Pending()); err != nil {
		t.Error(err)
	}

	if err := sched.DrainRemaining(); err != nil {
		t.Fatalf("draining the related-check fan-out: %v", err)
	}

	if err := apptest.NoOrphanLoadingRelated(c.Snapshot(), sched.Pending()); err != nil {
		t.Error(err)
	}
}

func TestApptestScheduler_StaleEnrichResult_LandingAfterNewer_IsRejectedNotAccepted(t *testing.T) {
	const id = "sfn-stale-0000001"
	ctx := context.Background()
	c, _ := newDetailParityHeadlessController(t)
	sched := apptest.NewScheduler(ctx, c)
	rec, restore := apptest.StartTraceRecorder()
	defer restore()

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: sfnInterleavingType})
	c.ApplyResourcesLoaded(sfnInterleavingType, []resource.Resource{sfnFixtureResource(id)}, nil, false)
	_, openTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	sched.Submit(openTasks...)
	op1 := runtime.TaskOpID(findTaskKind(openTasks, runtime.KindEnrichDetail).Payload)
	if op1 == 0 {
		t.Fatal("precondition failed: op1's enrich task TaskOpID is 0")
	}

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	sched.Submit(refreshTasks...)
	op2 := runtime.TaskOpID(findTaskKind(refreshTasks, runtime.KindEnrichDetail).Payload)
	if op2 == 0 || op2 == op1 {
		t.Fatalf("precondition failed: op2 = %d, want a fresh non-zero op distinct from op1 = %d", op2, op1)
	}

	if err := completeTaskForOp(sched, runtime.KindEnrichDetail, op2); err != nil {
		t.Fatalf("completing op2's enrich: %v", err)
	}
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
	// The "ami" placeholder's own fetch stays unresolved: that outstanding window
	// is what autoOpenSingleDetail's unrelated-type guard covers.

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

	// A real ec2 ResourcesLoaded cascades into its TaskKindProbeEnrich probe;
	// draining that to quiescence must leave the placeholder untouched too.
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
