// aws_codex_round2_test.go pins three related-checker contracts.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// AWS docs (CodePipeline resource ARN format): a pipeline's ARN is
// arn:aws:codepipeline:<region>:<account>:<pipelineName>, with no
// "pipeline/" segment (unlike, e.g., IAM policy ARNs).

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

// lambda:ListEventSourceMappings is the authoritative link between a stream
// and its triggers (docs/resources/kinesis.md, docs/resources/msk.md), so a
// cold lambda cache must not turn an API-confirmed trigger into a definitive
// zero.

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

	cache := resource.ResourceCache{}

	checker := checkerByTarget(t, "kinesis", "lambda")
	result := checker(context.Background(), clients, streamRes, cache)

	if result.Count() == 0 && !result.Truncated() {
		t.Fatalf("Count = %d (definitive zero), want a non-definitive-zero result — ListEventSourceMappings found a real mapping, the API result is authoritative even with a cold lambda cache", result.Count())
	}
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (one real event-source mapping found via the API)", result.Count())
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "checkout-consumer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ResourceIDs = %v, want to contain %q (bare function name derived from FunctionArn)", result.ResourceIDs(), "checkout-consumer")
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

	cache := resource.ResourceCache{}

	checker := checkerByTarget(t, "msk", "lambda")
	result := checker(context.Background(), clients, clusterRes, cache)

	if result.Count() == 0 && !result.Truncated() {
		t.Fatalf("Count = %d (definitive zero), want a non-definitive-zero result — ListEventSourceMappings found a real mapping, the API result is authoritative even with a cold lambda cache", result.Count())
	}
	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (one real event-source mapping found via the API)", result.Count())
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "checkout-consumer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ResourceIDs = %v, want to contain %q (bare function name derived from FunctionArn)", result.ResourceIDs(), "checkout-consumer")
	}
}

// GetIntegrationsOutput.NextToken means the private integration can be on any
// page. The fake errors on a call past its last page, so a checker that loops
// forever fails instead of hanging.

type fakeAPIGWV2IntegrationsPaginated struct {
	awsclient.APIGatewayV2API
	pages     [][]apigwv2types.Integration
	calls     int
	gotTokens []string
}

// GetAuthorizers answers rather than leaving the call to the embedded
// nil interface. EnrichAPIGatewayStage calls it for every v2 API, and a nil
// embedded field dereferences into a SIGSEGV that takes the whole unit
// package down before any other test reports.
func (f *fakeAPIGWV2IntegrationsPaginated) GetAuthorizers(
	_ context.Context,
	_ *apigatewayv2.GetAuthorizersInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetAuthorizersOutput, error) {
	// One authorizer keeps the apigw.no-authorizer finding out of these results.
	return &apigatewayv2.GetAuthorizersOutput{Items: []apigwv2types.Authorizer{{
		AuthorizerId: aws.String("auth-default"),
		Name:         aws.String("acme-jwt"),
	}}}, nil
}

func (f *fakeAPIGWV2IntegrationsPaginated) GetIntegrations(_ context.Context, params *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	f.calls++
	f.gotTokens = append(f.gotTokens, aws.ToString(params.NextToken))
	if f.calls > len(f.pages) {
		return nil, errAPIGWTooManyIntegrationsCalls
	}
	out := &apigatewayv2.GetIntegrationsOutput{Items: f.pages[f.calls-1]}
	if f.calls < len(f.pages) {
		out.NextToken = aws.String("integrations-page-token-1")
	}
	return out, nil
}

var errAPIGWTooManyIntegrationsCalls = &apigwTooManyCallsErr{}

type apigwTooManyCallsErr struct{}

func (*apigwTooManyCallsErr) Error() string {
	return "GetIntegrations called more times than there are pages — infinite loop guard tripped"
}

// An HTTP API private integration names its load balancer by a listener ARN
// in IntegrationUri, and that integration can sit on any page of
// GetIntegrations.
func TestApigw_Related_ELB_GetIntegrations_FollowsPagination(t *testing.T) {
	const listenerARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/checkout-nlb/5d1b75f4f1cee11e/8e4497da625e2d8a"
	fake := &fakeAPIGWV2IntegrationsPaginated{
		pages: [][]apigwv2types.Integration{
			{{IntegrationId: aws.String("int0001"), IntegrationType: apigwv2types.IntegrationTypeHttpProxy, ConnectionType: apigwv2types.ConnectionTypeInternet,
				IntegrationUri: aws.String("https://status.acme.example.com"), IntegrationMethod: aws.String("ANY"), PayloadFormatVersion: aws.String("1.0")}},
			{{IntegrationId: aws.String("int0002"), IntegrationType: apigwv2types.IntegrationTypeHttpProxy, ConnectionType: apigwv2types.ConnectionTypeVpcLink,
				ConnectionId: aws.String("vpcl-0checkout"), IntegrationUri: aws.String(listenerARN), IntegrationMethod: aws.String("ANY"), PayloadFormatVersion: aws.String("1.0")}},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake, Region: "us-east-1"}
	cache := resource.ResourceCache{"elb": {Resources: []resource.Resource{
		t568LB("checkout-nlb", "net", "5d1b75f4f1cee11e", "subnet-0a1b2c3d4e5f60001"),
		t568LB("checkout-nlb-canary", "net", "0f1e2d3c4b5a6978", "subnet-0a1b2c3d4e5f60001"),
	}}}

	result := checkerByTarget(t, "apigw", "elb")(context.Background(), clients, t568HTTPAPI("abc123def", "checkout-api"), cache)

	t568RequireExact(t, "apigw → elb", result, "checkout-nlb")
	if fake.calls != 2 || len(fake.gotTokens) != 2 || fake.gotTokens[1] != "integrations-page-token-1" {
		t.Fatalf("GetIntegrations calls = %d tokens %v, want 2 calls with the page-1 NextToken passed back", fake.calls, fake.gotTokens)
	}
}

func safeIndex(s []string, i int) string {
	if i < 0 || i >= len(s) {
		return "<missing>"
	}
	return s[i]
}
