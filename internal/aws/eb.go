package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eb", []string{"environment_name", "application_name", "status", "health", "version_label"})
	resource.Register("eb", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBEnvironments(ctx, c.ElasticBeanstalk)
	})
}

// FetchEBEnvironments calls the Elastic Beanstalk DescribeEnvironments API and converts
// the response into a slice of generic Resource structs.
func FetchEBEnvironments(ctx context.Context, api EBDescribeEnvironmentsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeEnvironments(ctx, &elasticbeanstalk.DescribeEnvironmentsInput{})
	if err != nil {
		return nil, err
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
			dateCreated = env.DateCreated.Format("2006-01-02 15:04:05")
		}

		envArn := ""
		if env.EnvironmentArn != nil {
			envArn = *env.EnvironmentArn
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(env, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     envID,
			Name:   envName,
			Status: status,
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
			RawJSON:   rawJSON,
			RawStruct: env,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
