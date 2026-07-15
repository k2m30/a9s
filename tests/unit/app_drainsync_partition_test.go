// app_drainsync_partition_test.go — RED-phase pin for app.DrainSyncPartition.
//
// TDD RED: app.DrainSyncPartition does not exist yet. This file is
// compile-red until the web detail-latency fix adds it to internal/app.
//
// Contract (per the fix task spec):
//
//	func DrainSyncPartition(
//	    ctx context.Context,
//	    c *Controller,
//	    pending []runtime.TaskRequest,
//	    isBackground func(runtime.TaskKind) bool,
//	    onEvent func(),
//	) []runtime.TaskRequest
//
// Behaves like DrainSyncContextProgress but partitions the pending queue by
// isBackground: tasks classified background are collected (never executed)
// and returned to the caller instead of being run inline — INCLUDING
// follow-up tasks emitted by tasks that WERE executed. Tasks classified
// blocking (isBackground returns false) run exactly as DrainSyncContextProgress
// would: executed via Core.ExecuteTask, results fed through Controller.Handle,
// any follow-ups re-enqueued (subject to the same background/blocking split).
//
// This lets a web request handler drain only the tasks whose result the HTTP
// response needs (blocking) and hand background tasks (related-check fan-out,
// detail enrichment, save-cache) to a goroutine that completes after the
// response has already been written — fixing the 18.5s synchronous
// related-check fan-out latency this fix task targets.
//
// Harness notes:
//   - Mirrors the fake-task seams in app_drainsync_test.go: real executable
//     TaskKindFetchIdentity (nil AWS clients -> deterministic IdentityError,
//     no network) and adapter-only kinds (TaskKindFlashTick, TaskKindEmitNavigate)
//     that ExecuteTask rejects with ErrAdapterOnlyTask.
//   - The "blocking task whose follow-up is background" case drives the real
//     handleAvailabilityChecked chain: seeding session.AvailQueue with exactly
//     one more resource type makes TaskKindProbeAvailability's execution
//     deterministically emit a TaskKindSaveCache follow-up once the queue and
//     AvailChecked/AvailTotal counters drain to equal (see
//     internal/runtime/handlers_availability.go:216-239). ProbeResourceAvailability
//     tolerates nil session.Clients (returns an Err-populated result, no
//     network, no panic), so this is fully hermetic like the rest of the
//     DrainSync suite.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// alwaysBackground classifies every kind as background — used to assert the
// "empty/all-blocking" DrainSyncContextProgress-equivalence case via its
// logical inverse (never used directly; see alwaysBlocking below).
func alwaysBlocking(runtime.TaskKind) bool { return false }

// TestDrainSyncPartition_MixedBatch_BlockingExecuted_BackgroundReturned
// verifies (a): a batch mixing a real blocking task (TaskKindFetchIdentity)
// with background-classified adapter-only tasks (TaskKindFlashTick,
// TaskKindEmitNavigate) executes the blocking task and returns the
// background tasks unexecuted, in the original relative order.
func TestDrainSyncPartition_MixedBatch_BlockingExecuted_BackgroundReturned(t *testing.T) {
	c := newTestController(t)

	// ActionOpenIdentity pushes ScreenIdentity so Snapshot().Body.Identity is
	// populated once FetchIdentity completes (snapshot() only fills
	// Body.Identity when the top screen is ScreenIdentity).
	c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	flashTick := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFlashTick}}
	emitNavigate := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindEmitNavigate}}
	fetchIdentity := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity}}

	pending := []runtime.TaskRequest{flashTick, fetchIdentity, emitNavigate}

	isBackground := func(k runtime.TaskKind) bool {
		return k == runtime.TaskKindFlashTick || k == runtime.TaskKindEmitNavigate
	}

	deferred := app.DrainSyncPartition(context.Background(), c, pending, isBackground, nil)

	if len(deferred) != 2 {
		t.Fatalf("DrainSyncPartition returned %d deferred tasks, want 2 (flash-tick, emit-navigate); got %v",
			len(deferred), taskKindStrings(deferred))
	}
	if deferred[0].Key.Kind != runtime.TaskKindFlashTick {
		t.Errorf("deferred[0].Kind = %q, want %q", deferred[0].Key.Kind, runtime.TaskKindFlashTick)
	}
	if deferred[1].Key.Kind != runtime.TaskKindEmitNavigate {
		t.Errorf("deferred[1].Kind = %q, want %q", deferred[1].Key.Kind, runtime.TaskKindEmitNavigate)
	}

	// The blocking task (FetchIdentity) must have actually executed: Handle
	// routes nil-client IdentityError to identityErrMsg, observable via Snapshot.
	snap := c.Snapshot()
	if snap.Body.Identity == nil {
		t.Fatal("Snapshot().Body.Identity is nil after DrainSyncPartition — FetchIdentity (blocking) was not executed")
	}
	if snap.Body.Identity.ErrorMsg == "" {
		t.Error("Snapshot().Body.Identity.ErrorMsg is empty — FetchIdentity (blocking) did not run to completion")
	}
}

// TestDrainSyncPartition_BlockingFollowUp_IsBackground_ReturnedNotExecuted
// verifies (b): a blocking task whose execution emits a background-classified
// follow-up task returns that follow-up unexecuted, rather than draining it
// inline. Drives the real handleAvailabilityChecked chain: session.AvailQueue
// is empty and AvailChecked+1 == AvailTotal, so executing the seed
// TaskKindProbeAvailability deterministically appends a single
// TaskKindSaveCache follow-up (queue-exhausted branch).
func TestDrainSyncPartition_BlockingFollowUp_IsBackground_ReturnedNotExecuted(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	// Queue already drained to the last in-flight probe: executing the seed
	// task below satisfies AvailChecked == AvailTotal, taking the
	// queue-exhausted branch that appends TaskKindSaveCache.
	s.AvailQueue = nil
	s.AvailChecked = 0
	s.AvailTotal = 1

	core := runtime.New(s, nil)
	c := app.New(core)

	seed := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindProbeAvailability, Scope: "ec2"}}

	isBackground := func(k runtime.TaskKind) bool {
		return k == runtime.TaskKindSaveCache
	}

	deferred := app.DrainSyncPartition(context.Background(), c, []runtime.TaskRequest{seed}, isBackground, nil)

	if len(deferred) != 1 {
		t.Fatalf("DrainSyncPartition returned %d deferred tasks, want 1 (save-cache follow-up); got %v",
			len(deferred), taskKindStrings(deferred))
	}
	if deferred[0].Key.Kind != runtime.TaskKindSaveCache {
		t.Errorf("deferred[0].Kind = %q, want %q — the follow-up emitted by the queue-exhausted branch of "+
			"handleAvailabilityChecked must be classified background and returned, not executed",
			deferred[0].Key.Kind, runtime.TaskKindSaveCache)
	}
}

// TestDrainSyncPartition_AllBlocking_BehavesLikeDrainSyncContextProgress
// verifies (c): when isBackground never matches, DrainSyncPartition drains
// the entire batch exactly like DrainSyncContextProgress (nothing deferred)
// and produces the same observable end state.
func TestDrainSyncPartition_AllBlocking_BehavesLikeDrainSyncContextProgress(t *testing.T) {
	c := newTestController(t)

	// ActionOpenIdentity pushes ScreenIdentity so Snapshot().Body.Identity is
	// populated once FetchIdentity completes (snapshot() only fills
	// Body.Identity when the top screen is ScreenIdentity).
	c.Apply(app.Action{Kind: app.ActionOpenIdentity})

	fetchIdentity := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity}}

	deferred := app.DrainSyncPartition(context.Background(), c, []runtime.TaskRequest{fetchIdentity}, alwaysBlocking, nil)

	if len(deferred) != 0 {
		t.Fatalf("DrainSyncPartition with alwaysBlocking classifier returned %d deferred tasks, want 0; got %v",
			len(deferred), taskKindStrings(deferred))
	}

	snap := c.Snapshot()
	if snap.Body.Identity == nil || snap.Body.Identity.ErrorMsg == "" {
		t.Error("DrainSyncPartition with alwaysBlocking classifier did not fully drain FetchIdentity — " +
			"expected identical behavior to DrainSyncContextProgress")
	}
}

// TestDrainSyncPartition_EmptyPending_ReturnsNil verifies (c)'s empty-queue
// edge: an empty pending slice returns nil (or empty) deferred tasks without
// touching controller state, matching DrainSyncContextProgress's immediate
// return on an empty queue.
func TestDrainSyncPartition_EmptyPending_ReturnsNil(t *testing.T) {
	c := newTestController(t)

	deferred := app.DrainSyncPartition(context.Background(), c, nil, alwaysBlocking, nil)

	if len(deferred) != 0 {
		t.Errorf("DrainSyncPartition(nil pending) returned %d deferred tasks, want 0", len(deferred))
	}
}

// TestDrainSyncPartition_OnEventCalledOnlyForExecutedTasks verifies that
// onEvent fires once per executed (blocking) task result, mirroring
// DrainSyncProgress semantics, and is NOT invoked for tasks that were
// deferred as background without executing.
func TestDrainSyncPartition_OnEventCalledOnlyForExecutedTasks(t *testing.T) {
	c := newTestController(t)

	flashTick := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFlashTick}}
	fetchIdentity := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity}}

	isBackground := func(k runtime.TaskKind) bool { return k == runtime.TaskKindFlashTick }

	eventCount := 0
	deferred := app.DrainSyncPartition(context.Background(), c, []runtime.TaskRequest{flashTick, fetchIdentity}, isBackground, func() {
		eventCount++
	})

	if len(deferred) != 1 || deferred[0].Key.Kind != runtime.TaskKindFlashTick {
		t.Fatalf("expected exactly 1 deferred flash-tick task, got %v", taskKindStrings(deferred))
	}
	if eventCount != 1 {
		t.Errorf("onEvent invoked %d times, want 1 (once for the executed FetchIdentity result, "+
			"not for the deferred flash-tick task)", eventCount)
	}
}
