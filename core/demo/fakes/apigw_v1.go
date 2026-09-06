// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// APIGWV1Fake implements aws.APIGatewayV1API plus the two Wave-2 REST calls
// the issue enricher reaches by type assertion. The four REST posture rows
// have no v2 equivalent, so demo mode serves REST APIs to demonstrate them.
type APIGWV1Fake struct {
	fix *fixtures.APIGWV1Fixtures
}

// NewAPIGWV1 constructs an APIGWV1Fake backed by the REST-lane fixtures.
func NewAPIGWV1() *APIGWV1Fake { return &APIGWV1Fake{fix: fixtures.NewAPIGWV1Fixtures()} }

func (f *APIGWV1Fake) GetRestApis(_ context.Context, _ *apigateway.GetRestApisInput, _ ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error) {
	return &apigateway.GetRestApisOutput{Items: f.fix.RestApis}, nil
}

func (f *APIGWV1Fake) GetAuthorizers(_ context.Context, input *apigateway.GetAuthorizersInput, _ ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error) {
	var id string
	if input != nil && input.RestApiId != nil {
		id = *input.RestApiId
	}
	return &apigateway.GetAuthorizersOutput{Items: f.fix.Authorizers[id]}, nil
}

func (f *APIGWV1Fake) GetStages(_ context.Context, input *apigateway.GetStagesInput, _ ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error) {
	var id string
	if input != nil && input.RestApiId != nil {
		id = *input.RestApiId
	}
	stages := f.fix.Stages[id]
	if stages == nil {
		stages = []apigwv1types.Stage{}
	}
	return &apigateway.GetStagesOutput{Item: stages}, nil
}
