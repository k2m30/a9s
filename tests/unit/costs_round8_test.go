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

// A bucket becomes immutable only when fetched at least 72h after its
// period's End (CE revises data 24-72h post-close). A bucket cached while
// its period was open stays refetchable after the period closes.

func TestCostsRound8_Item1_OpenPeriodBucket_RefetchableUntilPostClosureFetch_ThenImmutable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("round8-immutability-account")

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	julyPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"}
	julyMidMonth := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	august := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	muchLater := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	partial := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 500, Unit: "USD"}}}}
	s.Merge(q, partial, julyMidMonth)

	_, missing := s.Lookup(q, []costs.Period{julyPeriod}, august)
	if len(missing) == 0 {
		t.Error("July's bucket (cached while OPEN, mid-month) is reported as covered once the calendar rolls into August — lookupPeriod's `w.Closed(now)` check must key on bucket.FetchedAt vs the period's own End, not on whether NOW (the lookup call's own clock) has since passed the period")
	}

	final := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1234.5, Unit: "USD"}}}}
	s.Merge(q, final, august)

	gotRecs, missingAfter := s.Lookup(q, []costs.Period{julyPeriod}, august)
	if len(missingAfter) != 0 {
		t.Fatal("July's bucket still reported missing/stale immediately after the post-closure fetch merged")
	}
	if len(gotRecs) != 1 || gotRecs[0].Metrics[costs.MetricInvoice].Value != 1234.5 {
		t.Errorf("July's bucket after the post-closure fetch = %+v, want the FINAL 1234.5 record — the partial mid-July 500 snapshot must have been replaced, not preserved by Merge's closed-never-re-enters skip", gotRecs)
	}

	bogus := []costs.Record{{Period: julyPeriod, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 999, Unit: "USD"}}}}
	s.Merge(q, bogus, muchLater)

	finalRecs, _ := s.Lookup(q, []costs.Period{julyPeriod}, muchLater)
	if len(finalRecs) != 1 || finalRecs[0].Metrics[costs.MetricInvoice].Value != 1234.5 {
		t.Errorf("after the bucket became truly immutable (fetched post-closure at August), a later Merge call overwrote it: got %+v, want the 1234.5 record preserved", finalRecs)
	}
}

// core/web/server.go's drainBackgroundTasks drops a task whose TaskKey is
// already in flight, so distinct fetch shapes need distinct TaskKeys.

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

// drainBackgroundTasks' dedup set is unexported, so its algorithm
// (map[runtime.TaskKey]struct{} membership, "already running -> skip")
// is simulated against the TaskKeys two consecutive pivots produce.
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
	// SERVICE's own fetch is still in flight when REGION is requested.
	_, regionTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	admit(regionTasks)

	if admittedCostsFetches < 2 {
		t.Errorf("only %d of 2 distinct costs fetch shapes were admitted past the web dedup simulation — REGION's fetch was dropped because it shares SERVICE's still-in-flight TaskKey", admittedCostsFetches)
	}
}

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

// applyCostsSelect reads only cs.Store.Lookup, never task-in-flight state,
// so a shape whose fetch is dispatched but not landed shows zero records.
func TestCostsRound8_Item4_EnterWithNoSelectedRow_NoOps_LoadingShape(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

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

	_, pivotTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	if _, found := findFetchCostsTask(pivotTasks); !found {
		t.Fatal("precondition: REGION pivot did not dispatch its own fetch task — the reachable 'task in flight, not yet landed' state requires this")
	}

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != 0 {
		t.Fatalf("precondition: expected zero DISPLAYED rows for REGION (its fetch is still in flight, not landed), got %d: %+v", len(vs.Body.Costs.Rows), vs.Body.Costs.Rows)
	}

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

// CE's SERVICE dimension returns the full service name ("Amazon
// Relational Database Service", "Amazon Simple Storage Service"), never
// "RDS"/"S3".
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

// AWS's opt-in refusal from GetCostAndUsageWithResources arrives as
// AccessDeniedException ("Resource-level data granularity is an opt-in
// only feature...") -> a9saws.ErrCostsAccessDenied.
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

			// Loading gates Select (screen.Select's WaitForRows): the USAGE_TYPE
			// frame's own fetch must land before the next Enter.
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

// resourceDrillLiveShapedAccessDeniedErr replicates the error chain a
// GetCostAndUsageWithResources access-denied opt-in refusal arrives as,
// composing the wrapper types the AWS SDK itself uses:
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

// The refusal FooterNote carries the AWS API error message: the
// %v-formatted chain's "operation error ..." / "https response error ..."
// prefixes fill the footer line and truncate the actionable text.
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

// An empty (zero-record) fetch result is valid: a closed period CE reports
// zero spend for.
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

	older := window[len(window)-4]
	c.Handle(messages.CostsLoaded{Query: q, Grid: costs.GridResult{Fetched: true}, Window: []costs.Period{older}, Requests: 1})

	if got := c.Snapshot().Body.Costs.DataThrough; got != want {
		t.Errorf("DataThrough after an EMPTY-records delivery: got %q want %q (must stay pinned to the prior value — no stale regression)", got, want)
	}
}

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
