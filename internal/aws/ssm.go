package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ssm", []string{"name", "type", "version", "last_modified", "description", "risk"})
	resource.RegisterRevealFetcher("ssm", func(ctx context.Context, clients any, resourceID string) (string, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return "", fmt.Errorf("AWS clients not initialized")
		}
		return RevealSSMParameter(ctx, c.SSM, resourceID)
	})

	resource.RegisterPaginated("ssm", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
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
	input := &ssm.DescribeParametersInput{MaxResults: aws.Int32(DefaultPageSize)}
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

		// Compute risk: STALE if SecureString older than 365d; PLAINTEXT if String with sensitive name
		risk := ""
		lowerName := strings.ToLower(paramName)
		if paramType == "SecureString" && param.LastModifiedDate != nil && time.Since(*param.LastModifiedDate) > 365*24*time.Hour {
			risk = "STALE"
		} else if paramType == "String" && (strings.Contains(lowerName, "/password") || strings.Contains(lowerName, "/secret") || strings.Contains(lowerName, "/token")) {
			risk = "PLAINTEXT"
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
				"risk":          risk,
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
