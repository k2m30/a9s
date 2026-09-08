// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_lock_scope_test.go pins two facts about what the controller does
// on a caller's goroutine:
//
//   - reading a warm screen is a read. Two readers of the same session must
//     overlap, not queue behind each other.
//   - persisting a fetch result is not the caller's work. The marshal of the
//     whole row set must not sit in the latency of the absorption that
//     triggered it, and moving it off must leave the same file on disk.
package unit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixWarmController returns a controller holding n absorbed rows, with the
// list body memo already built.
func wipfixWarmController(t *testing.T, n int) *app.Controller {
	t.Helper()
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixEC2Rows(n),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	c.Snapshot() // build the memo, so every snapshot below is a warm one
	return c
}

// wipfixSnapshotOverlapRatio returns the wall time of two goroutines each
// taking iters warm snapshots, divided by the wall time of one goroutine
// taking iters of them. Readers that overlap keep the ratio near 1; readers
// that serialise on a write lock double it.
func wipfixSnapshotOverlapRatio(t *testing.T, c *app.Controller, iters int) (ratio float64, serial, parallel time.Duration) {
	t.Helper()
	start := time.Now()
	for range iters {
		c.Snapshot()
	}
	serial = time.Since(start)

	start = time.Now()
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iters {
				c.Snapshot()
			}
		}()
	}
	wg.Wait()
	parallel = time.Since(start)
	return float64(parallel) / float64(serial), serial, parallel
}

// wipfixSnapshotOverlapCeiling is how much slower two concurrent readers may
// be than one. Measured on this bench with 6000 warm rows: 2.13-2.18, i.e.
// exactly serialised, because Snapshot takes the write lock. Two readers that
// genuinely overlap cost about what one costs; 1.5 is the midpoint, far from
// both the ~1.1 an overlapping pair reaches and the ~2.15 a queue does.
const wipfixSnapshotOverlapCeiling = 1.5

// TestWarmSnapshot_TwoReadersOverlap pins row 34: building a warm screen reads
// state it does not change, so two web requests against one session must not
// take turns.
func TestWarmSnapshot_TwoReadersOverlap(t *testing.T) {
	c := wipfixWarmController(t, 6000)
	ratio, serial, parallel := wipfixSnapshotOverlapRatio(t, c, 200)
	if ratio > wipfixSnapshotOverlapCeiling {
		t.Errorf("two concurrent warm snapshots cost %v against %v for one goroutine "+
			"(ratio %.2f, ceiling %.2f) — they are serialising, so reading a screen that "+
			"changes nothing takes the write lock", parallel, serial, ratio, wipfixSnapshotOverlapCeiling)
	}
}

// TestColdSnapshot_ConcurrentFirstReadsAreWellFormed is the negative half:
// whatever lock a warm read takes, the FIRST read of a screen still has to
// build the memo, which is a mutation. Two goroutines arriving on a cold
// controller together are the hazard the write lock exists for — a fix that
// simply demotes Snapshot to a read lock races that build, and under -race
// this is the probe that says so. Both readers must come back with the whole
// list.
//
// Correctness, not timing: under the detector a memo build is a small
// fraction of a snapshot, so "warm is faster than cold" is not measurable
// there and would only add a flake.
func TestColdSnapshot_ConcurrentFirstReadsAreWellFormed(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixEC2Rows(6000),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	got := make([]int, 2)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if body := c.Snapshot().Body.List; body != nil {
				got[i] = len(body.Rows)
			}
		}()
	}
	wg.Wait()

	for i, n := range got {
		if n != 6000 {
			t.Errorf("cold reader %d rendered %d rows, want 6000 — the first read of a screen "+
				"still has to build its memo, and two arriving together must both get a whole list",
				i, n)
		}
	}
	if body := c.Snapshot().Body.List; body == nil || len(body.Rows) != 6000 {
		t.Errorf("the read after them rendered %v rows, want 6000", body)
	}
}

// wipfixAbsorbReturnBudget is how long a 6000-row absorption may keep its
// caller. Measured on this bench: the same absorption on a screen whose
// result is not persisted returns in 19-20 ms ordinary and 112-121 ms under
// -race; on one that is, it takes 63-79 ms and 679-682 ms, and the whole
// difference is the YAML marshal of the row set running on the caller's
// goroutine. Each budget is the midpoint between its build's two numbers, so
// the pin is red while the marshal is in the caller's path and green once the
// save lane owns its own goroutine.
func wipfixAbsorbReturnBudget() time.Duration {
	if wipfixRaceDetector {
		return 280 * time.Millisecond
	}
	return 35 * time.Millisecond
}

// TestLargeFetchAbsorb_CallerDoesNotWaitForTheMarshal pins row 35's latency
// half.
func TestLargeFetchAbsorb_CallerDoesNotWaitForTheMarshal(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := wipfixEC2Rows(6000)

	start := time.Now()
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	took := time.Since(start)

	if budget := wipfixAbsorbReturnBudget(); took > budget {
		t.Errorf("absorbing 6000 rows kept its caller for %v, budget %v (race=%v) — "+
			"the row set's YAML marshal is running in the latency of the fetch that "+
			"triggered it, and every key press waits behind it", took, budget, wipfixRaceDetector)
	}
}

// TestLargeFetchAbsorb_StillWritesTheSameFile pins row 35's other half:
// moving the marshal off the caller's goroutine must leave the file on disk
// exactly as it is today. The golden was captured from this bench at
// 8a2dee1c, where the marshal is synchronous.
func TestLargeFetchAbsorb_StillWritesTheSameFile(t *testing.T) {
	c := newTestController(t)
	cfg := os.Getenv("A9S_CONFIG_FOLDER")
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    wipfixEC2Rows(50),
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	pattern := filepath.Join(cfg, "cache", "*", "ec2.yaml")
	var written []byte
	deadline := time.Now().Add(5 * time.Second)
	for {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("globbing %s: %v", pattern, err)
		}
		if len(matches) == 1 {
			if b, readErr := os.ReadFile(matches[0]); readErr == nil && len(b) > 0 {
				written = b
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no ec2 type file under %s within 5s — a save moved off the caller's "+
				"goroutine must still land", pattern)
		}
		time.Sleep(5 * time.Millisecond)
	}

	golden, err := os.ReadFile(filepath.Join("testdata", "wipfix_ec2_50rows.yaml"))
	if err != nil {
		t.Fatalf("reading the golden: %v", err)
	}
	if got, want := wipfixWithoutSavedAt(written), wipfixWithoutSavedAt(golden); got != want {
		t.Errorf("the ec2 type file differs from the one this tree writes today "+
			"(%d bytes against %d); first difference at %s",
			len(got), len(want), wipfixFirstDiff(got, want))
	}
}

// wipfixWithoutSavedAt drops the save timestamp, the one line that cannot be
// identical across two runs.
func wipfixWithoutSavedAt(b []byte) string {
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "saved_at:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// wipfixFirstDiff names the first line at which got and want part company.
func wipfixFirstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range max(len(g), len(w)) {
		gl, wl := "<absent>", "<absent>"
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return "line " + strconv.Itoa(i+1) + ": got " + gl + ", want " + wl
		}
	}
	return "<none>"
}
