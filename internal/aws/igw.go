package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("igw", []string{"igw_id", "name", "vpc_id", "state"})

	resource.RegisterNavigableFields("igw", []resource.NavigableField{
		{FieldPath: "Attachments.VpcId", TargetType: "vpc"},
	})

	resource.RegisterRelated("igw", []resource.RelatedDef{
		{TargetType: "vpc", DisplayName: "VPCs", Checker: checkIGWVPC, NeedsTargetCache: true},
		{TargetType: "rtb", DisplayName: "Route Tables", Checker: checkIGWRTB, NeedsTargetCache: true},
	})

	resource.RegisterPaginated("igw", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchInternetGatewaysPage(ctx, c.EC2, continuationToken)
	})
}

// FetchInternetGateways calls the EC2 DescribeInternetGateways API and converts the
// response into a slice of generic Resource structs.
func FetchInternetGateways(ctx context.Context, api EC2DescribeInternetGatewaysAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchInternetGatewaysPage(ctx, api, token)
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

// FetchInternetGatewaysPage fetches a single page of internet gateways.
func FetchInternetGatewaysPage(ctx context.Context, api EC2DescribeInternetGatewaysAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeInternetGatewaysInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeInternetGateways(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching internet gateways: %w", err)
	}

	var resources []resource.Resource

	for _, igw := range output.InternetGateways {
		igwID := ""
		if igw.InternetGatewayId != nil {
			igwID = *igw.InternetGatewayId
		}

		name := ""
		for _, tag := range igw.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		// Extract VPC ID and state from attachments
		vpcID := ""
		state := "detached"
		if len(igw.Attachments) > 0 {
			if igw.Attachments[0].VpcId != nil {
				vpcID = *igw.Attachments[0].VpcId
			}
			state = string(igw.Attachments[0].State)
		}

		r := resource.Resource{
			ID:     igwID,
			Name:   name,
			Status: state,
			Fields: map[string]string{
				"igw_id": igwID,
				"name":   name,
				"vpc_id": vpcID,
				"state":  state,
			},
			RawStruct: igw,
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
