// DrainSyncPartition behaves like DrainSyncContextProgress but partitions the
// pending queue by isBackground: background tasks, including follow-ups
// emitted by executed tasks, are collected unexecuted and returned; blocking
// tasks run via Core.ExecuteTask, their results fed through Controller.Handle
// and their follow-ups re-enqueued under the same split. A web request handler
// drains only the tasks its response needs and hands background tasks
// (related-check fan-out, detail enrichment, save-cache) to a goroutine that
// completes after the response is written.
//
// ProbeResourceAvailability tolerates nil session.Clients (it returns an
// Err-populated result), so the availability chain here needs no network.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

func alwaysBlocking(runtime.TaskKind) bool { return false }

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
	c := newBlessedController(t, core)

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

func TestDrainSyncPartition_EmptyPending_ReturnsNil(t *testing.T) {
	c := newTestController(t)

	deferred := app.DrainSyncPartition(context.Background(), c, nil, alwaysBlocking, nil)

	if len(deferred) != 0 {
		t.Errorf("DrainSyncPartition(nil pending) returned %d deferred tasks, want 0", len(deferred))
	}
}

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
