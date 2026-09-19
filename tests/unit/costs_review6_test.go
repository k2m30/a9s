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

// Store.Save marshals the store's maps after c.mu is released, costs.Store has
// no mutex of its own, and costsDirtyStore is the same *Store pointer as
// cs.Store, so Save must not walk maps a concurrent delivery mutates under
// c.mu.

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

	// A concurrent map mutation racing yaml.Marshal's walk of the same map can
	// panic mid-marshal, write a torn temp file, or drop keys; LoadStore's
	// Recovered() reports a file on disk that did not parse.
	reloaded := costs.LoadStore("test-profile")
	if reloaded.Recovered() {
		t.Error("the on-disk cost cache was corrupt and had to be recovered (set aside) after concurrent deliveries overlapped Store.Save's yaml.Marshal — Save must never marshal the live, still-mutable Store maps unsynchronized against concurrent Merge/MergeCoverage calls (either a lock Save holds across the marshal, or a deep copy taken before c.mu is released)")
	}
}

// The granularity fallback re-plans a RESOURCE_ID-shaped frame at its parent
// granularity and period and must re-apply ClampResourceDrillWindow
// (core/costs/drill.go): the unclamped parent week can start before the
// 14-day resource-drill retention cutoff.
//
// now=2026-07-17 puts the cutoff at 2026-07-03. July's first, month-clipped
// week {2026-07-01, 2026-07-06} starts before the cutoff, so a single-period
// clamp drops it whole, while its day tiling (July 1-5) keeps July 3-5: the
// RESOURCE_ID drill from that week cell is allowed, but the week itself, as
// Fallback.SelectedPeriod, is not.

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
	weekPeriod := usageTop.Window[0] // the stale, boundary week this test targets
	if weekPeriod.Start != "2026-07-01" {
		t.Fatalf("test assumption wrong: July's first (clipped) week Start = %q, want \"2026-07-01\" — date arithmetic drifted", weekPeriod.Start)
	}
	if got := costs.ClampResourceDrillWindow([]costs.Period{weekPeriod}, now); len(got) != 0 {
		t.Fatalf("test assumption wrong: the week period %+v survives ClampResourceDrillWindow at now=%v (cutoff %s) — pick a staler week", weekPeriod, now, cutoff)
	}

	// A pushed child frame opens on its current column (applyCostsSelect's
	// PushDrill case); scrolling back to col 0 keeps the stale week[0] cell
	// selected. ActionScrollLeft clamps at col 0.
	for range usageTop.Window {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}

	// The USAGE_TYPE frame's own delivery: a non-zero cell at weekPeriod
	// (column 0, restored above), making the RESOURCE_ID drill's own
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

	// The RESOURCE_ID frame's already-clamped fetch returns zero records, which
	// fires the fallback.
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
	// Refusing the re-plan once clamping empties the parent window is equally
	// correct; only a dispatched task whose Range starts before the cutoff fails.
}

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
