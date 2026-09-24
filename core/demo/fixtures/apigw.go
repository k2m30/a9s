// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

// APIGWFixtures holds typed fixture data for API Gateway V2.
type APIGWFixtures struct {
	APIs []apigwtypes.Api
	// Integrations maps ApiId -> integrations, served by GetIntegrations.
	// Required for the apigw:kms, apigw:lambda and apigw:sfn related-panel
	// pivots (checkApigwKMS / checkApigwLambda / checkApigwSFN all read
	// Integration.IntegrationUri).
	Integrations map[string][]apigwtypes.Integration
	// DomainNames is served by GetDomainNames. Required (together with
	// ApiMappings) for the apigw:acm related-panel pivot (checkApigwACM).
	DomainNames []apigwtypes.DomainName
	// ApiMappings maps DomainName -> mappings, served by GetApiMappings.
	ApiMappings map[string][]apigwtypes.ApiMapping
	// RoutingRules maps DomainName -> routing rules, served by
	// ListRoutingRules.
	RoutingRules map[string][]apigwtypes.RoutingRule
	// Authorizers maps ApiId -> authorizers, served by GetAuthorizers.
	// Required for the apigw:role related-panel pivot (checkApigwRole).
	Authorizers map[string][]apigwtypes.Authorizer
	// Stages maps ApiId -> deployed stages, served by GetStages. Required for
	// the apigw.stage-config-issues / apigw.no-deployed-stages Wave 2 findings
	// (EnrichAPIGatewayStage).
	Stages map[string][]apigwtypes.Stage
}

const (
	// PublicAPIGWID is the graph-root API Gateway for the apigw:kms,
	// apigw:kms, apigw:lambda, apigw:acm and apigw:cf related-panel pivots.
	PublicAPIGWID = "abc123def4"
	// PublicAPIGWDomainName is the custom domain mapped to PublicAPIGWID,
	// required for the apigw:acm pivot (checkApigwACM).
	PublicAPIGWDomainName = "api.acme-corp.com"
	// RoutedAPIGWDomainName is a custom domain in ROUTING_RULE_ONLY mode: its
	// one routing rule invokes APIGWRESTTracingOff, and it has no API
	// mappings.
	RoutedAPIGWDomainName = "orders.acme-corp.com"
	// APIGWVpcLinkID is the VPC link ID bound to PublicAPIGWID's VPC_LINK
	// integration.
	APIGWVpcLinkID = "vpcl-0aaa111111111111a"
	// APIGWVpcLinkSecurityGroupID is the security group of the VPC link to
	// the internal NLB, which the acme-prod-nlb fixture (elb.go) carries.
	APIGWVpcLinkSecurityGroupID = "sg-0vpcl11111111111a"
	// APIGWBackendListenerARN is the acme-prod-nlb listener PublicAPIGWID's
	// private integration names in IntegrationUri — required for the
	// apigw:elb related-panel pivot (checkApigwELB).
	APIGWBackendListenerARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/net/acme-prod-nlb/abcdef1234567890/0f1e2d3c4b5a6978"
	// HealthyAPIGWID is the only API with a deployed stage that carries both
	// non-zero throttling and access logging, so it is the only demo API in
	// the apigw Healthy color bucket (colorAPIGW falls through to structural
	// Healthy only when EnrichAPIGatewayStage raises no finding at all).
	HealthyAPIGWID = "opq234rst5"
)

// NewAPIGWFixtures constructs APIGWFixtures from the canonical demo data.
var sharedAPIGWFixtures = sync.OnceValue(func() *APIGWFixtures {
	return &APIGWFixtures{
		APIs: []apigwtypes.Api{
			{
				ApiId:                     aws.String(PublicAPIGWID),
				Name:                      aws.String("acme-public-api"),
				ProtocolType:              apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:               aws.String("https://" + PublicAPIGWID + ".execute-api.us-east-1.amazonaws.com"),
				Description:               aws.String("Public REST API for Acme Corp mobile and web clients"),
				RouteSelectionExpression:  aws.String("${request.method} ${request.path}"),
				CreatedDate:               aws.Time(time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)),
				ApiKeySelectionExpression: aws.String("$request.header.x-api-key"),
				CorsConfiguration:         &apigwtypes.Cors{AllowMethods: []string{"GET", "POST", "PUT", "DELETE"}, AllowOrigins: []string{"https://app.acme-corp.com"}},
				// api-gateway-authorizer tag — required for lambda→apigw
				// related-panel pivot. checkLambdaAPIGW matches api.Tags[fnName].
				Tags: map[string]string{"Environment": "production", "api-gateway-authorizer": "custom-authorizer"},
			},
			{
				ApiId:                    aws.String("efg567hij8"),
				Name:                     aws.String("acme-websocket-api"),
				ProtocolType:             apigwtypes.ProtocolTypeWebsocket,
				ApiEndpoint:              aws.String("wss://efg567hij8.execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("WebSocket API for real-time order notifications"),
				RouteSelectionExpression: aws.String("$request.body.action"),
				CreatedDate:              aws.Time(time.Date(2025, 7, 5, 14, 30, 0, 0, time.UTC)),
			},
			{
				ApiId:                    aws.String("klm901nop2"),
				Name:                     aws.String("internal-service-api"),
				ProtocolType:             apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:              aws.String("https://klm901nop2.execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("Internal microservice-to-microservice API"),
				RouteSelectionExpression: aws.String("${request.method} ${request.path}"),
				CreatedDate:              aws.Time(time.Date(2025, 9, 1, 11, 0, 0, 0, time.UTC)),
			},
			// HealthyAPIGWID's "prod" stage has non-zero throttling and access
			// logging, so EnrichAPIGatewayStage raises no finding for it — the
			// only demo API that resolves to colorAPIGW's Healthy fallback.
			{
				ApiId:                    aws.String(HealthyAPIGWID),
				Name:                     aws.String("acme-partner-api"),
				ProtocolType:             apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:              aws.String("https://" + HealthyAPIGWID + ".execute-api.us-east-1.amazonaws.com"),
				Description:              aws.String("Partner integration API with production-grade throttling and access logging"),
				RouteSelectionExpression: aws.String("${request.method} ${request.path}"),
				CreatedDate:              aws.Time(time.Date(2025, 10, 12, 8, 0, 0, 0, time.UTC)),
			},
		},
		// Integrations for PublicAPIGWID — required for the apigw:kms and
		// apigw:lambda pivots (checkApigwKMS / checkApigwLambda both
		// scan GetIntegrations output for IntegrationUri containing
		// ":function:"). api-gateway-authorizer is a real lambda.go fixture
		// with KMSKeyArn set.
		Integrations: map[string][]apigwtypes.Integration{
			PublicAPIGWID: {
				{
					IntegrationId:   aws.String("integ-authorizer-1"),
					IntegrationType: apigwtypes.IntegrationTypeAwsProxy,
					IntegrationUri:  aws.String("arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:api-gateway-authorizer/invocations"),
				},
				// VPC_LINK integration to the private backend NLB — required
				// for the apigw:elb related-panel pivot (checkApigwELB).
				{
					IntegrationId:   aws.String("integ-vpclink-backend"),
					IntegrationType: apigwtypes.IntegrationTypeHttpProxy,
					ConnectionType:  apigwtypes.ConnectionTypeVpcLink,
					ConnectionId:    aws.String(APIGWVpcLinkID),
					IntegrationUri:  aws.String(APIGWBackendListenerARN),
					// CredentialsArn — required for the apigw:role related-panel
					// pivot (checkApigwRole). Matches the acme-ci-deploy-role
					// fixture (iam.go).
					CredentialsArn: aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
				},
			},
		},
		// Authorizers — required for the apigw:role related-panel pivot
		// (checkApigwRole)'s AuthorizerCredentialsArn path.
		Authorizers: map[string][]apigwtypes.Authorizer{
			PublicAPIGWID: {
				{
					AuthorizerId:             aws.String("auth-public-api-1"),
					Name:                     aws.String("acme-public-authorizer"),
					AuthorizerCredentialsArn: aws.String("arn:aws:iam::123456789012:role/acme-ci-deploy-role"),
				},
			},
			// The healthy API must carry an authorizer too, or
			// apigw.no-authorizer colours the only row in the
			// Healthy bucket.
			HealthyAPIGWID: {
				{
					AuthorizerId: aws.String("auth-healthy-api-1"),
					Name:         aws.String("acme-healthy-authorizer"),
				},
			},
			// APIGWHTTPNoAuthorizer is the one demo API without an authorizer,
			// so the internal API carries one.
			"klm901nop2": {
				{
					AuthorizerId: aws.String("auth-internal-api-1"),
					Name:         aws.String("acme-internal-authorizer"),
				},
			},
		},
		// DomainNames + ApiMappings — required for the apigw:acm pivot
		// (checkApigwACM: GetDomainNames -> GetApiMappings match on
		// ApiId -> harvest CertificateArn from DomainNameConfigurations).
		// The referenced cert is a real acm.go fixture for api.acme-corp.com.
		DomainNames: []apigwtypes.DomainName{
			{
				DomainName:  aws.String(PublicAPIGWDomainName),
				RoutingMode: apigwtypes.RoutingModeApiMappingOnly,
				DomainNameConfigurations: []apigwtypes.DomainNameConfiguration{
					{
						CertificateArn: aws.String(ProdACMCertARN2),
						EndpointType:   apigwtypes.EndpointTypeRegional,
					},
				},
			},
			{
				DomainName:  aws.String(RoutedAPIGWDomainName),
				RoutingMode: apigwtypes.RoutingModeRoutingRuleOnly,
				DomainNameConfigurations: []apigwtypes.DomainNameConfiguration{
					{
						CertificateArn: aws.String(ProdACMCertARN2),
						EndpointType:   apigwtypes.EndpointTypeRegional,
					},
				},
			},
		},
		RoutingRules: map[string][]apigwtypes.RoutingRule{
			RoutedAPIGWDomainName: {
				{
					RoutingRuleId: aws.String("rr0orders01"),
					Priority:      aws.Int32(100),
					Conditions: []apigwtypes.RoutingRuleCondition{
						{MatchBasePaths: &apigwtypes.RoutingRuleMatchBasePaths{AnyOf: []string{"orders"}}},
					},
					Actions: []apigwtypes.RoutingRuleAction{
						{InvokeApi: &apigwtypes.RoutingRuleActionInvokeApi{ApiId: aws.String(APIGWRESTTracingOff), Stage: aws.String("prod"), StripBasePath: aws.Bool(true)}},
					},
				},
			},
		},
		ApiMappings: map[string][]apigwtypes.ApiMapping{
			PublicAPIGWDomainName: {
				{ApiId: aws.String(PublicAPIGWID), Stage: aws.String("$default")},
			},
		},
		// Stages — the $default stage on PublicAPIGWID has no throttling
		// configured and no access logs, so it raises apigw.stage-config-issues
		// (EnrichAPIGatewayStage: DefaultRouteSettings.Throttling{Burst,Rate}Limit
		// == 0 OR AccessLogSettings == nil).
		Stages: map[string][]apigwtypes.Stage{
			PublicAPIGWID: {
				{
					StageName:   aws.String("$default"),
					CreatedDate: aws.Time(time.Date(2025, 3, 10, 9, 5, 0, 0, time.UTC)),
					DefaultRouteSettings: &apigwtypes.RouteSettings{
						ThrottlingBurstLimit: aws.Int32(0),
						ThrottlingRateLimit:  aws.Float64(0),
					},
					AccessLogSettings: nil,
				},
			},
			// HealthyAPIGWID's "prod" stage has real throttling limits and
			// access logging configured — no apigw.stage-config-issues finding.
			HealthyAPIGWID: {
				{
					StageName:   aws.String("prod"),
					CreatedDate: aws.Time(time.Date(2025, 10, 12, 8, 5, 0, 0, time.UTC)),
					DefaultRouteSettings: &apigwtypes.RouteSettings{
						ThrottlingBurstLimit: aws.Int32(500),
						ThrottlingRateLimit:  aws.Float64(1000),
					},
					AccessLogSettings: &apigwtypes.AccessLogSettings{
						DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigateway/acme-partner-api"),
						Format:         aws.String(`{"requestId":"$context.requestId","status":"$context.status"}`),
					},
				},
			},
		},
	}
})

func NewAPIGWFixtures() *APIGWFixtures {
	return sharedAPIGWFixtures()
}

const (
	// APIGWRESTNoAuthorizer is the REST API with no authorizer and a
	// resource policy that is absent or public.
	APIGWRESTNoAuthorizer = "rst001noauth"

	// APIGWHTTPNoAuthorizer is the HTTP API with no authorizer.
	APIGWHTTPNoAuthorizer = "efg567hij8"

	// APIGWRESTNoAccessLogs is the REST API whose stage records no access logs.
	APIGWRESTNoAccessLogs = "rst002nologs"

	// APIGWRESTTracingOff is the REST API whose stage has tracing switched off.
	APIGWRESTTracingOff = "rst003notrace"

	// APIGWRESTStageSecret is the REST API whose stage variables hold a value
	// that scans as a credential. The value itself never leaves the fixture.
	APIGWRESTStageSecret = "rst004secret"

	// APIGWRESTStageSecretStage is the stage on APIGWRESTStageSecret carrying
	// that variable.
	APIGWRESTStageSecretStage = "prod"
)

// APIGWV1Fixtures holds typed fixture data for the API Gateway v1 (REST)
// lane. The a9s apigw list merges v1 and v2, and the four REST posture findings
// have no v2 equivalent, so demo mode needs REST APIs to demonstrate them.
type APIGWV1Fixtures struct {
	RestApis []apigwv1types.RestApi
	// Authorizers maps RestApiId -> authorizers, served by GetAuthorizers.
	Authorizers map[string][]apigwv1types.Authorizer
	// Stages maps RestApiId -> stages, served by GetStages.
	Stages map[string][]apigwv1types.Stage
	// Resources maps RestApiId -> the resources below the root, each with its
	// methods embedded, served by GetResources.
	Resources map[string][]apigwv1types.Resource
}

// apigwV1RestAPI builds one REST API row with the given endpoint type.
func apigwV1RestAPI(id, name, endpoint string) apigwv1types.RestApi {
	return apigwv1types.RestApi{
		Id:          aws.String(id),
		Name:        aws.String(name),
		Description: aws.String("demo REST API"),
		CreatedDate: aws.Time(time.Date(2026, 2, 1, 9, 0, 0, 0, time.UTC)),
		EndpointConfiguration: &apigwv1types.EndpointConfiguration{
			Types: []apigwv1types.EndpointType{apigwv1types.EndpointType(endpoint)},
		},
	}
}

// apigwV1HealthyStage returns a REST stage with access logging on, tracing on
// and no credential in its variables, so it raises none of the REST findings.
func apigwV1HealthyStage(name string) apigwv1types.Stage {
	return apigwv1types.Stage{
		StageName:      aws.String(name),
		DeploymentId:   aws.String("dep001"),
		TracingEnabled: true,
		AccessLogSettings: &apigwv1types.AccessLogSettings{
			DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigw/acme-rest:*"),
			Format:         aws.String("$context.requestId"),
		},
		Variables: map[string]string{"backendStage": "prod"},
	}
}

// NewAPIGWV1Fixtures constructs the REST-lane fixtures. Each API is the only
// carrier of its finding; every other API is healthy for that finding.
var sharedAPIGWV1Fixtures = sync.OnceValue(func() *APIGWV1Fixtures { //nolint:gochecknoglobals // fixture singleton, matching this package's shape
	noLogs := apigwV1HealthyStage(APIGWRESTStageSecretStage)
	noLogs.AccessLogSettings = nil

	untraced := apigwV1HealthyStage(APIGWRESTStageSecretStage)
	untraced.TracingEnabled = false

	leaky := apigwV1HealthyStage(APIGWRESTStageSecretStage)
	leaky.Variables = map[string]string{
		"backendStage": "prod",
		// A synthetic value: the finding reports the key and the kind, never
		// the value, and nothing here is a real credential.
		"DB_PASSWORD": "demo-placeholder-not-a-real-secret",
	}

	authorizer := []apigwv1types.Authorizer{{
		Id:   aws.String("auth-rest-1"),
		Name: aws.String("acme-rest-authorizer"),
		Type: apigwv1types.AuthorizerTypeRequest,
	}}

	return &APIGWV1Fixtures{
		RestApis: []apigwv1types.RestApi{
			apigwV1RestAPI(APIGWRESTNoAuthorizer, "acme-orders-rest", "EDGE"),
			apigwV1RestAPI(APIGWRESTNoAccessLogs, "acme-unlogged-rest", "REGIONAL"),
			apigwV1RestAPI(APIGWRESTTracingOff, "acme-untraced-rest", "REGIONAL"),
			// Private, so the three endpoint words a REST API can carry
			// (edge, regional, private) all render in the demo. This API has
			// an authorizer, so its endpoint type raises no finding.
			apigwV1RestAPI(APIGWRESTStageSecret, "acme-leaky-rest", "PRIVATE"),
		},
		Authorizers: map[string][]apigwv1types.Authorizer{
			APIGWRESTNoAccessLogs: authorizer,
			APIGWRESTTracingOff:   authorizer,
			APIGWRESTStageSecret:  authorizer,
		},
		Stages: map[string][]apigwv1types.Stage{
			APIGWRESTNoAuthorizer: {apigwV1HealthyStage("prod")},
			APIGWRESTNoAccessLogs: {noLogs},
			APIGWRESTTracingOff:   {untraced},
			APIGWRESTStageSecret:  {leaky},
		},
		// acme-orders-rest's POST /orders proxies to the orders Lambda: the
		// REST API's Lambda row reads it from GetResources.
		Resources: map[string][]apigwv1types.Resource{
			APIGWRESTNoAuthorizer: {{
				Id:       aws.String("ord001"),
				ParentId: aws.String(APIGWRESTNoAuthorizer + "-root"),
				Path:     aws.String("/orders"),
				PathPart: aws.String("orders"),
				ResourceMethods: map[string]apigwv1types.Method{
					"POST": {
						HttpMethod:        aws.String("POST"),
						AuthorizationType: aws.String("NONE"),
						MethodIntegration: &apigwv1types.Integration{
							Type:       apigwv1types.IntegrationTypeAwsProxy,
							HttpMethod: aws.String("POST"),
							Uri:        aws.String("arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123456789012:function:" + lambdaProcessOrders + "/invocations"),
						},
					},
				},
			}},
		},
	}
})

// NewAPIGWV1Fixtures returns the shared REST-lane fixtures.
func NewAPIGWV1Fixtures() *APIGWV1Fixtures { return sharedAPIGWV1Fixtures() }

func init() {
	Register(Pin{ShortName: "apigw", Rows: 8, Issues: 0, CoverageGaps: []string{"dim"}})
}
