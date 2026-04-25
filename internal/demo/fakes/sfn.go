package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// SFNFake implements aws.SFNAPI against fixture data loaded at construction time.
type SFNFake struct {
	fix *fixtures.SFNFixtures
}

// NewSFN constructs an SFNFake backed by fixture data from the fixtures package.
func NewSFN() *SFNFake {
	return &SFNFake{fix: fixtures.NewSFNFixtures()}
}

func (f *SFNFake) ListStateMachines(_ context.Context, _ *sfn.ListStateMachinesInput, _ ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error) {
	return &sfn.ListStateMachinesOutput{StateMachines: f.fix.StateMachines}, nil
}

func (f *SFNFake) ListExecutions(_ context.Context, input *sfn.ListExecutionsInput, _ ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error) {
	var smARN string
	if input != nil && input.StateMachineArn != nil {
		if err := validateSFNArn(*input.StateMachineArn); err != nil {
			return nil, err
		}
		smARN = *input.StateMachineArn
	}
	return &sfn.ListExecutionsOutput{Executions: f.fix.Executions[smARN]}, nil
}

func (f *SFNFake) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return &sfn.GetExecutionHistoryOutput{}, nil
}

// DescribeStateMachine returns an empty state machine — demo mode does not
// model ASL definitions.
func (f *SFNFake) DescribeStateMachine(_ context.Context, input *sfn.DescribeStateMachineInput, _ ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	var arn string
	if input != nil && input.StateMachineArn != nil {
		if err := validateSFNArn(*input.StateMachineArn); err != nil {
			return nil, err
		}
		arn = *input.StateMachineArn
	}
	return &sfn.DescribeStateMachineOutput{StateMachineArn: &arn}, nil
}

// validateSFNArn mirrors the real SFN API which returns InvalidArn (not ValidationError)
// for non-ARN values passed to ARN-typed parameters.
func validateSFNArn(val string) error {
	if err := validateARN(val); err != nil {
		return &smithy.GenericAPIError{
			Code:    "InvalidArn",
			Message: "Invalid Arn: '" + val + "' is not a valid ARN",
		}
	}
	return nil
}

// ListTagsForResource returns an empty tag list — demo mode does not model SFN tags.
func (f *SFNFake) ListTagsForResource(_ context.Context, _ *sfn.ListTagsForResourceInput, _ ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error) {
	return &sfn.ListTagsForResourceOutput{}, nil
}
