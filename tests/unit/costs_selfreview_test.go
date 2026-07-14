// costs_selfreview_test.go — Cost Explorer: pre-Codex self-review pass
// (C2-C12, C1/resource-drill-refusal already pinned in costs_round8_test.go).
// Every item below is code-cited: traced against current source before
// writing, not guessed from the review prose alone.
//
// package unit_test (not unit): every finding here is reachable via the
// headless app.Controller / pure internal/costs / internal/aws surface —
// no TUI helper is needed (C2 explicitly requires NO TUI adapter
// involvement) — so this file reuses costs_state_test.go's
// newCostsController/topDrill/fixedCostsNow/monthRecord and
// costs_interaction_test.go's findFetchCostsTask/baseServiceQuery/
// fullMetricRecord directly (same package), same convention as
// costs_round8_test.go.
package unit_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/app"
	a9saws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ===========================================================================
// C2 (P1) — web/headless resource jump. Traced precisely: applyCostsSelect's
// RESOURCE_ID branch (internal/app/costs_state.go) returns a BARE
// runtime.TaskRequest{Key: KindFetchByIDDetail, ...} with no placeholder-
// list/AutoOpenSingle wiring. autoOpenSingleDetail (internal/app/handle.go)
// — the seam the related panel's own by-ID single-target drill uses
// (navigate.go:495-506's placeholder list + AutoOpenSingle) — only fires
// when the TOP screen is ScreenResourceList or ScreenChildList (handle.go
// line 322); the costs screen is neither, so nothing on the web/headless
// lane ever converts a successful KindFetchByIDDetail delivery into a
// detail navigation. The TUI's own special-cased m.fetchByIDDetail (fixed
// in an earlier round) is TUI-only and never reached here.
// ===========================================================================

func TestCostsSelfReview_C2_WebResourceJump_HeadlessReachesEC2Detail(t *testing.T) {
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001"
	c := newCostsController(t, fixedCostsNow)
	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Requests: 1,
	})
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // SERVICE -> USAGE_TYPE

	// Loading gates Select unconditionally now (screen.Select's
	// WaitForRows) — the USAGE_TYPE frame's own fetch must land before the
	// next Enter.
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
	resourceTop := topDrill(t, c)
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{{Period: resourceTop.Window[0], Keys: []string{demoEC2InstanceID}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 3, Unit: "USD"}}}}},
		Requests: 1,
	})

	// Enter on the resource row — headless/web lane only (Controller.Apply,
	// no TUI Model in this test at all).
	_, selectTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	var byIDTask *runtime.TaskRequest
	for i := range selectTasks {
		if selectTasks[i].Key.Kind == runtime.KindFetchByIDDetail {
			byIDTask = &selectTasks[i]
		}
	}
	if byIDTask == nil {
		t.Fatal("precondition: Enter on the resource row did not emit KindFetchByIDDetail")
	}

	// Deliver what the executor's KindFetchByIDDetail fetch produces on
	// success (ExecuteTask returns messages.ResourcesLoaded for this task
	// kind — runtime_adapter_related.go's own doc comment confirms this).
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: demoEC2InstanceID, Name: "web-prod-01", Fields: map[string]string{"instance_id": demoEC2InstanceID}}},
	})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindDetail {
		t.Fatalf("web/headless lane: Enter on a costs resource row + its fetch delivery did not reach the EC2 detail view — Body.Kind = %q, want %q (no TUI adapter involved in this test)", vs.Body.Kind, app.BodyKindDetail)
	}
	// detailFrameTitleLocked prefers Resource.Name over Resource.ID whenever
	// Name is non-empty (internal/app/detail_state.go) — this is the global
	// convention every detail screen follows, not something costs-specific
	// should special-case. The injected fixture sets Name "web-prod-01", so
	// the title carries that, not the raw ID.
	const wantName = "web-prod-01"
	if !strings.Contains(vs.FrameTitle, wantName) {
		t.Errorf("detail FrameTitle %q does not reference the resolved resource name %q", vs.FrameTitle, wantName)
	}
	// The resolved ID itself is still verifiable — just from the detail
	// body's own field rows, not the title (detailFrameTitleLocked's
	// Name-over-ID preference is a rendering choice, not proof the wrong
	// resource was resolved).
	idFound := false
	for _, f := range vs.Body.Detail.Fields {
		if strings.Contains(f.Value, demoEC2InstanceID) {
			idFound = true
			break
		}
	}
	if !idFound {
		t.Errorf("detail body Fields do not contain the resolved resource ID %q anywhere (want it in at least one FieldRow.Value): %+v", demoEC2InstanceID, vs.Body.Detail.Fields)
	}
}

// ===========================================================================
// C3 (P1) — mapAnomaly (internal/aws/costs.go) must normalize CE's anomaly
// date strings to date-only so marks match grid columns. Traced precisely:
// mapAnomaly sets `Period: costs.Period{Start: aws.ToString(a.AnomalyStartDate),
// End: aws.ToString(a.AnomalyEndDate)}` verbatim from the raw AWS response —
// no parsing/reformatting at all. AWS's GetAnomalies API reference documents
// these as ISO 8601; live CE sends full RFC3339 timestamps
// ("2026-06-15T00:00:00Z"), the synthetic demo fixture sends bare dates
// ("2026-06-15") — grid column matching is exact costs.Period string
// equality against date-only ("2006-01-02") periods, so an RFC3339-shaped
// Period.Start/End NEVER equals any grid column, and live anomalies never
// render as cell marks even though the fetch itself succeeds.
// ===========================================================================

type selfReviewAnomaliesStub struct {
	out *costexplorer.GetAnomaliesOutput
}

func (s *selfReviewAnomaliesStub) GetAnomalies(_ context.Context, _ *costexplorer.GetAnomaliesInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetAnomaliesOutput, error) {
	return s.out, nil
}

func TestCostsSelfReview_C3_AnomalyDateFormat_RFC3339AndDateOnly_ProduceMatchingColumnMarks(t *testing.T) {
	tests := []struct {
		name  string
		start string
		end   string
	}{
		{"date-only (demo fixture shape)", "2026-06-15", "2026-06-16"},
		{"RFC3339 timestamp (live CE shape)", "2026-06-15T00:00:00Z", "2026-06-16T00:00:00Z"},
	}
	wantPeriod := costs.Period{Start: "2026-06-15", End: "2026-06-16"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &selfReviewAnomaliesStub{out: &costexplorer.GetAnomaliesOutput{
				Anomalies: []cetypes.Anomaly{
					{
						AnomalyId:        strPtrSelfReview("anomaly-c3"),
						AnomalyScore:     &cetypes.AnomalyScore{CurrentScore: 90},
						Impact:           &cetypes.Impact{MaxImpact: 100},
						AnomalyStartDate: strPtrSelfReview(tt.start),
						AnomalyEndDate:   strPtrSelfReview(tt.end),
						RootCauses: []cetypes.RootCause{
							{Service: strPtrSelfReview("Amazon Elastic Compute Cloud - Compute")},
						},
					},
				},
			}}
			marks, err := a9saws.FetchCostAnomalies(context.Background(), stub, costs.Period{Start: "2026-06-01", End: "2026-07-01"})
			if err != nil {
				t.Fatalf("FetchCostAnomalies() error = %v", err)
			}
			if len(marks) != 1 {
				t.Fatalf("len(marks) = %d, want 1", len(marks))
			}
			if marks[0].Period != wantPeriod {
				t.Errorf("mark Period = %+v, want %+v (date-only, matching grid column format) — input was %q/%q", marks[0].Period, wantPeriod, tt.start, tt.end)
			}
		})
	}
}

func strPtrSelfReview(s string) *string { return &s }

// ===========================================================================
// C4 (P1/P2) — anomaly lifecycle, three angles.
// ===========================================================================

// C4(a): a successful fetch delivering ZERO anomalies must clear cached
// marks (PutAnomalies unconditional on success). Traced precisely:
// ApplyCostsLoaded (internal/app/costs_state.go) only calls
// `cs.Store.PutAnomalies(ev.Anomalies, cs.Now)` inside `if
// len(ev.Anomalies) > 0`, so a genuine "no anomalies this fetch" result
// never overwrites a stale cached mark from an earlier fetch.
//
// RECONCILED (specs/021-cost-explorer/architecture.md, Seam 2/6): this
// test's intent — an authoritative empty result clears — now has a
// mechanism-level pin at the typed seam in costs_screen_test.go
// (TestCostsScreen_ApplyFetchResult_AnomalyResult_WriteSemantics, case
// "requested, empty"), which also reconciles it against the X2 codex pin's
// opposite intent (a SKIP must preserve). This controller-level test stays
// unchanged as the full-stack acceptance pin — see the architecture doc's
// "Behavior pins survive the refactor unchanged."
func TestCostsSelfReview_C4a_ZeroAnomalyResult_ClearsCachedMarks(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	// The cursor starts on the newest (rightmost, currently-open) column by
	// default (FR-002) — anchor the stale anomaly there so no cursor
	// movement is needed to observe it via FooterNote.
	root := topDrill(t, c)
	curPeriod := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query: q,
		Grid:  costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Anomalies: []costs.AnomalyMark{{
			Impact:    costs.Amount{Value: 500, Unit: "USD"},
			Period:    curPeriod,
			Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon Elastic Compute Cloud - Compute"},
		}},
		Requests: 1,
	})
	if got := c.Snapshot().Body.Costs.FooterNote; !strings.Contains(got, "500") {
		t.Fatalf("precondition: FooterNote %q does not show the seeded anomaly's $500 impact", got)
	}

	// A SECOND, successful fetch of the SAME shape genuinely finds zero
	// anomalies this time (the prior anomaly resolved/expired) — the stale
	// mark must not survive. Per the executor discriminator
	// (costsAnomalyResultFromEvent: Requested = ev.Anomalies != nil), an
	// AUTHORITATIVE zero-anomaly result is a non-nil EMPTY slice — nil
	// means "skipped" (preserves marks), which is the opposite of this
	// scenario's intent.
	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	if got := c.Snapshot().Body.Costs.FooterNote; strings.Contains(got, "500") {
		t.Errorf("FooterNote %q still shows the stale $500 anomaly after a SECOND successful fetch found zero anomalies — PutAnomalies must run unconditionally on success, not only when len(ev.Anomalies) > 0", got)
	}
}

// C4(b): the anomaly fetch's own request cost must fold into
// CostsLoaded.Requests, not just the main cost-and-usage fetch's count.
// Traced precisely: internal/runtime/executor.go's KindFetchCosts case sets
// `Requests: result.RequestCount` — result is CostFetchResult from
// FetchCostAndUsage/FetchCostAndUsageWithResources ONLY; the anomaly fetch
// (awsclient.FetchCostAnomalies, a separate GetAnomalies API call) that
// rides alongside contributes nothing to Requests, silently undercounting
// billed CE calls whenever the anomaly overlay succeeds.
type selfReviewCostsAndAnomaliesAPI struct {
	usageOut     *costexplorer.GetCostAndUsageOutput
	anomaliesOut *costexplorer.GetAnomaliesOutput
}

func (s *selfReviewCostsAndAnomaliesAPI) GetCostAndUsage(_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	return s.usageOut, nil
}
func (s *selfReviewCostsAndAnomaliesAPI) GetCostAndUsageWithResources(_ context.Context, _ *costexplorer.GetCostAndUsageWithResourcesInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
	return &costexplorer.GetCostAndUsageWithResourcesOutput{}, nil
}
func (s *selfReviewCostsAndAnomaliesAPI) GetAnomalies(_ context.Context, _ *costexplorer.GetAnomaliesInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetAnomaliesOutput, error) {
	return s.anomaliesOut, nil
}
func (s *selfReviewCostsAndAnomaliesAPI) GetDimensionValues(_ context.Context, _ *costexplorer.GetDimensionValuesInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetDimensionValuesOutput, error) {
	return &costexplorer.GetDimensionValuesOutput{}, nil
}

func TestCostsSelfReview_C4b_AnomalyFetch_RequestCountFoldsIntoCostsLoadedRequests(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	s.Clients = &a9saws.ServiceClients{CostExplorer: &selfReviewCostsAndAnomaliesAPI{
		usageOut: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []cetypes.ResultByTime{
				{
					TimePeriod: &cetypes.DateInterval{Start: strPtrSelfReview("2026-06-01"), End: strPtrSelfReview("2026-07-01")},
					Groups: []cetypes.Group{
						{
							Keys:    []string{"Amazon Elastic Compute Cloud - Compute"},
							Metrics: map[string]cetypes.MetricValue{"UnblendedCost": {Amount: strPtrSelfReview("100.0"), Unit: strPtrSelfReview("USD")}},
						},
					},
				},
			},
		},
		anomaliesOut: &costexplorer.GetAnomaliesOutput{
			Anomalies: []cetypes.Anomaly{
				{AnomalyId: strPtrSelfReview("a-1"), AnomalyStartDate: strPtrSelfReview("2026-06-15"), AnomalyEndDate: strPtrSelfReview("2026-06-16")},
			},
		},
	}}
	core := runtime.New(s, nil)

	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.DimensionService}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	ev, err := core.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchCosts},
		Payload: runtime.FetchCostsPayload{Query: q},
	})
	if err != nil {
		t.Fatalf("ExecuteTask(KindFetchCosts) error = %v", err)
	}
	loaded, ok := ev.(messages.CostsLoaded)
	if !ok {
		t.Fatalf("ExecuteTask(KindFetchCosts) event type = %T, want messages.CostsLoaded", ev)
	}
	if len(loaded.Anomalies) != 1 {
		t.Fatalf("precondition: expected 1 anomaly in the delivery, got %d", len(loaded.Anomalies))
	}
	if loaded.Requests != 2 {
		t.Errorf("CostsLoaded.Requests = %d, want 2 (1 main GetCostAndUsage call + 1 GetAnomalies call) — the anomaly fetch's own request cost is being silently dropped", loaded.Requests)
	}
}

// C4(c): when the store's anomaly TTL is fresh, the dispatched payload must
// ask the executor to skip the anomaly fetch. FetchCostsPayload
// (internal/runtime/handlers_navigate.go) has NO such flag today — only
// Query and Window. Since this field does not exist yet, this pins its
// PRESENCE compile-safely via reflection (matching this session's
// established "quality-batch Currency field" pattern for a not-yet-landed
// struct field) rather than referencing a name directly, which would break
// the whole package's compilation until the coder adds it.
func TestCostsSelfReview_C4c_FetchCostsPayload_HasAnomalySkipFlag(t *testing.T) {
	payload := runtime.FetchCostsPayload{}
	fields := selfReviewStructFieldNames(payload)
	found := false
	for _, f := range fields {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "anomal") && (strings.Contains(lower, "skip") || strings.Contains(lower, "fresh")) {
			found = true
		}
	}
	if !found {
		t.Errorf("runtime.FetchCostsPayload has no anomaly-skip/fresh flag field (got fields: %v) — when the store's cached anomaly snapshot is still within its TTL, the dispatched payload must tell the executor to skip the redundant GetAnomalies call", fields)
	}
}

// selfReviewStructFieldNames returns v's exported struct field names via
// reflection — used to pin the PRESENCE of a not-yet-landed field without
// referencing it directly (which would break compilation for the whole
// package until the field exists).
func selfReviewStructFieldNames(v any) []string {
	t := reflect.TypeOf(v)
	if t.Kind() != reflect.Struct {
		return nil
	}
	names := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		names = append(names, t.Field(i).Name)
	}
	return names
}

// ===========================================================================
// C5 (P1) — settlement-lag immutability: a bucket becomes immutable only
// when fetched >= 72h AFTER its period's own End (CE revises data for
// 24-72h after period close). Traced precisely: costs/store.go's
// periodEntry.immutableAt today is `!e.FetchedAt.Before(end)` — fetched
// AT OR AFTER End, no lag at all (this is round8 item 1's own fix, now
// superseded by the narrower settlement-lag boundary; reconciled below).
// ===========================================================================

func TestCostsSelfReview_C5a_DailyBucket_FetchedMorningAfter_StaysRefetchable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("selfreview-c5a")
	q := costs.Query{Granularity: costs.GranularityDay.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	day := costs.Period{Start: "2026-07-01", End: "2026-07-02"}

	// Fetched the morning after the day closed (well within CE's 24-72h
	// settlement window) — must still be refetchable, not immutable.
	fetchedAt := time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC)
	s.Merge(q, []costs.Record{{Period: day, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 10, Unit: "USD"}}}}, fetchedAt)

	lookupNow := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	_, missing := s.Lookup(q, []costs.Period{day}, lookupNow)
	if len(missing) == 0 {
		t.Error("a daily bucket fetched the morning after its day closed (well within CE's 24-72h settlement window) is reported as covered — it must stay refetchable until fetched >= 72h after the period's own End")
	}
}

func TestCostsSelfReview_C5b_DailyBucket_Fetched4DaysAfter_IsImmutable(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("selfreview-c5b")
	q := costs.Query{Granularity: costs.GranularityDay.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	day := costs.Period{Start: "2026-07-01", End: "2026-07-02"}

	// Fetched 4 days after the day closed — past the 72h settlement lag —
	// must be immutable.
	fetchedAt := time.Date(2026, 7, 5, 8, 0, 0, 0, time.UTC)
	s.Merge(q, []costs.Record{{Period: day, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 10, Unit: "USD"}}}}, fetchedAt)

	lookupNow := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	got, missing := s.Lookup(q, []costs.Period{day}, lookupNow)
	if len(missing) != 0 {
		t.Errorf("a daily bucket fetched 4 days after its day closed (past the 72h settlement lag) is reported missing — want it immutable")
	}
	if len(got) != 1 {
		t.Fatalf("Lookup() records = %d, want 1", len(got))
	}
}

func TestCostsSelfReview_C5c_MergeCoverage_NeverRefreshesFetchedAt_OnBucketRecordsDidNotReplace(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := costs.LoadStore("selfreview-c5c")
	q := costs.Query{Granularity: costs.GranularityDay.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	day := costs.Period{Start: "2026-07-01", End: "2026-07-02"}

	// Original fetch: within the settlement window (refetchable).
	original := time.Date(2026, 7, 2, 8, 0, 0, 0, time.UTC)
	s.Merge(q, []costs.Record{{Period: day, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 10, Unit: "USD"}}}}, original)

	// A refetch attempt long after settlement (now >= End+72h) that returns
	// ZERO groups for this period (Merge sees no matching record, so the OLD
	// 10 USD record is never replaced) but the executor still knows the full
	// requested window and calls MergeCoverage for it, as it always does.
	refetchNow := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	s.Merge(q, nil, refetchNow) // zero records returned this round
	s.MergeCoverage(q, []costs.Period{day}, refetchNow)

	// The bucket must NOT be stamped immutable off this refetch — Merge
	// never actually replaced its records, so its true FetchedAt is still
	// `original` (within the settlement window at the time of ITS OWN
	// fetch) — MergeCoverage refreshing FetchedAt here would wrongly
	// promote the ORIGINAL, possibly-still-settling record to permanent.
	_, missing := s.Lookup(q, []costs.Period{day}, refetchNow)
	if len(missing) == 0 {
		t.Error("MergeCoverage refreshed FetchedAt on a bucket whose records it did not replace (a refetch returning zero groups) — the stale-promotion hole: this wrongly stamps the OLD, possibly-unsettled record as immutable instead of leaving it refetchable")
	}
}

// ===========================================================================
// C6 (P2) — UTC day truncation. Traced precisely: both Period.Closed
// (internal/costs/types.go) and ClampResourceDrillWindow
// (internal/costs/drill.go) build `time.Date(now.Year(), now.Month(),
// [now.Day(),] 0,0,0,0, time.UTC)` — extracting Y/M/D from `now` AS
// AUTHORED (whatever zone it carries) but then representing that as if it
// were already a UTC clock reading, instead of `now.UTC()` first. At a
// zone boundary where the local calendar date differs from the UTC
// calendar date, this silently compares against the WRONG day.
// ===========================================================================

func TestCostsSelfReview_C6_PeriodClosed_UsesUTCDate_NotLocalZoneDate(t *testing.T) {
	// UTC+13: local midnight July 1 is 2026-06-30 11:00 UTC — a full
	// calendar day earlier in UTC than in the local zone.
	tz := time.FixedZone("UTC+13", 13*3600)
	localMidnightJuly1 := time.Date(2026, 7, 1, 0, 0, 0, 0, tz)

	// A period ending exactly at UTC's "first of month" boundary this
	// local moment sees: June is closed only once the UTC calendar has
	// actually reached July 1. At this instant UTC is still June 30.
	juneToJuly := costs.Period{Start: "2026-06-01", End: "2026-07-01"}
	if juneToJuly.Closed(localMidnightJuly1) {
		t.Error("Period.Closed(now) reports June closed at local midnight July 1 in UTC+13 — the actual UTC calendar date at that instant is still 2026-06-30, so June has not closed yet; Closed must truncate on now.UTC()'s date, not now's own zone-local Y/M/D")
	}
}

func TestCostsSelfReview_C6_ClampResourceDrillWindow_UsesUTCDate_NotLocalZoneDate(t *testing.T) {
	tz := time.FixedZone("UTC+13", 13*3600)
	localMidnightJuly1 := time.Date(2026, 7, 1, 0, 0, 0, 0, tz) // 2026-06-30T11:00:00Z

	// A period starting exactly 14 days before the UTC calendar date
	// (2026-06-30) is 2026-06-16 — the buggy local-zone cutoff (computed
	// from "July 1", one calendar day later than the true UTC date) would
	// place the cutoff at 2026-06-17 instead, wrongly excluding this period.
	window := []costs.Period{
		{Start: "2026-06-16", End: "2026-06-17"}, // exactly 14 days before UTC's 2026-06-30
	}
	got := costs.ClampResourceDrillWindow(window, localMidnightJuly1)
	if len(got) != 1 {
		t.Errorf("ClampResourceDrillWindow dropped a period that is within the 14-day retention window measured from the UTC calendar date (2026-06-30) — got %d periods, want 1; using now's own zone-local Y/M/D instead of now.UTC() computes the cutoff a full day too late in UTC+13, wrongly clamping periods that should survive", len(got))
	}
}

// ===========================================================================
// C9 (P2) — CostsBody.DataThrough must derive from the store: re-entering
// the costs screen over a warm disk cache (no fetch fired) must still show
// a correct data-through footer. Traced precisely: EnsureCostsState
// (internal/app/costs_state.go) constructs a fresh CostsState with
// `Store: costs.LoadStore(...)` (warm from disk) but never initializes
// DataThrough — it stays "" until ApplyCostsLoaded sets it from a LIVE
// fetch delivery, which never happens when the shape is already fully
// covered by the warm store.
// ===========================================================================

func TestCostsSelfReview_C9_DataThrough_DerivesFromWarmStore_NoFetchNeeded(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile := "selfreview-c9"
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	window := costs.BuildWindow(costs.GranularityMonth, fixedCostsNow)

	// Pre-seed the on-disk store directly (simulating an earlier session's
	// fetch), fully covering the default root window with CLOSED periods'
	// data so no fetch is needed on next open.
	store := costs.LoadStore(profile)
	var recs []costs.Record
	for _, p := range window {
		if p.Closed(fixedCostsNow) {
			recs = append(recs, costs.Record{Period: p, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}})
		}
	}
	store.Merge(q, recs, fixedCostsNow.Add(-96*time.Hour))
	store.MergeCoverage(q, window, fixedCostsNow.Add(-96*time.Hour))
	if err := store.Save(); err != nil {
		t.Fatalf("seeding on-disk store: %v", err)
	}

	// Fresh controller + fresh CostsState, same profile: EnsureCostsState
	// loads the warm store from disk.
	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow)

	if got := c.Snapshot().Body.Costs.DataThrough; got == "" {
		t.Error("CostsBody.DataThrough is empty on a fresh CostsState over a warm disk store — it must derive from the store's own cached records, not only from a live fetch delivery that never happens when the shape is already fully covered")
	}
}

// ===========================================================================
// C10 (P2) — error taxonomy + retry. Traced precisely: classifyCostsError
// (internal/aws/costs.go) only matches "AccessDeniedException" in its
// switch, but ClassifyAWSError (internal/aws/errors.go, the shared
// taxonomy) matches BOTH "AccessDenied" and "AccessDeniedException" — some
// AWS API paths return the bare code. CE calls are also not wrapped in
// RetryOnThrottle at all (zero references in costs.go), unlike
// FetchEC2InstancesByIDs and other by-ID fetchers.
// ===========================================================================

func TestCostsSelfReview_C10a_ClassifyCostsError_MatchesBareAccessDeniedCode(t *testing.T) {
	bare := &selfReviewAPIError{code: "AccessDenied", message: "not authorized"}

	_, err := a9saws.FetchCostAndUsage(context.Background(), &selfReviewErroringCostsAPI{err: bare}, costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.DimensionService}})
	if err == nil {
		t.Fatal("FetchCostAndUsage() error = nil, want a classified error")
	}
	if !errors.Is(err, a9saws.ErrCostsAccessDenied) {
		t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsAccessDenied) for the bare \"AccessDenied\" code (not just \"AccessDeniedException\")", err)
	}
}

type selfReviewAPIError struct {
	code    string
	message string
}

func (e *selfReviewAPIError) Error() string                 { return e.code + ": " + e.message }
func (e *selfReviewAPIError) ErrorCode() string             { return e.code }
func (e *selfReviewAPIError) ErrorMessage() string          { return e.message }
func (e *selfReviewAPIError) ErrorFault() smithy.ErrorFault { return smithy.FaultUnknown }

type selfReviewErroringCostsAPI struct {
	err error
}

func (s *selfReviewErroringCostsAPI) GetCostAndUsage(_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	return nil, s.err
}

func TestCostsSelfReview_C10b_GetCostAndUsage_RetriesOnThrottle_ThenSucceeds(t *testing.T) {
	stub := &selfReviewFlakyThrottleCostsAPI{
		throttleErr: &selfReviewAPIError{code: "Throttling", message: "rate exceeded"},
		successOut: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []cetypes.ResultByTime{
				{
					TimePeriod: &cetypes.DateInterval{Start: strPtrSelfReview("2026-06-01"), End: strPtrSelfReview("2026-07-01")},
					Groups: []cetypes.Group{
						{Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[string]cetypes.MetricValue{"UnblendedCost": {Amount: strPtrSelfReview("50.0"), Unit: strPtrSelfReview("USD")}}},
					},
				},
			},
		},
	}
	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.DimensionService}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), stub, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v, want nil (the first page's throttle must be retried, not propagated)", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1 (the retried, successful page's data)", len(result.Records))
	}
	t.Logf("RequestCount after one throttle + one success = %d (informational: report to the coder to confirm this matches the existing counting semantics)", result.RequestCount)
}

type selfReviewFlakyThrottleCostsAPI struct {
	calls       int
	throttleErr error
	successOut  *costexplorer.GetCostAndUsageOutput
}

func (s *selfReviewFlakyThrottleCostsAPI) GetCostAndUsage(_ context.Context, _ *costexplorer.GetCostAndUsageInput, _ ...func(*costexplorer.Options)) (*costexplorer.GetCostAndUsageOutput, error) {
	s.calls++
	if s.calls == 1 {
		return nil, s.throttleErr
	}
	return s.successOut, nil
}

// ===========================================================================
// C11 (P2) — a CostsLoaded arriving after the costs screen was FULLY
// popped must still Merge+Save into the session store. Traced precisely:
// costsStateBeneathOverlay (internal/app/costs_state.go) searches the
// stack top-down for a ScreenCosts entry and returns nil when none exists
// (the screen was popped all the way off, not merely overlaid by
// Help/Identity) — ApplyCostsLoaded's `if cs == nil { return }` then drops
// the delivery entirely, discarding billed CE data.
// ===========================================================================

func TestCostsSelfReview_C11_CostsLoaded_AfterFullPop_StillMergesAndSaves(t *testing.T) {
	const profile = "demo" // newTestController's (blessed) hardcoded profile
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow)

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	root := topDrill(t, c)

	// Pop the costs screen ENTIRELY (root frame, depth 1 -> ActionBack
	// leaves the main menu) before the fetch's result arrives.
	c.Apply(app.Action{Kind: app.ActionBack})
	if vs := c.Snapshot(); vs.Body.Kind == app.BodyKindCosts {
		t.Fatal("precondition: costs screen still on top after ActionBack at root depth")
	}

	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{{Period: root.Window[len(root.Window)-1], Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 250, Unit: "USD"}}}}},
		Requests: 1,
	})

	// Independently verify on disk: a fresh Store load for the same
	// profile must see the merged record.
	fresh := costs.LoadStore(profile)
	got, missing := fresh.Lookup(q, []costs.Period{root.Window[len(root.Window)-1]}, fixedCostsNow)
	if len(missing) != 0 || len(got) != 1 {
		t.Errorf("CostsLoaded delivered after the costs screen was fully popped was not merged+saved to disk — LoadStore(%q) sees missing=%v got=%v, want the record present", profile, missing, got)
	}
}

// ===========================================================================
// C12 (P2) — FetchEC2InstancesByIDs must recover from AWS's all-or-nothing
// batch failure on an unknown ID. Traced precisely: DescribeInstances
// rejects the WHOLE batch call (InvalidInstanceID.NotFound naming the bad
// ID) when ANY requested ID is invalid/terminated-and-purged — it does not
// return a partial Reservations list omitting just the bad one. The
// current implementation only diffs "requested vs returned" AFTER a
// successful call; a batch-level error (even one naming exactly which ID
// is bad) returns zero resources for the WHOLE batch, dropping the live
// instances too.
// ===========================================================================

func TestCostsSelfReview_C12_BatchNotFound_RecoversLiveInstances(t *testing.T) {
	liveID := "i-0a1b2c3d4e5f60001"
	badID := "i-0000000000000dead"

	stub := &selfReviewEC2BatchAPI{badID: badID, liveID: liveID}
	resources, err := a9saws.FetchEC2InstancesByIDs(context.Background(), stub, []string{liveID, badID})
	if len(resources) != 1 || resources[0].ID != liveID {
		t.Errorf("FetchEC2InstancesByIDs recovery: got %d resources %+v, want 1 resource for %q — the live instance must be recovered even though the batch call initially errored on the unknown ID", len(resources), resources, liveID)
	}
	if err == nil {
		t.Error("FetchEC2InstancesByIDs recovery: error = nil, want a composite error naming the unrecovered bad ID")
	} else if !strings.Contains(err.Error(), badID) {
		t.Errorf("FetchEC2InstancesByIDs recovery error %v does not name the unrecovered ID %q", err, badID)
	}
}

// selfReviewEC2BatchAPI simulates AWS's real DescribeInstances behavior:
// a batch call naming ANY invalid ID fails the WHOLE call with
// InvalidInstanceID.NotFound (never a partial Reservations list). A retry
// with only the known-good IDs succeeds. This lets the test pass once the
// coder adds the retry-without-bad-ids recovery; today (no such retry
// exists) the mock's first call is the ONLY call made, so the whole batch
// errors and zero resources come back.
type selfReviewEC2BatchAPI struct {
	badID, liveID string
}

func (s *selfReviewEC2BatchAPI) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	for _, id := range in.InstanceIds {
		if id == s.badID {
			return nil, &selfReviewAPIError{
				code:    "InvalidInstanceID.NotFound",
				message: fmt.Sprintf("The instance ID '%s' does not exist", s.badID),
			}
		}
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{
			{Instances: []ec2types.Instance{{InstanceId: strPtrSelfReview(s.liveID)}}},
		},
	}, nil
}
