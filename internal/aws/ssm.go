package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.Register("ssm", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSSMParameters(ctx, c.SSM)
	})
	resource.RegisterFieldKeys("ssm", []string{"name", "type", "version", "last_modified", "description"})
	resource.RegisterRevealFetcher("ssm", func(ctx context.Context, clients interface{}, resourceID string) (string, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return "", fmt.Errorf("AWS clients not initialized")
		}
		return RevealSSMParameter(ctx, c.SSM, resourceID)
	})
}

// FetchSSMParameters calls the SSM DescribeParameters API and converts the
// response into a slice of generic Resource structs.
func FetchSSMParameters(ctx context.Context, api SSMDescribeParametersAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := api.DescribeParameters(ctx, &ssm.DescribeParametersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching SSM parameters: %w", err)
		}

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

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
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
