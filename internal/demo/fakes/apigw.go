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
