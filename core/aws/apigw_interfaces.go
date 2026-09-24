// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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

// APIGatewayV2GetAuthorizersAPI lists the authorizers configured for a given
// API. Used to resolve apigw→role via AuthorizerCredentialsArn.
type APIGatewayV2GetAuthorizersAPI interface {
	GetAuthorizers(ctx context.Context, params *apigatewayv2.GetAuthorizersInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error)
}

// The two Wave 2 interfaces below are deliberately NOT folded into
// APIGatewayV1API. The enricher type-asserts for them off clients.APIGatewayV1
// the way EnrichCodeArtifactRepository does for CodeArtifactListPackagesAPI,
// so a client or fake that implements only the aggregate keeps satisfying it
// instead of failing to compile.

// APIGatewayV1GetAuthorizersAPI lists the authorizers configured for a REST
// API. Used by Wave 2 enrichment to tell an unauthenticated API from one
// behind an authorizer.
type APIGatewayV1GetAuthorizersAPI interface {
	GetAuthorizers(ctx context.Context, params *apigateway.GetAuthorizersInput, optFns ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error)
}

// APIGatewayV1GetStagesAPI lists the stages of a REST API. Used by Wave 2
// enrichment to inspect access logging, tracing and stage variables.
type APIGatewayV1GetStagesAPI interface {
	GetStages(ctx context.Context, params *apigateway.GetStagesInput, optFns ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error)
}

// APIGatewayV1GetResourcesAPI lists a REST API's resources; with the
// "methods" embed each carries its methods and their integrations.
type APIGatewayV1GetResourcesAPI interface {
	GetResources(ctx context.Context, params *apigateway.GetResourcesInput, optFns ...func(*apigateway.Options)) (*apigateway.GetResourcesOutput, error)
}

// APIGatewayV1GetVpcLinksAPI lists the account's REST API VPC links.
type APIGatewayV1GetVpcLinksAPI interface {
	GetVpcLinks(ctx context.Context, params *apigateway.GetVpcLinksInput, optFns ...func(*apigateway.Options)) (*apigateway.GetVpcLinksOutput, error)
}

// APIGatewayV1GetBasePathMappingsAPI lists a custom domain's base path
// mappings, how an edge-optimized domain maps REST APIs.
type APIGatewayV1GetBasePathMappingsAPI interface {
	GetBasePathMappings(ctx context.Context, params *apigateway.GetBasePathMappingsInput, optFns ...func(*apigateway.Options)) (*apigateway.GetBasePathMappingsOutput, error)
}

// APIGatewayV2ListRoutingRulesAPI lists a custom domain's routing rules,
// each of which invokes a REST API stage.
type APIGatewayV2ListRoutingRulesAPI interface {
	ListRoutingRules(ctx context.Context, params *apigatewayv2.ListRoutingRulesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.ListRoutingRulesOutput, error)
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
	APIGatewayV2GetAuthorizersAPI  // related-panel: apigw→role
}
