package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"

	a9saws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/costs"
)

// Fetcher functions under test (internal/aws, package aws) are named to
// mirror the paginated-fetch convention already used across this package
// (e.g. FetchAlarmHistory), adapted to the four CE interfaces named in
// data-model.md's "AWS layer" section:
//
//	func FetchCostAndUsage(ctx, api CostsGetCostAndUsageAPI, q costs.Query) (a9saws.CostFetchResult, error)
//	func FetchCostAndUsageWithResources(ctx, api CostsGetCostAndUsageWithResourcesAPI, q costs.Query) (a9saws.CostFetchResult, error)
//	func FetchDimensionValues(ctx, api CostsGetDimensionValuesAPI, dim costs.Dimension, window costs.Period) (map[string]string, error)
//	func FetchCostAnomalies(ctx, api CostsGetAnomaliesAPI, window costs.Period) ([]costs.AnomalyMark, error)
//
// CostFetchResult{Records []costs.Record, Attrs map[string]string, RequestCount int}
// and the three typed sentinel errors (ErrCostsAccessDenied, ErrCostsDataUnavailable,
// ErrCostsThrottled) are likewise not spelled out verbatim in data-model.md
// beyond "classify errors ... typed sentinel errors" — this file pins that contract.

// strPtr is defined in aws_related_checker_mechanism_test.go (shared package unit_test).

// ---------------------------------------------------------------------------
// Mocks for the four CE interfaces (data-model.md "AWS layer")
// ---------------------------------------------------------------------------

type mockCostsGetCostAndUsageClient struct {
	pages []*costexplorer.GetCostAndUsageOutput
	err   error
	calls int
}

func (m *mockCostsGetCostAndUsageClient) GetCostAndUsage(
	ctx context.Context,
	params *costexplorer.GetCostAndUsageInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	idx := m.calls - 1
	if idx >= len(m.pages) {
		return &costexplorer.GetCostAndUsageOutput{}, nil
	}
	return m.pages[idx], nil
}

type mockCostsGetCostAndUsageWithResourcesClient struct {
	pages []*costexplorer.GetCostAndUsageWithResourcesOutput
	calls int
}

func (m *mockCostsGetCostAndUsageWithResourcesClient) GetCostAndUsageWithResources(
	ctx context.Context,
	params *costexplorer.GetCostAndUsageWithResourcesInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
	m.calls++
	idx := m.calls - 1
	if idx >= len(m.pages) {
		return &costexplorer.GetCostAndUsageWithResourcesOutput{}, nil
	}
	return m.pages[idx], nil
}

type mockCostsGetAnomaliesClient struct {
	output *costexplorer.GetAnomaliesOutput
}

func (m *mockCostsGetAnomaliesClient) GetAnomalies(
	ctx context.Context,
	params *costexplorer.GetAnomaliesInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetAnomaliesOutput, error) {
	return m.output, nil
}

type mockCostsGetDimensionValuesClient struct {
	output *costexplorer.GetDimensionValuesOutput
}

func (m *mockCostsGetDimensionValuesClient) GetDimensionValues(
	ctx context.Context,
	params *costexplorer.GetDimensionValuesInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetDimensionValuesOutput, error) {
	return m.output, nil
}

// ---------------------------------------------------------------------------
// GetCostAndUsage
// ---------------------------------------------------------------------------

func ceGroup(key string, amounts map[string]string) cetypes.Group {
	metrics := make(map[string]cetypes.MetricValue, len(amounts))
	for k, v := range amounts {
		val := v
		metrics[k] = cetypes.MetricValue{Amount: &val, Unit: strPtr("USD")}
	}
	return cetypes.Group{Keys: []string{key}, Metrics: metrics}
}

func TestFetchCostAndUsage_PaginatesAcrossPages_AccumulatesRecordsAndCountsRequests(t *testing.T) {
	page := func(token *string, key string) *costexplorer.GetCostAndUsageOutput {
		return &costexplorer.GetCostAndUsageOutput{
			NextPageToken: token,
			ResultsByTime: []cetypes.ResultByTime{
				{
					TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
					Groups: []cetypes.Group{
						ceGroup(key, map[string]string{
							"UnblendedCost":    "10.0000000000",
							"BlendedCost":      "10.0000000000",
							"AmortizedCost":    "10.0000000000",
							"NetAmortizedCost": "10.0000000000",
							"NetUnblendedCost": "10.0000000000",
						}),
					},
				},
			},
		}
	}

	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			page(strPtr("tok-1"), "Amazon Elastic Compute Cloud - Compute"),
			page(strPtr("tok-2"), "Amazon Simple Storage Service"),
			page(nil, "EC2 - Other"),
		},
	}

	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	if mock.calls != 3 {
		t.Errorf("mock.calls = %d, want 3", mock.calls)
	}
	if result.RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3", result.RequestCount)
	}
	if len(result.Records) != 3 {
		t.Fatalf("len(Records) = %d, want 3 (one per page)", len(result.Records))
	}
}

func TestFetchCostAndUsage_DimensionValueAttributesMapIntoAttrs(t *testing.T) {
	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			{
				DimensionValueAttributes: []cetypes.DimensionValuesWithAttributes{
					{Value: strPtr("123456789012"), Attributes: map[string]string{"description": "prod-account"}},
					{Value: strPtr("210987654321"), Attributes: map[string]string{"description": "staging-account"}},
				},
				ResultsByTime: []cetypes.ResultByTime{
					{
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
						Groups:     []cetypes.Group{ceGroup("123456789012", map[string]string{"UnblendedCost": "5.0000000000"})},
					},
				},
			},
		},
	}

	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("LINKED_ACCOUNT")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	want := map[string]string{"123456789012": "prod-account", "210987654321": "staging-account"}
	if len(result.Attrs) != len(want) {
		t.Fatalf("Attrs = %v, want %v", result.Attrs, want)
	}
	for k, v := range want {
		if result.Attrs[k] != v {
			t.Errorf("Attrs[%q] = %q, want %q", k, result.Attrs[k], v)
		}
	}
}

// TestFetchCostAndUsage_ParsesAllFourMetrics is reconciled from the former
// TestFetchCostAndUsage_ParsesAllFiveMetrics (closure wave):
// MetricNetUnblended is dropped from the fetcher's parsed set. The AWS
// response fixture still realistically includes "NetUnblendedCost" (CE
// still returns it) — only the fetcher's mapping of that key into Metrics
// goes away, so the fixture input is unchanged but the expected output
// drops to four entries summing to 1000, not five summing to 1500.
func TestFetchCostAndUsage_ParsesAllFourMetrics(t *testing.T) {
	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			{
				ResultsByTime: []cetypes.ResultByTime{
					{
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
						Groups: []cetypes.Group{
							ceGroup("Amazon Elastic Compute Cloud - Compute", map[string]string{
								"UnblendedCost":    "100.0000000000",
								"BlendedCost":      "200.0000000000",
								"AmortizedCost":    "300.0000000000",
								"NetAmortizedCost": "400.0000000000",
								"NetUnblendedCost": "500.0000000000", // still returned by CE; no longer parsed into Metrics
							}),
						},
					},
				},
			},
		},
	}

	// No RECORD_TYPE filter: per data-model.md's display mapping this is the
	// "invoice" query shape, so UnblendedCost lands under Metric("invoice").
	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(result.Records))
	}
	metrics := result.Records[0].Metrics
	if len(metrics) != 4 {
		t.Fatalf("len(Metrics) = %d, want 4 (the net-unblended metric dropped), got %+v", len(metrics), metrics)
	}
	// The exact 4-key set (invoice/blended/amortized/net-amortized) is
	// pinned individually below — len==4 together with those 4 named
	// checks already exhaustively rules out any 5th key, so no separate
	// costs.MetricNetUnblended absence check is needed. The constant
	// itself is unreferenced anywhere in tests/unit as of this edit — flag
	// for the coder: costs.MetricNetUnblended (internal/costs/types.go)
	// can now be deleted, it was only kept alive by this reference.
	var sum float64
	for _, amt := range metrics {
		sum += amt.Value
	}
	if sum != 1000 {
		t.Errorf("sum of all metric values = %v, want 1000 (100+200+300+400, NetUnblendedCost's 500 correctly dropped)", sum)
	}
	if got := metrics[costs.Metric("invoice")].Value; got != 100 {
		t.Errorf("Metrics[invoice] = %v, want 100 (UnblendedCost, no RECORD_TYPE filter)", got)
	}
	if got := metrics[costs.Metric("blended")].Value; got != 200 {
		t.Errorf("Metrics[blended] = %v, want 200 (BlendedCost)", got)
	}
	if got := metrics[costs.Metric("amortized")].Value; got != 300 {
		t.Errorf("Metrics[amortized] = %v, want 300 (AmortizedCost)", got)
	}
	if got := metrics[costs.Metric("net-amortized")].Value; got != 400 {
		t.Errorf("Metrics[net-amortized] = %v, want 400 (NetAmortizedCost)", got)
	}
}

func TestFetchCostAndUsage_PropagatesEstimatedFlag(t *testing.T) {
	tests := []struct {
		name      string
		estimated bool
	}{
		{name: "estimated result", estimated: true},
		{name: "final (non-estimated) result", estimated: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCostsGetCostAndUsageClient{
				pages: []*costexplorer.GetCostAndUsageOutput{
					{
						ResultsByTime: []cetypes.ResultByTime{
							{
								Estimated:  tt.estimated,
								TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-07-01"), End: strPtr("2026-08-01")},
								Groups:     []cetypes.Group{ceGroup("Amazon Elastic Compute Cloud - Compute", map[string]string{"UnblendedCost": "10.0000000000"})},
							},
						},
					},
				},
			}
			q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-07-01", End: "2026-08-01"}}
			result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
			if err != nil {
				t.Fatalf("FetchCostAndUsage() error = %v", err)
			}
			if got := result.Records[0].Estimated; got != tt.estimated {
				t.Errorf("Records[0].Estimated = %v, want %v", got, tt.estimated)
			}
		})
	}
}

func TestFetchCostAndUsage_ClassifiesErrors(t *testing.T) {
	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}

	t.Run("AccessDeniedException maps to ErrCostsAccessDenied", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &boundaryAPIError{code: "AccessDeniedException", message: "User is not authorized to perform: ce:GetCostAndUsage"}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsAccessDenied) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsAccessDenied)", err)
		}
	})

	t.Run("DataUnavailableException maps to ErrCostsDataUnavailable", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &cetypes.DataUnavailableException{Message: strPtr("Cost Explorer data is not available yet for this account")}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsDataUnavailable) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsDataUnavailable)", err)
		}
	})

	t.Run("ThrottlingException maps to ErrCostsThrottled", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &boundaryAPIError{code: "ThrottlingException", message: "Rate exceeded"}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsThrottled) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsThrottled)", err)
		}
	})

	t.Run("other errors pass through unclassified", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: errors.New("boom-network-timeout")}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if err == nil {
			t.Fatal("FetchCostAndUsage() error = nil, want a passthrough error")
		}
		if errors.Is(err, a9saws.ErrCostsAccessDenied) || errors.Is(err, a9saws.ErrCostsDataUnavailable) || errors.Is(err, a9saws.ErrCostsThrottled) {
			t.Errorf("FetchCostAndUsage() error = %v, want none of the typed sentinels for an unrelated failure", err)
		}
	})
}

// ---------------------------------------------------------------------------
// GetCostAndUsageWithResources
// ---------------------------------------------------------------------------

func TestFetchCostAndUsageWithResources_MapsRecordsAndCountsRequests(t *testing.T) {
	mock := &mockCostsGetCostAndUsageWithResourcesClient{
		pages: []*costexplorer.GetCostAndUsageWithResourcesOutput{
			{
				ResultsByTime: []cetypes.ResultByTime{
					{
						Estimated:  true,
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-07-05"), End: strPtr("2026-07-06")},
						Groups:     []cetypes.Group{ceGroup("i-0abcd1234ef567890", map[string]string{"UnblendedCost": "3.5000000000"})},
					},
				},
			},
		},
	}

	q := costs.Query{
		Granularity: "DAILY",
		GroupBy:     []costs.Dimension{costs.Dimension("RESOURCE_ID")},
		Filter:      filterPinning(costs.Dimension("SERVICE"), "Amazon Elastic Compute Cloud - Compute"),
		Range:       costs.Period{Start: "2026-07-05", End: "2026-07-06"},
	}
	result, err := a9saws.FetchCostAndUsageWithResources(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsageWithResources() error = %v", err)
	}
	if result.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", result.RequestCount)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(result.Records))
	}
	if !result.Records[0].Estimated {
		t.Error("Records[0].Estimated = false, want true")
	}
	if got := result.Records[0].Keys[0]; got != "i-0abcd1234ef567890" {
		t.Errorf("Records[0].Keys[0] = %q, want the resource id", got)
	}
}

// ---------------------------------------------------------------------------
// GetDimensionValues
// ---------------------------------------------------------------------------

func TestFetchDimensionValues_MapsValuesToAttrs(t *testing.T) {
	mock := &mockCostsGetDimensionValuesClient{
		output: &costexplorer.GetDimensionValuesOutput{
			DimensionValues: []cetypes.DimensionValuesWithAttributes{
				{Value: strPtr("123456789012"), Attributes: map[string]string{"description": "prod-account"}},
			},
		},
	}

	got, err := a9saws.FetchDimensionValues(context.Background(), mock, costs.Dimension("LINKED_ACCOUNT"), costs.Period{Start: "2026-06-01", End: "2026-07-01"})
	if err != nil {
		t.Fatalf("FetchDimensionValues() error = %v", err)
	}
	want := map[string]string{"123456789012": "prod-account"}
	if len(got) != len(want) || got["123456789012"] != "prod-account" {
		t.Errorf("FetchDimensionValues() = %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// GetAnomalies
// ---------------------------------------------------------------------------

// TestFetchCostAnomalies_MapsDimension is reconciled from the former
// TestFetchCostAnomalies_MapsRootCausesAndDimension (closure wave):
// AnomalyMark.RootCause/.Score/.ID die with the fields, so this test no
// longer asserts on m.ID/m.RootCause — Dimension (+ Period) becomes the
// ONLY mapped representation of a root cause. The AWS SDK response fixture
// itself is left as a realistic shape (AnomalyId/AnomalyScore/RootCauses
// are still real fields on cetypes.Anomaly) — only the ASSERTIONS on the
// now-dead AnomalyMark fields are removed.
func TestFetchCostAnomalies_MapsDimension(t *testing.T) {
	mock := &mockCostsGetAnomaliesClient{
		output: &costexplorer.GetAnomaliesOutput{
			Anomalies: []cetypes.Anomaly{
				{
					AnomalyId:        strPtr("anomaly-1a2b3c"),
					AnomalyScore:     &cetypes.AnomalyScore{CurrentScore: 92.0, MaxScore: 92.0},
					Impact:           &cetypes.Impact{MaxImpact: 452.10, TotalImpact: 452.10},
					MonitorArn:       strPtr("arn:aws:ce::123456789012:anomalymonitor/8f3b2c1a-0000-0000-0000-000000000000"),
					AnomalyStartDate: strPtr("2026-07-01"),
					AnomalyEndDate:   strPtr("2026-07-08"),
					RootCauses: []cetypes.RootCause{
						{
							Service:   strPtr("Amazon Elastic Compute Cloud - Compute"),
							Region:    strPtr("us-east-1"),
							UsageType: strPtr("BoxUsage:m5.2xlarge"),
							Impact:    &cetypes.RootCauseImpact{Contribution: 452.10},
						},
					},
				},
			},
		},
	}

	marks, err := a9saws.FetchCostAnomalies(context.Background(), mock, costs.Period{Start: "2026-07-01", End: "2026-07-08"})
	if err != nil {
		t.Fatalf("FetchCostAnomalies() error = %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("len(marks) = %d, want 1", len(marks))
	}
	m := marks[0]
	wantDim := map[costs.Dimension]string{
		costs.Dimension("SERVICE"):    "Amazon Elastic Compute Cloud - Compute",
		costs.Dimension("REGION"):     "us-east-1",
		costs.Dimension("USAGE_TYPE"): "BoxUsage:m5.2xlarge",
	}
	if len(m.Dimension) != len(wantDim) {
		t.Fatalf("Dimension = %v, want %v", m.Dimension, wantDim)
	}
	for k, v := range wantDim {
		if m.Dimension[k] != v {
			t.Errorf("Dimension[%q] = %q, want %q", k, m.Dimension[k], v)
		}
	}
	if m.Period != (costs.Period{Start: "2026-07-01", End: "2026-07-08"}) {
		t.Errorf("Period = %+v, want {2026-07-01 2026-07-08}", m.Period)
	}
}
