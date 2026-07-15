// costs_codex_test.go — Cost Explorer: external-review round (Codex, X1-X11)
// + two live-tmux repros. Every item traced against current source before
// writing; a trace disproving the reviewer's stated claim is called out
// inline where found. package unit_test: every finding is reachable via the
// headless app.Controller / internal/costs / internal/aws surface, reusing
// sibling unit_test files' helpers (newCostsController/topDrill/
// fixedCostsNow/monthRecord/fullMetricRecord/findFetchCostsTask/
// baseServiceQuery), same convention as costs_selfreview_test.go.
package unit_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ===========================================================================
// X1 (P1) — demo acceptance path. Traced precisely: costs.ResourceDrillAllowed
// (internal/costs/drill.go) refuses any SERVICE other than the literal
// resourceDrillAllowedService ("Amazon Elastic Compute Cloud - Compute")
// BEFORE ever reaching CostsResourceRowsByService — and the planted growth
// story lives under fixtures.CostsGrowthService = "EC2 - Other", which is
// NOT that string. So SC-001's spike -> usage type -> resource chain is
// refused at the USAGE_TYPE -> RESOURCE_ID Enter, regardless of the
// fixture's resource-row keying. Pinned via the constants (not literals):
// once the coder re-plants the story under CostExplorerServiceNameEC2, this
// test needs no edit.
// ===========================================================================

// deliverCodexDemoFetch executes payload's KindFetchCosts task against the
// real demo transport and delivers the resulting event into c — the actual
// production fetch path, not hand-built records.
func deliverCodexDemoFetch(t *testing.T, core *runtime.Core, c *app.Controller, payload runtime.FetchCostsPayload) {
	t.Helper()
	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchCosts}, Payload: payload}
	event, err := core.ExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteTask(KindFetchCosts) against the demo transport: %v", err)
	}
	loaded, ok := event.(messages.CostsLoaded)
	if !ok {
		t.Fatalf("ExecuteTask(KindFetchCosts) returned %T, want messages.CostsLoaded", event)
	}
	if loaded.Err != nil {
		t.Fatalf("CostsLoaded.Err: %v", loaded.Err)
	}
	c.Handle(loaded)
}

// codexGrowthServiceLabel returns the grid row label fixtures.CostsGrowthService
// renders under — stripCostsServiceVendorPrefix strips any "Amazon "/"AWS "
// vendor prefix off a SERVICE-dimension row (costs_body.go).
func codexGrowthServiceLabel() string {
	s := strings.TrimPrefix(fixtures.CostsGrowthService, "Amazon ")
	return strings.TrimPrefix(s, "AWS ")
}

// codexMoveCursorToNewestColumn scrolls the cursor to the last (newest)
// column of the current drill frame's window — ActionScrollRight clamps at
// the window's own end, so over-scrolling is safe.
func codexMoveCursorToNewestColumn(c *app.Controller) {
	vs := c.Snapshot()
	if vs.Body.Costs == nil {
		return
	}
	for range vs.Body.Costs.Columns {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
}

// codexMoveCursorToRow moves the cursor down until the highlighted row's
// Label equals want, returning false if no such row is ever reached.
func codexMoveCursorToRow(c *app.Controller, want string) bool {
	vs := c.Snapshot()
	if vs.Body.Costs == nil {
		return false
	}
	for range vs.Body.Costs.Rows {
		if vs.Body.Costs.Rows[vs.Body.Costs.CursorRow].Label == want {
			return true
		}
		c.Apply(app.Action{Kind: app.ActionMoveDown})
		vs = c.Snapshot()
	}
	return len(vs.Body.Costs.Rows) > 0 && vs.Body.Costs.Rows[vs.Body.Costs.CursorRow].Label == want
}

func TestCostsCodex_X1_GrowthStory_ResourceChain_EndToEnd_OverDemoTransport(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "codex-x1"
	s.Region = "us-east-1"
	s.Clients = demo.NewServiceClients()
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(time.Now())

	// Enter 1: SERVICE level. ActionCostPivot resets Cursor.Col to 0 (the
	// OLDEST column) as a side effect of digit-key pivoting (applyCostPivot
	// case 1-6: "top.Cursor.Row, top.Cursor.Col = 0, 0") — the root frame's
	// own FR-002 "opens at newest" default only holds before any pivot, so
	// this test must re-position to the newest column itself, matching a
	// real user who presses "1" then looks at "today"'s column.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	deliverCodexDemoFetch(t, core, c, payload)
	codexMoveCursorToNewestColumn(c)

	if !codexMoveCursorToRow(c, codexGrowthServiceLabel()) {
		t.Fatalf("precondition: no SERVICE row labeled %q in the demo grid after the fetch landed", codexGrowthServiceLabel())
	}
	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE level
	payload, found = findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: Enter 1 (SERVICE row) did not emit a fetch task for the USAGE_TYPE level")
	}
	deliverCodexDemoFetch(t, core, c, payload)

	// Enter 2: USAGE_TYPE level. A fresh push's Cursor is always (0,0) —
	// the OLDEST column of the child window (applyCostsSelect constructs a
	// bare costs.DrillLevel{}, never costsOpenAtNewestDrillLevel) — so
	// column 0 of "weeks of the current month" can be more than 14 days
	// before now whenever today isn't in the first week. Move to the
	// newest column first (matching a real user drilling into "today"),
	// same as the root frame's own FR-002 default, before locating the
	// growth-usage-type row.
	codexMoveCursorToNewestColumn(c)
	if !codexMoveCursorToRow(c, fixtures.CostsGrowthUsageType) {
		t.Fatalf("precondition: no USAGE_TYPE row labeled %q in the demo grid after the fetch landed", fixtures.CostsGrowthUsageType)
	}
	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> RESOURCE_ID level
	stack := c.GetCostsDrillStack()
	if len(stack) < 3 {
		vs := c.Snapshot()
		t.Fatalf("Enter 2 (USAGE_TYPE row %q under service %q) did not drill to RESOURCE_ID — the resource-level drill was refused: FooterNote %q. SC-001's spike -> usage type -> resource chain (<=3 Enters) does not work for the planted growth story.",
			fixtures.CostsGrowthUsageType, fixtures.CostsGrowthService, vs.Body.Costs.FooterNote)
	}
	payload, found = findFetchCostsTask(tasks)
	if !found {
		t.Fatal("RESOURCE_ID level push emitted no KindFetchCosts task (GetCostAndUsageWithResources)")
	}
	deliverCodexDemoFetch(t, core, c, payload)

	// Enter 3: RESOURCE_ID level — select the (only, or first) resource row;
	// this must navigate straight to the EC2 detail view.
	vs := c.Snapshot()
	if vs.Body.Costs == nil || len(vs.Body.Costs.Rows) == 0 {
		t.Fatalf("RESOURCE_ID grid has zero rows after the demo fetch landed — CostsResourceRowsByService has no entry for %q (or the story's service does not match it)", fixtures.CostsGrowthService)
	}
	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect})
	var byIDTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchByIDDetail {
			byIDTask = &tasks[i]
		}
	}
	if byIDTask == nil {
		t.Fatal("Enter 3 (resource row) did not emit KindFetchByIDDetail — SC-001's 3rd Enter must jump straight to the resource detail")
	}
	event, err := core.ExecuteTask(context.Background(), *byIDTask)
	if err != nil {
		t.Fatalf("ExecuteTask(KindFetchByIDDetail) against the demo transport: %v", err)
	}
	c.Handle(event)

	vs = c.Snapshot()
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("after 3 Enters from the growth-story spike, Body.Kind = %q, want %q (SC-001: spike -> usage type -> resource in <=3 Enters)", vs.Body.Kind, app.BodyKindDetail)
	}
}

// ===========================================================================
// X2 (P1) — a CostsLoaded produced under SkipAnomalies must not clear cached
// marks nor renew the anomaly TTL. Traced precisely: ApplyCostsLoaded
// (internal/app/costs_state.go) calls cs.Store.PutAnomalies(ev.Anomalies,
// cs.Now) UNCONDITIONALLY on every successful delivery — CostsLoaded carries
// no AnomaliesFetched/authoritative flag distinguishing "genuinely fetched
// zero" from "skipped, anomalies untouched" (the executor leaves
// ev.Anomalies nil in both cases when p.SkipAnomalies is true). A skip
// delivery therefore wrongly clears/re-stamps the cache.
//
// RECONCILED (architecture.md Seam 2/6): this pin's intent — a skip must
// preserve marks and TTL — now has a mechanism-level pin at the typed seam
// in costs_screen_test.go (TestCostsScreen_ApplyFetchResult_AnomalyResult_
// WriteSemantics, case "not requested"), reconciled there against C4a's
// opposite intent (an authoritative empty result must clear). This
// controller-level test stays as the full-stack acceptance pin.
// ===========================================================================

func TestCostsCodex_X2_SkipAnomalies_PreservesMarksAndTTL(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "codex-x2"
	t0 := fixedCostsNow

	// Gen 1 (t0): full delivery with a planted anomaly mark on the newest
	// column, persisted to disk.
	s1 := session.New()
	s1.Profile = profile
	s1.Region = "us-east-1"
	core1 := runtime.New(s1, nil)
	c1 := app.New(core1)
	c1.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c1.EnsureCostsState(t0)
	root := topDrill(t, c1)
	newestCol := root.Window[len(root.Window)-1]

	_, tasks := c1.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c1.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid:  costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 100)}},
		Anomalies: []costs.AnomalyMark{{
			Impact: costs.Amount{Value: 50, Unit: "USD"},
			Period: newestCol,
			Dimension: map[costs.Dimension]string{
				costs.DimensionService: "Amazon EC2",
			},
		}},
		Requests: 1,
	})
	vs := c1.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 || !vs.Body.Costs.Rows[0].Cells[len(vs.Body.Costs.Rows[0].Cells)-1].Anomaly {
		t.Fatal("precondition: planted anomaly mark did not render on the newest-column cell after Gen1's delivery")
	}
	c1.Close()

	// Gen 2 (t1 = t0+20h, within the 24h anomaly TTL): a metric cycle forces
	// a genuine shape-miss (Invoice -> Unblended has a distinct Filter) while
	// anomalies are still fresh from t0 — the executor's own SkipAnomalies
	// decision (ensureCostsShapeFetched) should fire, and this test
	// simulates exactly what that executor delivery looks like: Anomalies
	// nil because they were never re-fetched, not because CE returned zero.
	t1 := t0.Add(20 * time.Hour)
	s2 := session.New()
	s2.Profile = profile
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, nil)
	c2 := app.New(core2)
	c2.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c2.EnsureCostsState(t1)

	_, tasks = c2.Apply(app.Action{Kind: app.ActionCostMetric})
	skipPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: metric cycle did not emit a fetch task for the Unblended shape")
	}
	if !skipPayload.SkipAnomalies {
		t.Fatal("precondition: anomalies should still be fresh 20h after Gen1's fetch — SkipAnomalies should be true")
	}
	c2.Handle(messages.CostsLoaded{
		Query: skipPayload.Query,
		Grid:  costs.GridResult{Fetched: true, Records: []costs.Record{{Period: newestCol, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricUnblended: {Value: 90, Unit: "USD"}}}}},
		// Anomalies deliberately nil — SkipAnomalies delivery.
		Requests: 1,
	})

	// Cycle back to Invoice (already cached from Gen1) — no new fetch — and
	// confirm the original mark still renders (must not have been cleared).
	c2.Apply(app.Action{Kind: app.ActionCostMetric})
	c2.Apply(app.Action{Kind: app.ActionCostMetric})
	c2.Apply(app.Action{Kind: app.ActionCostMetric})
	c2.Apply(app.Action{Kind: app.ActionCostMetric}) // 5-entry cycle: Unblended -> Amortized -> NetAmortized -> Blended -> Invoice
	vs = c2.Snapshot()
	if vs.Body.Costs.Metric != string(costs.MetricInvoice) {
		t.Fatalf("precondition: expected to be back on Invoice metric, got %q", vs.Body.Costs.Metric)
	}
	if len(vs.Body.Costs.Rows) == 0 || !vs.Body.Costs.Rows[0].Cells[len(vs.Body.Costs.Rows[0].Cells)-1].Anomaly {
		t.Error("the planted anomaly mark was cleared by a SkipAnomalies delivery (Anomalies:nil is not the same as 'CE returned zero anomalies this fetch')")
	}
	c2.Close()

	// Gen 3 (t2 = t0+25h, past the ORIGINAL 24h TTL from t0, but only 5h
	// after Gen2's skip-delivery at t1): if the skip-delivery wrongly
	// renewed the TTL to t1, anomalies would still read "fresh" here
	// (t2-t1=5h<24h) and a fresh shape-miss would again SkipAnomalies. The
	// correct behavior (TTL anchored at t0, never touched by a skip
	// delivery) requires a real anomaly re-fetch now (t2-t0=25h>24h).
	t2 := t0.Add(25 * time.Hour)
	s3 := session.New()
	s3.Profile = profile
	s3.Region = "us-east-1"
	core3 := runtime.New(s3, nil)
	c3 := app.New(core3)
	t.Cleanup(c3.Close)
	c3.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c3.EnsureCostsState(t2)

	_, tasks = c3.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION — guaranteed shape-miss
	regionPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	if regionPayload.SkipAnomalies {
		t.Error("25h after the original anomaly fetch (t0), a fresh shape-miss still has SkipAnomalies=true — the anomaly TTL was wrongly renewed by the intervening SkipAnomalies delivery at t1, instead of staying anchored to the last REAL anomaly fetch")
	}
}

// ===========================================================================
// X3 (P2) — a warm-cache restart (cost data fully covered, anomaly slot
// absent) must still emit an anomalies-only fetch. Traced precisely:
// ensureCostsShapeFetched (internal/app/costs_state.go) returns nil the
// moment len(missing)==0, WITHOUT ever checking cs.Store.Anomalies(cs.Now)
// freshness — that check only happens inside the missing!=0 branch (there,
// only to set the SkipAnomalies flag on an already-needed main fetch).
//
// RECONCILED (architecture.md Seam 1): this pin's exact bug class ("warm
// cost rows suppress anomaly refresh") is the doc's own named example for
// why Grid/Anomalies must derive independently — pinned at the typed seam
// in costs_screen_test.go
// (TestCostsScreen_PlanFetch_GridAndAnomalyFreshnessDeriveIndependently,
// case "grid warm, anomalies stale"). This controller-level test stays as
// the full-stack acceptance pin.
// ===========================================================================

func TestCostsCodex_X3_WarmCostCache_AbsentAnomalies_StillEmitsFetch(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "codex-x3"
	now := fixedCostsNow
	q := baseServiceQuery()
	window := costs.BuildWindow(costs.GranularityMonth, now)

	// Pre-seed a store whose cost data fully covers the default root
	// window — closed periods old enough to be trusted regardless of fetch
	// age, and the OPEN (current-month) period fetched fresh (within
	// openPeriodTTL=24h of now) so ITS coverage doesn't confound this
	// test's own anomaly-only gap — but never registers any anomaly fetch
	// at all.
	store := costs.LoadStore(profile)
	var closedRecs, openRecs []costs.Record
	var closedWindow, openWindow []costs.Period
	for _, p := range window {
		rec := costs.Record{Period: p, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}}
		if p.Closed(now) {
			closedRecs = append(closedRecs, rec)
			closedWindow = append(closedWindow, p)
		} else {
			openRecs = append(openRecs, rec)
			openWindow = append(openWindow, p)
		}
	}
	store.Merge(q, closedRecs, now.Add(-96*time.Hour))
	store.MergeCoverage(q, closedWindow, now.Add(-96*time.Hour))
	store.Merge(q, openRecs, now)
	store.MergeCoverage(q, openWindow, now)
	if err := store.Save(); err != nil {
		t.Fatalf("seeding on-disk store: %v", err)
	}

	// Precondition: reload a throwaway Store the same way EnsureCostsState
	// will and confirm the cost data itself is genuinely fully covered
	// (missing==0) at now — isolating this test from any coverage bug
	// unrelated to the anomaly-freshness gap under test.
	if _, missing := costs.LoadStore(profile).Lookup(q, window, now); len(missing) != 0 {
		t.Fatalf("precondition: seeded store still reports %d missing periods — the cost-data seed itself is incomplete, not the anomaly gap this test targets: %+v", len(missing), missing)
	}

	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(now)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already default RowDim
	if _, found := findFetchCostsTask(tasks); !found {
		t.Error("a fully-covered cost cache with an absent/expired anomaly slot emitted no fetch task on warm restart — FR-014's anomaly markers can never appear until the user forces a refresh")
	}
}

// ===========================================================================
// X4 — the drilled frame's window must lie WITHIN the selected cell's
// period. Traced precisely: applyCostsSelect (internal/app/costs_state.go)
// computes anchor := selectedPeriod.Start then newWindow :=
// costs.BuildWindow(newGran, anchor) — for newGran==Month (drilling out of a
// YEAR cell), BuildWindow(Month, anchor) calls monthWindow(anchor, 12),
// which is a TRAILING 12-month window ENDING at anchor's month, not "the
// 12 months of anchor's own year" — so a year cell (Jan 1-Dec 31) drills
// into a window spanning Feb of the PRIOR year through January of the
// selected year, never the year's own Feb-Dec. Month->week (finerGranularity
// (Month)==Week) does NOT share this defect: BuildWindow(Week, anchor) is
// weekWindowsInMonth(anchor), always anchored on anchor's own month, never
// trailing — verified both at cursor col=0 and col=len-1 below.
//
// RECONCILED (architecture.md Seam 4): X4a's intent now has a
// mechanism-level GREEN pin at the typed constructor in
// costs_screen_test.go (TestCostsScreen_WindowWithin_YearToMonths_
// InsideSelectedYear) — WindowWithin replaces the overloaded BuildWindow
// this bug lived in. This controller-level test stays as the full-stack
// acceptance pin.
// ===========================================================================

func TestCostsCodex_X4a_YearCellDrill_MonthWindowStaysWithinSelectedYear(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year, cursor lands on the open (current) year
	yearPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zoom-out to year did not emit a fetch task")
	}
	root := topDrill(t, c)
	selectedYear := root.Window[root.Cursor.Col]

	c.Handle(messages.CostsLoaded{
		Query:    yearPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(selectedYear, "Amazon EC2", 100)}},
		Requests: 1,
	})

	c.Apply(app.Action{Kind: app.ActionSelect}) // -> MONTH level, anchored on the selected year
	stack := c.GetCostsDrillStack()
	if len(stack) < 2 {
		t.Fatal("precondition: Enter on the year cell did not push a MONTH-level frame")
	}
	child := stack[len(stack)-1]
	if len(child.Window) == 0 {
		t.Fatal("precondition: drilled MONTH window is empty")
	}
	first, last := child.Window[0], child.Window[len(child.Window)-1]
	if first.Start < selectedYear.Start || last.End > selectedYear.End {
		t.Errorf("MONTH window after drilling into year %+v spans [%s, %s) — outside the selected year's own [%s, %s) bounds (a trailing 12-month window ending at the year's own January, not that year's Feb-Dec)",
			selectedYear, first.Start, last.End, selectedYear.Start, selectedYear.End)
	}
}

func TestCostsCodex_X4b_MonthCellDrill_WeekWindowStaysWithinSelectedMonth_BothCursorEdges(t *testing.T) {
	tests := []struct {
		name        string
		atOldestCol bool
	}{
		{"cursor at oldest (col 0) column", true},
		{"cursor at newest (last) column", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCostsController(t, fixedCostsNow)
			root := topDrill(t, c)

			_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE — resets Cursor.Col to 0
			payload, found := findFetchCostsTask(tasks)
			if !found {
				t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
			}
			c.Handle(messages.CostsLoaded{
				Query:    payload.Query,
				Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(root.Window[len(root.Window)-1], "Amazon EC2", 100)}},
				Requests: 1,
			})
			if !tt.atOldestCol {
				codexMoveCursorToNewestColumn(c)
			}

			r := topDrill(t, c)
			selectedMonth := r.Window[r.Cursor.Col]

			c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE level (week granularity)
			stack := c.GetCostsDrillStack()
			if len(stack) < 2 {
				t.Fatal("precondition: Enter on the month cell did not push a USAGE_TYPE-level frame")
			}
			child := stack[len(stack)-1]
			if len(child.Window) == 0 {
				t.Fatal("precondition: drilled WEEK window is empty")
			}
			first, last := child.Window[0], child.Window[len(child.Window)-1]
			if first.Start < selectedMonth.Start || last.End > selectedMonth.End {
				t.Errorf("WEEK window after drilling into month %+v spans [%s, %s) — outside the selected month's own [%s, %s) bounds", selectedMonth, first.Start, last.End, selectedMonth.Start, selectedMonth.End)
			}
		})
	}
}

// ===========================================================================
// X6 (P2) — the STATE cursor must clamp when the display filter shrinks
// rows (metric change), so Enter always acts on the row the user actually
// sees highlighted. Traced precisely: applyCostsSelect reads
// cur.Cursor.Row directly against the CURRENT liveCostGrid — if Cursor.Row
// is stale from BEFORE a metric cycle shrank the visible row count (no move
// action re-clamped it), rowKey resolves to "" and, since the store already
// has records for the new shape, applyCostsSelect's own guard
// (records present -> no-op) fires instead of drilling the clamped row.
//
// UPDATED reconciliation (architecture.md gained Seam 8, CostsViewModel,
// after this note was first written under Seam 3/4 alone): the cursor
// clamp this test exercises is NOT a reducer-only concern after all — Seam
// 8's BuildViewModel is the seam that now clamps Cursor against the
// display-filtered Rows/VisibleCols in one place, and callers (including
// whatever runs before Select sees a row) read the clamped Cursor FROM the
// ViewModel. Pinned at the typed seam in costs_screen_test.go
// (TestCostsScreen_BuildViewModel_CursorClamp_AlwaysValidIndex). This
// controller-level test stays as the full-stack acceptance pin.
// ===========================================================================

func TestCostsCodex_X6_CursorBeyondFilteredEnd_EnterDrillsClampedRow_NeverNoOps(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	// Invoice: two non-zero rows (EC2, RDS) — cursor moves to row 1 (RDS).
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			fullMetricRecord(newestCol, "Amazon EC2", 100),
			fullMetricRecord(newestCol, "Amazon RDS", 80),
		}},
		Requests: 1,
	})
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 2 || vs.Body.Costs.CursorRow != 1 {
		t.Fatalf("precondition: expected 2 rows, cursor on row 1, got %d rows cursor=%d", len(vs.Body.Costs.Rows), vs.Body.Costs.CursorRow)
	}

	// Unblended: only EC2 has a value under this metric — RDS's row
	// disappears (zero/no-data), shrinking the grid to 1 row. Cursor.Row
	// stays 1 (stale) — no move action touches it.
	_, tasks = c.Apply(app.Action{Kind: app.ActionCostMetric})
	unblendedPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: metric cycle did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    unblendedPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{{Period: newestCol, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricUnblended: {Value: 90, Unit: "USD"}}}}},
		Requests: 1,
	})
	vs = c.Snapshot()
	if len(vs.Body.Costs.Rows) != 1 {
		t.Fatalf("precondition: expected the Unblended grid to shrink to 1 row (EC2 only), got %d", len(vs.Body.Costs.Rows))
	}

	stackBefore := len(c.GetCostsDrillStack())
	c.Apply(app.Action{Kind: app.ActionSelect})
	stackAfter := len(c.GetCostsDrillStack())
	if stackAfter <= stackBefore {
		t.Error("Enter with a stale cursor beyond the filtered row count no-op'd instead of drilling the clamped (last visible) row — records exist for this shape, so this must never be a no-op")
	}
}

// ===========================================================================
// X7 — Enter must never pin Equals[dim]=[""]. Traced precisely:
// applyCostsSelect's own AwaitedIdentity no-op gate deliberately excludes a
// just-pushed fresh child frame (cs.AwaitedIdentity is cleared right after
// the push, "always immediately drillable further, blind") — a second,
// fast Enter on that still-loading frame finds rowKey=="" (grid empty,
// nothing fetched yet) and, since the store has NO records anywhere for
// this brand-new shape either, the "records present -> no-op" guard does
// not fire — so it proceeds to drill, pinning Filter.Equals[dim]=[""].
//
// RECONCILED (architecture.md Seam 3/4): this pin's mechanism is the
// "loading -> WaitForRows" outcome, pinned at the typed seam in
// costs_screen_test.go (TestCostsScreen_Select_LoadingShape_
// AlwaysWaitForRows) — the new seam removes the old "blind chain"
// exclusion entirely (state.Loading gates Select unconditionally), closing
// this bug class outright rather than special-casing the empty-value
// symptom. This controller-level test stays as the full-stack acceptance
// pin.
// ===========================================================================

func TestCostsCodex_X7_FastEnterEnter_ThroughLoadingLevel_NeverPinsEmptyValue(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	// ActionCostPivot resets Cursor.Col to 0 (oldest) as a side effect —
	// use the resource-drill-allowed service name (Codex X1's own gate) and
	// re-position to the newest column so a fast SERVICE -> USAGE_TYPE ->
	// (fast Enter) chain actually reaches the RESOURCE_ID push this test
	// targets, instead of being refused earlier by an unrelated gate
	// (stale window or disallowed service).
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, awsclient.CostExplorerServiceNameEC2, 100)}},
		Requests: 1,
	})
	codexMoveCursorToNewestColumn(c)

	// Enter 1: SERVICE -> USAGE_TYPE (pushes a fresh, still-loading frame).
	c.Apply(app.Action{Kind: app.ActionSelect})
	if len(c.GetCostsDrillStack()) != 2 {
		t.Fatal("precondition: Enter 1 did not push a USAGE_TYPE frame")
	}
	codexMoveCursorToNewestColumn(c)

	// Enter 2: fast, before the USAGE_TYPE fetch has landed — the fresh
	// frame has zero rows and zero cached records anywhere for its shape.
	c.Apply(app.Action{Kind: app.ActionSelect})

	// The new seam gates Select on Loading unconditionally (screen.
	// WaitForRows) — the fast second Enter, on a still-loading USAGE_TYPE
	// frame, must now be a strict no-op: the stack must NOT advance to a
	// 3rd frame at all, closing the empty-value bug class outright rather
	// than special-casing its symptom.
	stack := c.GetCostsDrillStack()
	if len(stack) != 2 {
		t.Errorf("a fast Enter-Enter through a still-loading fresh drill level advanced the stack to depth %d, want 2 (a strict no-op — Loading gates Select unconditionally now)", len(stack))
	}
}

// ===========================================================================
// X8 — mixed currencies: with two Amount.Units in the same window, the
// TOTAL row must not render a bare numeric sum across incompatible units.
// Traced precisely: costs.BuildGrid sets Currency="" when len(unitSet)!=1,
// but Totals is still buildCellValues(totalsCells) — a plain numeric sum of
// every cell's .sum regardless of unit — with no suppression or note.
//
// RECONCILED (architecture.md Seam 5): the domain-level half of this pin
// (a USD+EUR sum cannot leave the domain layer as a bare number) is pinned
// at the typed seam in costs_screen_test.go
// (TestCostsScreen_SumCells_MixedUnits_NoTotalValueConsumed). This
// controller/render-level test stays as the acceptance pin for the
// RENDERED note itself.
// ===========================================================================

func TestCostsCodex_X8_MixedCurrencies_TotalNotBareNumber(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			{Period: newestCol, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}},
			{Period: newestCol, Keys: []string{"Amazon RDS"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 80, Unit: "EUR"}}},
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if vs.Body.Costs.Currency != "" {
		t.Fatalf("precondition: expected mixed-unit Currency=\"\", got %q", vs.Body.Costs.Currency)
	}
	// fmtCostAmount's own contract (costs_body.go): comma-grouped thousands,
	// exactly one decimal place, no currency symbol — what a bare numeric
	// sum of 100 (USD) + 80 (EUR) would render as if units were ignored.
	const naiveSum = "180.0"
	got := vs.Body.Costs.Totals[len(vs.Body.Costs.Totals)-1].Amount
	if got == naiveSum {
		t.Errorf("TOTAL over mixed currencies (USD+EUR) renders a bare numeric sum %q — must be suppressed with an explanatory note instead (spec edge case: mixed currencies)", got)
	}
}

// ===========================================================================
// X9 — LoadStore.Recovered()==true must surface a user-visible flash when
// the costs screen initializes. Traced precisely: costs.Store.Recovered()
// exists (store.go) but has zero references anywhere in
// internal/app/costs_state.go — a corrupt on-disk cache is silently
// discarded with no notification.
//
// RECONCILED (architecture.md Seam 6): the mechanism this pin originally
// needed (Recovered() becoming part of a typed InitOutcome result the
// caller cannot forget to check) was itself cut by a later ponytail-review
// pass — InitOutcome was one-field bool ceremony with no logic of its own,
// so callers read Store.Recovered() directly now. The real Recovered()
// behavior pin lives over a REAL Store in costs_store_test.go
// (TestStore_CorruptYAML_RenamedToBakFreshStoreNoPanic/
// TestStore_AlienVersion_RenamedToBakFreshStoreNoPanic). This
// controller-level test stays as the acceptance pin that the flash
// actually renders.
// ===========================================================================

func TestCostsCodex_X9_RecoveredStore_SurfacesFlashOnInit(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "codex-x9"

	path := costs.CachePath(profile)
	if path == "" {
		t.Fatal("precondition: costs.CachePath returned empty — cache.Root() not configured under A9S_CONFIG_FOLDER")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("not: valid: yaml: [structure"), 0o600); err != nil {
		t.Fatalf("seeding a corrupt cache file: %v", err)
	}

	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow)

	vs := c.Snapshot()
	if !strings.Contains(strings.ToLower(vs.Header.Flash.Text), "recover") {
		t.Errorf("costs screen initialized over a corrupt on-disk cache (Store.Recovered()==true) but Header.Flash.Text is %q — recovery must be flashed, never a silent data loss", vs.Header.Flash.Text)
	}
}

// ===========================================================================
// X10 — a by-ID resource-drill fetch that finds nothing (instance in
// another region/account) must return the user to the costs screen with an
// honest region-caveat footer note, never an empty stranded resource list.
// Traced precisely: KindFetchByIDDetail's executor case (executor.go)
// returns messages.Flash{IsError:true} on a zero-result fetch — Flash is
// never handled by the placeholder ScreenResourceList (AutoOpenSingle)
// applyCostsSelect pushed, and autoOpenSingleDetail only ever fires on a
// matching ResourcesLoaded, so nothing ever pops the placeholder back.
//
// RECONCILED (architecture.md Seam 7): per the dispatch, X10 stays
// entirely at the controller level (the not-found pop-back + footer
// caveat is an app-level concern, not the locator's own construction).
// The LOCATOR construction half used to also pin region/account populated
// from pinned filter dims at the typed seam
// (costs_screen_test.go:TestCostsScreen_ResourceLocator_
// PopulatesRegionAndAccountFromPinnedFilter) — a later ponytail-review
// pass cut ResourceLocator.Region/AccountID entirely (written-never-read),
// so that test is deleted; only Type/ID construction remains pinned there
// (TestCostsScreen_Select_ResourceLeaf_CatalogMapped_OpenResource). This
// test (X10) is not otherwise reconciled.
// ===========================================================================

func TestCostsCodex_X10_ResourceJump_NotFound_ReturnsToCostsWithHonestNote(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "codex-x10"
	s.Region = "us-east-1"
	// The demo EC2 fake is backed by the same fixture data as every other
	// demo test — a bogus ID naturally produces zero results, exactly like
	// a real DescribeInstances call for an instance in another region/account.
	s.Clients = demo.NewServiceClients()
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow)

	if resource.GetFetchByIDs("ec2") == nil {
		t.Fatal("precondition: no FetchByIDs registered for ec2")
	}
	byIDTask := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchByIDDetail, Scope: "ec2"},
		Payload: runtime.FetchByIDDetailPayload{TargetType: "ec2", ID: "i-doesnotexist00000"},
	}
	// Simulate applyCostsSelect's own placeholder push (the RESOURCE_ID ->
	// detail seam) directly via the exported intents surface — including the
	// RelatedIDSet stamp the real push applies (costs_state.go's
	// screen.OpenResource case), so the typed ByIDFetchFailed match against
	// the pending exact target lands the same way it does in production.
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenResourceList,
		Context: runtime.ScreenContext{ResourceType: "ec2"},
	}})
	c.EnsureListState()
	c.PatchListRelatedIDSet([]string{"i-doesnotexist00000"})

	event, err := core.ExecuteTask(context.Background(), byIDTask)
	if err != nil {
		t.Fatalf("ExecuteTask(KindFetchByIDDetail, not-found): %v", err)
	}
	c.Handle(event)

	vs := c.Snapshot()
	if vs.Body.Kind == app.BodyKindList && (vs.Body.List == nil || len(vs.Body.List.Rows) == 0) {
		t.Errorf("a not-found by-ID resource jump left the user stranded on an empty resource list (Body.Kind=%q) — must return to the costs screen with an honest footer note naming the region/account caveat", vs.Body.Kind)
	}
}

// ===========================================================================
// X11 — a legitimately empty finer-grain drill (a cell that HAD a monthly
// amount, but the finer window's fetch returns zero records because the
// charge is billed monthly, e.g. NoRegion support-fee-style items) must
// explain itself, not render a silent zero grid. Traced precisely:
// costsFooterNote (costs_body.go) returns "" the moment
// cursorRow>=len(grid.Rows) — no special-case distinguishes "genuinely
// nothing to show at this granularity" from any other empty state.
//
// RECONCILED (architecture.md Seam 8, CostsViewModel): the honesty-note
// mechanism this test needs is now sourced from ViewModel.Note (one
// source for renderers/Enter alike), pinned at the typed seam in
// costs_screen_test.go (TestCostsScreen_BuildViewModel_Note_
// EmptyFinerGrainHonesty). That seam-level pin only covers the pure
// "empty grid -> non-empty Note" contract (BuildViewModel's signature
// carries no drill-depth input); this controller-level test stays as the
// full-stack acceptance pin for the additional "not at root" gating the
// current adapter applies.
//
// RECONCILED AGAIN (costs_noregion_test.go's N3, the live-verified NoRegion
// defect): a zero finer-grain result for a NON-zero parent cell — exactly
// this test's own scenario — no longer stops at a note. It now re-plans
// ONE coarser (parent-granularity) re-fetch, and once that lands the data
// itself renders, with the note explaining WHY the columns are coarser
// than the drilled level rather than explaining an empty grid. The "note
// alone, no re-fetch" contract this test used to pin now belongs only to a
// genuinely ZERO-parent drill (costs_noregion_test.go's
// TestCostsNoRegion_N3_ZeroParentCell_NoFallback) — this test drives the
// fallback through to completion instead.
// ===========================================================================

func TestCostsCodex_X11_EmptyFinerGrainDrill_ExplainsInsteadOfSilentZeroGrid(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Support (Business)", 100)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "Support (Business)") {
		t.Fatal("precondition: no SERVICE row for the monthly-only support charge")
	}

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE (week granularity)
	usagePayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling into the monthly-only service emitted no fetch task")
	}
	// The finer-grain fetch succeeds but genuinely returns zero records —
	// this charge is only billed monthly. The parent (SERVICE) cell was
	// non-zero (100), so the N3 fallback must now re-plan a coarser
	// (parent-granularity) re-fetch here, rather than leaving a silent
	// empty grid behind a note alone.
	_, fallbackTasks := c.Handle(messages.CostsLoaded{Query: usagePayload.Query, Grid: costs.GridResult{Fetched: true}, Requests: 1})

	monthPayload, found := findFetchCostsTask(fallbackTasks)
	if !found {
		t.Fatal("a zero finer-grain result for the non-zero Support (Business) cell did not re-plan a coarser fetch (N3)")
	}
	c.Handle(messages.CostsLoaded{
		Query:    monthPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Support (Business)", 100)}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("a legitimately empty finer-grain drill (monthly-only charge) must render the coarser-granularity data once the fallback re-fetch lands, not stay a silent empty grid")
	}
	if vs.Body.Costs.FooterNote == "" {
		t.Error("a legitimately empty finer-grain drill (monthly-only charge, zero weekly records) renders a silent empty grid with no FooterNote explaining why — user cannot tell 'no data' from 'billed monthly, not visible at this granularity'")
	}
}
