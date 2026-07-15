package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// CFNFake implements aws.CFNAPI against fixture data loaded at construction time.
type CFNFake struct {
	fix *fixtures.CFNFixtures
}

// NewCFN constructs a CFNFake backed by fixture data from the fixtures package.
func NewCFN() *CFNFake {
	return &CFNFake{fix: fixtures.NewCFNFixtures()}
}

// DescribeStacks returns every fixture stack when input.StackName is empty,
// or the single stack matching input.StackName otherwise — matching the
// all-when-empty convention used by sibling demo fakes.
func (f *CFNFake) DescribeStacks(_ context.Context, input *cloudformation.DescribeStacksInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error) {
	if input == nil || input.StackName == nil || *input.StackName == "" {
		return &cloudformation.DescribeStacksOutput{Stacks: f.fix.Stacks}, nil
	}
	for _, stack := range f.fix.Stacks {
		if stack.StackName != nil && *stack.StackName == *input.StackName {
			return &cloudformation.DescribeStacksOutput{Stacks: []cfntypes.Stack{stack}}, nil
		}
	}
	return &cloudformation.DescribeStacksOutput{}, nil
}

func (f *CFNFake) DescribeStackEvents(_ context.Context, input *cloudformation.DescribeStackEventsInput, _ ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error) {
	var stackName string
	if input != nil && input.StackName != nil {
		stackName = *input.StackName
	}
	return &cloudformation.DescribeStackEventsOutput{StackEvents: f.fix.StackEvents[stackName]}, nil
}

func (f *CFNFake) ListStackResources(_ context.Context, input *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error) {
	var stackName string
	if input != nil && input.StackName != nil {
		stackName = *input.StackName
	}
	return &cloudformation.ListStackResourcesOutput{StackResourceSummaries: f.fix.StackResources[stackName]}, nil
}
