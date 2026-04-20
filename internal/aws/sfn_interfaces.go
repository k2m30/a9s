package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

// SFNListStateMachinesAPI defines the interface for the SFN ListStateMachines operation.
type SFNListStateMachinesAPI interface {
	ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
}

// SFNDescribeStateMachineAPI defines the interface for the SFN DescribeStateMachine
// operation. Used by sfn→role, sfn→kms (EncryptionConfiguration), sfn→lambda
// (parses ASL definition for Resource ARNs pointing at Lambda functions).
type SFNDescribeStateMachineAPI interface {
	DescribeStateMachine(ctx context.Context, params *sfn.DescribeStateMachineInput, optFns ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error)
}

// SFNListExecutionsAPI defines the interface for the SFN ListExecutions operation.
type SFNListExecutionsAPI interface {
	ListExecutions(ctx context.Context, params *sfn.ListExecutionsInput, optFns ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error)
}

// SFNGetExecutionHistoryAPI defines the interface for the SFN GetExecutionHistory operation.
type SFNGetExecutionHistoryAPI interface {
	GetExecutionHistory(ctx context.Context, params *sfn.GetExecutionHistoryInput, optFns ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error)
}

// SFNListTagsForResourceAPI for sfn→cfn (Tags -> aws:cloudformation:stack-name).
type SFNListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *sfn.ListTagsForResourceInput, optFns ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error)
}

// SFNAPI is the aggregate interface covering all SFN operations used by a9s fetchers.
// *sfn.Client structurally satisfies this interface.
type SFNAPI interface {
	SFNListStateMachinesAPI
	SFNListExecutionsAPI
	SFNGetExecutionHistoryAPI
	SFNDescribeStateMachineAPI
	SFNListTagsForResourceAPI
}
