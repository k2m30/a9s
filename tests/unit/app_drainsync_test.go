// TaskKindFetchIdentity against nil AWS clients returns messages.IdentityError
// synchronously, so these tests need no credentials or network.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
)

func TestDrainSync_RealExecutableTask_TerminatesWithoutHanging(t *testing.T) {
	c := newTestController(t)

	_, tasks := c.Apply(app.Action{Kind: app.ActionOpenIdentity})
	if len(tasks) == 0 {
		t.Skip("Apply(OpenIdentity) returned no tasks — test depends on TaskKindFetchIdentity wiring")
	}

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

// ExecuteTask returns ErrAdapterOnlyTask for adapter-only kinds
// (TaskKindFlashTick, TaskKindEmitNavigate); DrainSync skips them and keeps
// draining.
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
}

// With an already-cancelled context each task either errors immediately
// (context-aware AWS calls) or completes synchronously (in-memory demo path),
// so the loop exits at or before the maxDrainIterations cap.
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

// TaskKindConnect with no pre-supplied clients returns a ClientsReady{Err: ...}
// event without a network wait. ClientsReady is handled via BootstrapLive, not
// Controller.Handle, so the post-drain check is structural.
func TestDrainSync_SelectProfile_ConnectTask_TerminatesWithoutHanging(t *testing.T) {
	c := newTestController(t)

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

	snap := c.Snapshot()
	if snap.Body.Kind == "" {
		t.Error("Snapshot().Body.Kind is empty after DrainSync(Connect) — controller state is invalid")
	}
}

// Every task Controller.Apply returns carries a Snap whose Clients pointer is
// the session.Clients at the moment Apply returned it, and that Snap stays
// fixed after a later Clients swap: a pending task dispatched under one
// profile/region must not execute against another session's transport.
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

// A task dispatched while session.Clients was nil and drained after real
// clients were installed executes against the nil clients it was stamped with.
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
