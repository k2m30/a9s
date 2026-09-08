// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"

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
	// CloudFormation is a query-protocol API with no modeled not-found shape:
	// a stack that does not exist comes back as ValidationError.
	return nil, &smithy.GenericAPIError{
		Code:    "ValidationError",
		Message: "Stack with id " + *input.StackName + " does not exist",
		Fault:   smithy.FaultClient,
	}
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

// GetTemplate returns the fixture template body for the stack named or
// identified by input.StackName (the enricher passes StackId, falling back
// to StackName — this resolves either against the fixture stack list, as
// DescribeStacks's own StackName parameter accepts either in real AWS,
// before looking up TemplateBodies by canonical StackName). An unknown or
// empty identifier returns the same CFN-validation-shaped error real
// CloudFormation raises for a nonexistent stack, matching how sibling demo
// fakes (e.g. EKSFake.DescribeCluster) error on unknown ids instead of
// silently falling back to generic data. GenericTemplateBody is only used
// for a KNOWN stack with no dedicated TemplateBodies entry.
func (f *CFNFake) GetTemplate(_ context.Context, input *cloudformation.GetTemplateInput, _ ...func(*cloudformation.Options)) (*cloudformation.GetTemplateOutput, error) {
	var want string
	if input != nil && input.StackName != nil {
		want = *input.StackName
	}
	var stackName string
	found := false
	for _, stack := range f.fix.Stacks {
		if stack.StackName != nil && *stack.StackName == want {
			stackName = *stack.StackName
			found = true
			break
		}
		if stack.StackId != nil && *stack.StackId == want && stack.StackName != nil {
			stackName = *stack.StackName
			found = true
			break
		}
	}
	if !found {
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "Stack with id " + want + " does not exist",
		}
	}
	body, ok := f.fix.TemplateBodies[stackName]
	if !ok {
		body = fixtures.GenericTemplateBody
	}
	return &cloudformation.GetTemplateOutput{TemplateBody: &body}, nil
}
