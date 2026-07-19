// SPDX-License-Identifier: GPL-3.0-or-later

// app_rotation_task_ctx_test.go — RED pin for the code-review finding that a
// profile/region switch does not abort in-flight background AWS work.
//
// internal/tui/app_dispatch.go:251 (executeTaskCmd) does `ctx := m.appCtx`
// and forwards it to every Core.ExecuteTaskAt call dispatched through
// tasksToCmd — including TaskKindProbeAvailability/TaskKindProbeEnrich, the
// background probe lanes. m.appCtx (internal/tui/app.go:116) is a single
// context.WithCancel(Background) created once in tui.New and cancelled ONLY
// by tea.QuitMsg or Model.Cancel() (see app_cancellation_test.go's
// TestModel_QuitCancelsAppContext / TestModel_Cancel_CancelsAppContext,
// which already pin quit-time cancellation — not duplicated here).
// session.Rotate() (core/session/session.go:721, invoked by
// Core.HandleProfileSelected/HandleRegionSelected) never touches appCtx at
// all, so an already-dispatched probe survives a profile/region switch and
// keeps burning its own AWS budget (ProbeResourceAvailability wraps ctx in
// its own 10s context.WithTimeout, core/runtime/probes.go:667) until IT
// decides to stop, not until the switch does. This is distinct from the 30s
// fetchTimeout in core/runtime/fetchers.go: that bounds the interactive
// FetchResources/FetchIdentity/etc. lanes, not the background task lanes
// this file exercises.
//
// Harness design (see the score-mode feasibility note this test resolves):
// neither of the two files the original finding cited as an "established
// tea.Cmd-execution harness" actually run through the TUI's
// executeTaskCmd/tasksToCmd path — runtime_fetch_ctx_deadline_test.go and
// runtime_enrich_dispatch_window_test.go both drive core.Core methods
// directly, bypassing internal/tui.Model entirely (confirmed by reading
// both files). The real harness assembled here instead:
//
//  1. Registers resource.SetAvailabilityFetcherForTest fakes for
//     resource.AllShortNames()[0:4] — the exact window
//     Core.fireNextAvailabilityProbes(4) pops (core/runtime/handlers_availability.go:184/211/379)
//     — one fake blocks on ctx and captures it, the other three return
//     immediately. This avoids ever invoking a real production fetcher
//     against the zero-value *awsclient.ServiceClients this test supplies
//     (a real fetcher calling a nil AWS SDK sub-client would panic).
//  2. Drives messages.ClientsReady BEFORE messages.AvailabilityCacheLoaded
//     (the reverse of qa_sweep_once_per_session_test.go's
//     fireAvailabilitySweep helper, which deliberately keeps Clients nil).
//     Sending ClientsReady first sets session.Clients as a side effect of
//     Update() — its own returned cmd (FetchIdentity + LoadAvailCache) is
//     discarded unexecuted, since real identity/disk-cache work is not
//     needed and would touch real credentials/files. Sending
//     AvailabilityCacheLoaded second then produces a clean batch containing
//     ONLY the 4 TaskKindProbeAvailability cmds (handleAvailabilityCacheLoaded
//     has no other task source once Clients is already set and Command is
//     empty).
//  3. tasksToCmd routes every TaskKindProbeAvailability through
//     executeTaskCmd (app_dispatch.go:196-199) and coreUpdate wraps the
//     result in nested tea.Batch — runBatchConcurrently below walks the
//     resulting tea.BatchMsg tree and runs every leaf cmd on its own
//     goroutine, the concurrent counterpart to
//     qa_enrichment_rerun_overlap_test.go's msgIsReenrichOrRefetch (that
//     helper walks sequentially and would hang on this file's deliberately
//     blocking probe).
//  4. Rotation is driven through messages.ProfileSelected — the same
//     public message TestBug193_* (qa_correctness_bugs_test.go) uses for
//     the established profile-switch flow.
package unit

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// stubAvailFetcher is a resource.AvailabilityFetcher that returns
// immediately with an empty result — used for every dispatch-window member
// this file does not care about, so none of them touch a real production
// fetcher against a zero-value *awsclient.ServiceClients.
func stubAvailFetcher(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
	return resource.FetchResult{}, nil
}

// ctxCaptureAvailFetcher is a resource.AvailabilityFetcher that records the
// ctx it is invoked with, signals started, then blocks until ctx is done or
// a bounded 500ms fallback elapses — never an unbounded block, so a bug that
// keeps ctx alive forever cannot hang the test suite.
type ctxCaptureAvailFetcher struct {
	mu       sync.Mutex
	captured context.Context
	started  chan struct{}
	once     sync.Once
}

func newCtxCaptureAvailFetcher() *ctxCaptureAvailFetcher {
	return &ctxCaptureAvailFetcher{started: make(chan struct{})}
}

func (f *ctxCaptureAvailFetcher) fetch(ctx context.Context, _ any, _ string) (resource.FetchResult, error) {
	f.mu.Lock()
	f.captured = ctx
	f.mu.Unlock()
	f.once.Do(func() { close(f.started) })
	select {
	case <-ctx.Done():
	case <-time.After(500 * time.Millisecond):
	}
	return resource.FetchResult{}, nil
}

func (f *ctxCaptureAvailFetcher) ctx() context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.captured
}

// runBatchConcurrently executes cmd and, whenever it resolves to a
// tea.BatchMsg, recurses into every sub-cmd on its own goroutine — so a
// deliberately blocking sub-cmd in the batch (this file's probe fake) never
// stalls its siblings, mirroring how the real Bubble Tea runtime schedules a
// tea.Batch's members concurrently rather than one after another.
func runBatchConcurrently(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return
	}
	var wg sync.WaitGroup
	for _, sub := range batch {
		if sub == nil {
			continue
		}
		wg.Add(1)
		go func(c tea.Cmd) {
			defer wg.Done()
			runBatchConcurrently(c)
		}(sub)
	}
	wg.Wait()
}

// startBatch runs cmd via runBatchConcurrently on its own goroutine (so the
// caller can observe the target fetcher's started signal without waiting
// out every member's full runtime) and returns a wait func the caller MUST
// defer. Every spawned sub-goroutine reads the shared package-level fetcher
// registries (resource.SetAvailabilityFetcherForTest); without this wait a
// leftover goroutine can still be reading them when t.Cleanup deletes the
// entries at test end, or when the next test registers its own — a data
// race the -race detector catches.
func startBatch(cmd tea.Cmd) (wait func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		runBatchConcurrently(cmd)
	}()
	return func() { <-done }
}

// registerProbeWindow overrides the availability fetcher for the first 4
// names in resource.AllShortNames() — the exact window
// Core.fireNextAvailabilityProbes(4) pops on a fresh, never-swept pair.
// names[0] gets fake, every other member gets stubAvailFetcher. Registers
// t.Cleanup for every override.
func registerProbeWindow(t *testing.T, fake resource.AvailabilityFetcher) (target string) {
	t.Helper()
	names := resource.AllShortNames()
	if len(names) < 4 {
		t.Fatalf("resource.AllShortNames() returned %d types, need >= 4 to exercise fireNextAvailabilityProbes' dispatch window", len(names))
	}
	target = names[0]
	resource.SetAvailabilityFetcherForTest(target, fake)
	t.Cleanup(func() { resource.CleanupAvailabilityFetcherForTest(target) })
	for _, other := range names[1:4] {
		resource.SetAvailabilityFetcherForTest(other, stubAvailFetcher)
		t.Cleanup(func() { resource.CleanupAvailabilityFetcherForTest(other) })
	}
	return target
}

// dispatchProbeWindow drives the ClientsReady-then-AvailabilityCacheLoaded
// sequence documented in this file's package comment and returns the
// resulting model plus the clean 4-cmd probe batch. gen must match the
// session's current ConnectGen (0 on a fresh model, bumped by 1 per
// ProfileSelected/RegionSelected via Session.Rotate) or
// Core.HandleClientsReady's staleness guard silently drops the event.
func dispatchProbeWindow(t *testing.T, m tui.Model, region string, gen domain.Gen) (tui.Model, tea.Cmd) {
	t.Helper()
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: &awsclient.ServiceClients{},
		Gen:     gen,
		Region:  region,
	})
	return rootApplyMsg(m, messages.AvailabilityCacheLoaded{})
}

// TestExecuteTaskCmd_ProbeAvailabilityCtx_CancelledByProfileSwitch pins the
// upcoming fix: a background TaskKindProbeAvailability task already in
// flight when the user switches profile must have its ctx cancelled by the
// switch, not merely by its own probe-local timeout or by quit.
//
// RED today: session.Rotate() (invoked by Core.HandleProfileSelected) never
// touches m.appCtx, and executeTaskCmd forwards m.appCtx unchanged to every
// background task — so the captured ctx stays alive well past the switch.
func TestExecuteTaskCmd_ProbeAvailabilityCtx_CancelledByProfileSwitch(t *testing.T) {
	capture := newCtxCaptureAvailFetcher()
	registerProbeWindow(t, capture.fetch)

	// WithNoCache(true) is the blessed direct-construction escape hatch
	// (qa_controller_construction_discipline_test.go) — it only changes
	// ClientsReady's OWN task list (DemoPrefetchCounts instead of
	// FetchIdentity+LoadAvailCache), which this test discards unexecuted
	// either way; session.Clients still gets installed unconditionally
	// before that branch, and handleAvailabilityCacheLoaded never reads
	// NoCache at all, so the probe-dispatch flow below is unaffected.
	m := tui.New("profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	_, batchCmd := dispatchProbeWindow(t, m, "us-east-1", 0)

	wait := startBatch(batchCmd)
	defer wait()

	select {
	case <-capture.started:
	case <-time.After(2 * time.Second):
		t.Fatal("target probe never started — resource.AllShortNames()[0] was not dispatched by fireNextAvailabilityProbes(4)")
	}

	probeCtx := capture.ctx()
	if probeCtx == nil {
		t.Fatal("probe started but captured a nil ctx")
	}

	// The established profile-switch flow (TestBug193_* in
	// qa_correctness_bugs_test.go), fired while the probe above is still
	// in flight.
	_, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-b"})

	select {
	case <-probeCtx.Done():
		// Fixed: the switch cancelled this task's ctx.
	case <-time.After(150 * time.Millisecond):
		t.Errorf(
			"probe ctx not Done() within 150ms of ProfileSelected — a " +
				"profile/region switch does not cancel an already-dispatched " +
				"background task's context (internal/tui/app_dispatch.go:251 " +
				"forwards the single, quit-only-cancelled m.appCtx to every " +
				"executeTaskCmd/ExecuteTaskAt call, and session.Rotate() never " +
				"touches it), so the probe keeps burning its AWS budget after " +
				"the switch instead of aborting",
		)
	}
}

// TestExecuteTaskCmd_ProbeAvailabilityCtx_LiveAfterProfileSwitch pins the
// forward-looking half of the fix's contract: a background task dispatched
// for the NEW pair, after a profile switch has completed, must receive a
// live (not pre-cancelled) context — the fix must re-arm per pair, not just
// cancel-and-never-renew. GREEN today (nothing cancels m.appCtx short of
// quit) and must stay GREEN once the fix lands.
func TestExecuteTaskCmd_ProbeAvailabilityCtx_LiveAfterProfileSwitch(t *testing.T) {
	capture := newCtxCaptureAvailFetcher()
	registerProbeWindow(t, capture.fetch)

	m := tui.New("profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Rotate to a new pair before ever dispatching a probe for the first —
	// mirrors a switch made from the main menu before its sweep started.
	// Session.Rotate() (invoked by HandleProfileSelected) bumps ConnectGen
	// from 0 to 1, so the follow-up ClientsReady below must stamp Gen:1 or
	// HandleClientsReady's staleness guard silently drops it.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-b"})
	_, batchCmd := dispatchProbeWindow(t, m, "us-west-2", 1)

	wait := startBatch(batchCmd)
	defer wait()

	select {
	case <-capture.started:
	case <-time.After(2 * time.Second):
		t.Fatal("target probe never started after the switch — resource.AllShortNames()[0] was not dispatched by fireNextAvailabilityProbes(4)")
	}

	probeCtx := capture.ctx()
	if probeCtx == nil {
		t.Fatal("probe started but captured a nil ctx")
	}
	if err := probeCtx.Err(); err != nil {
		t.Errorf("probe dispatched for the NEW pair after a profile switch received an already-%v ctx — the new pair's background tasks must start live, not pre-cancelled by the previous switch", err)
	}
}
