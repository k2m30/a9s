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

// LambdaAPI is the aggregate interface covering all Lambda operations used by a9s fetchers.
// *lambda.Client structurally satisfies this interface.
type LambdaAPI interface {
	LambdaListFunctionsAPI
	LambdaListEventSourceMappingsAPI
	LambdaGetFunctionAPI
	LambdaListTagsAPI
}
