package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchSubnets calls the EC2 DescribeSubnets API and converts the
// response into a slice of generic Resource structs.
func FetchSubnets(ctx context.Context, api EC2DescribeSubnetsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchSubnetsPage(ctx, api, token)
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

// FetchSubnetsPage fetches a single page of subnets.
func FetchSubnetsPage(ctx context.Context, api EC2DescribeSubnetsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeSubnetsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeSubnets(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching subnets: %w", err)
	}

	var resources []resource.Resource

	for _, subnet := range output.Subnets {
		subnetID := ""
		if subnet.SubnetId != nil {
			subnetID = *subnet.SubnetId
		}

		name := ""
		for _, tag := range subnet.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		vpcID := ""
		if subnet.VpcId != nil {
			vpcID = *subnet.VpcId
		}

		cidrBlock := ""
		if subnet.CidrBlock != nil {
			cidrBlock = *subnet.CidrBlock
		}

		az := ""
		if subnet.AvailabilityZone != nil {
			az = *subnet.AvailabilityZone
		}

		state := string(subnet.State)

		availableIPs := ""
		if subnet.AvailableIpAddressCount != nil {
			availableIPs = fmt.Sprintf("%d", *subnet.AvailableIpAddressCount)
		}

		var findings []domain.Finding
		switch state {
		case "pending":
			findings = []domain.Finding{{Code: CodeSubnetStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
		case "unavailable":
			findings = []domain.Finding{{Code: CodeSubnetStateUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"}}
		case "failed":
			findings = []domain.Finding{{Code: CodeSubnetStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
		case "failed-insufficient-capacity":
			findings = []domain.Finding{{Code: CodeSubnetStateFailedInsufficientCapacity, Phrase: "failed-insufficient-capacity", Severity: domain.SevBroken, Source: "wave1"}}
		}

		r := resource.Resource{
			ID:   subnetID,
			Name: name,
			Fields: map[string]string{
				"subnet_id":         subnetID,
				"name":              name,
				"vpc_id":            vpcID,
				"cidr_block":        cidrBlock,
				"availability_zone": az,
				"state":             state,
				"available_ips":     availableIPs,
			},
			Findings:  findings,
			RawStruct: subnet,
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
