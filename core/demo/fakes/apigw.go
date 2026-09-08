// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// GetStages returns the fixture-registered stages for the requested API,
// backing EnrichAPIGatewayStage's throttling and access-log checks
// (apigw.stage-config-issues / apigw.no-deployed-stages).
func (f *APIGWFake) GetStages(_ context.Context, input *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	if input == nil || input.ApiId == nil {
		return &apigatewayv2.GetStagesOutput{}, nil
	}
	if !f.hasAPI(*input.ApiId) {
		return nil, &apigwtypes.NotFoundException{Message: notFoundMessage("Api", *input.ApiId)}
	}
	return &apigatewayv2.GetStagesOutput{Items: f.fix.Stages[*input.ApiId]}, nil
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

// GetVpcLinks returns the fixture-registered VPC links (account-wide),
// backing the apigw:elb related-panel pivot (checkApigwELB).
func (f *APIGWFake) GetVpcLinks(_ context.Context, _ *apigatewayv2.GetVpcLinksInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetVpcLinksOutput, error) {
	return &apigatewayv2.GetVpcLinksOutput{Items: f.fix.VpcLinks}, nil
}

// GetAuthorizers returns the fixture-registered authorizers for the requested
// API, backing the apigw:role related-panel pivot (checkApigwRole).
func (f *APIGWFake) GetAuthorizers(_ context.Context, input *apigatewayv2.GetAuthorizersInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error) {
	if input == nil || input.ApiId == nil {
		return &apigatewayv2.GetAuthorizersOutput{}, nil
	}
	return &apigatewayv2.GetAuthorizersOutput{Items: f.fix.Authorizers[*input.ApiId]}, nil
}

// hasAPI reports whether the fixtures register this API id. An API with no
// stages still answers empty; an API that does not exist does not.
func (f *APIGWFake) hasAPI(id string) bool {
	return slices.ContainsFunc(f.fix.APIs, func(a apigwtypes.Api) bool {
		return aws.ToString(a.ApiId) == id
	})
}
