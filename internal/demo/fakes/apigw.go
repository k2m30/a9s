package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// APIGWFake implements aws.APIGatewayV2API against fixture data loaded at construction time.
type APIGWFake struct {
	fix *fixtures.APIGWFixtures
}

// NewAPIGW constructs an APIGWFake backed by fixture data from the fixtures package.
func NewAPIGW() *APIGWFake {
	return &APIGWFake{fix: fixtures.NewAPIGWFixtures()}
}

func (f *APIGWFake) GetApis(_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{Items: f.fix.APIs}, nil
}

// GetStages returns an empty stage list for demo mode.
// Wave 2 enrichment uses this to check throttling and access-log settings;
// returning no stages produces no findings in demo mode.
func (f *APIGWFake) GetStages(_ context.Context, _ *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{}, nil
}

// GetDomainNames returns the fixture-registered custom domain names.
func (f *APIGWFake) GetDomainNames(_ context.Context, _ *apigatewayv2.GetDomainNamesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error) {
	return &apigatewayv2.GetDomainNamesOutput{Items: f.fix.DomainNames}, nil
}

// GetApiMappings returns the fixture-registered mappings for the requested domain.
func (f *APIGWFake) GetApiMappings(_ context.Context, input *apigatewayv2.GetApiMappingsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error) {
	if input == nil || input.DomainName == nil {
		return &apigatewayv2.GetApiMappingsOutput{}, nil
	}
	return &apigatewayv2.GetApiMappingsOutput{Items: f.fix.ApiMappings[*input.DomainName]}, nil
}

// GetIntegrations returns the fixture-registered integrations for the requested API.
func (f *APIGWFake) GetIntegrations(_ context.Context, input *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	if input == nil || input.ApiId == nil {
		return &apigatewayv2.GetIntegrationsOutput{}, nil
	}
	return &apigatewayv2.GetIntegrationsOutput{Items: f.fix.Integrations[*input.ApiId]}, nil
}
