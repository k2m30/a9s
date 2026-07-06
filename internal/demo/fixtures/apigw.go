package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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
}

const (
	// PublicAPIGWID is the graph-root API Gateway used to witness the
	// apigw:kms, apigw:lambda, apigw:acm and apigw:cf related-panel pivots.
	PublicAPIGWID = "abc123def4"
	// PublicAPIGWDomainName is the custom domain mapped to PublicAPIGWID —
	// required for the apigw:acm pivot witness (checkApigwACM).
	PublicAPIGWDomainName = "api.acme-corp.com"
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
		},
		// Integrations for PublicAPIGWID — required for the apigw:kms,
		// apigw:lambda pivot witnesses (checkApigwKMS / checkApigwLambda both
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
			},
		},
		// DomainNames + ApiMappings — required for the apigw:acm pivot
		// witness (checkApigwACM: GetDomainNames -> GetApiMappings match on
		// ApiId -> harvest CertificateArn from DomainNameConfigurations).
		// The referenced cert is a real acm.go fixture for api.acme-corp.com.
		DomainNames: []apigwtypes.DomainName{
			{
				DomainName: aws.String(PublicAPIGWDomainName),
				DomainNameConfigurations: []apigwtypes.DomainNameConfiguration{
					{
						CertificateArn: aws.String(ProdACMCertARN2),
						EndpointType:   apigwtypes.EndpointTypeRegional,
					},
				},
			},
		},
		ApiMappings: map[string][]apigwtypes.ApiMapping{
			PublicAPIGWDomainName: {
				{ApiId: aws.String(PublicAPIGWID), Stage: aws.String("$default")},
			},
		},
	}
})

func NewAPIGWFixtures() *APIGWFixtures {
	return sharedAPIGWFixtures()
}
