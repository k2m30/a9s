// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwv1types "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
)

// APIGWV1Fake implements aws.APIGatewayV1API. Demo mode has no V1 REST APIs
// — returning an empty list lets the apigw fetcher proceed to the V2 branch
// instead of surfacing a 501 "demo transport: no handler" error that would
// otherwise hide the entire apigw row from the main menu.
type APIGWV1Fake struct{}

// NewAPIGWV1 constructs an APIGWV1Fake.
func NewAPIGWV1() *APIGWV1Fake { return &APIGWV1Fake{} }

func (f *APIGWV1Fake) GetRestApis(_ context.Context, _ *apigateway.GetRestApisInput, _ ...func(*apigateway.Options)) (*apigateway.GetRestApisOutput, error) {
	return &apigateway.GetRestApisOutput{Items: []apigwv1types.RestApi{}}, nil
}

func (f *APIGWV1Fake) GetAuthorizers(_ context.Context, _ *apigateway.GetAuthorizersInput, _ ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error) {
	return &apigateway.GetAuthorizersOutput{Items: []apigwv1types.Authorizer{}}, nil
}

func (f *APIGWV1Fake) GetStages(_ context.Context, _ *apigateway.GetStagesInput, _ ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error) {
	return &apigateway.GetStagesOutput{Item: []apigwv1types.Stage{}}, nil
}
