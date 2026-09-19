// Both the TUI Model and the headless Controller are thin adapters around the
// same *app.Controller; each lane's surrounding plumbing (dispatchTaskRequests,
// the rendererState stack, autoOpenSingleDetail vs dispatchCostsByIDTask) must
// reach the same user-observable outcome the shared Controller computed.
//
// The outcome is taken at the surface each lane exposes: the headless lane via
// Controller.Snapshot(), the TUI lane via its rendered view text
// (rootViewContent), which is what a real user sees.
//
// A not-found by-ID fetch reaches both lanes as the same typed
// messages.ByIDFetchFailed, matched to the placeholder's exact TargetType+ID;
// an error messages.Flash never pops the placeholder.
package unit

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// m2NewHeadlessController mirrors newCostsController (costs_state_test.go)
// but lives in package unit (not unit_test), so this file can build both
// lanes' controllers side by side without a cross-package import.
func m2NewHeadlessController(t *testing.T, profile string, now time.Time) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(now)
	return c
}

func m2FindFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
	for _, tr := range tasks {
		if p, ok := tr.Payload.(runtime.FetchCostsPayload); ok {
			return p, true
		}
	}
	return runtime.FetchCostsPayload{}, false
}

func m2FullMetricRecord(p costs.Period, rowKey string, amount float64) costs.Record {
	return costs.Record{
		Period:  p,
		Keys:    []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

// m2MoveHeadlessCursorToNewestColumn scrolls c's costs cursor to the last
// (newest) column of the current drill frame's window — mirrors
// costs_codex_test.go's own codexMoveCursorToNewestColumn (package
// unit_test, unreachable from this package unit file): ActionScrollRight
// clamps at the window's own end, so over-scrolling is safe.
func m2MoveHeadlessCursorToNewestColumn(c *app.Controller) {
	vs := c.Snapshot()
	if vs.Body.Costs == nil {
		return
	}
	for range vs.Body.Costs.Columns {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
}

// m2MoveTUICursorToNewestColumn presses the scroll-right key enough times to
// reach the last column of whatever drill frame m is currently on. The TUI
// lane exposes no Snapshot() to size the loop exactly (unlike
// m2MoveHeadlessCursorToNewestColumn) — internal/tui/app_costs.go's
// ScrollRight case forwards to the same ActionScrollRight, which clamps at
// the window's own end, so a generous fixed over-press is safe.
func m2MoveTUICursorToNewestColumn(m tui.Model) tui.Model {
	for i := 0; i < 10; i++ {
		m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyRight))
	}
	return m
}

// The full drill chain (SERVICE -> USAGE_TYPE -> RESOURCE_ID) ending in a
// successful by-ID jump lands both lanes on the resource's own detail.

func TestCostsLaneParity_A_DrillToResourceJump_Success(t *testing.T) {
	const ec2Service = "Amazon Elastic Compute Cloud - Compute"
	// A real clock, not a pinned literal: the TUI lane's own costs clock
	// (internal/tui/runtime_adapter_navigate.go's EnsureCostsState(time.Now()))
	// is always the real wall clock, so the headless half must share the
	// SAME now the test itself uses to plant periods, or the two lanes'
	// 14-day resource-drill retention clamps (costs.ClampResourceDrillWindow)
	// diverge.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c := m2NewHeadlessController(t, "matrix-a-headless", now)
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, ec2Service, 900.0)}},
		Requests: 1,
	})
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE
	usagePayload, found := m2FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("headless precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageWeeks := costs.WindowWithin(period, costs.GranularityWeek, now)
	usagePeriod := usageWeeks[len(usageWeeks)-1] // newest week — the col-0 default (oldest week) falls outside the 14-day resource-drill clamp late in the month, refusing the drill below
	c.Handle(messages.CostsLoaded{
		Query:    usagePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(usagePeriod, "USE1-BoxUsage:m5.large", 450.0)}},
		Window:   usagePayload.Window,
		Requests: 1,
	})
	m2MoveHeadlessCursorToNewestColumn(c)                  // drill RESOURCE_ID off the newest week the record was planted at, not the col-0 oldest
	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> RESOURCE_ID
	resourcePayload, found := m2FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("headless precondition: USAGE_TYPE -> RESOURCE_ID drill did not emit a fetch task")
	}
	// The NEWEST day in the RESOURCE_ID frame's window, not the oldest
	// (window[0]) — costs.ClampResourceDrillWindow (core/costs/drill.go)
	// already trimmed resourcePayload's own Window down to the last
	// ResourceDrillWindowRetentionDays before this delivery ever landed, so
	// window[0]'s UNCLAMPED period can fall outside every column the
	// RESOURCE_ID frame actually renders — window[len-1]'s End is always the
	// clamp's own upper bound and is therefore always a real column.
	resourceWindow := costs.WindowWithin(usagePeriod, costs.GranularityDay, now)
	resourcePeriod := resourceWindow[len(resourceWindow)-1]
	// A REAL demo fixture ID (core/demo/fixtures/ec2.go's "web-prod-01")
	// — the by-ID fetch runs against the actual demo transport in both
	// lanes, so it must resolve as found, not a not-found Flash.
	const realID = "i-0a1b2c3d4e5f60001"
	c.Handle(messages.CostsLoaded{
		Query:    resourcePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(resourcePeriod, realID, 450.0)}},
		Requests: 1,
	})
	m2MoveHeadlessCursorToNewestColumn(c)                         // align the cursor with the newest-day cell the record above was planted at
	_, selectTasks := c.Apply(app.Action{Kind: app.ActionSelect}) // Enter on the RESOURCE_ID leaf
	var byIDPayload runtime.FetchByIDDetailPayload
	byIDFound := false
	for _, tr := range selectTasks {
		if p, ok := tr.Payload.(runtime.FetchByIDDetailPayload); ok {
			byIDPayload, byIDFound = p, true
		}
	}
	if !byIDFound {
		t.Fatal("headless precondition: RESOURCE_ID leaf Enter did not emit KindFetchByIDDetail")
	}
	headlessKind := byIDPayload.TargetType

	tui.Version = "1.0.2"
	m := newBlessedModel(t, "matrix-a-tui", "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	assertStackInSync(t, m, "after navigating to costs")

	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, ec2Service, 900.0)}},
		Requests: 1,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> USAGE_TYPE
	usageQuery := costs.Query{
		Granularity: costs.GranularityWeek.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {ec2Service}}},
	}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    usageQuery,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(usagePeriod, "USE1-BoxUsage:m5.large", 450.0)}},
		Requests: 1,
	})
	m = m2MoveTUICursorToNewestColumn(m)                 // drill RESOURCE_ID off the newest week (matching usagePeriod), not the col-0 oldest
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // -> RESOURCE_ID
	resourceQuery := costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionResourceID},
		Filter: costs.Filter{Equals: map[costs.Dimension][]string{
			costs.DimensionService:   {ec2Service},
			costs.DimensionUsageType: {"USE1-BoxUsage:m5.large"},
		}},
	}
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    resourceQuery,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(resourcePeriod, realID, 450.0)}},
		Requests: 1,
	})
	assertStackInSync(t, m, "before the by-ID drill")
	m = m2MoveTUICursorToNewestColumn(m) // align the cursor with the newest-day cell the record above was planted at

	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("TUI precondition: Enter on the RESOURCE_ID leaf returned a nil cmd")
	}
	msg := cmd()
	nav, ok := msg.(messages.Navigate)
	if !ok {
		t.Fatalf("TUI lane: by-ID fetch for a REAL demo instance ID = %#v (%T), want messages.Navigate (a found resource)", msg, msg)
	}
	nav.ReplaceCurrent = true
	m, _ = rootApplyMsg(m, nav)
	assertStackInSync(t, m, "after the by-ID resource jump")

	if headlessKind != "ec2" {
		t.Errorf("PARITY: headless lane's dispatched KindFetchByIDDetail targets %q, want \"ec2\"", headlessKind)
	}
	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, realID) {
		t.Errorf("PARITY: TUI lane — after a successful by-ID resource jump, the detail view does not show the resource's own ID (%s); headless lane's own dispatched target type was %q:\n%s", realID, headlessKind, plain)
	}
}

// A child frame's fetch returning zero at an eligible cell triggers a coarser
// re-fetch; both lanes must dispatch the same re-fetch task shape.

func TestCostsLaneParity_B_GranularityFallbackDelivery(t *testing.T) {
	// A real clock, not a pinned literal — for the same reason scenario A
	// gives: this scenario drives the TUI lane too, and the TUI lane's
	// costs clock is always time.Now(). A pinned month makes the headless
	// half build its window around that month while the TUI half builds
	// one around today's, so the two stop being the same scenario the
	// moment the calendar leaves the literal behind.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c := m2NewHeadlessController(t, "matrix-b-headless", now)
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 250)}},
		Requests: 1,
	})
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	childPayload, found := m2FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("headless precondition: drilling did not dispatch a fetch task")
	}
	_, fallbackTasks := c.Handle(messages.CostsLoaded{
		Query:  childPayload.Query,
		Grid:   costs.GridResult{Fetched: true},
		Window: childPayload.Window,
	})
	_, headlessFired := m2FindFetchCostsTask(fallbackTasks)

	tui.Version = "1.0.2"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 250)}},
		Requests: 1,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	childQuery := costs.Query{
		Granularity: costs.GranularityWeek.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon EC2"}}},
	}
	_, tuiCmd := rootApplyMsg(m, messages.CostsLoaded{Query: childQuery, Grid: costs.GridResult{Fetched: true}})

	if !headlessFired {
		t.Fatal("headless precondition: the N3 fallback did not fire at all (Handle returned no fetch task) — cannot compare lane parity on a scenario neither lane exercised")
	}
	if tuiCmd == nil {
		t.Error("PARITY: headless lane's Handle returned the N3 fallback's re-fetch task, but the TUI lane's Update returned a nil tea.Cmd for the identical delivery")
	}
}

// Ctrl+R on a warm costs screen dispatches a fetch for the active shape's open
// period in both lanes.

func TestCostsLaneParity_C_ForceRefresh(t *testing.T) {
	// Real clock, same rule as scenario A — the TUI lane driven below has
	// no other clock to share.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c := m2NewHeadlessController(t, "matrix-c-headless", now)
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})
	if c.Snapshot().Body.Costs.Loading {
		t.Fatal("headless precondition: still Loading after a full warming delivery")
	}
	headlessTasks := c.ForceRefreshCosts()
	_, headlessFound := m2FindFetchCostsTask(headlessTasks)

	// ctrl+r is bound to both "ctrl+r" and the raw control byte \x12 (keys.go);
	// the control-byte rune matches what a real terminal's ctrl+r produces.
	tui.Version = "1.0.2"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})
	_, tuiCmd := rootApplyMsg(m, rootSpecialKey('\x12'))

	if !headlessFound {
		t.Fatal("headless precondition: ForceRefreshCosts did not dispatch a fetch task for the warm open-period shape — cannot compare lane parity on a scenario neither lane exercised")
	}
	if tuiCmd == nil {
		t.Error("PARITY: headless lane's ForceRefreshCosts dispatched a fetch task, but the TUI lane's ctrl+r keystroke returned a nil tea.Cmd for the identical warm shape")
	}
}

// A not-found resource drill returns both lanes to the costs screen with a
// note, never stranded on the empty placeholder list.

// m2DrillHeadlessToStrandedByIDPlaceholder replays the SERVICE ->
// USAGE_TYPE -> RESOURCE_ID drill chain against the headless lane through
// Enter on the RESOURCE_ID leaf — pushing the placeholder and returning the
// Controller plus the dispatched KindFetchByIDDetail's own payload
// (TargetType+ID), so callers can construct whatever not-found signal
// they're testing without re-deriving the drill chain.
func m2DrillHeadlessToStrandedByIDPlaceholder(t *testing.T, profile string, now time.Time, bogusID string) (*app.Controller, runtime.FetchByIDDetailPayload) {
	t.Helper()
	const ec2Service = "Amazon Elastic Compute Cloud - Compute"
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c := m2NewHeadlessController(t, profile, now)
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, ec2Service, 900.0)}},
		Requests: 1,
	})
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE
	usagePayload, found := m2FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("headless precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageWeeks := costs.WindowWithin(period, costs.GranularityWeek, now)
	usagePeriod := usageWeeks[len(usageWeeks)-1] // newest week — the col-0 default (oldest week) falls outside the 14-day resource-drill clamp late in the month, refusing the drill below
	c.Handle(messages.CostsLoaded{
		Query:    usagePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(usagePeriod, "USE1-BoxUsage:m5.large", 450.0)}},
		Window:   usagePayload.Window,
		Requests: 1,
	})
	m2MoveHeadlessCursorToNewestColumn(c)                  // drill RESOURCE_ID off the newest week the record was planted at, not the col-0 oldest
	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> RESOURCE_ID
	resourcePayload, found := m2FindFetchCostsTask(tasks)
	if !found {
		t.Fatal("headless precondition: USAGE_TYPE -> RESOURCE_ID drill did not emit a fetch task")
	}
	// The NEWEST day, not window[0] — see TestCostsLaneParity_A's own
	// comment on the identical clamp/window mismatch.
	resourceWindow := costs.WindowWithin(usagePeriod, costs.GranularityDay, now)
	resourcePeriod := resourceWindow[len(resourceWindow)-1]
	c.Handle(messages.CostsLoaded{
		Query:    resourcePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(resourcePeriod, bogusID, 450.0)}},
		Requests: 1,
	})
	m2MoveHeadlessCursorToNewestColumn(c)                         // align the cursor with the newest-day cell the record above was planted at
	_, selectTasks := c.Apply(app.Action{Kind: app.ActionSelect}) // Enter on the RESOURCE_ID leaf
	var byIDPayload runtime.FetchByIDDetailPayload
	byIDFound := false
	for _, tr := range selectTasks {
		if p, ok := tr.Payload.(runtime.FetchByIDDetailPayload); ok {
			byIDPayload, byIDFound = p, true
		}
	}
	if !byIDFound {
		t.Fatal("headless precondition: RESOURCE_ID leaf Enter did not emit KindFetchByIDDetail")
	}
	return c, byIDPayload
}

func TestCostsLaneParity_D_DrillToResourceJump_NotFound(t *testing.T) {
	// A real clock, not a pinned literal — see TestCostsLaneParity_A's own
	// comment: the TUI lane's costs clock is always real wall-clock time.
	now := time.Now()
	const bogusID = "i-doesnotexistlaneparity1"

	c, byIDPayload := m2DrillHeadlessToStrandedByIDPlaceholder(t, "matrix-d-headless", now, bogusID)
	// executor.go's KindFetchByIDDetail case knows TargetType+ID from its payload
	// and constructs the typed messages.ByIDFetchFailed the TUI lane uses.
	c.Handle(messages.ByIDFetchFailed{
		TargetType: byIDPayload.TargetType,
		ID:         byIDPayload.ID,
		Reason:     fmt.Sprintf("%s %s not found", byIDPayload.TargetType, byIDPayload.ID),
	})

	headlessVS := c.Snapshot()

	// costsReview5DrillToStrandedByIDPlaceholder drills to its own fixed bogus
	// target, not bogusID.
	m, cmd := costsReview5DrillToStrandedByIDPlaceholder(t, "matrix-d-tui")
	msg := cmd()
	failed, ok := msg.(messages.ByIDFetchFailed)
	if !ok {
		t.Fatalf("TUI precondition: by-ID fetch for a bogus instance ID = %#v (%T), want messages.ByIDFetchFailed", msg, msg)
	}
	m, _ = rootApplyMsg(m, failed)
	assertStackInSync(t, m, "after the by-ID not-found result")

	if headlessVS.Body.Kind != app.BodyKindCosts {
		t.Errorf("PARITY: headless lane's Body.Kind after a not-found by-ID drill = %v, want BodyKindCosts (never stranded on the placeholder)", headlessVS.Body.Kind)
	}
	if headlessVS.Body.Costs == nil || headlessVS.Body.Costs.FooterNote == "" {
		t.Error("PARITY: headless lane landed back on costs but FooterNote (cs.ResourceRowNote folded in) is empty — the not-found reason must reach the user, not just silently pop")
	}
	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "TOTAL") {
		t.Errorf("PARITY: TUI lane — after a not-found by-ID drill, the costs screen's own grid (\"TOTAL\") is not showing; headless lane's own outcome was Body.Kind=%v, FooterNote=%q:\n%s", headlessVS.Body.Kind, headlessVS.Body.Costs.FooterNote, plain)
	}
}

// A costs screen opened before AWS connect completes recovers once
// messages.ClientsReady arrives: both lanes clear the pre-connect ErrorMsg and
// dispatch a retry fetch for the active shape.

func TestCostsLaneParity_E_PreConnectThenClientsReady(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	preConnectErr := "cost explorer: no client configured for this session"
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	// A fresh Controller's Core has nil Clients until a real ClientsReady lands.
	c := m2NewHeadlessController(t, "matrix-e-headless", now)
	c.Handle(messages.CostsLoaded{Query: q, Err: fmt.Errorf("%s", preConnectErr)})
	if got := c.Snapshot().Body.Costs.ErrorMsg; got == "" {
		t.Fatal("headless precondition: the pre-connect costs fetch failure must set ErrorMsg")
	}
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this headless controller's
	// session is never rotated.
	_, headlessTasks := c.Handle(messages.ClientsReady{Clients: demo.NewServiceClients(), Region: "us-east-1", Gen: 1})
	headlessVS := c.Snapshot()
	_, headlessRetried := m2FindFetchCostsTask(headlessTasks)

	// tui.New with no WithClients option has nil Clients() until
	// Init()/ClientsReady installs it.
	tui.Version = "1.0.2"
	m := newBlessedModel(t, "matrix-e-tui", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	m, _ = rootApplyMsg(m, messages.CostsLoaded{Query: q, Err: fmt.Errorf("%s", preConnectErr)})

	// Gen:1 — ConnectGen seeds at 1 (session.New()); this TUI model is never
	// rotated.
	m, tuiCmd := rootApplyMsg(m, messages.ClientsReady{Clients: demo.NewServiceClients(), Region: "us-east-1", Gen: 1})

	if headlessVS.Body.Costs.ErrorMsg != "" {
		t.Errorf("PARITY: headless lane's ErrorMsg %q did not clear after ClientsReady recovered the pre-connect failure", headlessVS.Body.Costs.ErrorMsg)
	}
	if !headlessRetried {
		t.Error("PARITY: headless lane's ClientsReady did not dispatch a retry fetch task for the pre-connect shape")
	}
	if tuiCmd == nil {
		t.Error("PARITY: headless lane's ClientsReady dispatched a retry fetch task, but the TUI lane's ClientsReady returned a nil tea.Cmd for the identical pre-connect failure")
	}
	assertStackInSync(t, m, "after ClientsReady recovers the pre-connect costs screen")
}

// Esc from a loading child drill frame clears the child's Loading state and
// renders the parent's data in both lanes, never blocked on the popped child's
// in-flight fetch.

func TestCostsLaneParity_F_EscFromLoadingChild(t *testing.T) {
	// Real clock, same rule as scenario A — the TUI lane driven below has
	// no other clock to share.
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	period := costs.Period{Start: start.Format("2006-01-02"), End: start.AddDate(0, 1, 0).Format("2006-01-02")}
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c := m2NewHeadlessController(t, "matrix-f-headless", now)
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 250)}},
		Requests: 1,
	})
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE child, dispatches a fetch, never resolved
	if _, found := m2FindFetchCostsTask(tasks); !found {
		t.Fatal("headless precondition: drilling did not dispatch a fetch task")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("headless precondition: the freshly-drilled child frame must be Loading before its own fetch resolves")
	}
	c.Apply(app.Action{Kind: app.ActionBack}) // Esc before the child's own fetch ever resolves
	headlessLoading := c.Snapshot().Body.Costs.Loading

	tui.Version = "1.0.2"
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	m, _ = rootApplyMsg(m, messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{m2FullMetricRecord(period, "Amazon EC2", 250)}},
		Requests: 1,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter)) // drills, dispatches a fetch, cmd never resolved
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	assertStackInSync(t, m, "after Esc from a loading child frame")

	if headlessLoading {
		t.Fatal("headless precondition: Loading still true after Back — cannot compare lane parity on a scenario the headless lane itself didn't clear")
	}
	if plain := stripANSI(rootViewContent(m)); !strings.Contains(plain, "TOTAL") {
		t.Errorf("PARITY: headless lane's Loading cleared after Esc from a loading child, but the TUI lane's own rendered grid (\"TOTAL\") is not showing — still appears blocked on the popped child's in-flight fetch:\n%s", plain)
	}
}

// An error messages.Flash whose text mentions the pending instance ID must not
// pop the headless placeholder; only a messages.ByIDFetchFailed matched to the
// exact pending target may pop it.

func TestCostsLaneParity_G_ErrorFlashMentioningPendingID_MustNotPopHeadless(t *testing.T) {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	const bogusID = "i-doesnotexistlaneparityg1"

	c, byIDPayload := m2DrillHeadlessToStrandedByIDPlaceholder(t, "matrix-g-headless", now, bogusID)

	c.Handle(messages.Flash{
		Text:    fmt.Sprintf("unrelated: some other background check failed for %s", byIDPayload.ID),
		IsError: true,
	})

	if vs := c.Snapshot(); vs.Body.Kind == app.BodyKindCosts {
		t.Errorf("a messages.Flash whose TEXT merely MENTIONS the pending instance ID (%s) popped the headless placeholder anyway — Body.Kind is back to BodyKindCosts after an ordinary error Flash with no typed messages.ByIDFetchFailed match; Controller.Handle's popAutoOpenSinglePlaceholderOnNotFound (X10) must be gone, replaced by the typed-match-only mechanism Scenario D pins", byIDPayload.ID)
	}
}
