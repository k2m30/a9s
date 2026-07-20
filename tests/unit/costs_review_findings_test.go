// costs_review_findings_test.go — Cost Explorer: 7 defects found by an
// external code review of the committed Phase-1/2 costs code, each
// independently verified against current code before being scoped here.
// Contract: specs/021-cost-explorer/data-model.md (updated Filter with
// NotEquals), spec.md FR-011/FR-016/FR-017.
//
// package unit (not unit_test): finding #3 needs the full TUI Model
// (newRootSizedModel/rootApplyMsg/assertStackInSync, all package-unit-only)
// alongside the six headless-Controller/Store/aws-layer findings, and Go
// permits only one package per file — everything here lives in package unit
// with small locally-prefixed (review*) helpers mirroring costs_state_test.go/
// costs_interaction_test.go's unit_test helpers, to avoid implying they are
// the same functions across packages.
//
// New API this file assumes (none of it exists on disk yet):
//   - costs.Filter.NotEquals map[Dimension][]string (finding #5) — data-model.md
//     was just updated to add this to the Filter contract.
//   - messages.CostsLoaded.Gen domain.Gen (finding #7) — named field only;
//     the GenStamp()/GenAspect()/AcceptZeroGen() methods and the
//     messages.IsStale guard in handle.go's CostsLoaded case are the coder's
//     wiring, following the IdentityError precedent this file does not
//     itself need to reference.
//
// Everything else drives fixes into EXISTING surface: core/costs/store.go
// (#1), core/runtime/executor.go's KindFetchCosts case (#2),
// internal/tui/app_input.go's generic Esc path (#3), core/app/costs_state.go
// (#4, #6), core/aws/costs.go's buildFilterExpression (#5).
package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

var reviewNow = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

// newCostsScreenController (costs_round3_test.go, same package) is the
// shared controller builder for isolated-cache tests; use it unless the
// test needs reviewCostsControllerNoIsolation below.

// reviewCostsControllerNoIsolation is the same construction as
// newCostsScreenController, minus the A9S_CONFIG_FOLDER isolation — for tests
// (#4, #6) that must control the on-disk cache path themselves BEFORE
// construction (pre-seeding a stale cache, or reading back a persisted one).
// Kept separate: this contract (caller controls isolation) cannot collapse
// into the shared helper without losing that property.
func reviewCostsControllerNoIsolation(t *testing.T, now time.Time) *app.Controller {
	t.Helper()
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenCosts},
	})
	c.EnsureCostsState(now)
	return c
}

func reviewTopDrill(t *testing.T, c *app.Controller) costs.DrillLevel {
	t.Helper()
	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindCosts {
		t.Fatalf("expected BodyKindCosts, got %q", vs.Body.Kind)
	}
	stack := c.GetCostsDrillStack()
	if len(stack) == 0 {
		t.Fatal("GetCostsDrillStack returned an empty stack")
	}
	return stack[len(stack)-1]
}

// reviewBaseServiceQuery is the default root-frame query shape: monthly,
// grouped by SERVICE, invoice mode (no filter).
func reviewBaseServiceQuery() costs.Query {
	return costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
	}
}

// reviewFullMetricRecord builds a realistic multi-metric record for period p.
func reviewFullMetricRecord(p costs.Period, rowKey string, amount float64) costs.Record {
	return costs.Record{
		Period: p,
		Keys:   []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{
			costs.MetricInvoice:      {Value: amount, Unit: "USD"},
			costs.MetricAmortized:    {Value: amount * 1.02, Unit: "USD"},
			costs.MetricNetAmortized: {Value: amount * 0.98, Unit: "USD"},
			costs.MetricBlended:      {Value: amount * 1.01, Unit: "USD"},
		},
	}
}

// reviewFindFetchCostsTask returns the FetchCostsPayload of the first
// KindFetchCosts TaskRequest in tasks, or ok=false when none is present.
func reviewFindFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
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
// #1 (P1) — Store.Lookup must return records whose native period falls
// INSIDE a requested display bucket, not merely at an exact key match.
// ===========================================================================

func TestCostsReview_F1_StoreLookup_ReturnsRecordsWhoseNativePeriodFallsInsideRequestedBucket(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	t.Run("week bucket over cached DAILY records", func(t *testing.T) {
		store := costs.LoadStore("test-profile-daily")
		q := costs.Query{Granularity: "DAILY", GroupBy: []costs.Dimension{costs.DimensionService}}

		var recs []costs.Record
		day := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC) // Monday
		for i := range 7 {
			start := day.AddDate(0, 0, i)
			recs = append(recs, reviewFullMetricRecord(
				costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 0, 1).Format("2006-01-02")},
				"Amazon EC2", 100.0+float64(i),
			))
		}
		store.Merge(q, recs, reviewNow)

		weekWindow := []costs.Period{{Start: "2026-01-05", End: "2026-01-12"}}
		got, missing := store.Lookup(q, weekWindow, reviewNow)

		if len(got) != 7 {
			t.Errorf("Lookup with a week bucket over 7 cached daily records: got %d records want 7 (records=%+v)", len(got), got)
		}
		if len(missing) != 0 {
			t.Errorf("Lookup with a week bucket fully covered by cached daily records reported missing periods: %+v", missing)
		}
	})

	t.Run("year bucket over cached MONTHLY records", func(t *testing.T) {
		store := costs.LoadStore("test-profile-monthly")
		q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.DimensionService}}

		var recs []costs.Record
		for m := 1; m <= 12; m++ {
			start := time.Date(2026, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
			recs = append(recs, reviewFullMetricRecord(
				costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")},
				"Amazon EC2", 1000.0+float64(m),
			))
		}
		store.Merge(q, recs, reviewNow)

		yearWindow := []costs.Period{{Start: "2026-01-01", End: "2027-01-01"}}
		got, missing := store.Lookup(q, yearWindow, reviewNow)

		if len(got) != 12 {
			t.Errorf("Lookup with a year bucket over 12 cached monthly records: got %d records want 12", len(got))
		}
		if len(missing) != 0 {
			t.Errorf("Lookup with a year bucket fully covered by cached monthly records reported missing periods: %+v", missing)
		}
	})
}

// ===========================================================================
// #2 (P1) — a RESOURCE_ID-grouped KindFetchCosts task must call
// GetCostAndUsageWithResources, not GetCostAndUsage.
// ===========================================================================

type reviewRecordingCostsAPI struct {
	calledPlain         bool
	calledWithResources bool
}

func (f *reviewRecordingCostsAPI) GetCostAndUsage(
	_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageOutput, error) {
	f.calledPlain = true
	return &costexplorer.GetCostAndUsageOutput{}, nil
}

func (f *reviewRecordingCostsAPI) GetCostAndUsageWithResources(
	_ context.Context, _ *costexplorer.GetCostAndUsageWithResourcesInput, _ ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
	f.calledWithResources = true
	return &costexplorer.GetCostAndUsageWithResourcesOutput{}, nil
}

func (f *reviewRecordingCostsAPI) GetAnomalies(
	_ context.Context, _ *costexplorer.GetAnomaliesInput, _ ...func(*costexplorer.Options),
) (*costexplorer.GetAnomaliesOutput, error) {
	return &costexplorer.GetAnomaliesOutput{}, nil
}

func (f *reviewRecordingCostsAPI) GetDimensionValues(
	_ context.Context, _ *costexplorer.GetDimensionValuesInput, _ ...func(*costexplorer.Options),
) (*costexplorer.GetDimensionValuesOutput, error) {
	return &costexplorer.GetDimensionValuesOutput{}, nil
}

func TestCostsReview_F2_KindFetchCosts_RoutesByGroupBy(t *testing.T) {
	cases := []struct {
		name              string
		groupBy           costs.Dimension
		wantWithResources bool
	}{
		{"SERVICE-grouped uses GetCostAndUsage", costs.DimensionService, false},
		{"RESOURCE_ID-grouped uses GetCostAndUsageWithResources", costs.DimensionResourceID, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &reviewRecordingCostsAPI{}
			s := session.New()
			s.Profile = "test-profile"
			s.Region = "us-east-1"
			s.Clients = &awsclient.ServiceClients{CostExplorer: fake}
			core := runtime.New(s, nil)

			q := costs.Query{
				Granularity: "DAILY",
				GroupBy:     []costs.Dimension{tc.groupBy},
				Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon EC2"}}},
			}
			ev, err := core.ExecuteTask(context.Background(), runtime.TaskRequest{
				Key:     runtime.TaskKey{Kind: runtime.KindFetchCosts, Scope: "costs"},
				Payload: runtime.FetchCostsPayload{Query: q},
			})
			if err != nil {
				t.Fatalf("ExecuteTask: %v", err)
			}
			loaded, ok := ev.(messages.CostsLoaded)
			if !ok {
				t.Fatalf("expected messages.CostsLoaded, got %T", ev)
			}
			if loaded.Err != nil {
				t.Fatalf("CostsLoaded.Err: %v", loaded.Err)
			}

			if fake.calledWithResources != tc.wantWithResources {
				t.Errorf("GroupBy=%v: calledWithResources got %v want %v", tc.groupBy, fake.calledWithResources, tc.wantWithResources)
			}
			if fake.calledPlain == tc.wantWithResources {
				t.Errorf("GroupBy=%v: expected exactly one of GetCostAndUsage/GetCostAndUsageWithResources to be called, got plain=%v withResources=%v",
					tc.groupBy, fake.calledPlain, fake.calledWithResources)
			}
		})
	}
}

// ===========================================================================
// #3 (P1) — Esc on a drilled costs screen must pop only the drill frame,
// keeping the TUI rendererState stack in sync with the controller.
// ===========================================================================

func TestCostsReview_F3_EscAtDrillDepth_PopsOnlyDrillFrame_TUIStackStaysSynced(t *testing.T) {
	tui.Version = "1.0.2"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	// Enter is a no-op while its shape is still awaited (drilling into an
	// invisible row is never correct) — a real session always has the
	// CostsLoaded delivery in between. The TUI navigates via time.Now()
	// (no injected clock at this layer), so the seeded record's period
	// anchors to the real current month — CacheKey matching (what
	// ApplyCostsLoaded actually checks) never depends on Range/window per
	// data-model.md, only the delivered record's own Period needs to land
	// inside the real window for a visible row to appear at the cursor.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(period, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	// Drill one level (Enter -> ActionSelect). Drilling mutates CostsState's
	// own DrillStack in place — it pushes neither a new controller screen
	// nor a new rendererState, so StackInSync must still hold.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	assertStackInSync(t, m, "after drilling one level (still ScreenCosts)")

	// Esc at drill depth 2 must pop ONLY the drill frame — ActionBack leaves
	// the controller's ScreenCosts on top, so the TUI's costs rendererState
	// must also stay on top. The generic Esc path (popRSWithCtrlPop)
	// unconditionally pops the TUI stack regardless of what ActionBack
	// actually did on the controller side, desyncing the two here.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	assertStackInSync(t, m, "after Esc at drill depth 2 (must still be on ScreenCosts)")
	if plain := stripANSI(rootViewContent(m)); strings.Contains(plain, "resource-types") {
		t.Errorf("Esc at drill depth 2 must pop only the drill frame, not the whole costs screen — got the main menu:\n%s", plain)
	}

	// Esc again — now at the root frame (depth 1) — pops the whole screen,
	// landing back on the main menu, both stacks staying in sync.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	assertStackInSync(t, m, "after Esc at drill depth 1 (root) — leaves the costs screen")
	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "resource-types") {
		t.Errorf("Esc at the root frame should return to the main menu, got:\n%s", plain)
	}
}

// ===========================================================================
// #4 (P2) — a stale (TTL-expired) open period must trigger a refetch and
// never render as current data.
// ===========================================================================

func TestCostsReview_F4_StaleOpenPeriod_TriggersRefetch_NeverRenderedAsCurrent(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	window := costs.BuildWindow(costs.GranularityMonth, reviewNow)
	staleFetchTime := reviewNow.Add(-25 * time.Hour) // > openPeriodTTL (24h)

	seed := costs.LoadStore("test-profile")
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	seed.Merge(reviewBaseServiceQuery(), recs, staleFetchTime)
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	c := reviewCostsControllerNoIsolation(t, reviewNow) // loads the seeded (stale) disk cache

	// Re-evaluate the CURRENT (unchanged) shape — the stale open month must
	// be detected and refetched, never silently rendered as current data.
	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already the current pivot

	if !vs.Body.Costs.Loading {
		t.Error("a stale (>24h) open period must set CostsBody.Loading — a shape check that only asks 'is the CacheKey present' (ignoring per-period freshness) would wrongly treat this as fresh")
	}
	if _, found := reviewFindFetchCostsTask(tasks); !found {
		t.Fatal("a stale open period did not emit a KindFetchCosts TaskRequest")
	}
}

// TestCostsReview_F4_VisibleWindow_MissingClosedPeriod_TriggersRefetch_NeverRenderedAsSilentZero
// closes the follow-up nuance an external reviewer found in the landed #4
// fix: ensureCostsShapeFetched only inspects Store.Lookup's missing periods
// for OPEN-period staleness, so a visible window whose OPEN period is fresh
// but which is missing an entirely-uncached CLOSED period (a corrupt/
// partial cache, or a future window-widening regression once Phase 3 drill
// re-windowing lands) silently renders that column instead of refetching —
// a latent FR-017 hole.
func TestCostsReview_F4_VisibleWindow_MissingClosedPeriod_TriggersRefetch_NeverRenderedAsSilentZero(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	window := costs.BuildWindow(costs.GranularityMonth, reviewNow)
	if len(window) < 3 {
		t.Fatalf("precondition: default window has %d columns, need at least 3", len(window))
	}
	gapIdx := len(window) / 2 // a closed month, deliberately left out of the seed

	seed := costs.LoadStore("test-profile")
	var recs []costs.Record
	for i, p := range window {
		if i == gapIdx {
			continue
		}
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	// Fresh fetch time: the OPEN period is NOT stale — isolates this test
	// from the stale-open-period case above, so it exercises only the
	// missing-CLOSED-period gap.
	seed.Merge(reviewBaseServiceQuery(), recs, reviewNow)
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	c := reviewCostsControllerNoIsolation(t, reviewNow)

	// Re-evaluate the CURRENT (unchanged) shape — the missing closed month
	// must be detected and refetched, never silently rendered as a zero
	// column.
	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already the current pivot

	if !vs.Body.Costs.Loading {
		t.Error("a visible window with a missing CLOSED period must set CostsBody.Loading — a shape check that only asks 'is the open period fresh' wrongly treats this as complete")
	}
	payload, found := reviewFindFetchCostsTask(tasks)
	if !found {
		t.Fatalf("a visible window with a missing closed period (month %s) did not emit a KindFetchCosts TaskRequest", window[gapIdx].Start)
	}
	if payload.Query.Range.Start != window[0].Start || payload.Query.Range.End != window[len(window)-1].End {
		t.Errorf("fetch task Query.Range: got %+v, want it to cover the full visible window [%s, %s) so the gap month is refetched",
			payload.Query.Range, window[0].Start, window[len(window)-1].End)
	}
}

// TestCostsReview_F4_VisibleWindow_FullyCovered_NoFetchTask is the control:
// a window where every period — closed and the fresh open one — is already
// cached must render instantly, with no fetch. Must not regress into
// "always refetch", which would defeat caching entirely.
func TestCostsReview_F4_VisibleWindow_FullyCovered_NoFetchTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	window := costs.BuildWindow(costs.GranularityMonth, reviewNow)

	seed := costs.LoadStore("test-profile")
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	seed.Merge(reviewBaseServiceQuery(), recs, reviewNow) // fresh — every period, closed and open, covered
	// Also seed a fresh anomaly slot (screen.PlanFetch's Grid/Anomalies
	// freshness derive independently now) so "zero fetch tasks" holds
	// unconditionally, not just for the grid half. covered is the window
	// under test.
	seed.PutAnomalies(nil, reviewNow, costs.Period{Start: window[0].Start, End: window[len(window)-1].End})
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	c := reviewCostsControllerNoIsolation(t, reviewNow)

	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already the current pivot

	if vs.Body.Costs.Loading {
		t.Error("a fully-covered visible window (every closed period + a fresh open period) must render instantly, got Loading=true")
	}
	if _, found := reviewFindFetchCostsTask(tasks); found {
		t.Error("a fully-covered visible window emitted an unnecessary KindFetchCosts TaskRequest")
	}
}

// ===========================================================================
// #5 (P2) — the unblended metric's query filter must EXCLUDE RECORD_TYPE in
// {Tax, Credit, Refund} via Filter.NotEquals, and buildFilterExpression must
// map NotEquals to a CE Not-expression.
// ===========================================================================

func TestCostsReview_F5_UnblendedMetric_FilterExcludesTaxCreditRefund_ViaNotEquals(t *testing.T) {
	c := newCostsScreenController(t, reviewNow)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostMetric}) // invoice -> unblended

	payload, found := reviewFindFetchCostsTask(tasks)
	if !found {
		t.Fatal("switching to unblended did not emit a KindFetchCosts TaskRequest")
	}
	if len(payload.Query.Filter.Equals) != 0 {
		t.Errorf("unblended query Filter.Equals: got %v want empty — unblended EXCLUDES record types, it does not include a positive subset", payload.Query.Filter.Equals)
	}
	got := payload.Query.Filter.NotEquals[costs.DimensionRecordType]
	want := map[string]bool{"Tax": true, "Credit": true, "Refund": true}
	if len(got) != len(want) {
		t.Fatalf("unblended query Filter.NotEquals[RECORD_TYPE]: got %v want exactly {Tax, Credit, Refund}", got)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected NotEquals[RECORD_TYPE] value %q, want one of Tax/Credit/Refund", v)
		}
	}
}

type reviewFilterCapturingCostsClient struct {
	lastFilter *cetypes.Expression
}

func (f *reviewFilterCapturingCostsClient) GetCostAndUsage(
	_ context.Context, params *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageOutput, error) {
	f.lastFilter = params.Filter
	return &costexplorer.GetCostAndUsageOutput{}, nil
}

func TestCostsReview_F5_BuildFilterExpression_MapsNotEqualsToCENotExpression(t *testing.T) {
	fake := &reviewFilterCapturingCostsClient{}
	q := costs.Query{
		Granularity: "MONTHLY",
		GroupBy:     []costs.Dimension{costs.DimensionService},
		Filter: costs.Filter{
			NotEquals: map[costs.Dimension][]string{
				costs.DimensionRecordType: {"Tax", "Credit", "Refund"},
			},
		},
	}

	if _, err := awsclient.FetchCostAndUsage(context.Background(), fake, q); err != nil {
		t.Fatalf("FetchCostAndUsage: %v", err)
	}

	if fake.lastFilter == nil {
		t.Fatal("no Filter was sent on GetCostAndUsageInput")
	}
	if fake.lastFilter.Not == nil {
		t.Fatal("Filter.NotEquals must map to a CE Not-expression, got Filter.Not == nil")
	}
	dims := fake.lastFilter.Not.Dimensions
	if dims == nil || string(dims.Key) != string(costs.DimensionRecordType) {
		t.Fatalf("Filter.Not.Dimensions: got %+v, want Key=RECORD_TYPE", dims)
	}
	gotVals := make(map[string]bool, len(dims.Values))
	for _, v := range dims.Values {
		gotVals[v] = true
	}
	for _, want := range []string{"Tax", "Credit", "Refund"} {
		if !gotVals[want] {
			t.Errorf("Filter.Not.Dimensions.Values missing %q, got %v", want, dims.Values)
		}
	}
}

// ===========================================================================
// #6 (P2) — a successful CostsLoaded merge must persist to the on-disk cost
// cache, not just live in the in-memory Store for the screen's lifetime.
// ===========================================================================

func TestCostsReview_F6_SuccessfulMerge_PersistsToDiskCache(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())

	c := reviewCostsControllerNoIsolation(t, reviewNow)
	window := reviewTopDrill(t, c).Window
	rec := reviewFullMetricRecord(window[len(window)-1], "Amazon EC2", 1200.0)

	c.Handle(messages.CostsLoaded{Query: reviewBaseServiceQuery(), Grid: costs.GridResult{Fetched: true, Records: []costs.Record{rec}}, Requests: 1})

	// A fresh Store loaded from the SAME on-disk path must see the merged
	// record — ApplyCostsLoaded's merge must be persisted, not just held in
	// memory for the lifetime of this screen.
	reloaded := costs.LoadStore("test-profile")
	got, _ := reloaded.Lookup(reviewBaseServiceQuery(), window, reviewNow)
	found := false
	for _, r := range got {
		if len(r.Keys) == 1 && r.Keys[0] == "Amazon EC2" {
			found = true
		}
	}
	if !found {
		t.Error("after a successful CostsLoaded merge, a fresh Store loaded from the same on-disk path does not see the record — the merge was never persisted to disk")
	}
}

// ===========================================================================
// #7 (P2) — CostsLoaded must carry a staleness stamp; a stale result
// arriving after a profile switch must be dropped.
// ===========================================================================

func TestCostsReview_F7_StaleCostsLoaded_DroppedAfterProfileSwitch(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(reviewNow)

	// First Rotate: ConnectGen becomes non-zero (it starts at 0 — a Gen=0
	// stamp is never stale per the AcceptZeroGen convention, so a genuine
	// staleness test needs a non-zero captured gen). Mirrors
	// generation_stamping_fetch_test.go's established two-Rotate pattern for
	// ConnectGen-stamped events (IdentityError/IdentityLoaded).
	s.Rotate()
	staleGen := s.ConnectGen
	if staleGen == 0 {
		t.Fatal("precondition: staleGen must be non-zero to test the IsStale guard")
	}
	// Second Rotate simulates the profile/region switch that supersedes the
	// in-flight costs fetch.
	s.Rotate()

	window := reviewTopDrill(t, c).Window
	before := c.Snapshot().Body.Costs

	c.Handle(messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(window[0], "Amazon EC2", 1200.0)}},
		Requests: 3,
		Gen:      staleGen,
	})

	after := c.Snapshot().Body.Costs
	if after.APICalls != before.APICalls {
		t.Errorf("APICalls after a stale CostsLoaded (stamp gen %d, current gen %d): got %d want %d (unchanged) — a stale result from a superseded profile/region must be dropped",
			staleGen, s.ConnectGen, after.APICalls, before.APICalls)
	}
	if len(after.Rows) != len(before.Rows) {
		t.Error("CostsBody.Rows changed after a stale CostsLoaded was delivered — the merge must never apply for a superseded session")
	}
}

// ===========================================================================
// Live-verified gap (tmux proof) — Enter on a resource row in the costs
// screen emits KindFetchByIDDetail (round8 item 3's pin, green at the
// Controller.Apply level) but the SCREEN NEVER CHANGES at the TUI layer.
// Traced precisely: internal/tui/app_costs.go's handleCostsKeyMsg dispatches
// EVERY task returned by ActionSelect through the generic m.executeTaskCmd —
// for KindFetchByIDDetail that fetches (ExecuteTask returns ResourcesLoaded)
// but never navigates. internal/tui/runtime_adapter_related.go:342-350
// special-cases the SAME task kind for the related panel: it routes to
// m.fetchByIDDetail(targetType, id) instead of executeTaskCmd, and THAT
// function (internal/tui/fetch_adapter.go:78) is the one that actually
// produces messages.Navigate{Target: TargetDetail, ...} on success. The
// costs key router has no equivalent translation, so the resource-drill
// Enter silently stays on the costs screen even though the fetch itself
// succeeds.
//
// Seam reused: TestApp_008_RelatedNavigate_SingleID_CacheMiss_AutoOpensDetail
// (related_navigate_count_spec008_test.go) pins fetchByIDDetail's effect via
// the rendered view content after the message round-trip — StripANSI(view)
// containing "detail --" plus the target's own identifying string, and NOT
// containing any remaining costs-screen marker. This test follows the same
// idiom, driven through the costs screen's own TUI key path (mirrors F3's
// rootApplyMsg/rootSpecialKey drive) instead of the related panel's.
// ===========================================================================

// newCostsDemoModel mirrors related_navigate_count_spec008_test.go's
// newRelatedDemoModel — a demo-clients-backed root model, needed here (unlike
// F3's plain newRootSizedModel) because this test's fix path calls the REAL
// registered EC2 FetchByIDs helper against m.core.Clients(), which must
// resolve to demo fixture data, not an unconnected/nil client.
func newCostsDemoModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

func TestCostsReview_ResourceRowEnter_TUI_NavigatesToEC2Detail_NotStuckOnCostsScreen(t *testing.T) {
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001" // core/demo/fixtures/ec2.go's "web-prod-01"
	m := newCostsDemoModel(t)

	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})

	// SERVICE root: one EC2 row, no metric switch/pivot needed. The record's
	// Period is anchored to the CURRENT calendar month (never a fixed date)
	// so it always lands in the root frame's default rightmost column —
	// costsOpenAtNewestDrillLevel — regardless of which month the suite runs
	// in, both for the drill's cell.Period (what the first Enter below
	// drills into) and for staying inside whatever trailing window the grid
	// actually renders.
	nowUTC := time.Now().UTC()
	nowMonthStart := time.Date(nowUTC.Year(), nowUTC.Month(), 1, 0, 0, 0, 0, time.UTC)
	currentMonth := costs.Period{Start: nowMonthStart.Format("2006-01-02"), End: nowMonthStart.AddDate(0, 1, 0).Format("2006-01-02")}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(currentMonth, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Requests: 1,
	})

	// Enter on the SERVICE row -> pins SERVICE=EC2, drills to USAGE_TYPE.
	// screen.Select's WaitForRows now gates on Loading unconditionally (the
	// architecture refactor removed the old "blind drill-through on a
	// never-fetched shape" carve-out), so the USAGE_TYPE frame's own fetch
	// must land before the second Enter can advance at all. Range is left
	// zero-valued so ApplyCostsLoaded matches by CacheKey (shape) alone —
	// this pure-TUI layer cannot observe the exact drilled window/anchor
	// (see the resourceIDRecords wide-tiling comment below for the same
	// constraint) — and the records tile a wide daily span around "now" so
	// SOME period lands inside whatever window the drill actually built.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// The pushed USAGE_TYPE frame opens with its cursor on costsCurrentCol's
	// pick (applyCostsSelect's PushDrill case) — the newest window column
	// whose Start has already begun, i.e. the week containing today, never
	// the raw last (possibly not-yet-elapsed) column and never the oldest
	// column. That week's Start is at most 6 days before today, so it is
	// always inside the RESOURCE_ID 14-day retention window regardless of
	// today's day-of-month or month boundary — no manual scrolling needed.
	usageTypeQuery := costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{
			costs.DimensionService: {"Amazon Elastic Compute Cloud - Compute"},
		}},
	}
	usageTypeToday := time.Now().UTC().Truncate(24 * time.Hour)
	var usageTypeRecords []costs.Record
	for i := range 60 {
		day := usageTypeToday.AddDate(0, 0, -i)
		p := costs.Period{Start: day.Format("2006-01-02"), End: day.AddDate(0, 0, 1).Format("2006-01-02")}
		usageTypeRecords = append(usageTypeRecords, costs.Record{
			Period:  p,
			Keys:    []string{"USE1-BoxUsage:m5.large"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 3, Unit: "USD"}},
		})
	}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    usageTypeQuery,
		Grid:     costs.GridResult{Fetched: true, Records: usageTypeRecords},
		Requests: 1,
	})

	// Enter again -> pins USAGE_TYPE, drills to RESOURCE_ID.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))

	// RESOURCE_ID frame: one row keyed by the demo EC2 instance ID. The
	// Query's Filter is reconstructed manually (CacheKey — Granularity +
	// GroupBy + Filter — is all ApplyCostsLoaded matches against when no
	// Range is set) to mirror exactly what the drill above pinned:
	// SERVICE=EC2 at USAGE_TYPE's frame, then USAGE_TYPE="USE1-BoxUsage:
	// m5.large" (the real row usageTypeRecords seeded and Enter resolved —
	// screen.Select never pins an empty value now) at RESOURCE_ID's.
	// Granularity is DAY: applyCostsSelect's drill makes each child frame
	// ONE STEP FINER than its parent (finerGranularity), so two drills down
	// from the MONTH root (MONTH -> WEEK -> DAY) land here at DAY, not
	// MONTH. The window's exact anchor cascades through each drilled
	// frame's own selected-cell Start (month-start, then week-start), not
	// literally time.Now(), and isn't independently observable at this
	// pure-TUI layer — so this tiles 60 raw daily periods ending today
	// (wider than any plausible drilled window near "now") instead of
	// trying to replicate the exact anchor chain.
	resourceIDQuery := costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionResourceID},
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{
			costs.DimensionService:   {"Amazon Elastic Compute Cloud - Compute"},
			costs.DimensionUsageType: {"USE1-BoxUsage:m5.large"},
		}},
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	resourceIDRecords := make([]costs.Record, 0, 60)
	for i := range 60 {
		day := today.AddDate(0, 0, -i)
		p := costs.Period{Start: day.Format("2006-01-02"), End: day.AddDate(0, 0, 1).Format("2006-01-02")}
		resourceIDRecords = append(resourceIDRecords, costs.Record{
			Period:  p,
			Keys:    []string{demoEC2InstanceID},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 3, Unit: "USD"}},
		})
	}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    resourceIDQuery,
		Grid:     costs.GridResult{Fetched: true, Records: resourceIDRecords},
		Requests: 1,
	})

	before := stripANSI(rootViewContent(m))
	if !strings.Contains(before, demoEC2InstanceID) {
		t.Fatalf("precondition: the RESOURCE_ID row for %s is not visible before the final Enter — the delivered records did not land in the drilled grid:\n%s", demoEC2InstanceID, before)
	}

	// Enter on the RESOURCE_ID row — the seam under test. This emits
	// KindFetchByIDDetail (round8 item 3, already green); the bug is what
	// happens to that task afterward at the TUI layer.
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd != nil {
		if follow := cmd(); follow != nil {
			m, _ = rootApplyMsg(m, follow)
		}
	}

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail --") || !strings.Contains(view, demoEC2InstanceID) {
		t.Errorf("Enter on the RESOURCE_ID row (EC2, a mapped by-ID-capable type) did not navigate to the EC2 detail view — handleCostsKeyMsg routes every task through the generic executeTaskCmd, which fetches but never emits messages.Navigate{Target: TargetDetail} for KindFetchByIDDetail (unlike runtime_adapter_related.go's special-cased m.fetchByIDDetail); got:\n%s", view)
	}
}
