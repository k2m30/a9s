// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_absorb_latency_test.go pins the C4 latency guarantee for the read
// half of a list load: absorbing a large fetch result must not hold the
// controller's write lock for the whole of the row work. The save half
// already meets this; the absorb half does the same amount of pure in-memory
// work with no disk in it and blocks every concurrent reader for the
// duration, which is what a key press feels as a stalled UI.
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

// wipfixAbsorbReaderBudget is the longest a reader may wait behind one
// absorption. Deliberately far above any plausible lock-free hand-off and far
// below the ~45 ms a 6000-row absorb takes under the lock, so neither a busy
// machine nor a fast one changes the verdict.
const wipfixAbsorbReaderBudget = 25 * time.Millisecond

// TestLargeFetchAbsorb_DoesNotBlockReadersForItsWholeDuration pins row 23.
func TestLargeFetchAbsorb_DoesNotBlockReadersForItsWholeDuration(t *testing.T) {
	c := newTestController(t)
	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})

	rows := make([]resource.Resource, 6000)
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

	var stop atomic.Bool
	worst := make(chan time.Duration, 1)
	go func() {
		var max time.Duration
		for !stop.Load() {
			start := time.Now()
			_ = c.Snapshot()
			if d := time.Since(start); d > max {
				max = d
			}
			time.Sleep(200 * time.Microsecond)
		}
		worst <- max
	}()

	// Let the reader settle so its own first-call costs are not measured as
	// contention.
	time.Sleep(5 * time.Millisecond)

	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    rows,
		Pagination:   &domain.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	stop.Store(true)
	got := <-worst

	if got > wipfixAbsorbReaderBudget {
		t.Errorf("a reader waited %v behind a 6000-row absorption, budget %v — "+
			"the row work is pure in-memory and belongs outside the lock, with only the "+
			"built body swapped under it", got, wipfixAbsorbReaderBudget)
	}

	// The absorption still has to have happened.
	if body := c.Snapshot().Body.List; body == nil || len(body.Rows) != len(rows) {
		got := 0
		if body != nil {
			got = len(body.Rows)
		}
		t.Errorf("the list holds %d rows after absorbing %d — moving work off the lock must not lose it",
			got, len(rows))
	}
}
