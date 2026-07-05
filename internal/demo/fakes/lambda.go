// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// LambdaFake implements aws.LambdaAPI against fixture data loaded at construction time.
type LambdaFake struct {
	fix *fixtures.LambdaFixtures
}

// NewLambda constructs a LambdaFake backed by fixture data from the fixtures package.
func NewLambda() *LambdaFake {
	return &LambdaFake{fix: fixtures.NewLambdaFixtures()}
}

func (f *LambdaFake) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return &lambda.ListFunctionsOutput{Functions: f.fix.Functions}, nil
}

func (f *LambdaFake) ListEventSourceMappings(_ context.Context, input *lambda.ListEventSourceMappingsInput, _ ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	if input.EventSourceArn == nil {
		return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: f.fix.EventSourceMappings}, nil
	}
	if err := validateARN(*input.EventSourceArn); err != nil {
		return nil, err
	}
	arn := aws.ToString(input.EventSourceArn)
	var filtered []lambdatypes.EventSourceMappingConfiguration
	for _, m := range f.fix.EventSourceMappings {
		if aws.ToString(m.EventSourceArn) == arn {
			filtered = append(filtered, m)
		}
	}
	return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: filtered}, nil
}

func (f *LambdaFake) GetFunction(_ context.Context, input *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	name := aws.ToString(input.FunctionName)
	for _, fn := range f.fix.Functions {
		if aws.ToString(fn.FunctionName) == name {
			code := &lambdatypes.FunctionCodeLocation{
				Location:       aws.String("https://s3.amazonaws.com/example-bucket/" + name + ".zip"),
				RepositoryType: aws.String("S3"),
			}
			if imageURI, ok := f.fix.ImageURIs[name]; ok {
				code.ImageUri = aws.String(imageURI)
				code.RepositoryType = aws.String("ECR")
				code.Location = nil
			}
			return &lambda.GetFunctionOutput{
				Configuration: &fn,
				Code:          code,
			}, nil
		}
	}
	return nil, &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "Function not found: " + name,
	}
}

func (f *LambdaFake) ListTags(_ context.Context, input *lambda.ListTagsInput, _ ...func(*lambda.Options)) (*lambda.ListTagsOutput, error) {
	if input == nil || input.Resource == nil {
		// No specific ARN requested — surface any fixture-registered tag set
		// so smoke tests on the fake itself can assert real tag data exists.
		for _, tags := range f.fix.Tags {
			return &lambda.ListTagsOutput{Tags: tags}, nil
		}
		return &lambda.ListTagsOutput{Tags: map[string]string{}}, nil
	}
	arn := aws.ToString(input.Resource)
	for _, fn := range f.fix.Functions {
		if aws.ToString(fn.FunctionArn) != arn {
			continue
		}
		if tags, ok := f.fix.Tags[aws.ToString(fn.FunctionName)]; ok {
			return &lambda.ListTagsOutput{Tags: tags}, nil
		}
		break
	}
	return &lambda.ListTagsOutput{Tags: map[string]string{}}, nil
}
