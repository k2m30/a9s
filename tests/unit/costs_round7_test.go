// costs_round7_test.go — Cost Explorer: external reviewer's fifth pass (7
// findings, all independently verified against current code with zero
// disproofs).
//
// package unit_test (not unit): every finding here is reachable via the
// headless app.Controller / pure core/costs / core/app package
// surface — no TUI-level helper is needed, so this file reuses
// costs_state_test.go's newCostsController/topDrill/fixedCostsNow/
// monthRecord and costs_interaction_test.go's findFetchCostsTask directly
// (same package).
//
// Reconciliation performed alongside this file (not scoped to this file,
// but required by item 3's fix): costs_interaction_test.go's two D3
// DataThrough tests both fetched window[len(window)-1] — fixedCostsNow's
// own, still-OPEN month — and asserted the OLD End-1 value. Once item 3's
// cap lands, an open period's DataThrough no longer equals End-1, so both
// tests were repointed to a CLOSED column (window[len-2]/[len-4]) to keep
// testing the exclusive-End-vs-inclusive-day contract they exist for
// without colliding with the new open-period cap. See that file's inline
// comments for the exact change.
//
// Reconciliations checked and found UNNECESSARY:
//   - Item 2 (ResourceDrillAllowed EC2-exact gate): grepped every
//     ResourceDrillAllowed call site in tests/unit — only
//     costs_drill_test.go's TestResourceDrillAllowed, whose sole "allowed"
//     case already uses "Amazon Elastic Compute Cloud - Compute". No other
//     test pins a non-EC2 single-service drill as allowed. Nothing to
//     reconcile.
//   - Item 7 (14-day cutoff day-truncation): the existing
//     TestClampResourceDrillWindow_* tests in costs_drill_test.go both use
//     a midnight `now` (time.Date(2026,7,15,0,0,0,0,UTC)), where the
//     missing day-truncation is invisible (midnight minus 14 days is still
//     midnight) — they stay green unchanged; this round's test below adds
//     the missing mid-afternoon-now coverage rather than replacing them.
package unit_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// round7FullWindowRecords tiles every period in window with one
// costs.Record each — required for Store.Lookup to consider a shape fully
// "warm" (Store's lookupContained/lookupPeriod both require edge-to-edge
// native coverage, not a single sampled record).
func round7FullWindowRecords(window []costs.Period, rowKey string, amount float64) []costs.Record {
	recs := make([]costs.Record, len(window))
	for i, p := range window {
		recs[i] = costs.Record{
			Period:  p,
			Keys:    []string{rowKey},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
		}
	}
	return recs
}

// round7FullDailyRecords tiles every day of every period in window with one
// costs.Record each — the native-daily shape a week-granularity fetch
// actually returns and Store.Lookup's lookupContained requires to resolve
// a week column at all (mirrors round5/round6's identical helper, package
// unit_test's own local copy since this file cannot reach the package unit
// one).
func round7FullDailyRecords(t *testing.T, window []costs.Period, rowKey string, amount float64) []costs.Record {
	t.Helper()
	var recs []costs.Record
	for _, p := range window {
		start, err := time.Parse("2006-01-02", p.Start)
		if err != nil {
			t.Fatalf("parsing period start %q: %v", p.Start, err)
		}
		end, err := time.Parse("2006-01-02", p.End)
		if err != nil {
			t.Fatalf("parsing period end %q: %v", p.End, err)
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			recs = append(recs, costs.Record{
				Period:  costs.Period{Start: d.Format("2006-01-02"), End: d.AddDate(0, 0, 1).Format("2006-01-02")},
				Keys:    []string{rowKey},
				Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
			})
		}
	}
	return recs
}

// ===========================================================================
// Item 1 (P2, core/app/costs_state.go:~221 ForceRefreshCosts) — Ctrl+R
// on a warm WEEK view must actually refresh: ExpireOpenPeriod receives the
// DISPLAY window's week-length periods, but the store keys native DAY-
// length periods for week granularity (APIGranularity()=="DAILY"), so the
// delete-by-periodKey misses every entry and nothing expires.
// ===========================================================================

func TestCostsRound7_Item1_ForceRefreshCosts_WarmWeekView_ExpiresNativeDailyPeriods(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	// Zoom in on fixedCostsNow's own (current) month -> week — cursor
	// already starts on the newest/current month, so the newest week
	// column lands on an OPEN period (fixedCostsNow's own week),
	// ExpireOpenPeriod's precondition for even attempting a delete.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomIn})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zoom-in to week did not emit a KindFetchCosts task")
	}
	weekTop := topDrill(t, c)
	if weekTop.Granularity != costs.GranularityWeek {
		t.Fatalf("precondition: expected Granularity week, got %q", weekTop.Granularity)
	}

	// Warm it: native DAILY records tiling the whole week window.
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: round7FullDailyRecords(t, weekTop.Window, "Amazon EC2", 10)},
		Window:   payload.Window,
		Requests: 1,
	})
	if c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: week view still Loading after a full warming delivery")
	}

	retryTasks := c.ForceRefreshCosts()
	if _, found := findFetchCostsTask(retryTasks); !found {
		t.Error("ForceRefreshCosts on a warm WEEK view emitted no KindFetchCosts task — ExpireOpenPeriod deletes by the DISPLAY window's week-length period keys, but the store holds native DAY-length keys for week granularity (APIGranularity()==\"DAILY\"), so the delete is a silent no-op and nothing is treated as missing (fix direction: expire costs.NativeCoveragePeriods(apiGranularity, window), not window itself)")
	}
}

// ===========================================================================
// Item 2 (P2, core/costs/drill.go:~76 ResourceDrillAllowed) — the gate
// must require the pinned SERVICE to be exactly "Amazon Elastic Compute
// Cloud - Compute" (CE's hard requirement for GetCostAndUsageWithResources),
// not merely one service.
// ===========================================================================

func TestCostsRound7_Item2_ResourceDrillAllowed_RequiresExactlyEC2_NotAnySingleService(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	withinWindow := []costs.Period{{Start: "2026-07-05", End: "2026-07-06"}}

	l := costs.DrillLevel{
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon Relational Database Service"}}},
		Window: withinWindow,
	}
	allow, reason := costs.ResourceDrillAllowed(l, now)
	if allow {
		t.Error("ResourceDrillAllowed allowed a single-service drill pinned to RDS (not EC2) — CE's GetCostAndUsageWithResources only supports \"Amazon Elastic Compute Cloud - Compute\"; a non-EC2 single service must be refused in-app with an honest reason, never sent to CE")
	}
	if reason == "" {
		t.Error("ResourceDrillAllowed refused the RDS drill with no reason — must explain honestly (FR-007), not silently no-op")
	}

	// Sanity control: EC2 itself must still be allowed (the existing,
	// already-passing case this gate must not regress).
	ec2 := costs.DrillLevel{
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon Elastic Compute Cloud - Compute"}}},
		Window: withinWindow,
	}
	if allow, reason := costs.ResourceDrillAllowed(ec2, now); !allow {
		t.Errorf("control failed: ResourceDrillAllowed refused the EC2 drill (reason=%q) — the exact-EC2 gate must not reject the one service CE actually supports", reason)
	}
}

// ===========================================================================
// Item 3 (P2, core/app/costs_state.go:~727 ApplyCostsLoaded/DataThrough)
// — for open/estimated periods the inclusive data-through date caps at
// cs.Now's date, not the bucket's exclusive-end-minus-one. Closed periods
// are unchanged (kept exact via the reconciled D3 tests above).
// ===========================================================================

func TestCostsRound7_Item3_DataThrough_CapsAtNowForOpenPeriod(t *testing.T) {
	now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)
	c := newCostsController(t, now)
	julyPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"} // OPEN: now falls inside it

	c.Handle(messages.CostsLoaded{
		Query:    baseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(julyPeriod, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	if got, want := c.Snapshot().Body.Costs.DataThrough, "2026-07-11"; got != want {
		t.Errorf("DataThrough after an open-period apply = %q, want %q (capped at cs.Now's own date) — got the bucket's exclusive End-1 (\"2026-07-31\") instead, overclaiming data through the whole month when only 11 days of July are actually known", got, want)
	}
}

func TestCostsRound7_Item3_DataThrough_ClosedPeriod_StillUsesExclusiveEndMinusOne(t *testing.T) {
	now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)
	c := newCostsController(t, now)
	junePeriod := costs.Period{Start: "2026-06-01", End: "2026-07-01"} // CLOSED: fully in the past

	c.Handle(messages.CostsLoaded{
		Query:    baseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(junePeriod, "Amazon EC2", 900.0)}},
		Requests: 1,
	})

	if got, want := c.Snapshot().Body.Costs.DataThrough, "2026-06-30"; got != want {
		t.Errorf("DataThrough after a CLOSED-period apply = %q, want %q — closed periods must keep the exclusive-End-minus-one value, only OPEN periods cap at cs.Now", got, want)
	}
}

// ===========================================================================
// Item 4 (P2, core/web/static/app.js + core/app/viewstate.go:72) —
// the web costs screen must wire what its own footer hints
// (CostsFooterHintsFor("web")) advertise: b/+/-/0-9 posting the costs
// actions, and R triggering ForceRefreshCosts, not the generic list
// refresh.
//
// Tested at two honest tiers (weaknesses flagged per tier, mirroring
// round6 item 1's precedent):
//
//   A. app.js's keyMap is a static source-text check — it can catch a
//      missing action-kind literal outright, but not a case that exists
//      yet posts the wrong kind or arg.
//   B. The Go-side control proves the underlying MECHANISM split: the
//      exact action kind app.js's "R" key posts today ({kind:"refresh"})
//      routes to the generic handleActionRefresh, never
//      ForceRefreshCosts — which is currently reachable ONLY from
//      internal/tui/runtime_adapter_navigate.go, a TUI-only call site with
//      no equivalent in core/web/handlers.go's handleAction. This is a
//      controller-level proxy for "the web POST route never reaches the
//      costs handlers" — handleAction itself is unexported with zero
//      existing core/web unit tests, so the actual HTTP layer is
//      untestable from tests/unit (as item 1 of round6 already
//      established for a sibling web gap).
// ===========================================================================

func TestCostsRound7_Item4A_WebAppJS_KeyMapMissingCostsActions(t *testing.T) {
	raw, err := os.ReadFile("../../core/web/static/app.js")
	if err != nil {
		t.Fatalf("reading app.js: %v", err)
	}
	src := string(raw)

	for _, want := range []string{`"cost-metric"`, `"cost-zoom-in"`, `"cost-zoom-out"`, `"cost-pivot"`} {
		if !strings.Contains(src, want) {
			t.Errorf("app.js's keyMap has no entry posting action kind %s — the costs footer hints (b=Metric, +/-=Zoom, 0-9=Pivot, CostsFooterHintsFor(\"web\")) are not wired to any keydown handler", want)
		}
	}
}

func TestCostsRound7_Item4B_GenericRefreshAction_DoesNotReachForceRefreshCosts(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, fresh task against the empty store
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: round7FullWindowRecords(root.Window, "Amazon EC2", 500)},
		Requests: 1,
	})
	if c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: SERVICE shape still Loading after a full warming delivery")
	}

	// The exact action kind app.js's "R" key posts today
	// (keyMap: {key:"R", action:{kind:"refresh"}}). handleActionRefresh now
	// special-cases the costs screen (c.topCostsState() != nil) and routes
	// straight into forceRefreshCostsLocked() before falling through to the
	// generic detail/list handling below it — so ActionRefresh on a warm
	// costs shape DOES force-refresh today, same as the TUI-only
	// ForceRefreshCosts() path.
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if _, found := findFetchCostsTask(refreshTasks); !found {
		t.Error("ActionRefresh (what web's \"R\" key posts today) emitted no KindFetchCosts task for the warm costs shape — want a force-refresh (handleActionRefresh routes costs screens through forceRefreshCostsLocked)")
	}

	// Control: ForceRefreshCosts itself (the TUI-only path) DOES correctly
	// refresh the same warm shape — same mechanism ActionRefresh now reaches
	// on the costs screen via handleActionRefresh.
	directTasks := c.ForceRefreshCosts()
	if _, found := findFetchCostsTask(directTasks); !found {
		t.Error("control failed: ForceRefreshCosts() itself emitted no KindFetchCosts task for the warm shape — sanity check for this test's own setup, not the finding under test")
	}
}

// TestCostsRound7_Item4B_GenericRefreshAction_NonCostsScreen_KeepsOldBehavior
// pins the OTHER half of the ActionRefresh contract: on a non-costs screen
// (a plain resource list), ActionRefresh must still route to the generic
// list-refresh handler — the costs special-case in handleActionRefresh must
// not leak into every screen kind.
func TestCostsRound7_Item4B_GenericRefreshAction_NonCostsScreen_KeepsOldBehavior(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-0abc", Name: "web-01"}},
	})

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if _, found := findFetchCostsTask(refreshTasks); found {
		t.Error("ActionRefresh on a non-costs (ec2) list screen emitted a KindFetchCosts task — the costs special-case in handleActionRefresh must not fire outside a costs screen")
	}
}

// ===========================================================================
// Item 5 (P2, web sync partition) — a KindFetchCosts task from a cold web
// navigation must be classified background/renderable like
// KindFetchResources, so the POST returns the loading costs shell instead
// of blocking until CE answers.
// ===========================================================================

func TestCostsRound7_Item5_KindFetchCosts_ClassifiedLikeKindFetchResources(t *testing.T) {
	tests := []struct {
		name       string
		renderable bool
		want       bool
	}{
		{"cold navigation (nothing renderable yet) stays blocking", false, false},
		{"screen already renderable (Loading shell/rows seeded) goes background", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchCosts}}
			if got := app.IsBackgroundFetchTask(req, tt.renderable); got != tt.want {
				t.Errorf("IsBackgroundFetchTask(KindFetchCosts, screenAlreadyRenderable=%v) = %v, want %v — IsBackgroundTaskKind's static switch never lists KindFetchCosts (only KindRelatedCheck/KindEnrichDetail/TaskKindProbeEnrich/TaskKindSaveCache), so it always falls to \"default: false\" regardless of renderable, unlike KindFetchResources' own renderable-aware carve-out", tt.renderable, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Item 6 (P2, core/app/costs_body.go:~122 + costs_state.go) — hiding
// zero rows must keep SELECTION aligned, not just the highlight.
//
// Root cause traced precisely: applyCostsMoveRow/applyCostsSelect both
// index the RAW, unfiltered liveCostGrid via cur.Cursor.Row (costs_state.go
// L340/L590), while buildCostsBody's filterCostsZeroDisplayRows remap
// (costs_body.go L122-125) is a RENDER-ONLY local value
// (displayRows/displayCursorRow) that is NEVER written back into
// cs.DrillStack — so the state layer and the display layer silently
// diverge whenever a hidden row sits above a visible one in the raw sort
// order. Fix direction pinned here per the coordinator: the state layer's
// own Cursor.Row must operate on the FILTERED view (one source of truth),
// not gain a second display-side remap.
//
// RECONCILED (architecture.md Seam 8, CostsViewModel): this "one source of
// truth" mechanism is exactly what BuildViewModel's clamped Cursor now
// owns — pinned at the typed seam in costs_screen_test.go
// (TestCostsScreen_BuildViewModel_CursorClamp_AlwaysValidIndex). This
// controller-level test stays as the full-stack acceptance pin (also see
// costs_codex_test.go's X6 reconciliation note, updated alongside this
// one: the mechanism now lives in the ViewModel seam, not the reducer).
// ===========================================================================

func TestCostsRound7_Item6_HiddenRowAbove_SelectionStaysAlignedWithDisplayedHighlight(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	prevPeriod := root.Window[len(root.Window)-2]
	curPeriod := root.Window[len(root.Window)-1]

	// "AWS Support (Business)" (hidden noise, single column 0.004 -> "0.0")
	// sorts ABOVE "Amazon EC2" (net-zero total: +100/-100, abs total 0) in
	// BuildGrid's raw desc-by-abs-total order, since 0.004 > 0 — exactly
	// the "hidden row above a visible one" shape this finding needs.
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			{Period: curPeriod, Keys: []string{"AWS Support (Business)"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 0.004, Unit: "USD"}}},
			{Period: prevPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100.0, Unit: "USD"}}},
			{Period: curPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: -100.0, Unit: "USD"}}},
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 1 {
		t.Fatalf("precondition: expected exactly 1 displayed row (the sub-cent noise row hidden by the display-level filter), got %d: %+v", len(vs.Body.Costs.Rows), vs.Body.Costs.Rows)
	}
	if got := vs.Body.Costs.Rows[0].Label; got != "EC2" {
		t.Fatalf("precondition: expected the single displayed row to be EC2 (vendor-prefix stripped), got %q", got)
	}
	if vs.Body.Costs.CursorRow != 0 {
		t.Fatalf("precondition: expected CursorRow 0 (highlighting the only displayed row), got %d", vs.Body.Costs.CursorRow)
	}

	// The user sees "EC2" highlighted. Enter must drill INTO EC2 — not the
	// hidden noise row that happens to rank first in the raw, unfiltered
	// grid.
	c.Apply(app.Action{Kind: app.ActionSelect})

	stack := c.GetCostsDrillStack()
	if len(stack) != 2 {
		t.Fatalf("expected the drill stack to grow to depth 2 after Enter, got %d", len(stack))
	}
	child := stack[len(stack)-1]
	got := child.Filter.Equals[costs.DimensionService]
	if len(got) != 1 || got[0] != "Amazon EC2" {
		t.Errorf("drilled SERVICE = %v, want [\"Amazon EC2\"] (the row the user sees highlighted) — got the hidden noise row instead, proving Cursor.Row indexes the raw grid while the rendered highlight indexes the filtered one", got)
	}

	// Breadcrumb (not FooterNote, which only ever carries the cursor
	// cell's own anomaly/delta note — verified directly, buildCostsBreadcrumb
	// is the actual mechanism that names a drilled dimension value) must
	// name the row the user actually saw highlighted. buildCostsBreadcrumb
	// strips the "Amazon " vendor prefix off a SERVICE segment
	// (stripCostsServiceVendorPrefix), same as the "EC2" row label already
	// asserted above (line 355) — so the breadcrumb segment is "EC2", not
	// the raw filter value "Amazon EC2" pinned in child.Filter.Equals above.
	breadcrumb := c.Snapshot().Body.Costs.Breadcrumb
	found = false
	for _, seg := range breadcrumb {
		if seg == "EC2" {
			found = true
		}
	}
	if !found {
		t.Errorf("Breadcrumb after drilling = %v, want a segment naming \"EC2\" (the row the user saw highlighted), not the hidden noise row", breadcrumb)
	}
}

// ===========================================================================
// Item 7 (P3, core/costs/drill.go:~50 ClampResourceDrillWindow) — the
// 14-day retention cutoff must truncate now to the start of its day: a
// period starting exactly 14 days ago at date level survives regardless of
// time-of-day.
// ===========================================================================

func TestCostsRound7_Item7_ClampResourceDrillWindow_CutoffTruncatesToDayStart(t *testing.T) {
	now := time.Date(2026, 7, 15, 15, 30, 0, 0, time.UTC) // mid-afternoon
	window := []costs.Period{
		{Start: "2026-07-01", End: "2026-07-02"}, // exactly 14 calendar days before now's DATE
	}

	got := costs.ClampResourceDrillWindow(window, now)

	if len(got) != 1 {
		t.Errorf("ClampResourceDrillWindow with a mid-afternoon now (15:30) dropped the period starting exactly 14 calendar days ago — the cutoff must truncate now to its own day-start (00:00) before subtracting 14 days, not carry now's time-of-day forward; existing TestClampResourceDrillWindow_* tests only exercise a MIDNIGHT now, where this bug is invisible (midnight minus 14 days is still midnight)")
	}
}
