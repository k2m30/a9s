// Core.CaptureDispatch's
// DispatchSnapshot.EnrichmentTypeGen is a copy, not the session's map.
//
// A dispatch-time snapshot exists to be immune to whatever the session does
// after capture. A Go map assignment copies the map header, not its contents,
// so a snapshot holding the session's map would see every later
// EnrichmentTypeGen bump.
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
// captured snapshot's value did not move.
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
// CaptureDispatch+ExecuteTaskAt (the reader of snap.EnrichmentTypeGen) on one
// goroutine while BumpEnrichmentTypeGen (the writer) runs on another.
// `go test -race` is the detector; without -race Go's map-misuse detection is
// best-effort.
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
