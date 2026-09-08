// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchLambdaFunctionsPage calls the Lambda ListFunctions API and returns
// a single page of functions. Pass an empty continuationToken for the first page.
func FetchLambdaFunctionsPage(ctx context.Context, api LambdaListFunctionsAPI, continuationToken string) (resource.FetchResult, error) {
	return FetchLambdaFunctionsPageWithEventSources(ctx, api, nil, continuationToken)
}

// FetchLambdaFunctionsPageWithEventSources calls the Lambda ListFunctions API
// and enriches each function with a first event source ARN when available.
func FetchLambdaFunctionsPageWithEventSources(
	ctx context.Context,
	api LambdaListFunctionsAPI,
	eventSourceAPI LambdaListEventSourceMappingsAPI,
	continuationToken string,
) (resource.FetchResult, error) {
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
		if eventSourceAPI != nil {
			eventSourceARN, _ = firstLambdaEventSourceARN(ctx, eventSourceAPI, functionName)
		}

		dlqTargetARN := ""
		if fn.DeadLetterConfig != nil {
			dlqTargetARN = aws.ToString(fn.DeadLetterConfig.TargetArn)
		}

		r := resource.Resource{
			ID:   functionName,
			Name: functionName,
			// Status intentionally unset — lifecycle state is emitted as a Finding.
			Fields: map[string]string{
				"function_name":      functionName,
				"runtime":            runtime,
				"state":              string(fn.State),
				"last_update_status": string(fn.LastUpdateStatus),
				"memory":             memory,
				"timeout":            timeout,
				"handler":            handler,
				"last_modified":      lastModified,
				"code_size":          codeSize,
				"log_group":          logGroup,
				"package_type":       packageType,
				"event_source_arn":   eventSourceARN,
				"dlq_target_arn":     dlqTargetARN,
				"arn":                aws.ToString(fn.FunctionArn),
			},
			RawStruct: fn,
		}

		// emit canonical Findings for every non-healthy branch colorLambda
		// reads, mirroring its own precedence: last-update failure, then
		// deprecated runtime, then lifecycle state, then no-DLQ fallback.
		switch {
		case fn.LastUpdateStatus == lambdatypes.LastUpdateStatusFailed:
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaLastUpdateFailed)}
		case isDeprecatedLambdaRuntime(runtime):
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaDeprecatedRuntime)}
		case fn.State == lambdatypes.StatePending:
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaStatePending)}
		case fn.State == lambdatypes.StateFailed:
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaStateFailed)}
		case fn.State == lambdatypes.StateInactive:
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaInactive)}
		case dlqTargetARN == "":
			r.Findings = []domain.Finding{wave1Finding(CodeLambdaNoDLQ)}
		}

		// Independent of the lifecycle switch above: ListFunctions already
		// carries the environment, so a pasted credential is readable in
		// Wave 1 and colours the row on its own.
		if fn.Environment != nil {
			addSecretScanFinding(&r, CodeLambdaEnvSecret, fn.Environment.Variables)
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
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

func firstLambdaEventSourceARN(ctx context.Context, api LambdaListEventSourceMappingsAPI, functionName string) (string, error) {
	if functionName == "" {
		return "", nil
	}

	input := &lambda.ListEventSourceMappingsInput{
		FunctionName: &functionName,
	}
	for {
		out, err := api.ListEventSourceMappings(ctx, input)
		if err != nil {
			return "", err
		}
		for _, m := range out.EventSourceMappings {
			if m.EventSourceArn != nil && *m.EventSourceArn != "" {
				return *m.EventSourceArn, nil
			}
		}
		if out.NextMarker == nil || *out.NextMarker == "" {
			return "", nil
		}
		input.Marker = out.NextMarker
	}
}
