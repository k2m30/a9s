// aws_codex_round2_test.go pins three externally-reviewed findings against
// HEAD. Each test is expected to be RED until the paired coder task lands
// the fix; the assertions encode the documented/correct mechanism, not the
// current (broken) behavior.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// ---------------------------------------------------------------------------
// Finding 1 — CodePipeline ARN format.
//
// AWS docs (CodePipeline resource ARN format): a pipeline's ARN is
// arn:aws:codepipeline:<region>:<account>:<pipelineName> — there is NO
// "pipeline/" segment (unlike, e.g., IAM policy ARNs). internal/aws/pipeline.go
// (fetchCodePipelinesPage, ~line 99) currently constructs
// arn:aws:codepipeline:<region>:<account>:pipeline/<name>, which is wrong.
// This test drives the real production fetch path
// (FetchCodePipelinesPageWithClients, mirroring
// TestPipeline_Related_EbRule_ResolvesViaRealFetcherOutput in
// aws_related_checker_round2_test.go) and asserts the correct ARN shape.
// ---------------------------------------------------------------------------

type fakeCodePipelineListPipelinesRound2 struct {
	awsclient.CodePipelineAPI
	pipelines []cptypes.PipelineSummary
}

func (f *fakeCodePipelineListPipelinesRound2) ListPipelines(_ context.Context, _ *codepipeline.ListPipelinesInput, _ ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error) {
	return &codepipeline.ListPipelinesOutput{Pipelines: f.pipelines}, nil
}

func TestPipeline_Fetch_ArnHasNoPipelineSegment(t *testing.T) {
	wantARN := "arn:aws:codepipeline:us-east-1:123456789012:acme-api-deploy"

	listAPI := &fakeCodePipelineListPipelinesRound2{
		pipelines: []cptypes.PipelineSummary{{Name: aws.String("acme-api-deploy")}},
	}

	identity := session.NewIdentityStore()
	identity.Set("123456789012", nil)
	fetchClients := &awsclient.ServiceClients{CodePipeline: listAPI, Region: "us-east-1"}
	fetchClients.SetIdentityStore(identity)

	fetchResult, err := awsclient.FetchCodePipelinesPageWithClients(context.Background(), fetchClients, "")
	if err != nil {
		t.Fatalf("FetchCodePipelinesPageWithClients returned error: %v", err)
	}
	if len(fetchResult.Resources) != 1 {
		t.Fatalf("got %d resources, want 1", len(fetchResult.Resources))
	}
	if got := fetchResult.Resources[0].Fields["arn"]; got != wantARN {
		t.Fatalf("Fields[arn] = %q, want %q (AWS CodePipeline resource ARNs have no \"pipeline/\" segment)", got, wantARN)
	}
}

// ---------------------------------------------------------------------------
// Finding 2 — cold-cache Lambda triggers must not report a definitive zero.
//
// internal/aws/related_common.go (lambdaEventSourceMappingLambdaCheck,
// ~lines 150-154): when lambda:ListEventSourceMappings returns a mapping but
// the lambda ResourceCache entry is not loaded ("cache[\"lambda\"]" absent),
// the checker returns RelatedCheckResult{TargetType:"lambda"} — Count:0,
// Truncated:false — a definitive zero. But the ListEventSourceMappings API
// call already succeeded and is authoritative (kinesis.md/msk.md: "ESM is
// the authoritative link" / "the API result is authoritative"); a cold
// lambda cache must not erase a real API-confirmed trigger. This test
// covers both call sites: checkKinesisLambda and checkMSKLambda.
// ---------------------------------------------------------------------------

type fakeLambdaListEventSourceMappingsRound2 struct {
	awsclient.LambdaAPI
	byEventSourceArn map[string][]string // eventSourceArn -> FunctionArn(s)
}

func (f *fakeLambdaListEventSourceMappingsRound2) ListEventSourceMappings(_ context.Context, params *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	if params.EventSourceArn == nil {
		return &lambda.ListEventSourceMappingsOutput{}, nil
	}
	fnArns, ok := f.byEventSourceArn[*params.EventSourceArn]
	if !ok {
		return &lambda.ListEventSourceMappingsOutput{}, nil
	}
	var mappings []lambdatypes.EventSourceMappingConfiguration
	for _, arn := range fnArns {
		mappings = append(mappings, lambdatypes.EventSourceMappingConfiguration{FunctionArn: aws.String(arn)})
	}
	return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: mappings}, nil
}

func TestKinesis_Related_Lambda_ColdCache_NotDefinitiveZero(t *testing.T) {
	streamARN := "arn:aws:kinesis:us-east-1:123456789012:stream/checkout-events"
	functionArn := "arn:aws:lambda:us-east-1:123456789012:function:checkout-consumer"

	fake := &fakeLambdaListEventSourceMappingsRound2{
		byEventSourceArn: map[string][]string{streamARN: {functionArn}},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	streamRes := resource.Resource{
		ID:   "checkout-events",
		Name: "checkout-events",
		Fields: map[string]string{
			"stream_arn": streamARN,
		},
	}

	// No "lambda" entry in the cache at all — the lambda list has not been
	// loaded yet.
	cache := resource.ResourceCache{}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, cache)

	if result.Count == 0 && !result.Truncated {
		t.Fatalf("Count = %d (definitive zero), want a non-definitive-zero result — ListEventSourceMappings found a real mapping, the API result is authoritative even with a cold lambda cache", result.Count)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (one real event-source mapping found via the API)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == "checkout-consumer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ResourceIDs = %v, want to contain %q (bare function name derived from FunctionArn)", result.ResourceIDs, "checkout-consumer")
	}
}

func TestMSK_Related_Lambda_ColdCache_NotDefinitiveZero(t *testing.T) {
	clusterArn := "arn:aws:kafka:us-east-1:123456789012:cluster/checkout-events/abc12345-6789-def0-1234-56789abcdef0-1"
	functionArn := "arn:aws:lambda:us-east-1:123456789012:function:checkout-consumer"

	fake := &fakeLambdaListEventSourceMappingsRound2{
		byEventSourceArn: map[string][]string{clusterArn: {functionArn}},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	clusterRes := resource.Resource{
		ID:        "checkout-events",
		Name:      "checkout-events",
		RawStruct: kafkatypes.Cluster{ClusterArn: aws.String(clusterArn)},
	}

	// No "lambda" entry in the cache at all.
	cache := resource.ResourceCache{}

	checker := checkerByTarget(t, "msk", "lambda")
	result := checker(context.Background(), clients, clusterRes, cache)

	if result.Count == 0 && !result.Truncated {
		t.Fatalf("Count = %d (definitive zero), want a non-definitive-zero result — ListEventSourceMappings found a real mapping, the API result is authoritative even with a cold lambda cache", result.Count)
	}
	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1 (one real event-source mapping found via the API)", result.Count)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == "checkout-consumer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ResourceIDs = %v, want to contain %q (bare function name derived from FunctionArn)", result.ResourceIDs, "checkout-consumer")
	}
}

// ---------------------------------------------------------------------------
// Finding 3 — GetVpcLinks pagination.
//
// internal/aws/apigw_related.go (checkApigwELB, ~line 390) calls
// apigatewayv2:GetVpcLinks exactly once with an empty input, ignoring
// NextToken. GetVpcLinksOutput.NextToken (AWS SDK Go v2 apigatewayv2 API)
// means the wanted VpcLink can be on any page — a single-page read misses
// links that are not on page 1. This test puts the wanted VpcLink on page 2
// and asserts checkApigwELB still finds the ELB, that the fake was called
// with the page-1 NextToken on its second invocation, and that the checker
// does not loop forever (the fake errors on a third call).
// ---------------------------------------------------------------------------

type fakeAPIGWV2VpcLinksPaginated struct {
	awsclient.APIGatewayV2API
	integrations []apigwv2types.Integration
	pages        [][]apigwv2types.VpcLink // pages[0] = page 1, pages[1] = page 2, ...
	calls        int
	gotTokens    []string
}

func (f *fakeAPIGWV2VpcLinksPaginated) GetIntegrations(_ context.Context, _ *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	return &apigatewayv2.GetIntegrationsOutput{Items: f.integrations}, nil
}

func (f *fakeAPIGWV2VpcLinksPaginated) GetVpcLinks(_ context.Context, params *apigatewayv2.GetVpcLinksInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetVpcLinksOutput, error) {
	f.calls++
	token := ""
	if params != nil && params.NextToken != nil {
		token = *params.NextToken
	}
	f.gotTokens = append(f.gotTokens, token)

	if f.calls > len(f.pages) {
		return nil, errAPIGWTooManyVpcLinksCalls
	}

	page := f.pages[f.calls-1]
	out := &apigatewayv2.GetVpcLinksOutput{Items: page}
	if f.calls < len(f.pages) {
		next := "vpclink-page-token-1"
		out.NextToken = &next
	}
	return out, nil
}

var errAPIGWTooManyVpcLinksCalls = &apigwTooManyCallsErr{}

type apigwTooManyCallsErr struct{}

func (*apigwTooManyCallsErr) Error() string {
	return "GetVpcLinks called more times than there are pages — infinite loop guard tripped"
}

func TestApigw_Related_ELB_GetVpcLinks_FollowsPagination(t *testing.T) {
	apiID := "abc123def"
	vpcLinkID := "vpcl-checkout-nlb"
	subnetID := "subnet-0checkout1"

	fake := &fakeAPIGWV2VpcLinksPaginated{
		integrations: []apigwv2types.Integration{
			{
				ConnectionType: apigwv2types.ConnectionTypeVpcLink,
				ConnectionId:   aws.String(vpcLinkID),
			},
		},
		pages: [][]apigwv2types.VpcLink{
			{
				// Page 1: unrelated VPC link — the wanted one is NOT here.
				{VpcLinkId: aws.String("vpcl-unrelated"), SubnetIds: []string{"subnet-unrelated"}},
			},
			{
				// Page 2: the wanted VPC link.
				{VpcLinkId: aws.String(vpcLinkID), SubnetIds: []string{subnetID}},
			},
		},
	}

	clients := &awsclient.ServiceClients{APIGatewayV2: fake}

	elbRes := resource.Resource{
		ID:   "checkout-nlb",
		Name: "checkout-nlb",
		RawStruct: elbv2types.LoadBalancer{
			AvailabilityZones: []elbv2types.AvailabilityZone{
				{SubnetId: aws.String(subnetID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"elb": resource.ResourceCacheEntry{
			Resources: []resource.Resource{elbRes},
		},
	}

	apiRes := resource.Resource{ID: apiID, Name: apiID}

	checker := checkerByTarget(t, "apigw", "elb")
	result := checker(context.Background(), clients, apiRes, cache)

	if result.Count < 1 {
		t.Fatalf("Count = %d, want >=1 — GetVpcLinks must page through to page 2 to find the wanted VpcLink", result.Count)
	}

	if fake.calls < 2 {
		t.Fatalf("GetVpcLinks called %d time(s), want >=2 (must follow NextToken to page 2)", fake.calls)
	}
	if len(fake.gotTokens) < 2 || fake.gotTokens[1] != "vpclink-page-token-1" {
		t.Fatalf("second GetVpcLinks call token = %q, want %q (the checker must pass back the page-1 NextToken)", safeIndex(fake.gotTokens, 1), "vpclink-page-token-1")
	}
	if fake.calls > len(fake.pages) {
		t.Fatalf("GetVpcLinks called %d times, only %d pages exist — infinite loop", fake.calls, len(fake.pages))
	}
}

func safeIndex(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return "<missing>"
	}
	return s[i]
}
