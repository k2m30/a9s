// costs_review3_test.go — Cost Explorer external review round 3: five
// findings (R1-R5) traced against current source before writing, same
// convention as costs_review2_test.go/costs_selfreview_test.go.
//
// package unit_test: every finding here is reachable via the exported
// core/aws.FetchEC2InstancesByIDs surface, the pure core/costs.Store/
// screen.BuildViewModel surface, or the headless app.Controller surface —
// no TUI helper needed. Reuses newCostsController/topDrill/fixedCostsNow/
// fullMetricRecord/findFetchCostsTask (costs_state_test.go/
// costs_interaction_test.go) and selfReviewAPIError/strPtrSelfReview
// (costs_selfreview_test.go), all defined in this same package.
//
// R1 (P1, core/aws/ec2_by_ids.go:~68): confirmed in source — when EVERY
// requested ID is named bad, `retry` ends up empty (len 0), and the guard
// `len(bad) == 0 || len(retry) == len(requested)` does not fire (bad is
// non-empty, retry is empty and not equal to requested), so the code falls
// through and calls fetchEC2InstancesByIDsOnce with an EMPTY id slice —
// DescribeInstances with no InstanceIds is an ACCOUNT-WIDE describe.
//
// R2 (core/costs/screen/screen.go PlanFetch + core/costs/store.go
// Store.Anomalies): confirmed in source — Store.Anomalies(now) is TTL-only
// (anomalyBucket carries FetchedAt/Marks, no covered-range field), and
// PlanFetch's Anomalies half derives purely from that TTL check regardless
// of what range the CURRENT frame actually needs, even though the anomaly
// fetch itself (FetchCostAnomaliesCounted) IS range-scoped via
// GetAnomalies's DateInterval. A fresh snapshot cached under the default
// trailing-12-months range is wrongly treated as covering a materially
// wider (e.g. year-zoom) range too.
//
// R3 (core/costs/store.go:~309, MergeCoverage): confirmed in source and
// DISTINCT from the existing TestCostsReview2_R2_ZeroRecordPeriod_
// MergeCoverage_NotReportedMissing (costs_review2_test.go) coverage — that
// test only covers a period NEVER before seen (no bucket exists yet, so
// MergeCoverage's own `if _, exists := entry.Periods[pk]; exists {
// continue }` guard doesn't apply). This finding targets a bucket that
// ALREADY exists but is TTL-stale (a still-open period, fetched once,
// now expired): a refetch that authoritatively returns zero groups this
// round leaves Merge with nothing to key off (no records for that period),
// and MergeCoverage's `exists` guard then skips it too — so FetchedAt is
// NEVER advanced and Lookup reports it missing forever, looping the
// refetch. The MergeCoverage doc comment's own stated worry ("would
// wrongly promote a still-unsettled record to look... permanently
// fetched") does not actually apply here: refreshing FetchedAt alone
// (never touching Records, never forcing immutableAt) only resets the 24h
// TTL clock — it does not promote the bucket past its own settlementLag
// closure check. The anti-promotion rule (a period ABSENT from the fetched
// range must never be re-stamped) is pinned separately below and stays
// exactly as-is.
//
// R4 (core/app/costs_state.go:~159, ensureCostsState): confirmed in
// source — costs.LoadStore(profile) is called unconditionally, with no
// c.core.NoCache() check anywhere in ensureCostsState or in Handle's
// dirtyStore.Save() flush, unlike every OTHER cache in the codebase
// (core/session/session.go's own NoCache-gated EnsureCacheStore family,
// core/runtime/handlers.go, core/runtime/probes.go). Demo mode
// (--demo sets NoCache=true) silently reads and writes a real
// ~/.a9s/cache/<profile>--costs.yaml file.
//
// R5 (core/costs/grid.go BuildGrid + core/costs/screen/screen.go
// BuildViewModel): confirmed as an explicit spec violation —
// specs/021-cost-explorer/data-model.md:91 ("sorted desc by row total over
// VISIBLE window") and spec.md:83/90 ("Row sort is descending by row total
// over the visible window... documented in the help screen") both name the
// VISIBLE window explicitly. BuildGrid sorts by the FULL window total (its
// own doc: "sorts rows desc by absolute row total", no viewport
// parameter — correctly, a pure grid-level concern). BuildViewModel
// (screen.go) slices columns and drops all-zero VISIBLE rows but never
// re-sorts — Rows keeps BuildGrid's full-window order verbatim.
package unit_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/app"
	a9saws "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/costs/screen"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// ===========================================================================
// R1 — all-IDs-bad must never fall through to an unfiltered DescribeInstances.
// ===========================================================================

// costsReview3EC2AllBadAPI counts DescribeInstances calls and simulates
// AWS's real batch-level InvalidInstanceID.NotFound naming EVERY requested
// ID — the "every ID in this batch is bad" case, distinct from
// costs_selfreview_test.go's C12 (one bad, one good) scenario. A call with
// an EMPTY InstanceIds list is simulated as a real, unfiltered
// account-wide describe would behave: it returns instances that were never
// requested.
type costsReview3EC2AllBadAPI struct {
	calls      int
	bad1, bad2 string
}

func (s *costsReview3EC2AllBadAPI) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	s.calls++
	if len(in.InstanceIds) == 0 {
		return &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{
			{Instances: []ec2types.Instance{{InstanceId: strPtrSelfReview("i-accountwide00000001")}}},
		}}, nil
	}
	return nil, &selfReviewAPIError{
		Code:    "InvalidInstanceID.NotFound",
		Message: fmt.Sprintf("The instance ID '%s' does not exist. The instance ID '%s' does not exist", s.bad1, s.bad2),
	}
}

func TestCostsReview3_R1_EC2ByIDs_AllIDsBad_NeverAccountWideDescribe(t *testing.T) {
	bad1 := "i-00000000000000ba1"
	bad2 := "i-00000000000000ba2"
	stub := &costsReview3EC2AllBadAPI{bad1: bad1, bad2: bad2}

	resources, err := a9saws.FetchEC2InstancesByIDs(context.Background(), stub, []string{bad1, bad2})

	if stub.calls != 1 {
		t.Errorf("DescribeInstances called %d times, want exactly 1 — when EVERY requested ID is bad, the empty retry list must never be sent as a second, ACCOUNT-WIDE DescribeInstances call", stub.calls)
	}
	for _, r := range resources {
		if r.ID == "i-accountwide00000001" {
			t.Errorf("resources contains %q — an unrelated instance from an unfiltered, account-wide DescribeInstances call that should never have been made", r.ID)
		}
	}
	if err == nil {
		t.Error("FetchEC2InstancesByIDs with every requested ID bad returned err=nil, want a composite error naming both unrecovered IDs")
	} else if !strings.Contains(err.Error(), bad1) || !strings.Contains(err.Error(), bad2) {
		t.Errorf("error %v does not name both unrecovered IDs %q, %q", err, bad1, bad2)
	}
}

// ===========================================================================
// R2 — anomaly freshness must account for the requested range, not TTL alone.
// ===========================================================================

func TestCostsReview3_R2_AnomalyFreshness_WiderRangeStillRefetches(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE pivot, forces the first shape-miss fetch
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	// An authoritative (non-nil, possibly empty) anomaly result seeds the
	// store's ONE global anomaly slot fresh at fixedCostsNow, scoped (in
	// reality) to the default trailing-12-months month-view range.
	c.Handle(messages.CostsLoaded{
		Query:     payload.Query,
		Grid:      costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 100)}},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{},
		Requests:  2,
	})

	// Zoom out to YEAR: a materially WIDER range than the month view's
	// trailing-12-months default (BuildWindow's own 5-trailing-years
	// window) — the anomaly snapshot just cached above never covered it.
	_, tasks = c.Apply(app.Action{Kind: app.ActionCostZoomOut})
	payload, found = findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zooming to YEAR granularity (an uncached grid shape too) emitted no fetch task at all")
	}
	if payload.SkipAnomalies {
		t.Error("SkipAnomalies=true after zooming to a materially wider (year) range — the cached anomaly snapshot only ever covered the narrower month-view range; Store.Anomalies' freshness check is TTL-only and range-blind")
	}
}

// ===========================================================================
// R3 — an authoritative zero-group refetch of an expired OPEN period must
// clear the stale bucket and stamp coverage; a period absent from the
// fetched range must never be re-stamped (unchanged).
// ===========================================================================

func TestCostsReview3_R3_ExpiredOpenBucket_AuthoritativeZeroRefetch_ClearsStaleAndStampsCovered(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	store := costs.LoadStore("review3-r3a")
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	openMonth := costs.Period{Start: "2026-07-01", End: "2026-08-01"}
	fetchedAt := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	// Seed a stale open bucket: fetched once with real spend, now TTL-expired.
	store.Merge(q, []costs.Record{{
		Period:  openMonth,
		Keys:    []string{"Amazon EC2"},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 42, Unit: "USD"}},
	}}, fetchedAt)

	expiredNow := fetchedAt.Add(25 * time.Hour) // openPeriodTTL (24h) has elapsed
	if _, missing := store.Lookup(q, []costs.Period{openMonth}, expiredNow); len(missing) == 0 {
		t.Fatal("precondition: the open bucket must be TTL-expired (reported missing) before the authoritative refetch below")
	}

	// The refetch is authoritative: CE genuinely returned zero groups for
	// openMonth this round. Merge alone has nothing to key off (no records
	// for this period), so it leaves the stale bucket's old FetchedAt
	// untouched — MergeCoverage must be the one to clear/re-stamp it.
	store.Merge(q, nil, expiredNow)
	store.MergeCoverage(q, []costs.Period{openMonth}, expiredNow)

	records, missing := store.Lookup(q, []costs.Period{openMonth}, expiredNow)
	if len(missing) != 0 {
		t.Errorf("Lookup still reports the re-fetched-to-zero open period as missing (%v) — the stale pre-TTL bucket was never cleared/re-stamped, looping the refetch forever even though CE just authoritatively confirmed zero", missing)
	}
	if len(records) != 0 {
		t.Errorf("Lookup returned %d stale records for a period CE just authoritatively refetched to zero: %+v", len(records), records)
	}
}

func TestCostsReview3_R3_PeriodAbsentFromFetchedRange_NeverRestamped(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	store := costs.LoadStore("review3-r3b")
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	untouched := costs.Period{Start: "2026-06-01", End: "2026-07-01"}
	fetchedRange := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}} // does NOT include untouched

	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	store.MergeCoverage(q, fetchedRange, now)

	_, missing := store.Lookup(q, []costs.Period{untouched}, now)
	if len(missing) == 0 {
		t.Error("MergeCoverage(q, fetchedRange, now) stamped a period OUTSIDE fetchedRange as covered — a period the executor never actually requested must stay missing, or a genuinely-never-fetched gap silently reads as covered-empty forever")
	}
}

// ===========================================================================
// R4 — NoCache must make the costs store memory-only: no disk read, no
// disk write.
// ===========================================================================

func TestCostsReview3_R4_NoCache_MemoryOnlyStore_NoDiskReadOrWrite(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "review3-r4"
	seedNow := fixedCostsNow

	seedWindow := costs.BuildWindow(costs.GranularityMonth, seedNow)
	seedQuery := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	seed := costs.LoadStore(profile)
	seed.Merge(seedQuery, []costs.Record{fullMetricRecord(seedWindow[len(seedWindow)-1], "Amazon EC2", 999)}, seedNow)
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}
	cachePath := costs.CachePath(profile)
	if cachePath == "" {
		t.Fatal("precondition: costs.CachePath returned empty")
	}
	before, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("precondition: reading seeded cache file: %v", err)
	}

	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	s.NoCache = true
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(seedNow)

	// "reads none": the root frame's default SERVICE-pivot query is
	// IDENTICAL to the seeded shape/window above — if LoadStore had read
	// the seeded file, this row would already be covered and rendered with
	// no fetch at all.
	vs := c.Snapshot()
	if vs.Body.Costs != nil {
		for _, r := range vs.Body.Costs.Rows {
			if r.Label == "EC2" {
				t.Errorf("NoCache controller rendered a row (%+v) sourced from the pre-seeded on-disk cache — LoadStore must never read disk under NoCache", r)
			}
		}
	}

	// "writes none": a genuine shape-miss fetch (REGION pivot, a shape the
	// seeded file never covered) delivered successfully must not touch the
	// cache file at all.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2})
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(seedWindow[len(seedWindow)-1], "NoRegion", 111)}},
		Requests: 1,
	})

	after, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("reading cache file after NoCache delivery: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("cache file content changed after a NoCache controller's successful CostsLoaded delivery — costs caching must be memory-only under NoCache, mirroring every other on-disk cache in the codebase")
	}
}

// ===========================================================================
// R5 — visible-window row ranking: BuildViewModel must re-sort by the
// VISIBLE columns, not preserve BuildGrid's full-window order.
// ===========================================================================

func TestCostsReview3_R5_BuildViewModel_RowsReRankByVisibleWindowNotFullWindow(t *testing.T) {
	p0 := costs.Period{Start: "2026-04-01", End: "2026-05-01"}
	p1 := costs.Period{Start: "2026-05-01", End: "2026-06-01"}
	p2 := costs.Period{Start: "2026-06-01", End: "2026-07-01"}
	p3 := costs.Period{Start: "2026-07-01", End: "2026-08-01"}

	grid := costs.Grid{
		RowDim:  costs.DimensionService,
		Columns: []costs.Period{p0, p1, p2, p3},
		Rows: []costs.GridRow{
			// BuildGrid's own full-window sort (its doc: "sorts rows desc by
			// absolute row total") legitimately ranks this row first — its
			// $10000 lands entirely in an off-screen column. This is the
			// exact "BuildGrid's full-window ordering as input" the view
			// must re-sort, not preserve.
			{Key: "OffscreenHeavy", Label: "OffscreenHeavy", Cells: []costs.CellValue{
				{Amount: costs.Amount{Value: 10000, Unit: "USD"}},
				{Amount: costs.Amount{Value: 0, Unit: "USD"}},
				{Amount: costs.Amount{Value: 0, Unit: "USD"}},
				{Amount: costs.Amount{Value: 0.20, Unit: "USD"}},
			}},
			{Key: "VisibleHeavy", Label: "VisibleHeavy", Cells: []costs.CellValue{
				{Amount: costs.Amount{Value: 0, Unit: "USD"}},
				{Amount: costs.Amount{Value: 0, Unit: "USD"}},
				{Amount: costs.Amount{Value: 30, Unit: "USD"}},
				{Amount: costs.Amount{Value: 40, Unit: "USD"}},
			}},
		},
	}

	// Viewport shows only the last 2 columns (p2, p3) — OffscreenHeavy's
	// $10000 is scrolled out; VisibleHeavy's $70 is entirely on-screen.
	vm := screen.BuildViewModel(grid, costs.DimensionService, screen.CursorPos{Row: 0, Col: 2}, screen.Viewport{Cols: 2, ScrollX: 2}, 0)

	if len(vm.Rows) != 2 {
		t.Fatalf("precondition: want both rows to survive the visible-window zero-filter, got %d: %+v", len(vm.Rows), vm.Rows)
	}
	if vm.Rows[0].Key != "VisibleHeavy" {
		t.Errorf("vm.Rows[0].Key = %q, want %q — data-model.md/spec.md: row sort is descending by row total over the VISIBLE window; OffscreenHeavy's $10000 falls entirely outside the visible [p2,p3) slice while VisibleHeavy's $70 is entirely inside it, so VisibleHeavy must rank first regardless of BuildGrid's full-window ordering", vm.Rows[0].Key, "VisibleHeavy")
	}
}

// ===========================================================================
// P1 — an anomaly-only (SkipGrid) delivery must never be mistaken for CE's
// own authoritative zero-group grid result. Traced precisely:
// executor.go:517 sets `Window: p.Window` UNCONDITIONALLY (SkipGrid or not),
// and when SkipGrid is true `result` stays its zero value, so `Records:
// result.Records` is nil too — a SkipGrid delivery is byte-for-byte
// indistinguishable from a genuine "CE queried this shape and found
// nothing" result at the ApplyCostsLoaded layer. Two call sites key off
// exactly that ambiguity:
//   - costs_state.go:982 `if len(ev.Window) > 0 { cs.Store.MergeCoverage(...) }`
//     — unconditional, regardless of whether the grid was ever queried this
//     round. MergeCoverage's own authoritative-zero-clears behavior
//     (fetchedWhileOpen + !FetchedAt.Equal(now)) then wipes a WARM,
//     non-stale bucket's real Records the moment an unrelated anomaly-only
//     refresh completes for the same shape/window.
//   - costs_state.go:1012 `applyCostsGranularityFallback`'s own trigger
//     (`!matchesAwaited || len(ev.Records) != 0`) shares the identical
//     blind spot: `matchesAwaited` (costs_state.go:930-943) matches on
//     Query shape/range alone, never on SkipGrid, so the SAME masquerading
//     delivery can wrongly fire the N3 re-plan too.
// ===========================================================================

func TestCostsReview3_P1_AnomalyOnlyDelivery_NeverClearsWarmGridCoverage(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile = "review3-p1a"
	// The seed is fetched STRICTLY EARLIER than fixedCostsNow (still well
	// within the 24h open-period TTL) — MergeCoverage's own
	// `!existing.FetchedAt.Equal(now)` guard would otherwise accidentally
	// mask this exact bug if both deliveries shared one identical "now".
	seedNow := fixedCostsNow.Add(-2 * time.Hour)

	window := costs.BuildWindow(costs.GranularityMonth, fixedCostsNow)
	newestCol := window[len(window)-1]
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	seed := costs.LoadStore(profile)
	seed.Merge(q, []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 500)}, seedNow)
	seed.MergeCoverage(q, window, seedNow)
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow) // LATER "now" than the seed

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 1 || vs.Body.Costs.Rows[0].Label != "EC2" {
		t.Fatalf("precondition: warm grid loaded from disk did not render the seeded EC2 row: %+v", vs.Body.Costs.Rows)
	}

	// An anomaly-only (SkipGrid) refresh for the SAME shape/window
	// completes — exactly ensureCostsShapeFetched's own X3 dispatch once
	// the grid is already covered but the anomaly TTL has expired. Its
	// Records is nil because the grid was never re-fetched this round, NOT
	// because CE authoritatively confirmed zero.
	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: false},
		Window:    window,
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	vs = c.Snapshot()
	if len(vs.Body.Costs.Rows) != 1 || vs.Body.Costs.Rows[0].Label != "EC2" {
		t.Errorf("an anomaly-only (SkipGrid) delivery wiped the warm grid's real spend row — got %+v, want the EC2 row to survive untouched", vs.Body.Costs.Rows)
	}
}

func TestCostsReview3_P1_AnomalyOnlyDelivery_NeverFiresGranularityFallback(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	// Drill from a NON-ZERO parent cell — makes the pushed child frame
	// Fallback-eligible (N3's own precondition), exactly like
	// TestCostsNoRegion_N3_GranularityFallback_ReplansOnceAtParentGranularity.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE pivot
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 250)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "EC2") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE, week granularity, Eligible=true
	childPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling the non-zero EC2 row emitted no fetch task")
	}

	stack := c.GetCostsDrillStack()
	top := stack[len(stack)-1]
	if !top.Fallback.Eligible {
		t.Fatal("precondition: the pushed child frame must be Fallback-eligible (drilled from a non-zero parent cell)")
	}

	// An UNRELATED anomaly-only (SkipGrid) delivery happens to match the
	// freshly-pushed child frame's own query shape (e.g. that shape's
	// anomaly slot separately expiring) — Records is nil because the grid
	// was never fetched this round, NOT because CE confirmed zero.
	_, fallbackTasks := c.Handle(messages.CostsLoaded{
		Query:     childPayload.Query,
		Grid:      costs.GridResult{Fetched: false},
		Window:    childPayload.Window,
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	if _, found := findFetchCostsTask(fallbackTasks); found {
		t.Error("an anomaly-only (SkipGrid) delivery matching the eligible child frame's shape fired the N3 granularity fallback — Records=nil from a SkipGrid delivery must never be mistaken for CE's own authoritative zero-group grid result")
	}
	stack = c.GetCostsDrillStack()
	top = stack[len(stack)-1]
	if top.Granularity != costs.GranularityWeek {
		t.Errorf("frame Granularity = %q after an anomaly-only delivery, want unchanged %q — the fallback must not have re-planned", top.Granularity, costs.GranularityWeek)
	}
	if top.Fallback.Fired {
		t.Error("Fallback.Fired = true after an anomaly-only (SkipGrid) delivery — the one-shot gate must only consume itself on a REAL zero-group grid result")
	}
}

// ===========================================================================
// P3 — Esc (ActionBack) at drill depth must clear CostsState-wide Loading/
// ErrorMsg inherited from the just-popped child frame. Traced precisely:
// applyCostsBack (costs_state.go:624-630) only pops cs.DrillStack — it never
// touches cs.Loading/cs.ErrorMsg/cs.AwaitedIdentity/cs.DrillRefusedReason/
// cs.ResourceRowNote, all of which are CostsState-WIDE fields, not
// per-DrillLevel. A child frame's own in-flight/failed fetch state survives
// the pop and renders against the (unrelated, already-available) parent.
// ===========================================================================

func TestCostsReview3_P3_Back_ClearsInheritedLoading(t *testing.T) {
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
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 500)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "EC2") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE child, shape-miss: cs.Loading=true
	if _, found := findFetchCostsTask(tasks); !found {
		t.Fatal("precondition: drilling did not dispatch a fetch task")
	}
	if vs := c.Snapshot(); !vs.Body.Costs.Loading {
		t.Fatal("precondition: the freshly-drilled child frame must be Loading before its own fetch resolves")
	}

	// Esc BEFORE the child's own fetch ever resolves — pops back to the
	// parent (root) frame, whose own data is already fully available.
	c.Apply(app.Action{Kind: app.ActionBack})

	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("Back (Esc) at drill depth left CostsState.Loading=true inherited from the popped child frame — the parent frame's own data is already available, it must not render as loading")
	}
}

func TestCostsReview3_P3_Back_ClearsInheritedErrorMsg(t *testing.T) {
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
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 500)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "EC2") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect})
	childPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling did not dispatch a fetch task")
	}
	// The child frame's own fetch fails.
	c.Handle(messages.CostsLoaded{Grid: costs.GridResult{Fetched: true}, Query: childPayload.Query, Err: errors.New("throttled"), Requests: 1})
	if vs := c.Snapshot(); vs.Body.Costs.ErrorMsg == "" {
		t.Fatal("precondition: the child frame's failed fetch must set ErrorMsg")
	}

	c.Apply(app.Action{Kind: app.ActionBack})

	vs := c.Snapshot()
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("Back (Esc) at drill depth left CostsState.ErrorMsg = %q inherited from the popped child frame's failed fetch — the parent frame's own data is fine, it must not render blocked behind a stale error", vs.Body.Costs.ErrorMsg)
	}
}

// ===========================================================================
// P5 — a costs fetch dispatched before AWS finishes connecting must recover
// once ClientsReady lands, mirroring list screens' own retry idiom. Traced
// precisely: maybeRefreshIntents (handlers.go:407-419) gates entirely on
// ev.HasActiveRL (topListState() != nil) — ScreenCosts is never considered,
// so a pre-connect costs ErrorMsg ("cost explorer: no client configured for
// this session", executor.go:453) never clears and no re-fetch is ever
// dispatched, even after the real connection completes.
// ===========================================================================

func TestCostsReview3_P5_ClientsReady_RecoversPreConnectCostsError(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "review3-p5"
	s.Region = "us-east-1"
	core := runtime.New(s, nil) // nil clients — pre-connect
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow)

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: q,
		Err: errors.New("cost explorer: no client configured for this session"),
	})
	if vs := c.Snapshot(); vs.Body.Costs.ErrorMsg == "" {
		t.Fatal("precondition: the pre-connect costs fetch failure must set ErrorMsg")
	}

	// The real connection now completes. Gen:1 — ConnectGen seeds at 1
	// (session.New()); this session is never rotated.
	c.Handle(messages.ClientsReady{Clients: demo.NewServiceClients(), Region: "us-east-1", Gen: 1})

	vs := c.Snapshot()
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("costs screen still shows the pre-connect ErrorMsg %q after ClientsReady landed — nothing re-triggers a costs fetch once the connection is actually ready (list screens retry via RefreshActiveListIntent, costs does not)", vs.Body.Costs.ErrorMsg)
	}
}
