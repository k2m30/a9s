package unit

// aws_apigw_v1_rest_test.go — Failing tests for v1 REST API Gateway merge behavior.
//
// CODER CHECKLIST — new exports required from internal/aws/:
//
//   1. Add go.mod dependency:
//      go get github.com/aws/aws-sdk-go-v2/service/apigateway
//
//   2. Interface in internal/aws/interfaces_apigateway.go:
//      import "github.com/aws/aws-sdk-go-v2/service/apigateway"
//
//      type APIGatewayV1GetRestApisAPI interface {
//          GetRestApis(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error)
//      }
//      type APIGatewayV1API interface {
//          APIGatewayV1GetRestApisAPI
//      }
//
//   3. New field in ServiceClients (internal/aws/client.go):
//      APIGatewayV1 APIGatewayV1API
//
//   4. New exported function in internal/aws/apigw.go:
//      // FetchAPIGatewaysPageMerged fetches a single page of API Gateways from both
//      // APIGateway V2 (HTTP/WEBSOCKET) and APIGateway V1 (REST), merging results.
//      // v1 REST APIs are returned with Fields["protocol"] == "REST".
//      func FetchAPIGatewaysPageMerged(ctx context.Context, clients *ServiceClients, continuationToken string) (resource.FetchResult, error)
//
//   5. Update init() registration in internal/aws/apigw.go to call
//      FetchAPIGatewaysPageMerged instead of FetchAPIGatewaysPage.
//
//   6. v1 SDK types used in the test:
//      package: github.com/aws/aws-sdk-go-v2/service/apigateway
//      types:   github.com/aws/aws-sdk-go-v2/service/apigateway/types
//      RestApi fields: Id *string, Name *string, Description *string
//      GetRestApisInput: (no required fields; Position *string for pagination)
//      GetRestApisOutput: Items []apigatewaytypes.RestApi, Position *string
//
// NOTE: This file will fail to compile until the coder:
//   (a) adds github.com/aws/aws-sdk-go-v2/service/apigateway to go.mod, AND
//   (b) adds the interfaces and FetchAPIGatewaysPageMerged function above.
// That is intentional — this is the TDD red phase.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Fake: APIGatewayV1GetRestApisAPI
//
// This uses the interface the coder will define. Until then, the
// compile-time check below will fail.
//
// CODER: After adding APIGatewayV1GetRestApisAPI to interfaces_apigateway.go and
// adding the v1 SDK to go.mod, restore the real implementation below and
// uncomment the apigateway/apigatewaytypes imports above.
// ---------------------------------------------------------------------------

// fakeAPIGWV1RestApis implements awsclient.APIGatewayV1GetRestApisAPI for testing.
// It returns a fixed GetRestApisOutput on every call (single page, no pagination).
type fakeAPIGWV1RestApis struct {
	output *apigateway.GetRestApisOutput
}

func (f *fakeAPIGWV1RestApis) GetRestApis(
	_ context.Context,
	_ *apigateway.GetRestApisInput,
	_ ...func(*apigateway.Options),
) (*apigateway.GetRestApisOutput, error) {
	if f.output == nil {
		return &apigateway.GetRestApisOutput{}, nil
	}
	return f.output, nil
}

// Compile-time check: fakeAPIGWV1RestApis satisfies awsclient.APIGatewayV1GetRestApisAPI.
var _ awsclient.APIGatewayV1GetRestApisAPI = (*fakeAPIGWV1RestApis)(nil)
var _ awsclient.APIGatewayV1API = (*fakeAPIGWV1RestApis)(nil)

// ---------------------------------------------------------------------------
// Fake: APIGatewayV2GetApisAPI (minimal — only GetApis and GetStages needed here)
// ---------------------------------------------------------------------------

type v1RestTestAPIGWV2Fake struct {
	awsclient.APIGatewayV2API
	items []apigwv2types.Api
}

func (f *v1RestTestAPIGWV2Fake) GetApis(
	_ context.Context,
	_ *apigatewayv2.GetApisInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{Items: f.items}, nil
}

func (f *v1RestTestAPIGWV2Fake) GetStages(
	_ context.Context,
	_ *apigatewayv2.GetStagesInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{}, nil
}

var _ awsclient.APIGatewayV2API = (*v1RestTestAPIGWV2Fake)(nil)

// ---------------------------------------------------------------------------
// TestFetchAPIGateways_IncludesRestV1
//
// Verifies that FetchAPIGatewaysPageMerged returns resources from BOTH
// APIGateway v2 (HTTP protocol) AND APIGateway v1 (REST protocol).
//
// CODER: After adding FetchAPIGatewaysPageMerged and the v1 types, replace
// the placeholder v1Fake below with a real fakeAPIGWV1RestApis that implements
// APIGatewayV1GetRestApisAPI, and set clients.APIGatewayV1 = v1Fake.
// ---------------------------------------------------------------------------

func TestFetchAPIGateways_IncludesRestV1(t *testing.T) {
	// Two v2 HTTP APIs
	v2Fake := &v1RestTestAPIGWV2Fake{
		items: []apigwv2types.Api{
			{
				ApiId:        aws.String("v2-api-001"),
				Name:         aws.String("my-http-api-1"),
				ProtocolType: apigwv2types.ProtocolTypeHttp,
				ApiEndpoint:  aws.String("https://v2-api-001.execute-api.us-east-1.amazonaws.com"),
			},
			{
				ApiId:        aws.String("v2-api-002"),
				Name:         aws.String("my-http-api-2"),
				ProtocolType: apigwv2types.ProtocolTypeHttp,
				ApiEndpoint:  aws.String("https://v2-api-002.execute-api.us-east-1.amazonaws.com"),
			},
		},
	}

	v1Fake := &fakeAPIGWV1RestApis{output: &apigateway.GetRestApisOutput{
		Items: []apigatewaytypes.RestApi{
			{Id: aws.String("rest-api-001"), Name: aws.String("my-rest-api-1"), Description: aws.String("First REST API")},
			{Id: aws.String("rest-api-002"), Name: aws.String("my-rest-api-2"), Description: aws.String("Second REST API")},
		},
	}}

	clients := &awsclient.ServiceClients{
		APIGatewayV2: v2Fake,
		APIGatewayV1: v1Fake,
	}

	result, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Resources) != 4 {
		t.Errorf("len(resources) = %d, want 4 (2 v2 + 2 v1 REST)", len(result.Resources))
	}

	foundREST := false
	foundHTTP := false
	for _, r := range result.Resources {
		proto := r.Fields["protocol"]
		if proto == "REST" {
			foundREST = true
		}
		if proto == "HTTP" {
			foundHTTP = true
		}
	}
	if !foundREST {
		t.Error("no resource with Fields[\"protocol\"]==\"REST\" — v1 REST APIs must be merged")
	}
	if !foundHTTP {
		t.Error("no resource with Fields[\"protocol\"]==\"HTTP\" — v2 HTTP APIs missing")
	}
}

// ---------------------------------------------------------------------------
// TestFetchAPIGateways_V1Only
//
// Verifies that when only v1 REST APIs exist (v2 returns empty), all
// resources appear and each has Fields["protocol"] == "REST".
// ---------------------------------------------------------------------------

func TestFetchAPIGateways_V1Only(t *testing.T) {
	v2Fake := &v1RestTestAPIGWV2Fake{
		items: []apigwv2types.Api{},
	}

	v1Fake := &fakeAPIGWV1RestApis{output: &apigateway.GetRestApisOutput{
		Items: []apigatewaytypes.RestApi{
			{Id: aws.String("rest-api-101"), Name: aws.String("my-rest-api-a"), Description: aws.String("REST A")},
			{Id: aws.String("rest-api-102"), Name: aws.String("my-rest-api-b"), Description: aws.String("REST B")},
			{Id: aws.String("rest-api-103"), Name: aws.String("my-rest-api-c"), Description: aws.String("REST C")},
		},
	}}
	clients := &awsclient.ServiceClients{
		APIGatewayV2: v2Fake,
		APIGatewayV1: v1Fake,
	}

	result, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With v1 returning 3 REST APIs and v2 empty, expect 3 total.
	if len(result.Resources) != 3 {
		t.Errorf("len(resources) = %d, want 3 (3 v1 REST only)", len(result.Resources))
	}

	for _, r := range result.Resources {
		proto := r.Fields["protocol"]
		if proto != "REST" {
			t.Errorf("resource %q has protocol=%q, want \"REST\"", r.ID, proto)
		}
	}
}
