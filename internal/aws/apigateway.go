package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("apigw", []string{"api_id", "name", "protocol", "endpoint", "description"})

	resource.RegisterPaginated("apigw", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAPIGatewaysPage(ctx, c.APIGatewayV2, continuationToken)
	})
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
	input := &apigatewayv2.GetApisInput{}
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
			ID:     apiID,
			Name:   name,
			Status: "",
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
