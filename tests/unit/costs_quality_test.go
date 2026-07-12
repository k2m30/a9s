// costs_quality_test.go — Cost Explorer: quality batch from a self-audit of
// the rendered surfaces against specs/021-cost-explorer/wireframe.md (items
// 1, 2, 5) plus a web/TUI drill-parity pass (items 3, 4).
//
// package unit (not unit_test): item 3's TUI-vs-Snapshot convergence check
// needs the TUI key-routing/render helpers (rootApplyMsg/rootViewContent/
// newRootSizedModel, tui_root_test.go) which only live in package unit —
// every other item reuses only EXPORTED app/costs/runtime surface via local
// round8-prefixed helpers (mirroring round6/round7's identical choice).
//
// Item 1 (P2) — DeltaTag gains a 4-tier color scale. wireframe.md only says
// "growth red shades, drop green shades, |Δ| < threshold neutral" — no
// exact percentages, so the thresholds below are a QA-CHOSEN CONTRACT,
// FLAGGED for the coordinator to confirm or adjust:
//
//	|Δ| <  5%        -> "neutral"
//	5% <= |Δ| < 25%  -> "growth-soft"  / "drop-soft"
//	|Δ| >= 25%       -> "growth-strong" / "drop-strong"
//
// DeltaTag stays a single string (5 values total) rather than splitting
// into a second Intensity field — mirrors CostCell's own existing contract
// ("one pre-resolved string drives color, same as ListRow.Color").
//
// Reconciliation performed for item 1 (swept every costs_*_test.go file
// asserting DeltaTag): costs_body_test.go (+23.45% EC2 / -6.25% RDS, both
// now "-soft"), costs_round3_test.go (zero-cell must be exactly "neutral"
// not merely != growth/drop; ELB's +100% is unambiguously "-strong").
// costs_view_test.go's two literal DeltaTag:"growth"/"drop" values were
// checked and need NO reconciliation — grepped the whole file for any
// assertion that depends on them (color/ANSI styling); none exists, they
// are inert filler in a body literal no test in that file inspects.
//
// Item 5 (P2) — a live ValidationException on a real account
// ("You haven't enabled historical data beyond 14 months.") on year zoom.
// costsHistoryHorizonMonths (internal/app/costs_state.go) is unexported and
// unreachable from tests/unit — mirrored here as a literal (13, matching
// the constant read directly off disk during scoring) rather than
// re-derived, with a comment tying it back explicitly.
//
// Item 4 (P2) — CostsBody has no Currency field today. Referencing a
// field that doesn't exist would be a COMPILE error for the entire
// tests/unit package (Go compiles per-package, not per-file) — unlike a
// runtime t.Errorf, that would silently prevent every OTHER test in this
// dispatch (and every pre-existing test) from running at all, defeating
// "run the runnable set." The struct-gains-a-field half of this finding is
// therefore pinned via reflection (compile-safe: reports a real RED result
// today, and remains checkable once the field lands) instead of a direct
// field reference.
package unit

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ---------------------------------------------------------------------------
// Local helpers (package unit cannot reach costs_state_test.go's/
// costs_interaction_test.go's package-unit_test equivalents)
// ---------------------------------------------------------------------------

var round8Now = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

// newCostsScreenController (costs_round3_test.go, same package) is the
// shared builder — closure-wave harness dedup collapsed this file's own
// former round8NewCostsController into it.

func round8TopDrill(t *testing.T, c *app.Controller) costs.DrillLevel {
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

func round8FindFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			if p, ok := tr.Payload.(runtime.FetchCostsPayload); ok {
				return p, true
			}
		}
	}
	return runtime.FetchCostsPayload{}, false
}

func round8MonthRecord(now time.Time, rowKey string, amount float64) costs.Record {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return costs.Record{
		Period:  costs.Period{Start: start.Format("2006-01-02"), End: end.Format("2006-01-02")},
		Keys:    []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

// ===========================================================================
// Item 1 — DeltaTag gains a neutral band and two intensity tiers per
// direction. See file header for the pinned, flagged threshold contract.
// ===========================================================================

func TestCostsQuality_Item1_DeltaTag_FourTierColorScale(t *testing.T) {
	tests := []struct {
		name      string
		prev, cur float64
		want      string
	}{
		{"tiny +1% wiggle stays neutral (the live symptom: was tagged identically to +135%)", 1000, 1010, "neutral"},
		{"just under the neutral band, -4.9%", 1000, 951, "neutral"},
		{"exactly +5% is the soft-growth boundary (not neutral)", 1000, 1050, "growth-soft"},
		{"soft growth +10%", 1000, 1100, "growth-soft"},
		{"soft drop -10%", 1000, 900, "drop-soft"},
		{"exactly +25% is the strong-growth boundary (not soft)", 1000, 1250, "growth-strong"},
		{"strong growth +135% (the live symptom's own example)", 1000, 2350, "growth-strong"},
		{"strong drop -60%", 1000, 400, "drop-strong"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCostsScreenController(t, round8Now)
			root := round8TopDrill(t, c)
			prevPeriod := root.Window[len(root.Window)-2]
			curPeriod := root.Window[len(root.Window)-1]

			c.Handle(messages.CostsLoaded{
				Query: costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
				Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
					{Period: prevPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: tt.prev, Unit: "USD"}}},
					{Period: curPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: tt.cur, Unit: "USD"}}},
				}},
				Requests: 1,
			})

			vs := c.Snapshot()
			if len(vs.Body.Costs.Rows) == 0 {
				t.Fatal("precondition: no rows after seeding the two-period record")
			}
			lastIdx := len(vs.Body.Costs.Columns) - 1
			got := vs.Body.Costs.Rows[0].Cells[lastIdx].DeltaTag
			if got != tt.want {
				pct := (tt.cur - tt.prev) / tt.prev * 100
				t.Errorf("DeltaTag for %.1f -> %.1f (%.1f%% delta) = %q, want %q", tt.prev, tt.cur, pct, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Item 2 — humanized anomaly footer: service, impact amount, and usage
// type, no raw "DIMENSION=value" dumps. Confirmed via direct trace:
// internal/aws/costs.go's mapAnomaly builds RootCause as literally
// fmt.Sprintf("%s=%s", dim, value) joined by ", " — this test originally
// fed exactly that shape as the seeded AnomalyMark.RootCause.
//
// Reconciled (closure wave): AnomalyMark.RootCause/.Score/.ID die with the
// fields — this test's Dimension-only fixture (SERVICE + USAGE_TYPE, no
// RootCause/ID at all) becomes the ONLY representation an anomaly carries,
// so costsFooterNote must derive the humanized footer straight from
// Dimension, not from any pre-baked raw-dump string. Impact IS already
// correctly populated end-to-end (fetcher and demo fixture both set it) —
// costsFooterNote simply never reads it.
// ===========================================================================

func TestCostsQuality_Item2_AnomalyFooter_Humanized_NoRawDimensionDump(t *testing.T) {
	c := newCostsScreenController(t, round8Now)
	root := round8TopDrill(t, c)
	curPeriod := root.Window[len(root.Window)-1] // cursor already opens here (FR-002)

	c.Handle(messages.CostsLoaded{
		Query: costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:  costs.GridResult{Fetched: true, Records: []costs.Record{round8MonthRecord(round8Now, "Amazon EC2", 1200.0)}},
		Anomalies: []costs.AnomalyMark{
			{
				Impact: costs.Amount{Value: 405, Unit: "USD"},
				Period: curPeriod,
				Dimension: map[costs.Dimension]string{
					costs.DimensionService:   "Amazon EC2",
					costs.DimensionUsageType: "USE1-NatGateway-Bytes",
				},
			},
		},
		Requests: 1,
	})

	vs := c.Snapshot()
	footer := vs.Body.Costs.FooterNote
	if footer == "" {
		t.Fatal("precondition: no FooterNote for the cursor-on-anomaly cell")
	}
	if strings.Contains(footer, "SERVICE=") || strings.Contains(footer, "USAGE_TYPE=") {
		t.Errorf("FooterNote %q contains a raw dimension=value dump — project rule forbids raw enum/dimension dumps anywhere", footer)
	}
	if !strings.Contains(footer, "EC2") {
		t.Errorf("FooterNote %q does not name the service (EC2)", footer)
	}
	if !strings.Contains(footer, "405") {
		t.Errorf("FooterNote %q does not include the anomaly impact amount ($405, AnomalyMark.Impact — already correctly populated by mapAnomaly/the demo fixture, just never read here)", footer)
	}
	if !strings.Contains(footer, "USE1-NatGateway-Bytes") {
		t.Errorf("FooterNote %q does not name the usage type", footer)
	}
}

// ===========================================================================
// Item 3 (narrowed to 3a only per the coordinator's own disproof
// correction — 3b is already implemented, costs.html's cursor cell already
// carries a "costs-cursor" class; 3c's substantive gap is round7's item 4,
// not a new finding here) — the web costs view's title must carry the same
// state line the TUI already computes, from ONE shared source exposed via
// Controller.Snapshot().FrameTitle.
//
// Confirmed via direct trace: internal/app/snapshot.go:71 hardcodes
// `vs.FrameTitle = string(runtime.ScreenCosts)` (the bare "costs" string)
// for the costs screen kind, while internal/tui/app_view.go's frameTitle()
// computes a RICH state line via its own package-private costsFrameTitle()
// function — bypassing vs.FrameTitle entirely. Every OTHER screen kind
// (menu/list/selector/detail) instead feeds vs.FrameTitle from ONE shared
// builder consumed by both renderers; costs is the outlier.
// ===========================================================================

func TestCostsQuality_Item3a_Snapshot_FrameTitle_IsStateLine_NotBareScreenID(t *testing.T) {
	c := newCostsScreenController(t, round8Now)

	vs := c.Snapshot()
	if vs.FrameTitle == "costs" || vs.FrameTitle == string(runtime.ScreenCosts) {
		t.Fatalf("Snapshot().FrameTitle = %q, want the state line (pivot · metric · granularity) — the bare screen ID leaves the web title blank of any drill context", vs.FrameTitle)
	}
	cb := vs.Body.Costs
	if cb == nil {
		t.Fatal("precondition: Body.Costs is nil")
	}
	for _, want := range []string{strings.ToLower(string(cb.Pivot)), cb.Metric, cb.Granularity} {
		if want == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(vs.FrameTitle), strings.ToLower(want)) {
			t.Errorf("Snapshot().FrameTitle = %q, want it to contain %q (from the current CostsBody: pivot=%q metric=%q granularity=%q)", vs.FrameTitle, want, cb.Pivot, cb.Metric, cb.Granularity)
		}
	}
}

func TestCostsQuality_Item3a_TUI_RenderedFrame_ContainsSharedSnapshotFrameTitle(t *testing.T) {
	headless := newCostsScreenController(t, round8Now)
	headlessTitle := headless.Snapshot().FrameTitle

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	rendered := stripANSI(rootViewContent(m))

	if !strings.Contains(rendered, headlessTitle) {
		t.Errorf("TUI-rendered frame does not contain Controller.Snapshot().FrameTitle (%q) verbatim — the TUI currently computes its own separate costsFrameTitle() instead of consuming the shared FrameTitle Snapshot() already exposes for every other screen kind (MenuFrameTitle/ListFrameTitle/etc feed BOTH renderers from ONE source); once unified, this duplicate TUI-only function should be retired. Rendered frame:\n%s", headlessTitle, rendered)
	}
}

// ===========================================================================
// Item 3c (trivial remnant only — arrows/Enter/Escape already wired,
// b/+/-/0-9 landed in round 7) — app.js's keyMap carries j/k/h/l as
// aliases for the movement actions.
// ===========================================================================

func TestCostsQuality_Item3c_WebAppJS_KeyMap_HasVimMovementAliases(t *testing.T) {
	raw, err := readQualityFile(t, "../../internal/web/static/app.js")
	if err != nil {
		t.Fatalf("reading app.js: %v", err)
	}
	for _, want := range []string{
		`{ key: "k",          action: { kind: "move-up" } }`,
		`{ key: "j",          action: { kind: "move-down" } }`,
		`{ key: "h",          action: { kind: "scroll-left" } }`,
		`{ key: "l",          action: { kind: "scroll-right" } }`,
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("app.js's keyMap is missing the vim-style alias entry %s", want)
		}
	}
}

// ===========================================================================
// Item 4 — currency in the footer. Grid.Currency (internal/costs/grid.go)
// is already correctly resolved by BuildGrid (single-currency -> that
// currency; mixed currencies -> "" per its own unitSet logic) but is
// silently dropped: CostsBody has no Currency field, and neither
// renderCostsFooter (TUI) nor costs.html (web) reference one.
// ===========================================================================

func TestCostsQuality_Item4_CostsBody_GainsCurrencyField(t *testing.T) {
	// Compile-safe existence check (see file header) — a direct
	// vs.Body.Costs.Currency reference would be a package-wide compile
	// error today.
	typ := reflect.TypeOf(app.CostsBody{})
	field, ok := typ.FieldByName("Currency")
	if !ok {
		t.Fatal("app.CostsBody has no \"Currency\" field — Grid.Currency (already correctly resolved by BuildGrid) is silently dropped before it ever reaches the footer")
	}
	if field.Type.Kind() != reflect.String {
		t.Errorf("app.CostsBody.Currency has kind %v, want string", field.Type.Kind())
	}
}

func TestCostsQuality_Item4_TUIFooter_ReferencesCurrency(t *testing.T) {
	raw, err := readQualityFile(t, "../../internal/tui/views/costs.go")
	if err != nil {
		t.Fatalf("reading internal/tui/views/costs.go: %v", err)
	}
	if !strings.Contains(raw, "Currency") {
		t.Error("internal/tui/views/costs.go never references a Currency field — the TUI footer (renderCostsFooter) has no currency slot")
	}
}

func TestCostsQuality_Item4_WebCostsTemplate_ReferencesCurrency(t *testing.T) {
	raw, err := readQualityFile(t, "../../internal/web/templates/costs.html")
	if err != nil {
		t.Fatalf("reading costs.html: %v", err)
	}
	if !strings.Contains(raw, "Currency") {
		t.Error("costs.html never references .Currency — the web footer has no currency slot either")
	}
}

// TestCostsQuality_Item4_GridCurrency_AlreadyResolvedCorrectly is a control
// confirming the DATA half of item 4 is already sound — only the threading
// to CostsBody/the two renderers is missing.
func TestCostsQuality_Item4_GridCurrency_AlreadyResolvedCorrectly(t *testing.T) {
	window := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	single := costs.BuildGrid(
		[]costs.Record{{Period: window[0], Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}}},
		costs.MetricInvoice, window,
	)
	if single.Currency != "USD" {
		t.Errorf("control failed: single-currency grid Currency = %q, want %q", single.Currency, "USD")
	}

	mixed := costs.BuildGrid(
		[]costs.Record{
			{Period: window[0], Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}},
			{Period: window[0], Keys: []string{"Amazon S3"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 50, Unit: "GBP"}}},
		},
		costs.MetricInvoice, window,
	)
	if mixed.Currency != "" {
		t.Errorf("control failed: mixed-currency grid Currency = %q, want \"\" (ambiguous, must not silently pick one)", mixed.Currency)
	}
}

// readQualityFile is a t.Helper os.ReadFile wrapper — separated out only so
// every static-file check above reads identically; not a meaningful
// abstraction on its own.
func readQualityFile(t *testing.T, path string) (string, error) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ===========================================================================
// Item 5 — history-horizon Range.Start clamp: zoom-out to YEAR must never
// build a window/query whose Range.Start precedes the same
// costsHistoryHorizonMonths CE entitlement horizon scroll-to-load already
// respects at month granularity. Live-verified: a real CE call with an
// unclamped Range.Start throws ValidationException "You haven't enabled
// historical data beyond 14 months."
//
// costsHistoryHorizonMonths (internal/app/costs_state.go, = 13) is
// unexported and unreachable from tests/unit — mirrored here as a literal,
// tied back explicitly rather than re-derived.
// ===========================================================================

func TestCostsQuality_Item5_YearZoom_RangeStart_ClampedToHistoryHorizon(t *testing.T) {
	const historyHorizonMonths = 13 // mirrors internal/app/costs_state.go's costsHistoryHorizonMonths

	c := newCostsScreenController(t, round8Now)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year
	payload, found := round8FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zoom-out to year did not emit a fetch task")
	}
	yearTop := round8TopDrill(t, c)
	if yearTop.Granularity != costs.GranularityYear {
		t.Fatalf("precondition: expected Granularity year, got %q", yearTop.Granularity)
	}
	if len(yearTop.Window) == 0 {
		t.Fatal("precondition: year window is empty")
	}

	horizonStart := time.Date(round8Now.Year(), round8Now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(historyHorizonMonths - 1), 0)

	earliestStart, err := time.Parse("2006-01-02", yearTop.Window[0].Start)
	if err != nil {
		t.Fatalf("parsing year window's earliest Start %q: %v", yearTop.Window[0].Start, err)
	}
	if earliestStart.Before(horizonStart) {
		t.Errorf("year window's earliest period Start = %q, want no earlier than %q (the same %d-month history horizon costsHistoryHorizonMonths already enforces for scroll-to-load) — CE rejects a Range.Start beyond its default history entitlement with ValidationException \"You haven't enabled historical data beyond 14 months.\"", yearTop.Window[0].Start, horizonStart.Format("2006-01-02"), historyHorizonMonths)
	}

	// The executor-bound query itself (not just the display window) is
	// what actually reaches GetCostAndUsage and must be clamped too.
	queryStart, err := time.Parse("2006-01-02", payload.Query.Range.Start)
	if err != nil {
		t.Fatalf("parsing executor query Range.Start %q: %v", payload.Query.Range.Start, err)
	}
	if queryStart.Before(horizonStart) {
		t.Errorf("year-zoom executor query Range.Start = %q, want no earlier than %q", payload.Query.Range.Start, horizonStart.Format("2006-01-02"))
	}
}

// TestCostsQuality_Item5_YearView_RendersCoveredSpan_NotError confirms the
// year view, once its window/query is clamped, renders whatever span IS
// covered rather than failing — the live symptom's actual user-facing
// outcome (a full-screen error) is what the clamp exists to prevent.
func TestCostsQuality_Item5_YearView_RendersCoveredSpan_NotError(t *testing.T) {
	c := newCostsScreenController(t, round8Now)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year
	payload, found := round8FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zoom-out to year did not emit a fetch task")
	}

	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{round8MonthRecord(round8Now, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("year view shows ErrorMsg %q after a matching (clamped) fetch delivery — want the covered span rendered, not an error state", vs.Body.Costs.ErrorMsg)
	}
}
