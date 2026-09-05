// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// GetFunction resolves by bare function name or full ARN, matching the real
// Lambda API's dual-accepting FunctionName parameter — callers that only
// have a rotationLambdaARN (e.g. checkSecretsRole/checkSecretsSNS) must
// still resolve.
func (f *LambdaFake) GetFunction(_ context.Context, input *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	name := aws.ToString(input.FunctionName)
	for _, fn := range f.fix.Functions {
		if aws.ToString(fn.FunctionName) != name && aws.ToString(fn.FunctionArn) != name {
			continue
		}
		bareName := aws.ToString(fn.FunctionName)
		code := &lambdatypes.FunctionCodeLocation{
			Location:       aws.String("https://s3.amazonaws.com/example-bucket/" + bareName + ".zip"),
			RepositoryType: aws.String("S3"),
		}
		if imageURI, ok := f.fix.ImageURIs[bareName]; ok {
			code.ImageUri = aws.String(imageURI)
			code.RepositoryType = aws.String("ECR")
			code.Location = nil
		}
		var concurrency *lambdatypes.Concurrency
		if reserved, ok := f.fix.ReservedConcurrency[bareName]; ok {
			concurrency = &lambdatypes.Concurrency{ReservedConcurrentExecutions: aws.Int32(reserved)}
		}
		return &lambda.GetFunctionOutput{
			Configuration: &fn,
			Code:          code,
			Concurrency:   concurrency,
		}, nil
	}
	return nil, &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "Function not found: " + name,
	}
}

// GetPolicy returns the fixture-registered resource policy for the function.
// A function with no registered policy answers ResourceNotFoundException,
// exactly as real Lambda does — that is the healthy shape, not an outage.
func (f *LambdaFake) GetPolicy(_ context.Context, input *lambda.GetPolicyInput, _ ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	name := aws.ToString(input.FunctionName)
	if policy, ok := f.fix.Policies[name]; ok {
		return &lambda.GetPolicyOutput{Policy: aws.String(policy)}, nil
	}
	return nil, &smithy.GenericAPIError{
		Code:    "ResourceNotFoundException",
		Message: "The resource you requested does not exist.",
	}
}

// ListFunctionUrlConfigs returns the fixture-registered function URLs.
// Unlike GetPolicy, real Lambda answers with an empty list rather than an
// error when a function has none.
func (f *LambdaFake) ListFunctionUrlConfigs(_ context.Context, input *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error) {
	name := aws.ToString(input.FunctionName)
	return &lambda.ListFunctionUrlConfigsOutput{FunctionUrlConfigs: f.fix.FunctionURLConfigs[name]}, nil
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
