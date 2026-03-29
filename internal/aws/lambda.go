package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("lambda", []string{"function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size", "log_group", "package_type"})

	resource.RegisterPaginated("lambda", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchLambdaFunctionsPage(ctx, c.Lambda, continuationToken)
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
	input := &lambda.ListFunctionsInput{}
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

		r := resource.Resource{
			ID:     functionName,
			Name:   functionName,
			Status: runtime,
			Fields: map[string]string{
				"function_name": functionName,
				"runtime":       runtime,
				"memory":        memory,
				"timeout":       timeout,
				"handler":       handler,
				"last_modified": lastModified,
				"code_size":     codeSize,
				"log_group":     logGroup,
				"package_type":  packageType,
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
