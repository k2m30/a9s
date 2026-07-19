// runtime_dispatchsnapshot_isolation_test.go — pins the upcoming
// maps.Clone fix for Core.CaptureDispatch's DispatchSnapshot.EnrichmentTypeGen
// aliasing bug.
//
// DispatchSnapshot is documented (executor.go:46-47) as capturing "the
// session state ExecuteTask reads, taken at DISPATCH time (synchronously,
// before the async command goroutine runs)" — the whole point of a
// dispatch-time snapshot is immunity to whatever the session does AFTER
// capture. CaptureDispatch (executor.go:61-72) currently assigns
// c.session.EnrichmentTypeGen straight into the snapshot's
// EnrichmentTypeGen field, which is a map — a Go map assignment copies the
// map HEADER, not its contents, so the snapshot and the live session share
// the exact same underlying map. Any later write to that map — the inline
// EnrichmentTypeGen[name]++ bumps in handlers_availability.go (startEnrichment,
// handleEnrichmentChecked) or the BumpEnrichmentTypeGen accessor
// (accessors.go:195) — is visible through an already-captured snapshot,
// defeating the whole purpose of capturing one.
package unit

import (
	"context"
	"sync"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// TestCaptureDispatch_EnrichmentTypeGen_SnapshotIsolatedFromLaterBumps is the
// deterministic, single-goroutine pin: capture a snapshot, bump the live
// session's per-type generation for the SAME type afterward, and assert the
// already-captured snapshot's value did not move. This must fail today
// because CaptureDispatch aliases the map instead of cloning it.
func TestCaptureDispatch_EnrichmentTypeGen_SnapshotIsolatedFromLaterBumps(t *testing.T) {
	c := newExecutorCore(t)
	const shortName = "ec2"

	c.BumpEnrichmentTypeGen(shortName)
	snap := c.CaptureDispatch()
	if got := snap.EnrichmentTypeGen[shortName]; got != 1 {
		t.Fatalf("snap.EnrichmentTypeGen[%s] = %d immediately after capture, want 1", shortName, got)
	}

	// A dispatch-time snapshot must be immune to a session mutation that
	// happens strictly AFTER it was captured.
	c.BumpEnrichmentTypeGen(shortName)

	if got := snap.EnrichmentTypeGen[shortName]; got != 1 {
		t.Errorf("snap.EnrichmentTypeGen[%s] = %d after a LATER BumpEnrichmentTypeGen call, want unchanged 1 — "+
			"CaptureDispatch must copy the map (e.g. maps.Clone) instead of aliasing session.EnrichmentTypeGen by reference",
			shortName, got)
	}
}

// TestCaptureDispatch_EnrichmentTypeGen_ConcurrentBumpDoesNotRace hammers
// CaptureDispatch+ExecuteTaskAt (the reader of snap.EnrichmentTypeGen, see
// executor.go:124's TaskKindProbeEnrich case) on one goroutine while
// BumpEnrichmentTypeGen (the writer) runs concurrently on another — a
// genuine concurrent map read/write across the aliased map once
// CaptureDispatch stops cloning. `go test -race` is the intended detector
// for this test; it may still pass without -race (Go's built-in map-misuse
// detection is best-effort, not guaranteed), which is acceptable — the
// deterministic isolation test above is the primary TDD failure signal.
func TestCaptureDispatch_EnrichmentTypeGen_ConcurrentBumpDoesNotRace(t *testing.T) {
	c := newExecutorCore(t)

	var enricherType string
	for _, td := range catalog.All() {
		if c.HasIssueEnricher(td.ShortName) {
			enricherType = td.ShortName
			break
		}
	}
	if enricherType == "" {
		t.Skip("no registered types have issue enrichers; cannot exercise the snap.EnrichmentTypeGen read path")
	}

	const iterations = 100
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		for i := 0; i < iterations; i++ {
			snap := c.CaptureDispatch()
			_, _ = c.ExecuteTaskAt(ctx, req(runtime.TaskKindProbeEnrich, enricherType), snap)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			c.BumpEnrichmentTypeGen(enricherType)
		}
	}()
	wg.Wait()
}
