package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("sg", []string{"group_id", "group_name", "vpc_id", "description"})

	resource.RegisterPaginated("sg", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchSecurityGroupsPage(ctx, c.EC2, continuationToken)
	})
}

// FetchSecurityGroups calls the EC2 DescribeSecurityGroups API and returns all
// pages of security groups. Used by existing tests and the legacy fetcher.
func FetchSecurityGroups(ctx context.Context, api EC2DescribeSecurityGroupsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSecurityGroupsPage(ctx, api, token)
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

// FetchSecurityGroupsPage calls the EC2 DescribeSecurityGroups API and returns
// a single page of security groups. Pass an empty continuationToken for the first page.
func FetchSecurityGroupsPage(ctx context.Context, api EC2DescribeSecurityGroupsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeSecurityGroupsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeSecurityGroups(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching security groups: %w", err)
	}

	var resources []resource.Resource
	for _, sg := range output.SecurityGroups {
		// Extract GroupId
		groupID := ""
		if sg.GroupId != nil {
			groupID = *sg.GroupId
		}

		// Extract GroupName
		groupName := ""
		if sg.GroupName != nil {
			groupName = *sg.GroupName
		}

		// Extract VpcId
		vpcID := ""
		if sg.VpcId != nil {
			vpcID = *sg.VpcId
		}

		// Extract Description
		description := ""
		if sg.Description != nil {
			description = *sg.Description
		}

		r := resource.Resource{
			ID:     groupID,
			Name:   groupName,
			Status: "", // SGs have no status field
			Fields: map[string]string{
				"group_id":    groupID,
				"group_name":  groupName,
				"vpc_id":      vpcID,
				"description": description,
			},
			RawStruct: sg,
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

