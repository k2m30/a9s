// costs_round8_test.go — Cost Explorer: external reviewer's sixth pass (5
// findings, all independently verified against current code with zero
// disproofs during scoring).
//
// package unit_test (not unit): every finding here is reachable via the
// headless app.Controller / pure core/costs package surface — no TUI
// helper is needed, so this file reuses costs_state_test.go's
// newCostsController/topDrill/fixedCostsNow/monthRecord and
// costs_interaction_test.go's findFetchCostsTask/baseServiceQuery/
// fullMetricRecord directly (same package).
package unit_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	smithy "github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/k2m30/a9s/v3/core/app"
	a9saws "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ===========================================================================
// Item 1 (P1, core/costs/store.go:~173 lookupPeriod) — a bucket cached
// while its period was OPEN must not become immutable the moment the
// calendar rolls past the period's End. Traced precisely:
//
//   - lookupPeriod's `w.Closed(now) || now.Sub(bucket.FetchedAt) <
//     openPeriodTTL` checks CLOSURE AGAINST THE CURRENT LOOKUP's now, never
//     against bucket.FetchedAt — a bucket fetched mid-July (while open)
//     passes `w.Closed(now)` the instant a LATER Lookup call happens in
//     August, regardless of how stale/partial that mid-July snapshot was.
//   - Merge's own `if batch[0].Period.Closed(now) { if _, exists :=
//     entry.Periods[pk]; exists { continue } }` has the SAME defect from the
//     other side: a legitimate post-closure re-fetch (now=August, period
//     now closed) finds the STALE mid-July entry already exists and skips
//     the overwrite — the partial snapshot never gets replaced. Both gates
//     must key on fetched-AFTER-closure (bucket.FetchedAt vs the period's
//     own End), not "is the period closed relative to THIS call's now".
//
// This single test pins the full life cycle: refetchable while merely
// open-and-stale, re-enters on the first post-closure fetch, THEN truly
// immutable against a bogus later overwrite.
//
// Reconciled (self-review C5): immutability now additionally requires a
// 72h settlement lag after the period's own End (CE revises data 24-72h
// post-close), not merely "fetched at/after End". This test's own
// "august" fetch time (Aug 5, 4 days / 96h after the July period's Aug 1
// End) already exceeds that lag, so its "then immutable" assertions stay
// valid unchanged under the narrower rule — see
// TestCostsSelfReview_C5a_DailyBucket_FetchedMorningAfter_StaysRefetchable
// / _C5b_..._Fetched4DaysAfter_IsImmutable (costs_selfreview_test.go) for
// the narrow boundary this test does not itself probe (a fetch WITHIN the
// 72h lag, which must stay refetchable, not immutable).
// ===========================================================================

func TestCostsRound8_Item1_OpenPeriodBucket_RefetchableUntilPostClosureFetch_ThenImmutable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("round8-immutability-account")

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	julyPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"}
	julyMidMonth := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	august := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	muchLater := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	// Merge a PARTIAL July bucket while July is still open (mid-month) —
	// exactly what a normal open-period fetch produces.
	partial := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 500, Unit: "USD"}}}}
	s.Merge(q, partial, julyMidMonth)

	// Roll the calendar into August — July is now closed. The mid-July
	// bucket was never fetched AFTER July closed, so it must still be
	// reported missing/stale (refetchable), not treated as permanently
	// valid just because "now" has moved past the period.
	_, missing := s.Lookup(q, []costs.Period{julyPeriod}, august)
	if len(missing) == 0 {
		t.Error("July's bucket (cached while OPEN, mid-month) is reported as covered once the calendar rolls into August — lookupPeriod's `w.Closed(now)` check must key on bucket.FetchedAt vs the period's own End, not on whether NOW (the lookup call's own clock) has since passed the period")
	}

	// A post-closure fetch (in August, after July closed) delivering the
	// FINAL, complete record MUST re-enter — overwrite the partial
	// mid-July snapshot — not be skipped by the "closed never re-enters"
	// rule, which today only checks "does an entry already exist", not
	// "was that entry ITSELF already fetched after closure".
	final := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1234.5, Unit: "USD"}}}}
	s.Merge(q, final, august)

	gotRecs, missingAfter := s.Lookup(q, []costs.Period{julyPeriod}, august)
	if len(missingAfter) != 0 {
		t.Fatal("July's bucket still reported missing/stale immediately after the post-closure fetch merged")
	}
	if len(gotRecs) != 1 || gotRecs[0].Metrics[costs.MetricInvoice].Value != 1234.5 {
		t.Errorf("July's bucket after the post-closure fetch = %+v, want the FINAL 1234.5 record — the partial mid-July 500 snapshot must have been replaced, not preserved by Merge's closed-never-re-enters skip", gotRecs)
	}

	// NOW it is truly immutable: a bucket that WAS fetched after its own
	// period closed must reject a further overwrite attempt, even much
	// later — this is the one case the existing skip rule is correctly
	// designed for.
	bogus := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 999, Unit: "USD"}}}}
	s.Merge(q, bogus, muchLater)

	finalRecs, _ := s.Lookup(q, []costs.Period{julyPeriod}, muchLater)
	if len(finalRecs) != 1 || finalRecs[0].Metrics[costs.MetricInvoice].Value != 1234.5 {
		t.Errorf("after the bucket became truly immutable (fetched post-closure at August), a later Merge call overwrote it: got %+v, want the 1234.5 record preserved", finalRecs)
	}
}

// ===========================================================================
// Item 2 (P2, core/app/costs_state.go:~367 ensureCostsShapeFetched) —
// distinct fetch shapes need distinct TaskKeys. Traced precisely:
// ensureCostsShapeFetched hardcodes Key: runtime.TaskKey{Kind:
// KindFetchCosts, Scope: "costs"} for EVERY query shape, and
// core/web/server.go's sessionEntry.inFlight (map[runtime.TaskKey]
// struct{}) silently drops any drainBackgroundTasks task whose Key is
// already tracked (drainBackgroundTasks: "if _, running :=
// entry.inFlight[t.Key]; running { continue }") — a shape switch mid-flight
// (e.g. SERVICE -> REGION before SERVICE's fetch completes) gets its
// second fetch dropped, leaving the UI stuck Loading for the new shape.
// ===========================================================================

func TestCostsRound8_Item2_DistinctFetchShapes_GetDistinctTaskKeys(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	_, serviceTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	servicePayload, found := findFetchCostsTask(serviceTasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	var serviceKey runtime.TaskKey
	for _, tr := range serviceTasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			serviceKey = tr.Key
		}
	}

	_, regionTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	regionPayload, found := findFetchCostsTask(regionTasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	var regionKey runtime.TaskKey
	for _, tr := range regionTasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			regionKey = tr.Key
		}
	}

	if servicePayload.Query.CacheKey() == regionPayload.Query.CacheKey() {
		t.Fatal("precondition: SERVICE and REGION pivots produced the SAME CacheKey — this test needs two genuinely different shapes")
	}
	if serviceKey == regionKey {
		t.Errorf("SERVICE and REGION pivot fetches share the identical TaskKey %+v — must derive Scope from the query shape (CacheKey+range), not a constant \"costs\" string, or the web inFlight dedup silently drops the second fetch while the first is still outstanding", serviceKey)
	}
}

// TestCostsRound8_Item2_WebDedupSimulation_DistinctShapesBothAdmitted
// mirrors core/web/server.go's drainBackgroundTasks dedup exactly
// (map[runtime.TaskKey]struct{} membership, "already running -> skip") —
// that struct itself is unexported and unreachable from tests/unit, so this
// simulates its documented algorithm against the REAL TaskKeys two
// consecutive pivots actually produce.
func TestCostsRound8_Item2_WebDedupSimulation_DistinctShapesBothAdmitted(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	inFlight := map[runtime.TaskKey]struct{}{}
	var admittedCostsFetches int

	admit := func(tasks []runtime.TaskRequest) {
		for _, tr := range tasks {
			if tr.Key.Kind != runtime.KindFetchCosts {
				continue
			}
			if _, running := inFlight[tr.Key]; running {
				continue
			}
			inFlight[tr.Key] = struct{}{}
			admittedCostsFetches++
		}
	}

	_, serviceTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	admit(serviceTasks)
	// SERVICE's own fetch is still tracked as in-flight (never completed)
	// when REGION is requested — the exact race the live symptom describes.
	_, regionTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	admit(regionTasks)

	if admittedCostsFetches < 2 {
		t.Errorf("only %d of 2 distinct costs fetch shapes were admitted past the web dedup simulation — REGION's fetch was dropped because it shares SERVICE's still-in-flight TaskKey", admittedCostsFetches)
	}
}

// ===========================================================================
// Item 3 (P2, ~656 applyCostsSelect) — Enter on a mapped resource row must
// OPEN the resource detail view, not just set a footer note. Traced
// precisely: applyCostsSelect's nextDim=="" branch only ever sets
// cs.ResourceRowNote and returns nil — it never emits a navigation task.
// The seam it must use (mirrors HandleRelatedNavigate's count-1
// auto-navigate, core/runtime/handlers_related.go:142-148):
// runtime.TaskRequest{Key: TaskKey{Kind: KindFetchByIDDetail, Scope:
// "ec2"}, Payload: FetchByIDDetailPayload{TargetType: "ec2", ID:
// resourceID}}, gated by resource.GetFetchByIDs("ec2") != nil.
// core/aws/catalog_compute.go's EC2 ResourceTypeDef has no
// FetchByIDs: entry today (confirmed directly — "ami"/"ebs-snap" in the
// SAME file do), so this is a two-part fix: register EC2's FetchByIDs,
// then wire applyCostsSelect to emit the task.
//
// Reconciliation: supersedes round5's FooterNote-only pin for the
// SUPPORTED (EC2) case — costs_round5_test.go's
// TestCostsRound5_E_ResourceRowEnter_SupportedService_NotSilentNoOp is
// rewritten below to assert the KindFetchByIDDetail task instead. The
// UNSUPPORTED case needs NO further reconciliation: round7's exact-EC2
// ResourceDrillAllowed gate already made "reach RESOURCE_ID with a
// non-EC2 service" structurally unreachable, so
// TestCostsRound5_E_ResourceRowEnter_UnsupportedService_DefinedBehavior_NotSilentNoOp
// (already rewritten in that round to pin the earlier USAGE_TYPE ->
// RESOURCE_ID refusal) already matches "note stays for unsupported" and
// needs no change.
// ===========================================================================

// Reuses costs_round5_test.go's round5DrillToResourceRow directly (same
// package, unit_test) rather than duplicating it — that helper already
// drills SERVICE -> USAGE_TYPE -> RESOURCE_ID and seeds exactly the one
// row this test needs.
func TestCostsRound8_Item3_ResourceRowEnter_SupportedService_EmitsFetchByIDDetailTask(t *testing.T) {
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001" // core/demo/fixtures/ec2.go's "web-prod-01"
	c := round5DrillToResourceRow(t, "Amazon Elastic Compute Cloud - Compute", demoEC2InstanceID)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // Enter on the resource row

	var navTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchByIDDetail {
			navTask = &tasks[i]
		}
	}
	if navTask == nil {
		t.Fatal("Enter on a mapped EC2 resource row emitted no KindFetchByIDDetail task — it must navigate to the real resource detail view (the same seam count-1 related-panel drills use), not just set a footer note")
	}
	if navTask.Key.Scope != "ec2" {
		t.Errorf("KindFetchByIDDetail Key.Scope = %q, want %q", navTask.Key.Scope, "ec2")
	}
	payload, ok := navTask.Payload.(runtime.FetchByIDDetailPayload)
	if !ok {
		t.Fatalf("KindFetchByIDDetail Payload type = %T, want runtime.FetchByIDDetailPayload", navTask.Payload)
	}
	if payload.TargetType != "ec2" {
		t.Errorf("Payload.TargetType = %q, want %q", payload.TargetType, "ec2")
	}
	if payload.ID != demoEC2InstanceID {
		t.Errorf("Payload.ID = %q, want %q", payload.ID, demoEC2InstanceID)
	}
}

// ===========================================================================
// Item 4 (P2, ~644 applyCostsSelect) — Enter with no selected row (loading
// shape, or every row display-filtered) must no-op: no drill frame, no
// SERVICE=[""] filter, no fetch task, no billed request. Traced precisely:
// rowKey stays "" when cur.Cursor.Row doesn't resolve against
// liveCostGrid's (already display-filtered, per round7 item 6) Rows, but
// nothing gates on rowKey=="" before `pinned.Equals[cur.RowDim] =
// []string{rowKey}` and the frame push / fetch dispatch that follows.
// ===========================================================================

// TestCostsRound8_Item4_EnterWithNoSelectedRow_NoOps_LoadingShape was
// reconciled after an investigation dispatch: the ORIGINAL construction
// (newCostsController, then Enter immediately — never calling
// EnsureCostsFetch/ensureCostsShapeFetched at all) is not reachable via any
// production adapter. Traced precisely: internal/tui/
// runtime_adapter_navigate.go's NavigateKindPushCosts case and core/app/
// navigate.go's applyNavResult NavigateKindPushCosts case (the TUI and
// headless/web entry points respectively — the only two EnsureCostsState
// call sites in the whole repo) BOTH call EnsureCostsState/ensureCostsState
// and then unconditionally call EnsureCostsFetch/ensureCostsShapeFetched in
// the very next statement — no adapter ever leaves the root shape
// completely un-dispatched.
//
// A REACHABLE equivalent does exist, though, and it hits the identical
// undefended branch in applyCostsSelect: every pivot/zoom/metric/drill
// handler (ensureCostsShapeFetched's ~8 call sites in costs_state.go) fires
// a fetch task for the NEW shape the instant it becomes current, but that
// task can still be in flight (dispatched, not yet landed) when Enter is
// pressed. From applyCostsSelect's own vantage point — it only ever
// inspects cs.Store.Lookup, never task-in-flight state — "fetch dispatched
// but not landed" and "fetch never dispatched" are byte-for-byte
// indistinguishable: both show zero records for the shape. So the
// reachable pivot-then-Enter-before-landing sequence below exercises the
// SAME broken branch, and per applyCostsSelect's own doc comment
// (core/app/costs_state.go:684-689) this is an ACKNOWLEDGED, deliberately
// unfixed gap pending a has-this-shape-ever-been-requested tri-state — this
// test is EXPECTED to stay red, not a false positive.
func TestCostsRound8_Item4_EnterWithNoSelectedRow_NoOps_LoadingShape(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	// Seed the SERVICE shape so the store has SOME landed data (ruling out
	// the "nothing has EVER been fetched, in the store or in flight"
	// degenerate case entirely).
	_, seedTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	seedPayload, found := findFetchCostsTask(seedTasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    seedPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Requests: 1,
	})

	// Pivot to REGION — a genuinely uncovered shape. This is the real
	// production sequence: the pivot handler immediately dispatches
	// REGION's own fetch task (Loading+task), matching every adapter's
	// contract above.
	_, pivotTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	if _, found := findFetchCostsTask(pivotTasks); !found {
		t.Fatal("precondition: REGION pivot did not dispatch its own fetch task — the reachable 'task in flight, not yet landed' state requires this")
	}

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 0 {
		t.Fatalf("precondition: expected zero DISPLAYED rows for REGION (its fetch is still in flight, not landed), got %d: %+v", len(vs.Body.Costs.Rows), vs.Body.Costs.Rows)
	}

	// Enter BEFORE REGION's CostsLoaded ever lands.
	stackBefore := len(c.GetCostsDrillStack())
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if got := len(c.GetCostsDrillStack()); got != stackBefore {
		t.Errorf("Enter with no selected row (REGION's fetch dispatched but not yet landed) changed drill stack depth: got %d want %d — no frame should be pushed", got, stackBefore)
	}
	if _, found := findFetchCostsTask(tasks); found {
		t.Error("Enter with no selected row emitted a KindFetchCosts task — a billed request for a bogus shape")
	}
	stack := c.GetCostsDrillStack()
	top := stack[len(stack)-1]
	if vals := top.Filter.Equals[top.RowDim]; len(vals) == 1 && vals[0] == "" {
		t.Errorf("Enter with no selected row pinned an empty-string filter value %v on RowDim %q — must not drill at all when nothing is actually selected", vals, top.RowDim)
	}
}

func TestCostsRound8_Item4_EnterWithNoSelectedRow_NoOps_AllRowsDisplayFiltered(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "AWS Support (Business)", 0.004)}}, // sub-cent noise, hidden by the display-level zero-row filter
		Requests: 1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 0 {
		t.Fatalf("precondition: expected zero DISPLAYED rows (the sole seeded row is sub-cent noise, filtered out), got %d: %+v", len(vs.Body.Costs.Rows), vs.Body.Costs.Rows)
	}

	stackBefore := len(c.GetCostsDrillStack())
	_, selectTasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if got := len(c.GetCostsDrillStack()); got != stackBefore {
		t.Errorf("Enter with every row display-filtered changed drill stack depth: got %d want %d", got, stackBefore)
	}
	if _, found := findFetchCostsTask(selectTasks); found {
		t.Error("Enter with every row display-filtered emitted a KindFetchCosts task")
	}
}

// ===========================================================================
// Item 5 (P3, controller command path) — :costs/:ce must work through
// Controller.Apply(ActionCommand), not only the TUI's own colon path.
// Traced precisely: resource.IsCostsCommand is referenced ONLY from
// internal/tui/app_input.go:326 (the TUI's private colon-mode key
// handler); Controller.handleActionCommand's switch (core/app/
// actions_view.go) has no "costs"/"ce" case at all and falls through to
// resource.FindResourceType(a.Arg), which correctly returns nil for the
// synthetic "costs" entry (it is deliberately excluded from
// AllResourceTypes()) — so the web command palette (and any other
// headless caller) can never reach the costs screen via ":costs"/":ce"
// today.
// ===========================================================================

func TestCostsRound8_Item5_ControllerActionCommand_CostsAndCE_NavigateToCostsScreen(t *testing.T) {
	for _, arg := range []string{"costs", "ce"} {
		t.Run(arg, func(t *testing.T) {
			c := newTestController(t) // app_controller_test.go's blessed helper — see qa_controller_construction_discipline_test.go

			vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: arg})
			if vs.Body.Kind != app.BodyKindCosts {
				t.Errorf(":%s via Controller.Apply(ActionCommand) did not navigate to the costs screen — Body.Kind = %q, want %q", arg, vs.Body.Kind, app.BodyKindCosts)
			}
		})
	}
}

// ===========================================================================
// Closure-wave new pins (5 findings, appended to round 8's own file per the
// same dispatch).
// ===========================================================================

// (a) Demo fixture service labels must carry REAL Cost Explorer names, not
// AWS-console shorthands: CE's SERVICE dimension always returns the full
// service name ("Amazon Relational Database Service", "Amazon Simple
// Storage Service"), never "RDS"/"S3" — the demo transport's fixture data
// must match that shape so a11 demo mode exercises the exact same label
// space as a live account.
func TestCostsDemo_ServiceLabels_UseRealCENames_NotShorthands(t *testing.T) {
	client := newDemoCostsClient()
	q := demoCostsQuery(costs.Filter{}, costs.DimensionService)

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage(SERVICE): %v", err)
	}
	if len(result.Records) == 0 {
		t.Fatal("precondition: FetchCostAndUsage(SERVICE) returned no records")
	}

	services := map[string]struct{}{}
	for _, rec := range result.Records {
		for _, k := range rec.Keys {
			services[k] = struct{}{}
		}
	}

	for _, shorthand := range []string{"RDS", "S3"} {
		if _, found := services[shorthand]; found {
			t.Errorf("demo fixture service labels include the shorthand %q — CE never returns a bare console abbreviation for the SERVICE dimension, only the full service name", shorthand)
		}
	}
	for _, wantSubstr := range []string{"Relational Database Service", "Simple Storage Service"} {
		found := false
		for s := range services {
			if strings.Contains(s, wantSubstr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("demo fixture service labels %v do not include any name containing %q", services, wantSubstr)
		}
	}
}

// (b) A classified resource-drill refusal (ANY classified costs error —
// live tmux verification on a real account caught the initial pin too
// narrow: AWS's actual opt-in refusal from GetCostAndUsageWithResources
// arrives as AccessDeniedException ("Resource-level data granularity is an
// opt-in only feature...") -> a9saws.ErrCostsAccessDenied, not
// ErrCostsDataUnavailable) must surface as a FooterNote on the CURRENT
// costs screen, carry the underlying error message, pop the just-pushed
// RESOURCE_ID frame back to its parent, and never set the blocking
// ErrorMsg that blanks the whole grid. Non-resource-shaped queries are
// UNAFFECTED and keep the blocking ErrorMsg — already covered by
// TestCostsBody_Snapshot_ErrorMsg_NeverEmptyGrid (costs_body_test.go),
// which uses a root SERVICE-shaped query with the same ErrCostsAccessDenied
// sentinel and asserts ErrorMsg gets set; that control is untouched here.
//
// Traced precisely: ApplyCostsLoaded (core/app/costs_state.go) only
// guards the FooterNote/pop path with
// `isResourceDrillQuery(ev.Query) && errors.Is(ev.Err, awsclient.ErrCostsDataUnavailable)`
// — ErrCostsAccessDenied falls through to the unconditional
// `cs.ErrorMsg = ev.Err.Error()` branch even for a RESOURCE_ID-shaped query.
func TestCostsRound8_ResourceDrillRefusal_SurfacesAsFooterNote_NotBlockingErrorMsg(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"AccessDeniedException_OptInRefusal", a9saws.ErrCostsAccessDenied},
		{"DataUnavailableException", a9saws.ErrCostsDataUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCostsController(t, fixedCostsNow)
			c.Handle(messages.CostsLoaded{
				Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
				Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
				Requests: 1,
			})
			_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // SERVICE -> USAGE_TYPE

			// Loading gates Select unconditionally now (screen.Select's
			// WaitForRows) — the USAGE_TYPE frame's own fetch must land
			// before the next Enter.
			drill1Payload, found := findFetchCostsTask(drill1Tasks)
			if !found {
				t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
			}
			usageTypeTop := topDrill(t, c)
			c.Handle(messages.CostsLoaded{
				Query:    drill1Payload.Query,
				Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(usageTypeTop.Window[0], "USE1-BoxUsage:m5.large", 1200.0)}},
				Window:   usageTypeTop.Window,
				Requests: 1,
			})

			_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // USAGE_TYPE -> RESOURCE_ID
			payload, found := findFetchCostsTask(tasks)
			if !found {
				t.Fatal("precondition: RESOURCE_ID drill did not emit a fetch task")
			}
			if depth := len(c.GetCostsDrillStack()); depth != 3 {
				t.Fatalf("precondition: expected drill depth 3 (RESOURCE_ID just pushed), got %d", depth)
			}

			c.Handle(messages.CostsLoaded{
				Grid: costs.GridResult{Fetched: true}, Query: payload.Query,
				Err:      tt.err,
				Requests: 1,
			})

			vs := c.Snapshot()
			if vs.Body.Kind != app.BodyKindCosts {
				t.Fatalf("resource-drill refusal (%v) navigated away from the costs screen entirely — Body.Kind = %q, want %q (no pushed error screen)", tt.err, vs.Body.Kind, app.BodyKindCosts)
			}
			if vs.Body.Costs.ErrorMsg != "" {
				t.Errorf("resource-drill refusal (%v) set the blocking ErrorMsg %q — ANY classified costs error at the resource-drill level must surface as a lighter FooterNote refusal instead, not blank the whole grid", tt.err, vs.Body.Costs.ErrorMsg)
			}
			if vs.Body.Costs.FooterNote == "" {
				t.Errorf("resource-drill refusal (%v) produced no FooterNote — must surface an observable, human-readable refusal note carrying the underlying error, not a silent no-op", tt.err)
			} else if !strings.Contains(vs.Body.Costs.FooterNote, tt.err.Error()) {
				t.Errorf("FooterNote %q does not carry the underlying error message %q", vs.Body.Costs.FooterNote, tt.err.Error())
			}
			if depth := len(c.GetCostsDrillStack()); depth != 2 {
				t.Errorf("resource-drill refusal (%v) did not pop the just-pushed RESOURCE_ID frame back to its parent: drill depth got %d want 2", tt.err, depth)
			}
		})
	}
}

// resourceDrillLiveShapedAccessDeniedErr replicates the EXACT error chain a
// real GetCostAndUsageWithResources access-denied opt-in refusal arrives as
// on a live account (caught by tmux smoke on two real profiles), by
// composing the same wrapper types the AWS SDK itself uses:
// smithy.OperationError -> aws-sdk-go-v2's own awshttp.ResponseError (its
// Error() says "https response error", distinct from smithy's own
// unexported-lookalike "http response error") -> smithy.GenericAPIError,
// then wrapped exactly as core/aws/costs.go's classifyCostsError wraps
// it: fmt.Errorf("%w: %w", ErrCostsAccessDenied, err).
func resourceDrillLiveShapedAccessDeniedErr(apiMessage string) error {
	apiErr := &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: apiMessage,
	}
	respErr := &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: 403}},
			Err:      apiErr,
		},
		RequestID: "req-shaped-costs-round8-fixture",
	}
	opErr := &smithy.OperationError{
		ServiceID:     "Cost Explorer",
		OperationName: "GetCostAndUsageWithResources",
		Err:           respErr,
	}
	return fmt.Errorf("%w: %w", a9saws.ErrCostsAccessDenied, opErr)
}

// TestCostsRound8_ResourceDrillRefusal_FooterNote_CarriesAPIMessage_NotWrapperNoise
// pins the live-smoke finding: the refusal FooterNote must surface the
// actionable AWS API error MESSAGE ("Resource-level data granularity is an
// opt-in only feature...") rather than the raw %v-formatted error chain,
// whose "operation error ..." / "https response error ..." wrapper prefixes
// consume the footer line and truncate the actionable text away before it
// is ever seen. costsResourceDrillRefusalNote (core/app/costs_state.go)
// currently does exactly that: fmt.Sprintf("...: %v", err) on the full
// chain — this test is RED against that.
func TestCostsRound8_ResourceDrillRefusal_FooterNote_CarriesAPIMessage_NotWrapperNoise(t *testing.T) {
	const optInMessage = "Resource-level data granularity is an opt-in only feature. You can enable this feature from the PAYER account's Cost Management preferences"
	liveShapedErr := resourceDrillLiveShapedAccessDeniedErr(optInMessage)

	c := newCostsController(t, fixedCostsNow)
	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Requests: 1,
	})
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // SERVICE -> USAGE_TYPE
	drill1Payload, found := findFetchCostsTask(drill1Tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageTypeTop := topDrill(t, c)
	c.Handle(messages.CostsLoaded{
		Query:    drill1Payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(usageTypeTop.Window[0], "USE1-BoxUsage:m5.large", 1200.0)}},
		Window:   usageTypeTop.Window,
		Requests: 1,
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // USAGE_TYPE -> RESOURCE_ID
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: RESOURCE_ID drill did not emit a fetch task")
	}

	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: payload.Query,
		Err:      liveShapedErr,
		Requests: 1,
	})

	note := c.Snapshot().Body.Costs.FooterNote
	if note == "" {
		t.Fatal("resource-drill refusal produced no FooterNote")
	}
	if !strings.Contains(note, optInMessage) {
		t.Errorf("FooterNote %q does not contain the underlying API error MESSAGE %q — the actionable AWS text must survive classification, not just err.Error()'s wrapper chain", note, optInMessage)
	}
	if strings.Contains(note, "operation error") {
		t.Errorf("FooterNote %q contains the raw smithy.OperationError wrapper fragment %q — this is transport noise that must never reach the footer, it consumes the line and pushes the actionable text past the truncation point", note, "operation error")
	}
	if strings.Contains(note, "https response error") {
		t.Errorf("FooterNote %q contains the raw awshttp.ResponseError wrapper fragment %q — this is transport noise that must never reach the footer", note, "https response error")
	}
}

// (c) An empty (zero-record) fetch result is a genuine, valid outcome (a
// closed period CE reports zero spend for) — it must not regress
// DataThrough back to a stale/earlier value. Traced against the existing
// D3 family (costs_interaction_test.go): those tests cover out-of-order
// NON-EMPTY deliveries; this covers the EMPTY-delivery case specifically.
func TestCostsRound8_DataThrough_UnchangedByEmptyResult(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	q := baseServiceQuery()

	newer := window[len(window)-2] // a closed period
	c.Handle(messages.CostsLoaded{Query: q, Grid: costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newer, "Amazon EC2", 1200.0)}}, Requests: 1})

	end, err := time.Parse("2006-01-02", newer.End)
	if err != nil {
		t.Fatalf("parsing period end %q: %v", newer.End, err)
	}
	want := end.AddDate(0, 0, -1).Format("2006-01-02")
	if got := c.Snapshot().Body.Costs.DataThrough; got != want {
		t.Fatalf("precondition: DataThrough after the first (non-empty) delivery = %q, want %q", got, want)
	}

	// A second delivery, for an OLDER period, with ZERO records — a real
	// empty result, not an error — must not regress DataThrough backward
	// or clear it.
	older := window[len(window)-4]
	c.Handle(messages.CostsLoaded{Query: q, Grid: costs.GridResult{Fetched: true}, Window: []costs.Period{older}, Requests: 1})

	if got := c.Snapshot().Body.Costs.DataThrough; got != want {
		t.Errorf("DataThrough after an EMPTY-records delivery: got %q want %q (must stay pinned to the prior value — no stale regression)", got, want)
	}
}

// (d) The open-period '*' marker (already pinned at MONTH granularity by
// TestRenderCosts_OpenPeriodColumnGetsAsteriskSuffix) must ALSO behave
// correctly at DAY granularity: a fully closed month's day columns must
// carry no '*' at all, even though day-length periods are individually
// much shorter-lived than the month they tile. Traced against
// core/app/costs_body.go:77's `Open: !p.Closed(cs.Now)` — this is
// granularity-agnostic by construction, so this is a permanent regression
// guard against ever special-casing Open by Granularity instead of by each
// period's own Closed(now).
func TestCostsRound8_DayGranularity_ClosedMonth_ColumnsCarryNoOpenAsterisk(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	// Cursor starts on fixedCostsNow's own (July, OPEN) month — move it one
	// column left to June (CLOSED) before zooming in, so MONTH->WEEK anchors
	// on June, not July.
	c.Apply(app.Action{Kind: app.ActionScrollLeft})
	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // MONTH -> WEEK, anchored on June
	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // WEEK -> DAY

	dayTop := topDrill(t, c)
	if dayTop.Granularity != costs.GranularityDay {
		t.Fatalf("precondition: expected Granularity day, got %q", dayTop.Granularity)
	}
	for _, p := range dayTop.Window {
		if !p.Closed(fixedCostsNow) {
			t.Fatalf("precondition broken: day period %+v is not closed relative to fixedCostsNow (%v) — this test needs an entirely-closed-month day window; the cursor-left-then-zoom-in navigation did not land in June", p, fixedCostsNow)
		}
	}

	vs := c.Snapshot()
	if len(vs.Body.Costs.Columns) == 0 {
		t.Fatal("precondition: zero day columns rendered")
	}
	for _, col := range vs.Body.Costs.Columns {
		if col.Open {
			t.Errorf("day-granularity column %q (within a fully closed month) is marked Open — the '*' open-period marker must never appear on a closed month's days", col.Label)
		}
	}
}

// (e) The demo fixture must be multi-account: a LINKED_ACCOUNT pivot
// against the demo transport must return at least 2 distinct accounts
// (name-(id)-labelled rows, via costs.ApplyRowAttrs — already pinned
// generically by TestCostsReview2_R8_LinkedAccountPivot_RowLabel_NameParensID),
// not the single-account fixture that exists today
// (fixtures.CostsDemoAccountID, "123456789012", is the ONLY account the
// demo transport currently ever returns for a LINKED_ACCOUNT group-by).
func TestCostsDemo_LinkedAccountPivot_ReturnsMultipleAccounts(t *testing.T) {
	client := newDemoCostsClient()
	q := demoCostsQuery(costs.Filter{}, costs.DimensionLinkedAccount)

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage(LINKED_ACCOUNT): %v", err)
	}
	if len(result.Records) == 0 {
		t.Fatal("precondition: FetchCostAndUsage(LINKED_ACCOUNT) returned no records")
	}

	accounts := map[string]struct{}{}
	for _, rec := range result.Records {
		for _, k := range rec.Keys {
			accounts[k] = struct{}{}
		}
	}
	if len(accounts) < 2 {
		t.Errorf("LINKED_ACCOUNT pivot against the demo fixture returned %d distinct account(s) %v, want >= 2 — the demo fixture must be multi-account so this pivot is exercised realistically, not degenerate to a single always-same row", len(accounts), accounts)
	}
}
