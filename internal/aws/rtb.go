package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("rtb", []string{"route_table_id", "name", "vpc_id", "routes_count", "associations_count", "blackhole_routes_count", "is_main"})

	resource.RegisterPaginated("rtb", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRouteTablesPage(ctx, c.EC2, continuationToken)
	})
}

// FetchRouteTables calls the EC2 DescribeRouteTables API and converts the
// response into a slice of generic Resource structs.
func FetchRouteTables(ctx context.Context, api EC2DescribeRouteTablesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRouteTablesPage(ctx, api, token)
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

// FetchRouteTablesPage fetches a single page of route tables.
func FetchRouteTablesPage(ctx context.Context, api EC2DescribeRouteTablesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeRouteTablesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeRouteTables(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching route tables: %w", err)
	}

	var resources []resource.Resource

	for _, rtb := range output.RouteTables {
		rtbID := ""
		if rtb.RouteTableId != nil {
			rtbID = *rtb.RouteTableId
		}

		name := ""
		for _, tag := range rtb.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if rtb.VpcId != nil {
			vpcID = *rtb.VpcId
		}

		routesCount := fmt.Sprintf("%d", len(rtb.Routes))
		associationsCount := fmt.Sprintf("%d", len(rtb.Associations))

		// Determine if this is the main route table
		isMain := "false"
		for _, assoc := range rtb.Associations {
			if assoc.Main != nil && *assoc.Main {
				isMain = "true"
				break
			}
		}

		// Count blackhole routes (target deleted)
		blackholeCount := 0
		for _, route := range rtb.Routes {
			if route.State == ec2types.RouteStateBlackhole {
				blackholeCount++
			}
		}

		r := resource.Resource{
			ID:   rtbID,
			Name: name,
			Fields: map[string]string{
				"route_table_id":         rtbID,
				"name":                   name,
				"vpc_id":                 vpcID,
				"routes_count":           routesCount,
				"associations_count":     associationsCount,
				"blackhole_routes_count": fmt.Sprintf("%d", blackholeCount),
				"is_main":                isMain,
			},
			RawStruct: rtb,
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
