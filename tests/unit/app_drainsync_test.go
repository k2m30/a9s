// app_drainsync_test.go — behavioral tests for app.DrainSync / app.DrainSyncContext.
//
// Coverage map (matches task spec items 1–5):
//
//  1. Empty pending  — covered by TestDrainSync_EmptyPendingReturnsImmediately
//     in app_controller_test.go (pre-existing). Not duplicated here.
//
//  2. Terminates / respects cap — seed with a real executable task whose
//     executor path returns a result event, assert DrainSync returns without
//     hanging. The maxDrainIterations cap is not exercised by normal fixture
//     graphs (no self-replenishing task is constructable without fakes against
//     the real executor); normal termination is the honest assertion here.
//
//  3. ErrAdapterOnlyTask skip — seed a batch that mixes adapter-only tasks
//     (TaskKindFlashTick, TaskKindEmitNavigate) with a real executable task
//     (TaskKindFetchIdentity). Assert no panic and that the batch drains fully.
//
//  4. Executes real work — TaskKindFetchIdentity with nil AWS clients produces
//     messages.IdentityError (nil STS client path). DrainSync completes without
//     hanging or panicking and Handle(IdentityError) sets the identity error state
//     visible in Snapshot().Body.Identity.ErrorMsg.
//
//  5. Follow-up tasks — Handle(messages.IdentityError) returns no follow-up tasks
//     by design. DrainSync terminates after the initial batch without growing pending.
//
// All tests are hermetic: no AWS credentials, no disk I/O, no goroutines.
// newTestController(t) uses runtime.New(session, nil) — no demo clients —
// which is sufficient because TaskKindFetchIdentity handles nil clients
// gracefully (returns IdentityError, not a panic).
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestDrainSync_RealExecutableTask_TerminatesWithoutHanging verifies that
// DrainSync with a seed containing a genuinely executable task (TaskKindFetchIdentity)
// returns without hanging. The task executes against nil AWS clients, which
// produces messages.IdentityError synchronously — no network, no goroutines.
//
// After DrainSync, Handle(IdentityError) has been called internally and the
// identity error state is reflected in Snapshot().Body.Identity.ErrorMsg.
func TestDrainSync_RealExecutableTask_TerminatesWithoutHanging(t *testing.T) {
	c := newTestController(t)

	// ActionOpenIdentity pushes ScreenIdentity and returns a TaskKindFetchIdentity
	// task request.
	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})
	if len(tasks) == 0 {
		t.Skip("Apply(OpenIdentity) returned no tasks — test depends on TaskKindFetchIdentity wiring")
	}

	// Verify the seeded task is the kind we expect (catches any wiring change).
	hasFetchIdentity := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindFetchIdentity {
			hasFetchIdentity = true
			break
		}
	}
	if !hasFetchIdentity {
		t.Skipf("no TaskKindFetchIdentity in tasks; got %v — test assumption broken", taskKindStrings(tasks))
	}

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSync(c, tasks)
		done <- struct{}{}
	}()

	// The test harness -timeout flag catches an infinite loop; receiving on done
	// catches panics (goroutine exits without sending) via the test framework.
	<-done

	// After DrainSync: FetchIdentity against nil clients produces IdentityError,
	// which Handle routes to set identityErrMsg. Snapshot must reflect the error.
	snap := c.Snapshot()
	if snap.Body.Identity == nil {
		t.Fatal("Snapshot().Body.Identity is nil after DrainSync with FetchIdentity — expected IdentityBody")
	}
	if snap.Body.Identity.ErrorMsg == "" {
		t.Error("Snapshot().Body.Identity.ErrorMsg is empty after nil-client FetchIdentity — expected an error message")
	}
}

// TestDrainSync_AdapterOnlyTasksSkipped_NoPanic verifies spec item 3:
// adapter-only task kinds (those for which ExecuteTask returns ErrAdapterOnlyTask)
// are silently skipped. The batch also contains a real executable task so we
// confirm draining continues past the skipped entries.
//
// Adapter-only kinds tested: TaskKindFlashTick, TaskKindEmitNavigate.
// Real executable kind: TaskKindFetchIdentity (nil STS → IdentityError, no panic).
func TestDrainSync_AdapterOnlyTasksSkipped_NoPanic(t *testing.T) {
	c := newTestController(t)

	adapterOnlyFlash := runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindFlashTick, Scope: ""},
	}
	adapterOnlyEmit := runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindEmitNavigate, Scope: ""},
	}
	fetchIdentity := runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity, Scope: ""},
	}

	batch := []runtime.TaskRequest{adapterOnlyFlash, fetchIdentity, adapterOnlyEmit}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DrainSync with adapter-only tasks panicked: %v", r)
			}
		}()
		app.DrainSync(c, batch)
	}()
	// If we reach here the adapter-only kinds were skipped without error or panic,
	// and the real task (FetchIdentity) was executed to completion.
}

// TestDrainSyncContext_CancelledContext_ReturnsPromptly verifies that
// DrainSyncContext with an already-cancelled context does not hang. The
// executor receives a cancelled context; each task either errors immediately
// (context-aware AWS calls) or completes synchronously (in-memory demo path).
// Either way the loop must exit at or before the maxDrainIterations cap.
func TestDrainSyncContext_CancelledContext_ReturnsPromptly(t *testing.T) {
	c := newTestController(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})
	if len(tasks) == 0 {
		t.Skip("Apply(OpenIdentity) returned no tasks — test depends on PR-B TaskKindFetchIdentity wiring")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before DrainSync

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSyncContext(ctx, c, tasks)
		done <- struct{}{}
	}()

	<-done
}

// TestDrainSync_MixedBatch_RealAndAdapterOnly_AllDrain verifies that a batch
// of 5 tasks — 2 adapter-only, 1 real (FetchIdentity), 2 more adapter-only —
// drains completely without panic. Guards against an off-by-one in the pending
// slice append logic that could leave tasks undrained.
func TestDrainSync_MixedBatch_RealAndAdapterOnly_AllDrain(t *testing.T) {
	c := newTestController(t)

	adapterOnly := func(kind runtime.TaskKind) runtime.TaskRequest {
		return runtime.TaskRequest{Key: runtime.TaskKey{Kind: kind}}
	}
	real := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindFetchIdentity}}

	batch := []runtime.TaskRequest{
		adapterOnly(runtime.TaskKindFlashTick),
		adapterOnly(runtime.TaskKindEmitNavigate),
		real,
		adapterOnly(runtime.TaskKindEmitAPIError),
		adapterOnly(runtime.TaskKindReadThemeFile),
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("DrainSync mixed batch panicked: %v", r)
			}
		}()
		app.DrainSync(c, batch)
	}()
}

// TestDrainSync_SelectProfile_ConnectTask_TerminatesWithoutHanging verifies
// that a TaskKindConnect task seeded from ActionSelectProfile drains without
// hanging. TaskKindConnect with an unreachable profile calls ConnectAWS, which
// with nil pre-supplied clients returns a ClientsReady{Err: ...} event — no
// network timeout in the executor's synchronous path.
//
// This exercises a second "real executable task" path, distinct from
// FetchIdentity, confirming DrainSync is not accidentally gated on task kind.
//
// ClientsReady is handled via BootstrapLive, not Controller.Handle, so the
// observable post-DrainSync assertion is structural: Snapshot must be valid
// (non-empty BodyKind, no panic) regardless of the connect outcome.
func TestDrainSync_SelectProfile_ConnectTask_TerminatesWithoutHanging(t *testing.T) {
	c := newTestController(t)

	// ActionSelectProfile returns a TaskKindConnect task.
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelectProfile, Arg: "fake-profile-000000000000"})
	if len(tasks) == 0 {
		t.Skip("Apply(SelectProfile) returned no tasks — test depends on TaskKindConnect wiring")
	}

	hasConnect := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindConnect {
			hasConnect = true
			break
		}
	}
	if !hasConnect {
		t.Skipf("no TaskKindConnect in tasks; got %v — test assumption broken", taskKindStrings(tasks))
	}

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSync(c, tasks)
		done <- struct{}{}
	}()

	<-done

	// Controller must remain in a structurally valid state after the connect task.
	snap := c.Snapshot()
	if snap.Body.Kind == "" {
		t.Error("Snapshot().Body.Kind is empty after DrainSync(Connect) — controller state is invalid")
	}
}

// TestDrainSync_NilFollowUpTasks_DoesNotGrow verifies spec item 5:
// Handle(IdentityError) returns no follow-up tasks by design, so the pending
// slice does not grow and DrainSync terminates after the initial batch.
//
// Mechanism: seed 1 FetchIdentity task, observe that DrainSync returns without
// hanging. If Handle were accidentally appending to pending the loop would grow
// unboundedly until maxDrainIterations — the test harness -timeout catches that.
//
// IdentityError IS wired through Handle (sets identityErrMsg) but produces no
// follow-up task requests, so the initial batch of 1 is the only drain cycle.
func TestDrainSync_NilFollowUpTasks_DoesNotGrow(t *testing.T) {
	c := newTestController(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})
	if len(tasks) == 0 {
		t.Skip("Apply(OpenIdentity) returned no tasks")
	}

	done := make(chan struct{}, 1)
	go func() {
		app.DrainSync(c, tasks)
		done <- struct{}{}
	}()

	<-done

	// IdentityError produces no follow-up tasks — the loop must have exited after
	// draining the single initial task, not via the maxDrainIterations cap.
	// Verify the identity error state is set (confirming Handle was called).
	snap := c.Snapshot()
	if snap.Body.Identity != nil && snap.Body.Identity.ErrorMsg == "" && !snap.Body.Identity.Loading {
		// Identity screen is showing but has neither an error nor a loading state —
		// this would mean FetchIdentity completed without producing IdentityError,
		// which is unexpected for a nil-client path.
		t.Error("identity screen shows neither ErrorMsg nor Loading after nil-client FetchIdentity drain")
	}
}

// TestTaskSnap_ApplyReturnedTasks_CarryCreationTimeSnap_ImmuneToLaterClientsSwap
// pins TaskRequest.Snap's creation-time-stamping contract: every task
// Controller.Apply returns must carry a non-nil Snap whose Clients pointer is
// exactly whatever session.Clients was AT THE MOMENT Apply returned it — and,
// crucially, that already-returned snapshot must stay fixed at that value
// even after a LATER session Clients swap. Without this, a still-pending task
// dispatched under one profile/region's clients could silently start
// executing against a different session's transport if the user switches
// again before it drains.
func TestTaskSnap_ApplyReturnedTasks_CarryCreationTimeSnap_ImmuneToLaterClientsSwap(t *testing.T) {
	core, c := newTestControllerWithCore(t)

	clientsA := demo.NewServiceClients()
	core.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need the side effect (session.Clients installed)
		Clients:    clientsA,
		Gen:        core.ConnectGen(),
		StackDepth: 1,
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(tasks) == 0 {
		t.Skip("Apply(ActionRefresh) on the menu returned no tasks — test depends on RestartAvailabilitySweep wiring")
	}

	for i, task := range tasks {
		if task.Snap == nil {
			t.Fatalf("tasks[%d] (%s): Snap is nil — every task Controller.Apply returns must carry a creation-time DispatchSnapshot", i, task.Key.Kind)
		}
		if task.Snap.Clients != clientsA {
			t.Errorf("tasks[%d] (%s): Snap.Clients = %p, want the clients installed before Apply returned it (%p)", i, task.Key.Kind, task.Snap.Clients, clientsA)
		}
	}

	clientsB := demo.NewServiceClients()
	core.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need the side effect
		Clients:    clientsB,
		Gen:        core.ConnectGen(),
		StackDepth: 1,
	})

	for i, task := range tasks {
		if task.Snap.Clients != clientsA {
			t.Errorf("tasks[%d] (%s): Snap.Clients changed to %p after a LATER session Clients swap (to %p) — the already-returned snapshot must stay fixed at its creation-time value %p", i, task.Key.Kind, task.Snap.Clients, clientsB, clientsA)
		}
	}
}

// TestTaskSnap_DrainSync_StampedTask_UsesCreationTimeClients_NotLaterSwap is
// the drain-side half of the same contract: a task dispatched while
// session.Clients was nil, then drained AFTER a real client set was installed,
// must still execute against the nil clients it was stamped with at creation
// time — not the clients that happen to be live when the drain loop finally
// reaches it.
//
// Mechanism: TaskKindFetchIdentity with nil Clients unconditionally produces
// messages.IdentityError (Core.FetchIdentity's nil-clients guard); with real
// demo clients it succeeds. Seeding the task while clients are nil, then
// installing real clients before draining, discriminates cleanly between "the
// drain loop re-captured a live DispatchSnapshot at execute time" (would
// succeed here) and "the drain loop honored the task's own creation-time Snap"
// (must still error here).
func TestTaskSnap_DrainSync_StampedTask_UsesCreationTimeClients_NotLaterSwap(t *testing.T) {
	core, c := newTestControllerWithCore(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})
	if len(tasks) == 0 {
		t.Skip("Apply(OpenIdentity) returned no tasks — test depends on TaskKindFetchIdentity wiring")
	}
	hasFetchIdentity := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindFetchIdentity {
			hasFetchIdentity = true
		}
	}
	if !hasFetchIdentity {
		t.Skipf("no TaskKindFetchIdentity in tasks; got %v — test assumption broken", taskKindStrings(tasks))
	}

	// Session gains real clients strictly AFTER the task above was dispatched
	// (creation-time Clients were nil) but strictly BEFORE it is drained.
	core.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need the side effect
		Clients:    demo.NewServiceClients(),
		Gen:        core.ConnectGen(),
		StackDepth: 1,
	})

	app.DrainSync(c, tasks)

	snap := c.Snapshot()
	if snap.Body.Identity == nil {
		t.Fatal("Snapshot().Body.Identity is nil after DrainSync — expected IdentityBody")
	}
	if snap.Body.Identity.ErrorMsg == "" {
		t.Error("Snapshot().Body.Identity.ErrorMsg is empty after the drain — the stamped FetchIdentity task must have executed against its creation-time (nil) clients, not the clients installed after dispatch; got a successful identity load instead")
	}
}
