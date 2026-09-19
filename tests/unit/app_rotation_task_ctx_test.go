// SPDX-License-Identifier: GPL-3.0-or-later

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

// stubAvailFetcher returns immediately with an empty result, so no probe in
// the window reaches a real fetcher against a zero-value
// *awsclient.ServiceClients.
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

// dispatchProbeWindow sends ClientsReady then AvailabilityCacheLoaded and
// returns the model plus the 4-cmd probe batch. ClientsReady's own cmd is
// discarded unexecuted, so no identity or disk-cache work runs. gen must match
// the session's ConnectGen (session.New seeds 1, each Rotate adds 1) or
// HandleClientsReady drops the event as stale.
func dispatchProbeWindow(t *testing.T, m tui.Model, region string, gen domain.Gen) (tui.Model, tea.Cmd) {
	t.Helper()
	m, _ = rootApplyMsg(m, messages.ClientsReady{
		Clients: &awsclient.ServiceClients{},
		Gen:     gen,
		Region:  region,
	})
	return rootApplyMsg(m, messages.AvailabilityCacheLoaded{})
}

func TestExecuteTaskCmd_ProbeAvailabilityCtx_CancelledByProfileSwitch(t *testing.T) {
	capture := newCtxCaptureAvailFetcher()
	registerProbeWindow(t, capture.fetch)

	// WithNoCache(true) only changes ClientsReady's own task list, which is
	// discarded unexecuted; session.Clients is installed either way.
	m := newBlessedModel(t, "profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	_, batchCmd := dispatchProbeWindow(t, m, "us-east-1", 1)

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

	_, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-b"})

	select {
	case <-probeCtx.Done():
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

func TestExecuteTaskCmd_ProbeAvailabilityCtx_LiveAfterProfileSwitch(t *testing.T) {
	capture := newCtxCaptureAvailFetcher()
	registerProbeWindow(t, capture.fetch)

	m := newBlessedModel(t, "profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Session.Rotate() bumps ConnectGen from 1 to 2, so the follow-up ClientsReady
	// must stamp Gen:2 or HandleClientsReady drops it as stale.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "profile-b"})
	_, batchCmd := dispatchProbeWindow(t, m, "us-west-2", 2)

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
