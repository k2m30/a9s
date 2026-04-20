package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMDescribeParametersAPI defines the interface for the SSM DescribeParameters operation.
type SSMDescribeParametersAPI interface {
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

// SSMGetParameterAPI defines the interface for the SSM GetParameter operation.
type SSMGetParameterAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// SSMDescribeInstanceInformationAPI defines the interface for the SSM DescribeInstanceInformation operation.
type SSMDescribeInstanceInformationAPI interface {
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

// SSMAPI is the aggregate interface covering all SSM operations used by a9s fetchers.
// *ssm.Client structurally satisfies this interface.
type SSMAPI interface {
	SSMDescribeParametersAPI
	SSMGetParameterAPI
	SSMDescribeInstanceInformationAPI
}
