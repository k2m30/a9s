// costs_review6_test.go — Round 6, four new external-review findings.
// package unit_test (headless-only; reuses newCostsController / topDrill /
// findFetchCostsTask / fullMetricRecord / codexMoveCursorToRow /
// codexMoveCursorToNewestColumn, same helpers costs_review3_test.go and
// siblings already share).
package unit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ===========================================================================
// Q1 (internal/app/handle.go:~170) — Store.Save marshals the store's own
// maps AFTER c.mu is released (the write-must-not-block-under-the-lock
// design, handle.go's own doc comment). costs.Store carries no mutex of its
// own (confirmed by reading its field list — profile/path/data/recovered/
// revision, nothing synchronization-shaped) and costsDirtyStore is the SAME
// *Store pointer as cs.Store (costs_state.go:1030 `c.costsDirtyStore =
// cs.Store`), not a copy — a concurrent delivery's c.mu-protected mutation
// of cs.Store's maps races, unsynchronized, against Save's yaml.Marshal
// walking those same maps outside the lock.
// ===========================================================================

func TestCostsReview6_Q1_ConcurrentDeliveryDuringSave_NeverCorruptsOnDiskCache(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	col := root.Window[len(root.Window)-1]

	// Widen Store.Save's yaml.Marshal window: seed many distinct query
	// shapes so the marshal has real map-walking work to do while a
	// concurrent delivery mutates the same maps.
	for i := range 150 {
		svc := fmt.Sprintf("Seed Service %d", i)
		c.Handle(messages.CostsLoaded{
			Query: costs.Query{
				Granularity: costs.GranularityMonth.APIGranularity(),
				GroupBy:     []costs.Dimension{costs.DimensionUsageType},
				Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {svc}}},
			},
			Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(col, svc, float64(i))}},
			Window:   root.Window,
			Requests: 1,
		})
	}

	// Many goroutines, each repeatedly delivering a DIFFERENT query shape
	// to the SAME controller — every delivery marks the Store dirty and
	// triggers its own Save() (handle.go, unlocked), so a genuine overlap
	// between one delivery's own Save (marshaling cs.Store's maps) and
	// another's c.mu-protected mutation of those SAME maps is a real race,
	// not a contrived one: this is production's own dispatch shape,
	// nothing test-only about the interleaving.
	var wg sync.WaitGroup
	for g := range 12 {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range 15 {
				svc := fmt.Sprintf("Race Service %d-%d", g, i)
				c.Handle(messages.CostsLoaded{
					Query: costs.Query{
						Granularity: costs.GranularityMonth.APIGranularity(),
						GroupBy:     []costs.Dimension{costs.DimensionUsageType},
						Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {svc}}},
					},
					Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(col, svc, float64(g*100+i))}},
					Window:   root.Window,
					Requests: 1,
				})
			}
		}(g)
	}
	wg.Wait()

	// The final on-disk cache must round-trip cleanly. A concurrent map
	// mutation racing yaml.Marshal's reflection walk of the SAME map can
	// panic mid-marshal, write a torn/garbled temp file, or silently drop
	// keys — LoadStore's own Recovered() is exactly the "the file on disk
	// did not parse, set aside" signal every other corrupt-cache pin in
	// this suite (X9, R4) already keys off.
	reloaded := costs.LoadStore("test-profile")
	if reloaded.Recovered() {
		t.Error("the on-disk cost cache was corrupt and had to be recovered (set aside) after concurrent deliveries overlapped Store.Save's yaml.Marshal — Save must never marshal the live, still-mutable Store maps unsynchronized against concurrent Merge/MergeCoverage calls (either a lock Save holds across the marshal, or a deep copy taken before c.mu is released)")
	}
}

// ===========================================================================
// Q2 (internal/app/costs_state.go:~1055) — the N3 granularity fallback
// re-plans a RESOURCE_ID-shaped frame at its parent granularity/period
// WITHOUT re-applying ClampResourceDrillWindow (internal/costs/drill.go) —
// a clamped-then-allowed RESOURCE_ID drill whose finer (day-level) fetch
// genuinely returns zero re-queries the UNCLAMPED parent (week) window,
// which can start before the 14-day resource-drill retention cutoff.
//
// Repro: now=2026-07-17 puts the retention cutoff at 2026-07-03. July's
// first (month-clipped) week is {Start:2026-07-01, End:2026-07-06} — its
// OWN Start (07-01) is before the cutoff (07-03), so
// ClampResourceDrillWindow treats the whole week as stale (single-period
// clamp is all-or-nothing). But daysWithinPeriod's day-tiling of that same
// week yields July 1-5, of which July 3-5 survive clamping — so the
// RESOURCE_ID drill from that week cell is genuinely ALLOWED (window
// non-empty), while the week cell ITSELF, as the fallback's own
// Fallback.SelectedPeriod, is not.
// ===========================================================================

func TestCostsReview6_Q2_FallbackOnResourceFrame_ReclampsAgainstRetentionCutoff(t *testing.T) {
	now := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	const cutoff = "2026-07-03"
	const ec2Service = "Amazon Elastic Compute Cloud - Compute"

	c := newCostsController(t, now)
	root := topDrill(t, c)
	julyCol := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(julyCol, ec2Service, 900.0)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "Elastic Compute Cloud - Compute") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE (week granularity)
	usagePayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageTop := topDrill(t, c)
	if len(usageTop.Window) == 0 {
		t.Fatal("precondition: USAGE_TYPE frame has an empty Window")
	}
	weekPeriod := usageTop.Window[0] // cursor defaults to column 0 on a fresh drill
	if weekPeriod.Start != "2026-07-01" {
		t.Fatalf("test assumption wrong: July's first (clipped) week Start = %q, want \"2026-07-01\" — date arithmetic drifted", weekPeriod.Start)
	}
	if got := costs.ClampResourceDrillWindow([]costs.Period{weekPeriod}, now); len(got) != 0 {
		t.Fatalf("test assumption wrong: the week period %+v survives ClampResourceDrillWindow at now=%v (cutoff %s) — pick a staler week", weekPeriod, now, cutoff)
	}

	// The USAGE_TYPE frame's own delivery: a non-zero cell at weekPeriod
	// (column 0, the cursor's default), making the RESOURCE_ID drill's own
	// Fallback.Eligible true.
	c.Handle(messages.CostsLoaded{
		Query:    usagePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(weekPeriod, "USE1-BoxUsage:m5.large", 250.0)}},
		Window:   usagePayload.Window,
		Requests: 1,
	})

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> RESOURCE_ID (day granularity), drilling from the stale week cell
	resourcePayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: USAGE_TYPE -> RESOURCE_ID drill did not emit a fetch task — the day-level clamped window is unexpectedly empty (date arithmetic drifted)")
	}
	resourceTop := topDrill(t, c)
	if !resourceTop.Fallback.Eligible {
		t.Fatal("precondition: RESOURCE_ID frame's Fallback.Eligible is false — the parent week cell must have been non-zero")
	}
	if resourceTop.Fallback.SelectedPeriod != weekPeriod {
		t.Fatalf("precondition: Fallback.SelectedPeriod = %+v, want the stale week %+v", resourceTop.Fallback.SelectedPeriod, weekPeriod)
	}

	// The RESOURCE_ID frame's own (already-clamped) fetch genuinely
	// returns zero records — N3 fires.
	_, fallbackTasks := c.Handle(messages.CostsLoaded{
		Query:    resourcePayload.Query,
		Grid:     costs.GridResult{Fetched: true},
		Window:   resourcePayload.Window,
		Requests: 1,
	})

	if fallbackTask, found := findFetchCostsTask(fallbackTasks); found {
		if fallbackTask.Query.Range.Start < cutoff {
			t.Errorf("N3 fallback on a RESOURCE_ID frame re-planned using the UNCLAMPED stale parent week (Start=%s) — the re-fetch queries a Range starting %s, before the 14-day resource-drill retention cutoff (%s); ClampResourceDrillWindow must be re-applied to the fallback's own re-planned Window before dispatching, exactly as the original PushDrill applies it", weekPeriod.Start, fallbackTask.Query.Range.Start, cutoff)
		}
	}
	// A nil/no-task outcome (the fix refuses the re-plan entirely once
	// clamping empties the parent window) is an equally valid fixed
	// behavior — not asserted as a failure here, only a dispatched task
	// whose Range starts before the cutoff is.
}

// ===========================================================================
// Q3 (internal/costs/window.go:79 daysWithinPeriod) — ignores sel.End
// entirely, tiling the WHOLE ISO week containing sel.Start instead of just
// sel itself. WindowWithin(oneDayPeriod, Day, now) must return exactly that
// one day, not all 7 days of its enclosing week.
// ===========================================================================

func TestCostsReview6_Q3_WindowWithin_OneDayPeriod_TilesExactlyThatDay(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	oneDay := costs.Period{Start: "2026-07-15", End: "2026-07-16"}

	got := costs.WindowWithin(oneDay, costs.GranularityDay, now)

	if len(got) != 1 {
		t.Fatalf("WindowWithin(%+v, Day, now) = %d periods %+v, want exactly 1 (sel's own day) — daysWithinPeriod ignores sel.End and tiles the whole ISO week containing sel.Start instead", oneDay, len(got), got)
	}
	if got[0] != oneDay {
		t.Errorf("WindowWithin(%+v, Day, now)[0] = %+v, want the input period back unchanged", oneDay, got[0])
	}
}

// ===========================================================================
// Q4 behavior (internal/app/actions_list.go:170) — Go evaluates return
// operands left-to-right, so `return c.snapshot(), c.forceRefreshCostsLocked()`
// runs c.snapshot() BEFORE forceRefreshCostsLocked() mutates cs.Loading —
// Apply(ActionRefresh) on a warm costs screen returns the STALE pre-refresh
// snapshot (Loading=false), not the post-refresh one a caller (TUI or
// headless) needs to render the refresh as already in flight.
// ===========================================================================

func TestCostsReview6_Q4_ApplyActionRefresh_ReturnsPostRefreshSnapshot(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	col := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(col, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})
	if c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: still Loading after a full warming delivery")
	}

	vs, tasks := c.Apply(app.Action{Kind: app.ActionRefresh})

	if len(tasks) == 0 {
		t.Fatal("precondition: ActionRefresh on a warm open-period shape did not dispatch a fetch task")
	}
	if !vs.Body.Costs.Loading {
		t.Error("Apply(ActionRefresh)'s OWN returned ViewState has Loading=false even though forceRefreshCostsLocked (which the SAME call just dispatched a fetch task from) sets Loading=true — c.snapshot() was evaluated BEFORE the mutating call in the return statement's left-to-right operand order, so the caller sees the pre-refresh snapshot, not the post-refresh one")
	}
}
