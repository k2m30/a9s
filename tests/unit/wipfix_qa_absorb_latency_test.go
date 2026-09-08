// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_absorb_latency_test.go pins the C4 latency guarantee for the read
// half of a list load: absorbing a large fetch result must not hold the
// controller's write lock for the row work. The save half already meets this;
// the absorb half does the same amount of pure in-memory work under the lock,
// and every key press waits behind it.
//
// What is measured is the LOCKED SPAN, not a reader's end-to-end wait: the
// wait also contains the reader's own body build and the GC assist it pays for
// the cache-write marshal that already runs outside the lock, neither of which
// this row is about. The probe is Controller.GetListLane — an RLock, two
// pointer reads, no allocation — so how long it blocks is how long the writer
// held the lock, and nothing else.
package unit

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixAbsorbHoldBudget is the longest the controller lock may be held while
// a fetch result is absorbed, and wipfixAbsorbGrowthAllowance bounds how much
// that hold may grow when the result doubles. A hold that scales with the row
// count is row work under the lock however small the constant; a swap does not
// care how many rows it swaps.
//
// Two builds, two budgets, each measured rather than derived from the other.
// Holds observed on this bench, absorbing 6000 then 12000 rows:
//
//	build      row work under the lock   only the swap under it
//	ordinary   46 ms / 76 ms             4.1 ms / 4.9 ms
//	-race      227 ms / 452 ms           10.1 ms / 18.9 ms
//
// The detector instruments every memory access, so it inflates both columns
// and re-introduces a slope in the swap column: the bookkeeping for a swap
// still scales with what is swapped. The budgets sit between the two columns
// of their own row — a factor above what a swap costs there, several factors
// below what the row work costs — so each build's pin is red before the work
// moves off the lock and green after it, and neither build's number was
// guessed from the other's.
func wipfixAbsorbBudgets() (budget, growth time.Duration) {
	if wipfixRaceDetector {
		return 40 * time.Millisecond, 15 * time.Millisecond
	}
	return 10 * time.Millisecond, 4 * time.Millisecond
}

// wipfixEC2Rows returns n rows in the shape the ec2 fetcher writes.
func wipfixEC2Rows(n int) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := "i-" + strconv.Itoa(1000000000000000+i)
		rows[i] = resource.Resource{
			ID:   id,
			Name: "example-instance-" + strconv.Itoa(i),
			Fields: map[string]string{
				"instance_id": id, "state": "running", "instance_type": "t3.micro",
				"az": "us-east-1a", "private_ip": "10.0.1.10", "vpc_id": "vpc-0123456789abcdef0",
			},
		}
	}
	return rows
}

// wipfixLockHoldDuringAbsorb absorbs n rows on a fresh controller and returns
// the longest a concurrent lock-taking reader was blocked — the writer's hold,
// since the probe itself does no work worth measuring.
func wipfixLockHoldDuringAbsorb(t *testing.T, n int) time.Duration {
	t.Helper()
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := wipfixEC2Rows(n)

	var stop atomic.Bool
	worst := make(chan time.Duration, 1)
	go func() {
		var longest time.Duration
		for !stop.Load() {
			start := time.Now()
			c.GetListLane()
			if d := time.Since(start); d > longest {
				longest = d
			}
		}
		worst <- longest
	}()

	// Let the probe loop reach steady state so its own first-call costs are
	// not read as contention.
	time.Sleep(5 * time.Millisecond)

	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	stop.Store(true)
	held := <-worst

	if body := c.Snapshot().Body.List; body == nil || len(body.Rows) != n {
		got := 0
		if body != nil {
			got = len(body.Rows)
		}
		t.Fatalf("the list holds %d rows after absorbing %d — moving work off the lock must not lose it",
			got, n)
	}
	return held
}

// TestLargeFetchAbsorb_HoldsTheLockOnlyForTheSwap pins row 23.
func TestLargeFetchAbsorb_HoldsTheLockOnlyForTheSwap(t *testing.T) {
	budget, growth := wipfixAbsorbBudgets()
	small := wipfixLockHoldDuringAbsorb(t, 6000)
	large := wipfixLockHoldDuringAbsorb(t, 12000)

	if small > budget {
		t.Errorf("absorbing 6000 rows held the controller lock for %v, budget %v (race=%v) — "+
			"the row work is pure in-memory and belongs outside the lock, with only the "+
			"built body swapped under it", small, budget, wipfixRaceDetector)
	}
	if large > budget {
		t.Errorf("absorbing 12000 rows held the controller lock for %v, budget %v (race=%v)",
			large, budget, wipfixRaceDetector)
	}
	if large-small > growth {
		t.Errorf("the lock hold grew from %v at 6000 rows to %v at 12000 — a hold that scales "+
			"with the result is row work under the lock; a swap does not", small, large)
	}
}
