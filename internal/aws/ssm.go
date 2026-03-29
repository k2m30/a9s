package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ssm", []string{"name", "type", "version", "last_modified", "description"})
	resource.RegisterRevealFetcher("ssm", func(ctx context.Context, clients interface{}, resourceID string) (string, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return "", fmt.Errorf("AWS clients not initialized")
		}
		return RevealSSMParameter(ctx, c.SSM, resourceID)
	})

	resource.RegisterPaginated("ssm", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSSMParametersPage(ctx, c.SSM, continuationToken)
	})
}

// FetchSSMParameters calls the SSM DescribeParameters API and returns all pages
// of parameters. Used by existing tests and the legacy fetcher.
func FetchSSMParameters(ctx context.Context, api SSMDescribeParametersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSSMParametersPage(ctx, api, token)
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

// FetchSSMParametersPage calls the SSM DescribeParameters API and returns a single
// page of parameters. Pass an empty continuationToken for the first page.
func FetchSSMParametersPage(ctx context.Context, api SSMDescribeParametersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ssm.DescribeParametersInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeParameters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching SSM parameters: %w", err)
	}

	var resources []resource.Resource
	for _, param := range output.Parameters {
		paramName := ""
		if param.Name != nil {
			paramName = *param.Name
		}

		paramType := string(param.Type)

		version := ""
		if param.Version != 0 {
			version = fmt.Sprintf("%d", param.Version)
		}

		lastModified := ""
		if param.LastModifiedDate != nil {
			lastModified = param.LastModifiedDate.Format("2006-01-02 15:04")
		}

		description := ""
		if param.Description != nil {
			description = *param.Description
		}

		r := resource.Resource{
			ID:     paramName,
			Name:   paramName,
			Status: "",
			Fields: map[string]string{
				"name":          paramName,
				"type":          paramType,
				"version":       version,
				"last_modified": lastModified,
				"description":   description,
			},
			RawStruct: param,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
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

// RevealSSMParameter calls the SSM GetParameter API with decryption enabled
// and returns the parameter value string.
func RevealSSMParameter(ctx context.Context, api SSMGetParameterAPI, paramName string) (string, error) {
	output, err := api.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &paramName,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("revealing SSM parameter: %w", err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return "", nil
	}
	return *output.Parameter.Value, nil
}
