// costs_round6_test.go — Cost Explorer: external reviewer's fourth pass (6
// findings) plus a live-reproduced state corruption (items 2/7) and a
// mid-write spec amendment (item 9, display-level zero-row filtering).
//
// package unit (not unit_test): item 2 needs the TUI key-routing helpers
// (rootApplyMsg/rootKeyPress/newRootSizedModel, tui_root_test.go) which
// only live in package unit — every other item reuses only EXPORTED
// app/costs/runtime/aws surface, so it's cheaper to keep the whole file in
// one package (local round6* helpers below) than to split it.
//
// Item 7 is NOT a red pin: my own scoring-time verification (a scratch
// BuildGrid run against the reviewer's exact scenario) DISPROVED the
// claimed NaN-demotion mechanism — see its doc comment. Kept as a cheap
// GREEN regression pin per the coordinator's explicit instruction, since it
// still guards the user-visible symptom the reviewer described.
//
// Item 1 is a static template-content check, not a template EXECUTION
// test — see its doc comment for why (no exported web-render hook, zero
// existing internal/web unit tests).
package unit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	"github.com/k2m30/a9s/v3/internal/app"
	a9saws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Local helpers (package unit cannot reach costs_state_test.go's/
// costs_interaction_test.go's package-unit_test equivalents)
// ---------------------------------------------------------------------------

var round6Now = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

// newCostsScreenController (costs_round3_test.go, same package) is the
// shared builder — closure-wave harness dedup collapsed round6's own
// former round6NewCostsController into it.

func round6TopDrill(t *testing.T, c *app.Controller) costs.DrillLevel {
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

func round6FindFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			if p, ok := tr.Payload.(runtime.FetchCostsPayload); ok {
				return p, true
			}
		}
	}
	return runtime.FetchCostsPayload{}, false
}

func round6FindCostRowOK(rows []app.CostRow, label string) (app.CostRow, bool) {
	for _, r := range rows {
		if r.Label == label {
			return r, true
		}
	}
	return app.CostRow{}, false
}

// round6MonthRecord builds one costs.Record for rowKey, priced amount,
// within now's own month — mirrors costs_state_test.go's monthRecord
// (package unit_test, not reachable here).
func round6MonthRecord(now time.Time, rowKey string, amount float64) costs.Record {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return costs.Record{
		Period:  costs.Period{Start: start.Format("2006-01-02"), End: end.Format("2006-01-02")},
		Keys:    []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

// round6FullWindowRecords tiles every period in window with one
// costs.Record each, so Store.Lookup considers the whole window fully
// covered (exact per-period key match) — required to make a shape
// genuinely "warm" for ensureCostsShapeFetched's missing-window check.
func round6FullWindowRecords(window []costs.Period, rowKey string, amount float64) []costs.Record {
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

// ===========================================================================
// Item 1 (P2, internal/app/viewstate.go:95 + internal/web) — BodyKindCosts
// must render the grid in web mode, not the generic "Loading…" fallthrough.
//
// WEAKNESS FLAG: internal/web/render.go's renderPage/renderMainFragment are
// unexported, and there are zero existing internal/web unit tests (web
// rendering is exercised only by tests/e2e's Playwright specs, outside
// tests/unit's reach). This is therefore a static source-text check on the
// template FILE, not a template EXECUTION test — it catches a missing
// "costs" case outright, but cannot catch a case that exists yet reads the
// wrong field or renders the wrong sub-template.
// ===========================================================================

func TestCostsRound6_Item1_WebBodyTemplate_HasCostsCase(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "web", "templates", "body.html")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading body.html: %v", err)
	}
	src := string(raw)

	if !strings.Contains(src, `"costs"`) {
		t.Fatal(`body.html has no "costs" BodyKind case — BodyKindCosts falls through to the generic "Loading…" div in web mode instead of rendering the grid`)
	}
	if !strings.Contains(src, ".Costs") {
		t.Error(`body.html's "costs" case does not appear to reference .Costs (the CostsBody field) — it must actually render the grid, not merely switch on the kind string`)
	}
}

// ===========================================================================
// Item 2 (P2, internal/tui/app_costs.go:31) — ActionScrollLeft/ScrollRight
// must capture and dispatch the []runtime.TaskRequest from ctrl.Apply like
// the zoom/metric/Enter branches do, at the real TUI key-routing layer.
// ===========================================================================

func TestCostsRound6_Item2_TUI_ScrollLeftAtOldestColumn_DispatchesFetchCmd(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})

	// internal/costs/window.go's defaultMonthColumns: the default root
	// window is exactly 12 trailing months, cursor starts at the newest
	// (rightmost) column. 11 scroll-lefts walk it to column 0; the 12th
	// crosses the oldest loaded column — extending the range within CE's
	// 13-month horizon — which must dispatch a KindFetchCosts fetch.
	const defaultMonthColumns = 12
	for range defaultMonthColumns - 1 {
		m, _ = rootApplyMsg(m, rootKeyPress("h"))
	}
	_, cmd := rootApplyMsg(m, rootKeyPress("h"))
	if cmd == nil {
		t.Fatal("pressing ScrollLeft ('h') past the oldest loaded column produced a nil tea.Cmd — handleCostsKeyMsg's ScrollLeft branch (internal/tui/app_costs.go) does not capture/dispatch the []runtime.TaskRequest ctrl.Apply returns, unlike the zoom/metric/Enter branches, so the scroll-to-load fetch never executes")
	}
}

// ===========================================================================
// Item 3 (P2, internal/costs/grid.go:123) — the row filter must drop only
// rows whose cells are ALL zero/no-data: a row with +100 in one period and
// -100 in another (net zero total) must still render; TOTAL unchanged.
// ===========================================================================

func TestCostsRound6_Item3_BuildGrid_NetZeroRow_StillRenders_TotalUnchanged(t *testing.T) {
	window := []costs.Period{
		{Start: "2026-01-01", End: "2026-02-01"},
		{Start: "2026-02-01", End: "2026-03-01"},
	}
	recs := []costs.Record{
		{Period: window[0], Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}},
		{Period: window[1], Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: -100, Unit: "USD"}}},
		{Period: window[0], Keys: []string{"Amazon RDS"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 50, Unit: "USD"}}},
		{Period: window[1], Keys: []string{"Amazon RDS"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 60, Unit: "USD"}}},
	}
	grid := costs.BuildGrid(recs, costs.MetricInvoice, window)

	found := false
	for _, r := range grid.Rows {
		if r.Key != "Amazon EC2" {
			continue
		}
		found = true
		if r.Cells[0].Amount.Value != 100 {
			t.Errorf("EC2 cell[0] = %v, want 100", r.Cells[0].Amount.Value)
		}
		if r.Cells[1].Amount.Value != -100 {
			t.Errorf("EC2 cell[1] = %v, want -100", r.Cells[1].Amount.Value)
		}
	}
	if !found {
		t.Error("a row whose total is 0 (+100 then -100) but whose individual cells are both non-zero was dropped — the row filter must drop only rows whose cells are ALL zero/no-data, not rows whose SUM happens to net to zero")
	}
	if len(grid.Totals) != 2 {
		t.Fatalf("Totals: got %d, want 2", len(grid.Totals))
	}
	if got := grid.Totals[0].Amount.Value; got != 150 {
		t.Errorf("Totals[0] = %v, want 150 (100 EC2 + 50 RDS)", got)
	}
	if got := grid.Totals[1].Amount.Value; got != -40 {
		t.Errorf("Totals[1] = %v, want -40 (-100 EC2 + 60 RDS)", got)
	}
}

// ===========================================================================
// Item 4 (P2, internal/aws/costs.go:80) — invoiceMetricKey must not remap
// invoice -> unblended for a RECORD_TYPE clause coming from a RECORD_TYPE
// DRILL in invoice mode; only the unblended display shape's own exclusion
// filter does.
// ===========================================================================

type round6MockGetCostAndUsageClient struct {
	output *costexplorer.GetCostAndUsageOutput
}

func (m *round6MockGetCostAndUsageClient) GetCostAndUsage(
	_ context.Context,
	_ *costexplorer.GetCostAndUsageInput,
	_ ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageOutput, error) {
	return m.output, nil
}

func TestCostsRound6_Item4_InvoiceModeRecordTypeDrill_ParsesUnblendedCostIntoInvoiceKey(t *testing.T) {
	amt := "500.00"
	unit := "USD"
	client := &round6MockGetCostAndUsageClient{
		output: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []cetypes.ResultByTime{
				{
					TimePeriod: &cetypes.DateInterval{Start: aws.String("2026-01-01"), End: aws.String("2026-02-01")},
					Groups: []cetypes.Group{
						{
							Keys: []string{"Usage"},
							Metrics: map[string]cetypes.MetricValue{
								"UnblendedCost": {Amount: &amt, Unit: &unit},
							},
						},
					},
				},
			},
		},
	}

	// A record-type DRILL in invoice mode: Filter.Equals[RECORD_TYPE] is
	// set (the user pivoted by RECORD_TYPE, digit 6, then drilled Enter
	// into a specific row — internal/app/costs_state.go's
	// applyCostPivot/applyCostsSelect never touch cs.Metric, which stays
	// "invoice"). This is NOT the unblended display shape's own
	// NotEquals[RECORD_TYPE] exclusion filter, which legitimately maps to
	// "unblended".
	q := costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionRecordType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionRecordType: {"Usage"}}},
		Range:       costs.Period{Start: "2026-01-01", End: "2026-02-01"},
	}

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage: %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("Records: got %d, want 1", len(result.Records))
	}
	rec := result.Records[0]
	got, ok := rec.Metrics[costs.MetricInvoice]
	if !ok {
		t.Fatalf("Metrics[MetricInvoice] missing (got %+v) — invoiceMetricKey remapped an Equals[RECORD_TYPE] drill in invoice mode to \"unblended\" instead of \"invoice\", so the controller's cs.Metric==invoice read finds nothing and renders an empty grid", rec.Metrics)
	}
	if got.Value != 500.00 {
		t.Errorf("Metrics[MetricInvoice].Value = %v, want 500", got.Value)
	}
	if _, ok := rec.Metrics[costs.MetricUnblended]; ok {
		t.Errorf("Metrics also carries MetricUnblended for the same UnblendedCost value — this is an invoice-mode drill, only MetricInvoice should be populated")
	}
}

// ===========================================================================
// Item 5 (P2, internal/app/costs_state.go:300) — a stale ErrorMsg must
// clear when the requested shape is fully covered by cache (switch to a
// warm shape renders the grid, not the old error) and when a retry starts.
// ===========================================================================

func TestCostsRound6_Item5_StaleErrorMsg_ClearsOnWarmShapeSwitch(t *testing.T) {
	c := newCostsScreenController(t, round6Now)

	// Warm REGION (digit 2) first.
	_, regionTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2})
	regionPayload, found := round6FindFetchCostsTask(regionTasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query: regionPayload.Query,
		Grid:  costs.GridResult{Fetched: true, Records: round6FullWindowRecords(regionPayload.Window, "us-east-1", 500)},
		// A non-nil (even empty) Anomalies slice is an authoritative fetch
		// result (costsAnomalyResultFromEvent: Requested = ev.Anomalies !=
		// nil) — seeded fresh here so re-selecting this shape later stays a
		// true zero-task cache hit, not just SkipGrid=true.
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})
	if c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: REGION shape still Loading after its own CostsLoaded delivery")
	}

	// Switch to SERVICE (digit 1) and fail it.
	_, serviceTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	servicePayload, found := round6FindFetchCostsTask(serviceTasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: servicePayload.Query,
		Err:      errors.New("cost explorer: access denied"),
		Requests: 1,
	})
	if got := c.Snapshot().Body.Costs.ErrorMsg; got == "" {
		t.Fatal("precondition: SERVICE shape fetch failure did not set ErrorMsg")
	}

	// Switch BACK to the already-warm REGION shape.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2})
	if _, found := round6FindFetchCostsTask(tasks); found {
		t.Fatal("precondition: REGION shape re-selected but emitted a fetch task — expected a cache hit (already fully warm)")
	}

	vs := c.Snapshot()
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %q, want \"\" after switching to a fully-covered (warm) shape — a stale error from a DIFFERENT shape must not keep masking a renderable grid", vs.Body.Costs.ErrorMsg)
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("Rows is empty after switching to the warm REGION shape — the stale ErrorMsg is still short-circuiting buildCostsBody before it ever reaches liveCostGrid")
	}
}

func TestCostsRound6_Item5_StaleErrorMsg_ClearsWhenRetryStarts(t *testing.T) {
	c := newCostsScreenController(t, round6Now)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE (root default, fresh task)
	payload, found := round6FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: payload.Query,
		Err:      errors.New("cost explorer: request throttled"),
		Requests: 1,
	})
	if got := c.Snapshot().Body.Costs.ErrorMsg; got == "" {
		t.Fatal("precondition: SERVICE shape fetch failure did not set ErrorMsg")
	}

	retryTasks := c.ForceRefreshCosts()
	if _, found := round6FindFetchCostsTask(retryTasks); !found {
		t.Fatal("precondition: ForceRefreshCosts did not emit a new fetch task for the still-uncovered shape")
	}

	vs := c.Snapshot()
	// ensureCostsShapeFetched DID set the internal cs.Loading=true (proven
	// by retryTasks being non-empty above) — but buildCostsBody's
	// ErrorMsg-early-return branch (internal/app/costs_body.go:28-44)
	// constructs the returned CostsBody WITHOUT ever copying cs.Loading
	// into it, so Loading reads back false from the outside regardless.
	// Same root cause as the warm-switch half of this finding: the stale
	// ErrorMsg branch keeps short-circuiting buildCostsBody, this time
	// hiding the fact that a retry is genuinely in flight.
	if !vs.Body.Costs.Loading {
		t.Error("Loading is false immediately after ForceRefreshCosts dispatched a new fetch — the UI must show a loading state while a retry is in flight, not a frozen stale error with no loading indication")
	}
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("ErrorMsg = %q, want \"\" once a retry fetch has started — the UI must not keep showing a stale error while a new attempt is already in flight", vs.Body.Costs.ErrorMsg)
	}
}

// ===========================================================================
// Item 6 (P3, internal/tui/views/costs.go:78) — with more rows than height,
// the data-through/anomaly footer line must still render: clipCostsRows
// must reserve room for the 2 trailing lines (blank + footer) RenderCosts
// unconditionally appends afterward, so the total output never exceeds the
// requested height budget.
// ===========================================================================

func TestCostsRound6_Item6_FooterSurvives_HeightBudget_WhenClipping(t *testing.T) {
	body := app.CostsBody{
		DataThrough: "2026-07-10",
		Columns:     []app.CostColumn{{Label: "Jul'26", Open: true}},
		Rows:        make([]app.CostRow, 20),
		Totals:      []app.CostCell{{Amount: "1,234.5"}},
		CursorRow:   0,
	}
	for i := range body.Rows {
		body.Rows[i] = app.CostRow{
			Label: fmt.Sprintf("Service%02d", i),
			Cells: []app.CostCell{{Amount: fmt.Sprintf("%d.0", 100-i)}},
		}
	}

	const height = 8
	out := views.RenderCosts(body, 80, height)
	lines := strings.Split(out, "\n")
	if len(lines) > height {
		t.Errorf("RenderCosts height budget: got %d lines, want <= %d (height=%d) — the footer's 2 trailing lines push the output past the requested height whenever clipCostsRows already consumed the full budget on header+data+TOTAL; the clip budget must reserve room for them", len(lines), height, height)
	}
	if !strings.Contains(out, "data through 2026-07-10") {
		t.Errorf("rendered output at height=%d with %d rows does not contain the footer's data-through text — got:\n%s", height, len(body.Rows), out)
	}
}

// ===========================================================================
// Item 7 (live repro) — row totals must treat missing cells as zero, never
// NaN, so a row with real amounts in covered columns and no record for one
// visible column keeps its real total and is never demoted below tiny
// complete rows.
//
// GREEN BY DESIGN: verified during scoring, not merely assumed. I built
// this exact scenario against the CURRENT BuildGrid (a scratch test,
// discarded after running) and it already sorts correctly — the claimed
// "NaN total" mechanism does not reproduce: a missing column's cellAgg is
// the zero value (sum=0), never NaN; BuildGrid's per-row total is a plain
// sum over every column, so a missing column simply contributes 0, and NaN
// can only enter via a genuinely malformed Amount.Value (never produced on
// this path — internal/aws/costs.go's mapCEGroup hard-errors on an
// unparseable amount rather than substituting NaN). Kept per the
// coordinator's instruction as a cheap regression pin for the user-visible
// symptom the reviewer described ("all-zero viewports on every pivot"),
// since a future refactor of BuildGrid's total/sort math could reintroduce
// it silently.
// ===========================================================================

func TestCostsRound6_Item7_MissingCellRow_SortsByRealTotal_GreenRegressionPin(t *testing.T) {
	window := []costs.Period{
		{Start: "2025-07-01", End: "2025-08-01"},
		{Start: "2025-08-01", End: "2025-09-01"},
		{Start: "2025-09-01", End: "2025-10-01"},
	}
	recs := []costs.Record{
		{Period: window[0], Keys: []string{"BigSpender"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 5000, Unit: "USD"}}},
		// window[1] deliberately has NO BigSpender record — an uncovered/
		// missing column for this row, matching the live scroll-left
		// dropped-task repro (item 2).
		{Period: window[2], Keys: []string{"BigSpender"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 5000, Unit: "USD"}}},

		{Period: window[0], Keys: []string{"TinySpender"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1, Unit: "USD"}}},
		{Period: window[1], Keys: []string{"TinySpender"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1, Unit: "USD"}}},
		{Period: window[2], Keys: []string{"TinySpender"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1, Unit: "USD"}}},
	}
	grid := costs.BuildGrid(recs, costs.MetricInvoice, window)

	if len(grid.Rows) != 2 {
		t.Fatalf("Rows: got %d, want 2", len(grid.Rows))
	}
	if grid.Rows[0].Key != "BigSpender" {
		t.Errorf("row order: got [0]=%q want BigSpender first (10000 real total vs TinySpender's 3) — a missing column must not corrupt the sort", grid.Rows[0].Key)
	}
}

// ===========================================================================
// Item 8 (NEW) — cost cache schema version bump: internal/costs/store.go's
// schemaVersion goes to 2, so a version-1 file (this branch's earlier
// builds wrote incompatible shapes/keys under the same version number)
// gets set aside to .bak with a fresh store, exercising the existing
// alien-version recovery path (already correct and covered by
// TestStore_AlienVersion_RenamedToBakFreshStoreNoPanic in
// costs_store_test.go, whose fixture uses "version: 999" — a version that
// stays alien regardless of schemaVersion's own value, so it needs no
// reconciliation).
//
// No existing test pins "version: 1" as a successful, non-recovered load —
// checked costs_store_test.go directly; its round-trip test always Saves
// then Loads (version-agnostic), and its two disk-fixture tests use
// "version: 999" (alien-version) and "version: 1" + malformed YAML
// (corrupt-parse, fails regardless of version). No reconciliation needed.
// ===========================================================================

func TestCostsRound6_Item8_VersionOneFile_TreatedAsAlien_AfterSchemaBump(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile := "round6-schema-account"
	path := costs.CachePath(profile)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	v1 := []byte("version: 1\nqueries: {}\n")
	if err := os.WriteFile(path, v1, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	s := costs.LoadStore(profile)

	if !s.Recovered() {
		t.Error("Recovered() = false, want true — a well-formed version:1 cache file (this branch's earlier, incompatible on-disk shape) must be treated as alien once schemaVersion is bumped to 2")
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("original version:1 cache file still present at the primary path, want it renamed away to .bak")
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("reading .bak: %v", err)
	}
	if string(bak) != string(v1) {
		t.Errorf(".bak content = %q, want the original version:1 bytes %q", bak, v1)
	}
}

// ===========================================================================
// Item 9 (NEW, mid-write spec amendment) — display-level zero-row filter:
// a row whose every visible cell FORMATS as "0.0" (real sub-cent data that
// rounds to zero at display precision) must not render — on any pivot
// EXCEPT LINKED_ACCOUNT, where rows always show. Interaction with item 3:
// the offsetting +100/-100 row still renders (its cells format non-zero:
// "100.0"/"-100.0"); a row of sub-cent noise does not — pinned at the
// CostsBody row level, one render-level absence check, and the
// LINKED_ACCOUNT exemption.
//
// Reconciliation check performed: grepped every tests/unit/costs*.go file
// for sub-$1 fractional Amount.Value literals — none exist. No existing
// test seeds a sub-cent row and expects it rendered, so nothing needed
// updating.
//
// RECONCILED (architecture.md Seam 8, CostsViewModel): this whole group's
// mechanism — filterCostsZeroDisplayGridRows' sub-cent hide + the
// LINKED_ACCOUNT exemption (internal/app/costs_state.go's liveCostGrid) —
// is pinned at the typed seam in costs_screen_test.go
// (TestCostsScreen_BuildViewModel_DisplayFilter_SubCentHidden_
// LinkedAccountExempt). These four tests stay unchanged as the full-stack
// (controller -> CostsBody) acceptance pins.
// ===========================================================================

func TestCostsRound6_Item9_SubCentNoiseRow_HiddenAtDisplayLevel_DefaultPivot(t *testing.T) {
	c := newCostsScreenController(t, round6Now)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	payload, found := round6FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			round6MonthRecord(round6Now, "Amazon EC2", 100.0),
			round6MonthRecord(round6Now, "AWS Support (Business)", 0.004), // real, non-zero, formats "0.0"
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if _, ok := round6FindCostRowOK(vs.Body.Costs.Rows, "EC2"); !ok {
		t.Fatal("precondition: real spender row (EC2, stripped label) missing from Rows")
	}
	if row, ok := round6FindCostRowOK(vs.Body.Costs.Rows, "Support (Business)"); ok {
		t.Errorf("sub-cent noise row (every visible cell formats \"0.0\") must not render for a non-LINKED_ACCOUNT pivot, found: %+v", row)
	}
}

func TestCostsRound6_Item9_NetZeroRow_StillRenders_AtDisplayLevel_PerCellNotAggregateFilter(t *testing.T) {
	c := newCostsScreenController(t, round6Now)
	root := round6TopDrill(t, c)
	if len(root.Window) < 2 {
		t.Fatalf("precondition: root window too short (%d columns)", len(root.Window))
	}
	prevPeriod := root.Window[len(root.Window)-2]
	curPeriod := root.Window[len(root.Window)-1]

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	payload, found := round6FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	rec := func(period costs.Period, amount float64) costs.Record {
		return costs.Record{
			Period:  period,
			Keys:    []string{"Amazon EC2"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
		}
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{rec(prevPeriod, 100.0), rec(curPeriod, -100.0)}},
		Requests: 1,
	})

	vs := c.Snapshot()
	row, ok := round6FindCostRowOK(vs.Body.Costs.Rows, "EC2")
	if !ok {
		t.Fatal("a net-zero row (aggregate total 0, but EVERY individual cell formats non-zero: \"100.0\"/\"-100.0\") must still render at the CostsBody level — a display-level filter that checks the row's aggregate total instead of each visible cell would wrongly drop it too")
	}
	lastIdx := len(vs.Body.Costs.Columns) - 1
	if got := row.Cells[lastIdx].Amount; got != "-100.0" {
		t.Errorf("net-zero row's last (current-period) cell Amount = %q, want \"-100.0\"", got)
	}
}

func TestCostsRound6_Item9_LinkedAccountPivot_SubCentRow_StillRenders(t *testing.T) {
	c := newCostsScreenController(t, round6Now)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 3}) // LINKED_ACCOUNT
	payload, found := round6FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: LINKED_ACCOUNT pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{round6MonthRecord(round6Now, "123456789012", 0.004)}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if _, ok := round6FindCostRowOK(vs.Body.Costs.Rows, "123456789012"); !ok {
		t.Error("a sub-cent row under a LINKED_ACCOUNT pivot must still render (the account list itself is the information) — must not be hidden by the display-level zero-row filter")
	}
}

func TestCostsRound6_Item9_RenderCosts_NoiseRowLabel_AbsentFromOutput(t *testing.T) {
	c := newCostsScreenController(t, round6Now)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	payload, found := round6FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query: payload.Query,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			round6MonthRecord(round6Now, "Amazon EC2", 100.0),
			round6MonthRecord(round6Now, "AWS Support (Business)", 0.004),
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if vs.Body.Costs == nil {
		t.Fatal("precondition: Body.Costs is nil")
	}
	out := views.RenderCosts(*vs.Body.Costs, 160, 24)
	if strings.Contains(out, "Support (Business)") {
		t.Errorf("rendered grid still shows the sub-cent noise row's label — RenderCosts output:\n%s", out)
	}
	if !strings.Contains(out, "EC2") {
		t.Errorf("rendered grid is missing the real spender row's label (EC2) — RenderCosts output:\n%s", out)
	}
}
