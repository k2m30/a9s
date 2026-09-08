// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// LambdaListFunctionsAPI defines the interface for the Lambda ListFunctions operation.
type LambdaListFunctionsAPI interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
}

// LambdaListEventSourceMappingsAPI defines the interface for the Lambda
// ListEventSourceMappings operation.
type LambdaListEventSourceMappingsAPI interface {
	ListEventSourceMappings(ctx context.Context, params *lambda.ListEventSourceMappingsInput, optFns ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error)
}

// LambdaListTagsAPI defines the interface for the Lambda ListTags operation.
type LambdaListTagsAPI interface {
	ListTags(ctx context.Context, params *lambda.ListTagsInput, optFns ...func(*lambda.Options)) (*lambda.ListTagsOutput, error)
}

// LambdaGetFunctionAPI defines the interface for the Lambda GetFunction operation.
type LambdaGetFunctionAPI interface {
	GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
}

// LambdaGetPolicyAPI defines the interface for the Lambda GetPolicy
// operation — the function's resource policy. Deliberately NOT part of the
// LambdaAPI aggregate: only the Wave-2 posture enricher needs it, and it
// type-asserts (mirroring EC2DescribeInstanceAttributeAPI's precedent) so a
// narrow client that cannot serve it degrades to no findings.
type LambdaGetPolicyAPI interface {
	GetPolicy(ctx context.Context, params *lambda.GetPolicyInput, optFns ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error)
}

// LambdaListFunctionUrlConfigsAPI defines the interface for the Lambda
// ListFunctionUrlConfigs operation. Same aggregate-exclusion rationale as
// LambdaGetPolicyAPI.
type LambdaListFunctionUrlConfigsAPI interface {
	ListFunctionUrlConfigs(ctx context.Context, params *lambda.ListFunctionUrlConfigsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error)
}

// LambdaPostureAPI is the set of read-only calls EnrichLambdaPosture makes.
// GetFunction is in it for one reason: Lambda answers
// ResourceNotFoundException from the other two both for "this function has no
// policy / no URL config" (healthy) and for "this function no longer exists"
// (a race), and only asking whether the function is there separates them.
type LambdaPostureAPI interface {
	LambdaGetPolicyAPI
	LambdaListFunctionUrlConfigsAPI
	LambdaGetFunctionAPI
}

// LambdaAPI is the aggregate interface covering all Lambda operations used by a9s fetchers.
// *lambda.Client structurally satisfies this interface.
type LambdaAPI interface {
	LambdaListFunctionsAPI
	LambdaListEventSourceMappingsAPI
	LambdaGetFunctionAPI
	LambdaListTagsAPI
}
