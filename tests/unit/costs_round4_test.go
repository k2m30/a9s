// costs_round4_test.go — Cost Explorer: user feedback round 4 (live-usage
// bugs on a second account) plus the evergreen-demo prerequisite for the
// upcoming smoke walk.
//
// package unit_test (not unit): every item here is reachable via the
// headless app.Controller / pure internal/costs package — no TUI-level
// helper is needed, so this file reuses costs_state_test.go's
// newCostsController/topDrill/fixedCostsNow/monthRecord directly (same
// package).
//
// *** Scoring corrections applied (see the confirmed-score dispatch):
// *** - Item 3 tests week->day spill ONLY. The month->week half of the
// ***   original claim ("+ on April produced a window starting Mar 30")
// ***   does NOT reproduce: weekWindowsInMonth already clips correctly
// ***   (independently traced by hand against April 1, 2026 — a Wednesday
// ***   — through mondayOnOrBefore + the s/e clip logic). Only
// ***   dayWindowsInWeek (week->day) has zero month-boundary clipping.
// *** - Item 2 pins the OBSERVABLE live symptom end-to-end (seeded daily
// ***   data must survive a month->week zoom as non-zero service rows),
// ***   not a specific internal mechanism — see the test's own doc comment
// ***   for the empirical red/green finding.
package unit_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// round4FindFetchCostsTask returns the FetchCostsPayload of the first
// KindFetchCosts TaskRequest in tasks, or ok=false when none is present.
func round4FindFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			if p, ok := tr.Payload.(runtime.FetchCostsPayload); ok {
				return p, true
			}
		}
	}
	return runtime.FetchCostsPayload{}, false
}

// ===========================================================================
// 1 (P1) — CE range clamp: no query built anywhere may carry Range.End
// later than the first day of the month after "now"'s month (CE rejects
// "end date past the beginning of next month" — a live-verified crash).
// ===========================================================================

func TestCostsRound4_CERangeClamp_BuildWindow_NeverPastFirstOfNextMonth(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC) // near month-end
	cutoff := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	for _, gran := range []costs.Granularity{costs.GranularityYear, costs.GranularityMonth, costs.GranularityWeek, costs.GranularityDay} {
		window := costs.BuildWindow(gran, now)
		if len(window) == 0 {
			t.Fatalf("%s: BuildWindow returned an empty window", gran)
		}
		lastEnd, err := time.Parse("2006-01-02", window[len(window)-1].End)
		if err != nil {
			t.Fatalf("%s: parsing last window End %q: %v", gran, window[len(window)-1].End, err)
		}
		if lastEnd.After(cutoff) {
			t.Errorf("%s: BuildWindow's last period End %q is after %s — CE rejects a Range.End past the first day of the month after now (\"end date past the beginning of next month\")",
				gran, window[len(window)-1].End, cutoff.Format("2006-01-02"))
		}
	}
}

func TestCostsRound4_CERangeClamp_ZoomOutToYear_ExecutorQueryClamped(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	c := newCostsController(t, now)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year
	top := topDrill(t, c)
	if top.Granularity != costs.GranularityYear {
		t.Fatalf("precondition: expected Granularity year after zoom-out, got %q", top.Granularity)
	}

	payload, found := round4FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("zoom-out to year did not emit a KindFetchCosts task against the empty store")
	}
	end, err := time.Parse("2006-01-02", payload.Query.Range.End)
	if err != nil {
		t.Fatalf("parsing executor Query.Range.End %q: %v", payload.Query.Range.End, err)
	}
	if end.After(cutoff) {
		t.Errorf("zoom-out-to-year executor query Range.End = %q, want <= %s (CE rejects \"end date past the beginning of next month\" — this is the live crash)",
			payload.Query.Range.End, cutoff.Format("2006-01-02"))
	}
}

// ===========================================================================
// 2 (P1) — week bucket alignment: zooming from a month cell into weeks over
// seeded DAILY records must produce a grid whose service rows carry data
// (live bug: weeks rendered an empty zero grid while days had data).
//
// This pins the OBSERVABLE symptom end-to-end, not a specific internal
// mechanism (root cause unconfirmed statically — see the round-4 score
// rationale). Empirically run against current code with the cursor on the
// default (newest, FR-002) column: [RESULT REPORTED BELOW].
// ===========================================================================

func TestCostsRound4_WeekZoom_ServiceRowsCarryData_NotEmptyGrid(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	targetPeriod := root.Window[root.Cursor.Col] // default cursor: newest column (FR-002)

	start, err := time.Parse("2006-01-02", targetPeriod.Start)
	if err != nil {
		t.Fatalf("parsing target period start %q: %v", targetPeriod.Start, err)
	}
	end, err := time.Parse("2006-01-02", targetPeriod.End)
	if err != nil {
		t.Fatalf("parsing target period end %q: %v", targetPeriod.End, err)
	}
	var recs []costs.Record
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dayEnd := d.AddDate(0, 0, 1)
		recs = append(recs, costs.Record{
			Period:  costs.Period{Start: d.Format("2006-01-02"), End: dayEnd.Format("2006-01-02")},
			Keys:    []string{"Amazon EC2"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 10, Unit: "USD"}},
		})
	}
	c.Handle(messages.CostsLoaded{
		Query: costs.Query{
			Granularity: costs.GranularityWeek.APIGranularity(), // "DAILY"
			GroupBy:     []costs.Dimension{costs.DimensionService},
		},
		Grid:     costs.GridResult{Fetched: true, Records: recs},
		Requests: 1,
	})

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week

	weekTop := topDrill(t, c)
	if weekTop.Granularity != costs.GranularityWeek {
		t.Fatalf("precondition: expected Granularity week after zoom-in, got %q", weekTop.Granularity)
	}

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 {
		t.Fatal("week-granularity body has ZERO service rows after zooming into a month with a full month of seeded daily data — live bug: weeks rendered an empty grid while days had data")
	}
	nonZeroFound := false
	for _, row := range vs.Body.Costs.Rows {
		for _, cell := range row.Cells {
			if cell.Amount != "" && cell.Amount != "0.0" {
				nonZeroFound = true
			}
		}
	}
	if !nonZeroFound {
		t.Errorf("week-granularity body's service rows are all zero after zooming into a month with a full month of seeded daily data (rows: %+v)", vs.Body.Costs.Rows)
	}
}

// TestCostsRound4_WeekZoom_ServiceRowsCarryData_NotEmptyGrid_NonCurrentMonth
// is the fallback reproduction: the newest-column (open/current month) case
// above is GREEN today — reported per the round-4 score dispatch's explicit
// instruction ("if it happens to be green today, report that loudly"). This
// variant puts the cursor on an OLDER, already-closed month instead.
func TestCostsRound4_WeekZoom_ServiceRowsCarryData_NotEmptyGrid_NonCurrentMonth(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	targetIdx := len(root.Window) - 4 // a closed month, a few months back
	if targetIdx < 0 {
		t.Fatalf("precondition: window too short (%d columns)", len(root.Window))
	}
	for range len(root.Window) - 1 - targetIdx {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if got := topDrill(t, c).Cursor.Col; got != targetIdx {
		t.Fatalf("precondition: cursor did not land on column %d, got %d", targetIdx, got)
	}
	targetPeriod := topDrill(t, c).Window[targetIdx]

	start, err := time.Parse("2006-01-02", targetPeriod.Start)
	if err != nil {
		t.Fatalf("parsing target period start %q: %v", targetPeriod.Start, err)
	}
	end, err := time.Parse("2006-01-02", targetPeriod.End)
	if err != nil {
		t.Fatalf("parsing target period end %q: %v", targetPeriod.End, err)
	}
	var recs []costs.Record
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dayEnd := d.AddDate(0, 0, 1)
		recs = append(recs, costs.Record{
			Period:  costs.Period{Start: d.Format("2006-01-02"), End: dayEnd.Format("2006-01-02")},
			Keys:    []string{"Amazon EC2"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 10, Unit: "USD"}},
		})
	}
	c.Handle(messages.CostsLoaded{
		Query: costs.Query{
			Granularity: costs.GranularityWeek.APIGranularity(), // "DAILY"
			GroupBy:     []costs.Dimension{costs.DimensionService},
		},
		Grid:     costs.GridResult{Fetched: true, Records: recs},
		Requests: 1,
	})

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week, anchored on the closed target month

	weekTop := topDrill(t, c)
	if weekTop.Granularity != costs.GranularityWeek {
		t.Fatalf("precondition: expected Granularity week after zoom-in, got %q", weekTop.Granularity)
	}

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 {
		t.Fatal("week-granularity body has ZERO service rows after zooming into a CLOSED month with a full month of seeded daily data — live bug: weeks rendered an empty grid while days had data")
	}
	nonZeroFound := false
	for _, row := range vs.Body.Costs.Rows {
		for _, cell := range row.Cells {
			if cell.Amount != "" && cell.Amount != "0.0" {
				nonZeroFound = true
			}
		}
	}
	if !nonZeroFound {
		t.Errorf("week-granularity body's service rows are all zero after zooming into a CLOSED month with a full month of seeded daily data (rows: %+v)", vs.Body.Costs.Rows)
	}
}

// ===========================================================================
// 3 (P2) — day-zoom anchoring: week->day must land entirely inside the
// enclosing month (dayWindowsInWeek has no month-boundary clipping, unlike
// weekWindowsInMonth — the confirmed half of the live bug).
// ===========================================================================

func TestCostsRound4_DayZoom_WeekToDay_NeverSpillsBeforeMonthStart(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	// April 2026: April 1 is a Wednesday, so its enclosing ISO week starts
	// on Monday March 30 — the exact spill condition dayWindowsInWeek must
	// guard against.
	aprilIdx := -1
	for i, p := range root.Window {
		if strings.HasPrefix(p.Start, "2026-04") {
			aprilIdx = i
			break
		}
	}
	if aprilIdx == -1 {
		t.Fatalf("precondition: default window does not include April 2026, window=%+v", root.Window)
	}
	for range len(root.Window) - 1 - aprilIdx {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if got := topDrill(t, c).Cursor.Col; got != aprilIdx {
		t.Fatalf("precondition: cursor did not land on April's column, got %d want %d", got, aprilIdx)
	}

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week, anchored on April
	weekTop := topDrill(t, c)
	if weekTop.Granularity != costs.GranularityWeek {
		t.Fatalf("precondition: expected week granularity, got %q", weekTop.Granularity)
	}
	if weekTop.Window[0].Start != "2026-04-01" {
		t.Fatalf("precondition: week window[0].Start got %q want %q (month->week must already clip correctly — this half of the original claim was disproved during scoring)", weekTop.Window[0].Start, "2026-04-01")
	}

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // week -> day, anchored on week[0] (cursor resets to 0)
	dayTop := topDrill(t, c)
	if dayTop.Granularity != costs.GranularityDay {
		t.Fatalf("precondition: expected day granularity, got %q", dayTop.Granularity)
	}

	for _, p := range dayTop.Window {
		start, err := time.Parse("2006-01-02", p.Start)
		if err != nil {
			t.Fatalf("parsing day window period start %q: %v", p.Start, err)
		}
		if start.Month() != time.April || start.Year() != 2026 {
			t.Errorf("day window contains period %+v outside April 2026 — week->day zoom must not spill before the month's start", p)
		}
	}
}

// ===========================================================================
// 4 (P2) — scroll-to-load (new behavior): ActionScrollLeft at the oldest
// loaded column must extend the range (KindFetchCosts + Loading), not
// clamp dead, when older history exists within CE's 13-month horizon.
// ActionScrollRight at the newest period stays a clamp.
// ===========================================================================

func TestCostsRound4_ScrollLeftAtOldestColumn_ExtendsRange_NotDeadClamp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	windowLen := len(root.Window)

	for range windowLen - 1 {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if got := topDrill(t, c).Cursor.Col; got != 0 {
		t.Fatalf("precondition: cursor did not land on column 0, got %d", got)
	}

	// One more scroll-left, past the oldest loaded column — CE's 13-month
	// resource horizon leaves headroom beyond the default 12-month window,
	// so this must extend the range, not clamp dead.
	_, tasks := c.Apply(app.Action{Kind: app.ActionScrollLeft})

	if _, found := round4FindFetchCostsTask(tasks); !found {
		t.Error("ActionScrollLeft at the oldest loaded column emitted no KindFetchCosts task — must extend the range (older history may exist within CE's 13-month horizon), not clamp dead")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Error("after ActionScrollLeft extends the range, Loading must be true while the fetch is outstanding")
	}
}

func TestCostsRound4_ScrollRightAtNewestColumn_StaysClamp_NoFetch(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	windowLen := len(root.Window)
	if got := root.Cursor.Col; got != windowLen-1 {
		t.Fatalf("precondition: cursor did not start at the newest column, got %d want %d", got, windowLen-1)
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionScrollRight})

	if _, found := round4FindFetchCostsTask(tasks); found {
		t.Error("ActionScrollRight at the newest period emitted a KindFetchCosts task — nothing newer exists, this must stay a dead clamp")
	}
	if got := topDrill(t, c).Cursor.Col; got != windowLen-1 {
		t.Errorf("cursor after scrolling right past the newest column: got %d want %d (clamped)", got, windowLen-1)
	}
}

func TestCostsRound4_ScrollLeftExtension_WindowGrows_WhenResultMerges(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	windowLen := len(root.Window)
	oldestStart := root.Window[0].Start

	for range windowLen - 1 {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	_, tasks := c.Apply(app.Action{Kind: app.ActionScrollLeft})

	payload, found := round4FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: the oldest-column scroll-left did not emit a KindFetchCosts task")
	}

	extendedStart, err := time.Parse("2006-01-02", oldestStart)
	if err != nil {
		t.Fatalf("parsing oldest column Start %q: %v", oldestStart, err)
	}
	extraMonthStart := extendedStart.AddDate(0, -1, 0)
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			{
				Period:  costs.Period{Start: extraMonthStart.Format("2006-01-02"), End: oldestStart},
				Keys:    []string{"Amazon EC2"},
				Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}},
			},
		}},
		Requests: 1,
	})

	newWindow := topDrill(t, c).Window
	if len(newWindow) <= windowLen {
		t.Fatalf("window did not grow after the extension's result merged: got %d columns, want more than %d", len(newWindow), windowLen)
	}
	newStart, err := time.Parse("2006-01-02", newWindow[0].Start)
	if err != nil {
		t.Fatalf("parsing new window[0].Start %q: %v", newWindow[0].Start, err)
	}
	if !newStart.Before(extendedStart) {
		t.Errorf("window[0].Start after extension got %q, want an earlier month than the original oldest column %q", newWindow[0].Start, oldestStart)
	}
}

// ===========================================================================
// 5 (P2) — error body keeps state: ErrorMsg does not blank out
// Pivot/Metric/Granularity (live bug: title rendered "Costs: by · ·").
// ===========================================================================

func TestCostsRound4_ErrorBody_KeepsPivotMetricGranularity(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Err: fmt.Errorf("AccessDenied: user is not authorized to perform ce:GetCostAndUsage"),
	})

	vs := c.Snapshot()
	if vs.Body.Costs.ErrorMsg == "" {
		t.Fatal("precondition: ErrorMsg was not set after delivering an error result")
	}
	if vs.Body.Costs.Pivot != string(costs.DimensionService) {
		t.Errorf("error body Pivot got %q want %q — the frame title must still show what's being viewed while erroring (live bug: \"Costs: by · ·\")", vs.Body.Costs.Pivot, costs.DimensionService)
	}
	if vs.Body.Costs.Metric != string(costs.MetricInvoice) {
		t.Errorf("error body Metric got %q want %q", vs.Body.Costs.Metric, costs.MetricInvoice)
	}
	if vs.Body.Costs.Granularity != string(costs.GranularityMonth) {
		t.Errorf("error body Granularity got %q want %q", vs.Body.Costs.Granularity, costs.GranularityMonth)
	}
}

// ===========================================================================
// 6 (P3) — help documents the sort rule: the Cost Explorer help section
// states that rows sort by total spend across the visible window, largest
// absolute first.
// ===========================================================================

func TestCostsRound4_HelpSection_DocumentsSortRule(t *testing.T) {
	sections := domain.HelpGroupsFor(domain.HelpFromCosts, "ctrl+z")

	var costSection *domain.HelpSection
	for i := range sections {
		if sections[i].Title == "COST EXPLORER" {
			costSection = &sections[i]
			break
		}
	}
	if costSection == nil {
		t.Fatal("no \"COST EXPLORER\" help section found")
	}

	found := false
	for _, hint := range costSection.Hints {
		text := strings.ToLower(hint.Key + " " + hint.Help)
		if strings.Contains(text, "sort") && strings.Contains(text, "total") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("COST EXPLORER help section has no line documenting the sort rule (rows sort by total spend across the visible window, largest absolute first), got hints: %+v", costSection.Hints)
	}
}
