package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sfn"

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
		smARN = *input.StateMachineArn
	}
	return &sfn.ListExecutionsOutput{Executions: f.fix.Executions[smARN]}, nil
}

func (f *SFNFake) GetExecutionHistory(_ context.Context, _ *sfn.GetExecutionHistoryInput, _ ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error) {
	return &sfn.GetExecutionHistoryOutput{}, nil
}
