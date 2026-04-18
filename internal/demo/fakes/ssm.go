package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SSMFake implements aws.SSMAPI against fixture data loaded at construction time.
type SSMFake struct {
	fix *fixtures.SSMFixtures
}

// NewSSM constructs an SSMFake backed by fixture data from the fixtures package.
func NewSSM() *SSMFake {
	return &SSMFake{fix: fixtures.NewSSMFixtures()}
}

func (f *SSMFake) DescribeParameters(_ context.Context, _ *ssm.DescribeParametersInput, _ ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error) {
	return &ssm.DescribeParametersOutput{Parameters: f.fix.Parameters}, nil
}

func (f *SSMFake) GetParameter(_ context.Context, input *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if input.Name == nil {
		return nil, fmt.Errorf("GetParameter: Name is required")
	}
	val, ok := f.fix.ParameterValues[*input.Name]
	if !ok {
		val = fmt.Sprintf("[demo value for %s]", *input.Name)
	}
	return &ssm.GetParameterOutput{
		Parameter: &ssmtypes.Parameter{
			Name:  input.Name,
			Value: aws.String(val),
		},
	}, nil
}

// DescribeInstanceInformation is a no-op stub satisfying SSMDescribeInstanceInformationAPI.
// Demo mode does not model SSM managed instances.
func (f *SSMFake) DescribeInstanceInformation(_ context.Context, _ *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	return &ssm.DescribeInstanceInformationOutput{}, nil
}
