package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
)

// APIGatewayV1GetRestApisAPI defines the interface for the APIGateway v1 GetRestApis operation.
type APIGatewayV1GetRestApisAPI interface {
	GetRestApis(ctx context.Context, params *apigateway.GetRestApisInput, optFns ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error)
}

// APIGatewayV2GetApisAPI defines the interface for the API Gateway V2 GetApis operation.
type APIGatewayV2GetApisAPI interface {
	GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
}

// APIGatewayV2GetStagesAPI defines the interface for the API Gateway V2 GetStages operation.
// Used by Wave 2 enrichment to inspect stage-level configuration per API.
type APIGatewayV2GetStagesAPI interface {
	GetStages(ctx context.Context, params *apigatewayv2.GetStagesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error)
}

// APIGatewayV2GetDomainNamesAPI lists custom domain names registered for
// HTTP/WebSocket APIs. Used to resolve apigw→acm, apigw→r53.
type APIGatewayV2GetDomainNamesAPI interface {
	GetDomainNames(ctx context.Context, params *apigatewayv2.GetDomainNamesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetDomainNamesOutput, error)
}

// APIGatewayV2GetApiMappingsAPI returns API→stage mappings for a given
// custom domain. Used with GetDomainNames to determine which domains map
// to a given API.
type APIGatewayV2GetApiMappingsAPI interface {
	GetApiMappings(ctx context.Context, params *apigatewayv2.GetApiMappingsInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApiMappingsOutput, error)
}

// APIGatewayV2GetIntegrationsAPI lists integrations (Lambda, SFN, SNS, HTTP)
// for a given API. Used to resolve apigw→lambda/sfn/sns.
type APIGatewayV2GetIntegrationsAPI interface {
	GetIntegrations(ctx context.Context, params *apigatewayv2.GetIntegrationsInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error)
}

// APIGatewayV1API is the aggregate interface covering APIGateway v1 (REST) operations used by a9s fetchers.
// *apigateway.Client structurally satisfies this interface.
type APIGatewayV1API interface {
	APIGatewayV1GetRestApisAPI
}

// APIGatewayV2API is the aggregate interface covering all APIGatewayV2 operations used by a9s fetchers.
// *apigatewayv2.Client structurally satisfies this interface.
type APIGatewayV2API interface {
	APIGatewayV2GetApisAPI
	APIGatewayV2GetStagesAPI       // Wave 2 enrichment
	APIGatewayV2GetDomainNamesAPI  // custom domain → ACM/R53 pivot
	APIGatewayV2GetApiMappingsAPI  // domain → API mapping pivot
	APIGatewayV2GetIntegrationsAPI // Lambda/SFN/SNS integration pivot
}
