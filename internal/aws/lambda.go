package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("lambda", []string{
		"function_name",
		"runtime",
		"memory",
		"timeout",
		"handler",
		"last_modified",
		"code_size",
		"log_group",
		"package_type",
		"event_source_arn",
		"arn",
	})

	resource.RegisterPaginated("lambda", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaFunctionsPage(ctx, c.Lambda, continuationToken)
	})

	// Role is a full ARN on Lambda (arn:aws:iam::.../role/name); the navigable
	// field uses the ARN path directly. The target role fixture must carry the ARN
	// as its ID for the infrastructure integrity check to pass.
	resource.RegisterNavigableFields("lambda", []resource.NavigableField{
		{FieldPath: "Role", TargetType: "role"},
	})

	resource.RegisterRelated("lambda", []resource.RelatedDef{
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkLambdaRole, NeedsTargetCache: true},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkLambdaAlarms, NeedsTargetCache: true},
		{TargetType: "sqs", DisplayName: "SQS Queues", Checker: checkLambdaSQS, NeedsTargetCache: false},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkLambdaCFN, NeedsTargetCache: false},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkLambdaLogs, NeedsTargetCache: true},
		{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkLambdaEbRule, NeedsTargetCache: true},
	})
}

// FetchLambdaFunctions calls the Lambda ListFunctions API and returns all pages
// of functions. Used by existing tests and the legacy fetcher.
func FetchLambdaFunctions(ctx context.Context, api LambdaListFunctionsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchLambdaFunctionsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

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
		// Use State as Status when not Active — surfaces Failed/Pending/Inactive
		// for issue detection. Fall back to runtime for healthy functions.
		status := runtime
		if fn.State != "" && fn.State != lambdatypes.StateActive {
			status = string(fn.State)
		}

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

		r := resource.Resource{
			ID:     functionName,
			Name:   functionName,
			Status: status,
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
				"arn":              aws.ToString(fn.FunctionArn),
			},
			RawStruct: fn,
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
