package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchAPIGatewaysPageMerged fetches a single page of API Gateways from both
// APIGateway V2 (HTTP/WEBSOCKET) and APIGateway V1 (REST), merging results.
// On the first page (continuationToken == ""), all V1 REST APIs are fully
// paginated (using Position-based pagination) and the first page of V2 is
// fetched. On subsequent pages, only V2 pagination continues.
// V1 REST API resources have Fields["protocol"] == "REST".
// If c.APIGatewayV1 is nil, only V2 resources are returned.
func FetchAPIGatewaysPageMerged(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	var resources []resource.Resource

	// On the first page, fetch all V1 REST APIs (fully paginated).
	if continuationToken == "" && c.APIGatewayV1 != nil {
		var position *string
		for {
			input := &apigateway.GetRestApisInput{
				Limit: aws.Int32(DefaultPageSize),
			}
			if position != nil {
				input.Position = position
			}
			out, err := c.APIGatewayV1.GetRestApis(ctx, input)
			if err != nil {
				return resource.FetchResult{}, fmt.Errorf("fetching REST API gateways: %w", err)
			}
			for _, item := range out.Items {
				apiID := aws.ToString(item.Id)
				name := aws.ToString(item.Name)
				description := aws.ToString(item.Description)
				r := resource.Resource{
					ID:   apiID,
					Name: name,
					Fields: map[string]string{
						"api_id":      apiID,
						"name":        name,
						"protocol":    "REST",
						"endpoint":    "",
						"description": description,
					},
					RawStruct: item,
				}
				resources = append(resources, r)
			}
			if out.Position == nil {
				break
			}
			position = out.Position
		}
	}

	// Fetch the current page of V2 APIs.
	v2Result, err := FetchAPIGatewaysPage(ctx, c.APIGatewayV2, continuationToken)
	if err != nil {
		return resource.FetchResult{}, err
	}
	resources = append(resources, v2Result.Resources...)

	return resource.FetchResult{
		Resources:  resources,
		Pagination: v2Result.Pagination,
	}, nil
}

// FetchAPIGateways calls the API Gateway V2 GetApis API and converts
// the response into a slice of generic Resource structs.
func FetchAPIGateways(ctx context.Context, api APIGatewayV2GetApisAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchAPIGatewaysPage(ctx, api, token)
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

// FetchAPIGatewaysPage fetches a single page of API Gateways.
func FetchAPIGatewaysPage(ctx context.Context, api APIGatewayV2GetApisAPI, continuationToken string) (resource.FetchResult, error) {
	input := &apigatewayv2.GetApisInput{
		MaxResults: aws.String(fmt.Sprintf("%d", DefaultPageSize)),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.GetApis(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching API gateways: %w", err)
	}

	var resources []resource.Resource

	for _, item := range output.Items {
		apiID := ""
		if item.ApiId != nil {
			apiID = *item.ApiId
		}

		name := ""
		if item.Name != nil {
			name = *item.Name
		}

		protocol := string(item.ProtocolType)

		endpoint := ""
		if item.ApiEndpoint != nil {
			endpoint = *item.ApiEndpoint
		}

		description := ""
		if item.Description != nil {
			description = *item.Description
		}

		r := resource.Resource{
			ID:   apiID,
			Name: name,
			Fields: map[string]string{
				"api_id":      apiID,
				"name":        name,
				"protocol":    protocol,
				"endpoint":    endpoint,
				"description": description,
			},
			RawStruct: item,
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
