package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eb", []string{"environment_name", "application_name", "status", "health", "version_label"})

	resource.RegisterPaginated("eb", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBEnvironmentsPage(ctx, c.ElasticBeanstalk, continuationToken)
	})
}

// FetchEBEnvironments calls the Elastic Beanstalk DescribeEnvironments API and converts
// the response into a slice of generic Resource structs.
func FetchEBEnvironments(ctx context.Context, api EBDescribeEnvironmentsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEBEnvironmentsPage(ctx, api, token)
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

// FetchEBEnvironmentsPage fetches a single page of Elastic Beanstalk environments.
func FetchEBEnvironmentsPage(ctx context.Context, api EBDescribeEnvironmentsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &elasticbeanstalk.DescribeEnvironmentsInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeEnvironments(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Elastic Beanstalk environments: %w", err)
	}

	var resources []resource.Resource

	for _, env := range output.Environments {
		envName := ""
		if env.EnvironmentName != nil {
			envName = *env.EnvironmentName
		}

		envID := ""
		if env.EnvironmentId != nil {
			envID = *env.EnvironmentId
		}

		appName := ""
		if env.ApplicationName != nil {
			appName = *env.ApplicationName
		}

		status := string(env.Status)
		health := string(env.Health)

		versionLabel := ""
		if env.VersionLabel != nil {
			versionLabel = *env.VersionLabel
		}

		solutionStack := ""
		if env.SolutionStackName != nil {
			solutionStack = *env.SolutionStackName
		}

		platformArn := ""
		if env.PlatformArn != nil {
			platformArn = *env.PlatformArn
		}

		endpointURL := ""
		if env.EndpointURL != nil {
			endpointURL = *env.EndpointURL
		}

		dateCreated := ""
		if env.DateCreated != nil {
			dateCreated = env.DateCreated.Format("2006-01-02 15:04")
		}

		envArn := ""
		if env.EnvironmentArn != nil {
			envArn = *env.EnvironmentArn
		}

		r := resource.Resource{
			ID:     envID,
			Name:   envName,
			Status: health,
			Fields: map[string]string{
				"environment_name": envName,
				"environment_id":   envID,
				"application_name": appName,
				"status":           status,
				"health":           health,
				"version_label":    versionLabel,
				"solution_stack":   solutionStack,
				"platform_arn":     platformArn,
				"endpoint_url":     endpointURL,
				"date_created":     dateCreated,
				"environment_arn":  envArn,
			},
			RawStruct: env,
		}

		resources = append(resources, r)
	}

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
