package unit

// FetchAPIGatewaysPageMerged (core/aws/apigw.go) fetches a single page of API
// Gateways from both APIGateway V2 (HTTP/WEBSOCKET) and APIGateway V1 (REST),
// merging results; v1 REST APIs are returned with Fields["protocol"] ==
// "REST". ServiceClients.APIGatewayV1 satisfies APIGatewayV1API
// (core/aws/apigw_interfaces.go).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

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

var _ awsclient.APIGatewayV1GetRestApisAPI = (*fakeAPIGWV1RestApis)(nil)
var _ awsclient.APIGatewayV1API = (*fakeAPIGWV1RestApis)(nil)

type v1RestTestAPIGWV2Fake struct {
	awsclient.APIGatewayV2API
	items []apigwv2types.Api
}

// GetAuthorizers answers rather than leaving the call to the embedded
// nil interface. EnrichAPIGatewayStage calls it for every v2 API, and a nil
// embedded field dereferences into a SIGSEGV that takes the whole unit
// package down before any other test reports.
func (f *v1RestTestAPIGWV2Fake) GetAuthorizers(
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

func TestFetchAPIGateways_IncludesRestV1(t *testing.T) {
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
