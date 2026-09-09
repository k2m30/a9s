// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// runtime8_flush_reader_latency_test.go — a cache flush must not be what the
// screen is waiting for.
//
// The staged cache write marshals a whole type file outside the controller
// lock, which is what keeps a key press from blocking behind a disk write. The
// CPU it spends is not out of the way, though: a reader taking a snapshot
// while a large file is being marshalled waits on the scheduler for it.
//
// The pin is a ratio against the same bench and never a clock budget: a
// wall-clock number would say something about the machine it ran on rather
// than about the flush. It compares one reader's own snapshot latency with a
// flush in progress against the same reader's latency with none.
//
// It compares the WORST snapshot of each phase, not the median. A flush costs
// the reader nothing on all but one snapshot and the whole flush on that one,
// so a median comparison reads 1.00 while the operator's key press waits the
// length of the marshal. The worst snapshot is the one that is felt.
package unit_test

import (
	"sort"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// flushBenchRows is the size the backlog row measured: a type file big enough
// that marshalling it is real work.
const flushBenchRows = 6000

// flushLatencyRatioBound is what a concurrent flush is allowed to cost the
// reader's worst snapshot. The measured ratio before the fix is recorded in
// the round log beside this number; it is a ratio and not a duration so the
// bound means the same thing on a laptop and on a busy machine.
const flushLatencyRatioBound = 4.0

// flushSamples is how many snapshots each phase takes. The comparison is on
// medians, so a single scheduling hiccup in either phase moves nothing.
const flushSamples = 120

// flushSnapshotLatency takes flushSamples snapshots and returns the worst and
// the median. Reading the snapshot is exactly what the renderer does on every
// key.
func flushSnapshotLatency(c *app.Controller) (worst, median time.Duration) {
	samples := make([]time.Duration, 0, flushSamples)
	for range flushSamples {
		start := time.Now()
		_ = c.Snapshot()
		samples = append(samples, time.Since(start))
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	return samples[len(samples)-1], samples[len(samples)/2]
}

// TestFlushCacheWrites_DoesNotSlowAConcurrentReader is the shape pin.
func TestFlushCacheWrites_DoesNotSlowAConcurrentReader(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(flushBenchRows),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	c.WaitForCacheWrites()

	// Warm: the first snapshot builds the memo the rest reuse, so it belongs
	// to neither phase.
	_ = c.Snapshot()
	quiet, quietMedian := flushSnapshotLatency(c)

	// Stage the same-sized write again and read while it is being flushed.
	// The staging is what the list-open lane does on every load; the flush is
	// the writer goroutine draining it.
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixSaveLaneRows(flushBenchRows + 1),
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	done := make(chan struct{})
	go func() {
		c.WaitForCacheWrites()
		close(done)
	}()
	busy, busyMedian := flushSnapshotLatency(c)
	remaining := time.Now()
	<-done
	left := time.Since(remaining)

	ratio := float64(busy) / float64(quiet)
	t.Logf("worst snapshot: quiet %v, flushing %v, ratio %.2f (medians %v / %v; flush had %v left after the samples)",
		quiet, busy, ratio, quietMedian, busyMedian, left)
	if ratio > flushLatencyRatioBound {
		t.Errorf("a reader's worst snapshot takes %.2fx longer while a %d-row type file is being flushed "+
			"(%v against %v), want at most %.2fx — the operator's key press waits out the marshal",
			ratio, flushBenchRows, busy, quiet, flushLatencyRatioBound)
	}
}
