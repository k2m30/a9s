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
// a week column.
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

// ExpireOpenPeriod receives the display window's week-length periods, but
// the store keys native day-length periods for week granularity
// (APIGranularity()=="DAILY").

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

// CE's GetCostAndUsageWithResources requires the pinned SERVICE to be
// exactly "Amazon Elastic Compute Cloud - Compute".

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

	ec2 := costs.DrillLevel{
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon Elastic Compute Cloud - Compute"}}},
		Window: withinWindow,
	}
	if allow, reason := costs.ResourceDrillAllowed(ec2, now); !allow {
		t.Errorf("control failed: ResourceDrillAllowed refused the EC2 drill (reason=%q) — the exact-EC2 gate must not reject the one service CE actually supports", reason)
	}
}

// For open/estimated periods the inclusive data-through date caps at
// cs.Now's date; closed periods use the bucket's exclusive end minus one.

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

// app.js's keyMap is checked as source text: a missing action-kind literal
// fails, a literal that posts the wrong kind or arg does not.

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

	// app.js's "R" key posts {kind:"refresh"}; handleActionRefresh routes the
	// costs screen (c.topCostsState() != nil) into forceRefreshCostsLocked().
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if _, found := findFetchCostsTask(refreshTasks); !found {
		t.Error("ActionRefresh (what web's \"R\" key posts today) emitted no KindFetchCosts task for the warm costs shape — want a force-refresh (handleActionRefresh routes costs screens through forceRefreshCostsLocked)")
	}

	directTasks := c.ForceRefreshCosts()
	if _, found := findFetchCostsTask(directTasks); !found {
		t.Error("control failed: ForceRefreshCosts() itself emitted no KindFetchCosts task for the warm shape — sanity check for this test's own setup, not the finding under test")
	}
}

func TestCostsRound7_Item4B_GenericRefreshAction_NonCostsScreen_KeepsOldBehavior(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-0abc", Name: "web-01"}},
	})

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if _, found := findFetchCostsTask(refreshTasks); found {
		t.Error("ActionRefresh on a non-costs (ec2) list screen emitted a KindFetchCosts task — the costs special-case in handleActionRefresh must not fire outside a costs screen")
	}
}

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
	// BuildGrid's raw desc-by-abs-total order, since 0.004 > 0.
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

	// buildCostsBreadcrumb names the drilled dimension value and strips the
	// "Amazon " vendor prefix off a SERVICE segment
	// (stripCostsServiceVendorPrefix).
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
