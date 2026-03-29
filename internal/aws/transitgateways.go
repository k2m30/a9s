package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("tgw", []string{"tgw_id", "name", "state", "owner_id", "description"})

	resource.RegisterPaginated("tgw", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchTransitGatewaysPage(ctx, c.EC2, continuationToken)
	})
}

// FetchTransitGateways calls the EC2 DescribeTransitGateways API and converts the
// response into a slice of generic Resource structs.
func FetchTransitGateways(ctx context.Context, api EC2DescribeTransitGatewaysAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchTransitGatewaysPage(ctx, api, token)
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

// FetchTransitGatewaysPage fetches a single page of transit gateways.
func FetchTransitGatewaysPage(ctx context.Context, api EC2DescribeTransitGatewaysAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeTransitGatewaysInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeTransitGateways(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching transit gateways: %w", err)
	}

	var resources []resource.Resource

	for _, tgw := range output.TransitGateways {
		tgwID := ""
		if tgw.TransitGatewayId != nil {
			tgwID = *tgw.TransitGatewayId
		}

		// Extract Name from Tags
		name := ""
		for _, tag := range tgw.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		state := string(tgw.State)

		ownerID := ""
		if tgw.OwnerId != nil {
			ownerID = *tgw.OwnerId
		}

		description := ""
		if tgw.Description != nil {
			description = *tgw.Description
		}

		r := resource.Resource{
			ID:     tgwID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"tgw_id":      tgwID,
				"name":        name,
				"state":       state,
				"owner_id":    ownerID,
				"description": description,
			},
			RawStruct: tgw,
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
