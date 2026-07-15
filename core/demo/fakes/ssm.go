// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// DescribeInstanceInformation serves fixture-enrolled SSM managed-instance
// IDs, filtered by the InstanceIds filter when present — required for the
// ec2:ssm related-panel pivot witness (checkEC2SSM).
func (f *SSMFake) DescribeInstanceInformation(_ context.Context, input *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	wanted := make(map[string]struct{}, len(f.fix.ManagedInstanceIDs))
	for _, id := range f.fix.ManagedInstanceIDs {
		wanted[id] = struct{}{}
	}
	if input != nil {
		for _, filter := range input.Filters {
			if filter.Key == nil || *filter.Key != "InstanceIds" {
				continue
			}
			filtered := make(map[string]struct{}, len(filter.Values))
			for _, v := range filter.Values {
				if _, ok := wanted[v]; ok {
					filtered[v] = struct{}{}
				}
			}
			wanted = filtered
		}
	}
	var infos []ssmtypes.InstanceInformation
	for id := range wanted {
		infos = append(infos, ssmtypes.InstanceInformation{InstanceId: aws.String(id)})
	}
	return &ssm.DescribeInstanceInformationOutput{InstanceInformationList: infos}, nil
}
