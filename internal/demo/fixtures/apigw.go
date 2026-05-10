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
}

// NewAPIGWFixtures constructs APIGWFixtures from the canonical demo data.
var sharedAPIGWFixtures = sync.OnceValue(func() *APIGWFixtures {
	return &APIGWFixtures{
		APIs: []apigwtypes.Api{
			{
				ApiId:                     aws.String("abc123def4"),
				Name:                      aws.String("acme-public-api"),
				ProtocolType:              apigwtypes.ProtocolTypeHttp,
				ApiEndpoint:               aws.String("https://abc123def4.execute-api.us-east-1.amazonaws.com"),
				Description:               aws.String("Public REST API for Acme Corp mobile and web clients"),
				RouteSelectionExpression:  aws.String("${request.method} ${request.path}"),
				CreatedDate:               aws.Time(time.Date(2025, 3, 10, 9, 0, 0, 0, time.UTC)),
				ApiKeySelectionExpression: aws.String("$request.header.x-api-key"),
				CorsConfiguration:         &apigwtypes.Cors{AllowMethods: []string{"GET", "POST", "PUT", "DELETE"}, AllowOrigins: []string{"https://app.acme-corp.com"}},
				Tags:                      map[string]string{"Environment": "production"},
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
	}
})

func NewAPIGWFixtures() *APIGWFixtures {
	return sharedAPIGWFixtures()
}
