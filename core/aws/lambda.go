// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchLambdaFunctionsPage calls the Lambda ListFunctions API and returns
// a single page of functions. Pass an empty continuationToken for the first
// page. event_source_arn is always emitted empty (a per-function
// ListEventSourceMappings call here would turn a single-page list into an
// N+1 fetch — the checker-owned mechanism in related_common.go's
// lambdaEventSourceMappingLambdaCheck is the sole ListEventSourceMappings
// caller, one filtered call per open kinesis/msk resource, never per lambda).
func FetchLambdaFunctionsPage(ctx context.Context, api LambdaListFunctionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &lambda.ListFunctionsInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListFunctions(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Lambda functions: %w", err)
	}

	var resources []resource.Resource
	for _, fn := range output.Functions {
		functionName := ""
		if fn.FunctionName != nil {
			functionName = *fn.FunctionName
		}

		runtime := string(fn.Runtime)

		memory := ""
		if fn.MemorySize != nil {
			memory = fmt.Sprintf("%d", *fn.MemorySize)
		}

		timeout := ""
		if fn.Timeout != nil {
			timeout = fmt.Sprintf("%d", *fn.Timeout)
		}

		handler := ""
		if fn.Handler != nil {
			handler = *fn.Handler
		}

		lastModified := ""
		if fn.LastModified != nil {
			lastModified = *fn.LastModified
		}

		codeSize := ""
		if fn.CodeSize != 0 {
			codeSize = formatBytes(fn.CodeSize)
		}

		logGroup := "/aws/lambda/" + functionName
		if fn.LoggingConfig != nil && fn.LoggingConfig.LogGroup != nil && *fn.LoggingConfig.LogGroup != "" {
			logGroup = *fn.LoggingConfig.LogGroup
		}

		packageType := string(fn.PackageType)
		eventSourceARN := ""

		dlqTargetARN := ""
		if fn.DeadLetterConfig != nil {
			dlqTargetARN = aws.ToString(fn.DeadLetterConfig.TargetArn)
		}

		// ListFunctions returns no State, StateReasonCode or LastUpdateStatus,
		// so "state" and "last_update_status" are written by the wave-2
		// GetFunction read (EnrichLambdaPosture), never here.
		r := resource.Resource{
			ID:   functionName,
			Name: functionName,
			Fields: map[string]string{
				"function_name":    functionName,
				"runtime":          runtime,
				"memory":           memory,
				"timeout":          timeout,
				"handler":          handler,
				"last_modified":    lastModified,
				"code_size":        codeSize,
				"log_group":        logGroup,
				"package_type":     packageType,
				"event_source_arn": eventSourceARN,
				"dlq_target_arn":   dlqTargetARN,
				"arn":              aws.ToString(fn.FunctionArn),
			},
			RawStruct: fn,
		}

		if isDeprecatedLambdaRuntime(runtime, time.Now()) {
			r.Findings = append(r.Findings, wave1Finding(CodeLambdaDeprecatedRuntime))
		}
		if dlqTargetARN == "" {
			r.Findings = append(r.Findings, wave1Finding(CodeLambdaNoDLQ))
		}

		// ListFunctions already carries the environment, so a pasted
		// credential is readable in Wave 1 and colours the row on its own.
		if fn.Environment != nil {
			addSecretScanFinding(&r, CodeLambdaEnvSecret, fn.Environment.Variables)
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil {
		nextToken = *output.NextMarker
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
