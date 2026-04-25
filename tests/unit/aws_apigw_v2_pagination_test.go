package unit

// aws_apigw_v2_pagination_test.go — Failing tests for EnrichAPIGatewayStage pagination.
//
// These tests document the REQUIRED behavior after the coder implements
// pagination for GetStages (API Gateway V2).
//
// GetStages V2 returns up to 100 stages per page and a NextToken when more
// exist. After full pagination, counts are exact. If pagination exceeds
// PerParentPageCap = 10 pages per API, the count is capped and marked "1000+".
// Per-stage throttle and access-log checks must be applied to stages on ALL pages.
//
// Contract assertions:
//   - 2 pages (100 + 20 stages) → Fields["stages_count"] == "120"; GetStages called twice
//   - NextToken always non-nil (huge API) → capped at PerParentPageCap; "1000+"
//   - All pages return 0 stages → stages_count == "0", no per-stage findings

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Pagination-aware fake for APIGatewayV2 GetStages
// ---------------------------------------------------------------------------

// apigwPaginatedFake implements APIGatewayV2API and serves paginated
// GetStages responses. Pages are keyed by API ID.
type apigwPaginatedFake struct {
	awsclient.APIGatewayV2API

	// pages maps apiID → ordered pages of GetStagesOutput.
	pages map[string][]*apigatewayv2.GetStagesOutput

	// callCounts tracks how many times GetStages was called per API ID.
	callCounts map[string]int
}

func newAPiGWPaginatedFake() *apigwPaginatedFake {
	return &apigwPaginatedFake{
		pages:      make(map[string][]*apigatewayv2.GetStagesOutput),
		callCounts: make(map[string]int),
	}
}

func (f *apigwPaginatedFake) GetStages(
	_ context.Context,
	in *apigatewayv2.GetStagesInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetStagesOutput, error) {
	apiID := ""
	if in != nil && in.ApiId != nil {
		apiID = *in.ApiId
	}
	idx := f.callCounts[apiID]
	f.callCounts[apiID] = idx + 1

	pages := f.pages[apiID]
	if idx >= len(pages) {
		return &apigatewayv2.GetStagesOutput{
			Items:     []apigwtypes.Stage{},
			NextToken: nil,
		}, nil
	}
	return pages[idx], nil
}

// Compile-time check: apigwPaginatedFake satisfies APIGatewayV2API.
var _ awsclient.APIGatewayV2API = (*apigwPaginatedFake)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeStagesGood builds n stages that are properly configured (throttling
// enabled, access logs enabled) — will not generate any finding.
func makeStagesGood(n int) []apigwtypes.Stage {
	stages := make([]apigwtypes.Stage, n)
	for i := range stages {
		stages[i] = apigwtypes.Stage{
			StageName: aws.String(fmt.Sprintf("stage-%d", i)),
			DefaultRouteSettings: &apigwtypes.RouteSettings{
				ThrottlingBurstLimit: aws.Int32(500),
				ThrottlingRateLimit:  aws.Float64(1000),
			},
			AccessLogSettings: &apigwtypes.AccessLogSettings{
				DestinationArn: aws.String(fmt.Sprintf("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigw/stage-%d", i)),
				Format:         aws.String(`{"requestId":"$context.requestId"}`),
			},
		}
	}
	return stages
}

// ---------------------------------------------------------------------------
// Test: GetStages pagination — 2 pages (100 + 20 stages)
// ---------------------------------------------------------------------------

// TestEnrichAPIGatewayStage_PaginatesStages verifies that the enricher
// follows NextToken across two pages and writes the total count (120) to
// Fields["stages_count"].
func TestEnrichAPIGatewayStage_PaginatesStages(t *testing.T) {
	const apiID = "paginated-api-001"

	fake := newAPiGWPaginatedFake()

	// Page 1: 100 good stages, NextToken="s1"
	// Page 2: 20 good stages, NextToken nil (last page)
	fake.pages[apiID] = []*apigatewayv2.GetStagesOutput{
		{
			Items:     makeStagesGood(100),
			NextToken: aws.String("s1"),
		},
		{
			Items:     makeStagesGood(20),
			NextToken: nil,
		},
	}

	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apiID)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// stages_count must reflect both pages
	updates, ok := result.FieldUpdates[apiID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", apiID)
	}
	wantCount := "120"
	if updates["stages_count"] != wantCount {
		t.Errorf("stages_count = %q, want %q", updates["stages_count"], wantCount)
	}

	// GetStages must have been called twice
	calls := fake.callCounts[apiID]
	if calls != 2 {
		t.Errorf("GetStages called %d times, want 2", calls)
	}

	// No findings — all stages are properly configured
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

// ---------------------------------------------------------------------------
// Test: GetStages capped at PerParentPageCap → "1000+"
// ---------------------------------------------------------------------------

// TestEnrichAPIGatewayStage_CappedAtPerParentPageCap verifies that when
// NextToken is always non-nil (simulating an API with enormous stage count),
// the enricher stops after PerParentPageCap pages and writes "1000+" to
// stages_count.
func TestEnrichAPIGatewayStage_CappedAtPerParentPageCap(t *testing.T) {
	const apiID = "huge-api-001"

	fake := newAPiGWPaginatedFake()

	// Build PerParentPageCap+2 pages, all with NextToken set, 100 stages/page.
	pages := make([]*apigatewayv2.GetStagesOutput, awsclient.PerParentPageCap+2)
	for i := range pages {
		pages[i] = &apigatewayv2.GetStagesOutput{
			Items:     makeStagesGood(100),
			NextToken: aws.String(fmt.Sprintf("stage-token-%d", i+1)),
		}
	}
	fake.pages[apiID] = pages

	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apiID)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// GetStages must be called exactly PerParentPageCap times
	calls := fake.callCounts[apiID]
	if calls != awsclient.PerParentPageCap {
		t.Errorf("GetStages called %d times, want exactly %d (PerParentPageCap)", calls, awsclient.PerParentPageCap)
	}

	// stages_count must carry "+" suffix
	updates, ok := result.FieldUpdates[apiID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", apiID)
	}
	sc := updates["stages_count"]
	if !strings.HasSuffix(sc, "+") {
		t.Errorf("stages_count = %q, want suffix \"+\" (approximate)", sc)
	}
	wantPrefix := fmt.Sprintf("%d+", awsclient.PerParentPageCap*100)
	if sc != wantPrefix {
		t.Errorf("stages_count = %q, want %q", sc, wantPrefix)
	}
}

// ---------------------------------------------------------------------------
// Test: zero stages across all pages → "0", no per-stage findings
// ---------------------------------------------------------------------------

// TestEnrichAPIGatewayStage_ZeroStagesAcrossPages verifies that when all
// pages return 0 stages, stages_count is "0" and no per-stage findings
// are emitted.
func TestEnrichAPIGatewayStage_ZeroStagesAcrossPages(t *testing.T) {
	const apiID = "empty-api-001"

	fake := newAPiGWPaginatedFake()

	// Single page, 0 stages, no NextToken
	fake.pages[apiID] = []*apigatewayv2.GetStagesOutput{
		{
			Items:     []apigwtypes.Stage{},
			NextToken: nil,
		},
	}

	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apiID)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// stages_count must be "0"
	updates, ok := result.FieldUpdates[apiID]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", apiID)
	}
	if updates["stages_count"] != "0" {
		t.Errorf("stages_count = %q, want \"0\"", updates["stages_count"])
	}

	// A "no deployed stages" finding must be emitted per docs/attention-signals.md Wave 2.
	f, ok := result.Findings[apiID]
	if !ok {
		t.Errorf("expected 1 finding for 0 stages (no deployed stages), got 0")
	} else {
		if f.Severity != "~" {
			t.Errorf("finding Severity = %q, want \"~\"", f.Severity)
		}
		if !strings.Contains(strings.ToLower(f.Summary), "no deployed") {
			t.Errorf("finding Summary = %q, must contain \"no deployed\"", f.Summary)
		}
	}
}
