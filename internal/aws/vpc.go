package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchVPCs calls the EC2 DescribeVpcs API and converts the
// response into a slice of generic Resource structs.
func FetchVPCs(ctx context.Context, api EC2DescribeVpcsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchVPCsPage(ctx, api, token)
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

// FetchVPCsPage fetches a single page of VPCs.
func FetchVPCsPage(ctx context.Context, api EC2DescribeVpcsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVpcsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeVpcs(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching VPCs: %w", err)
	}

	var resources []resource.Resource

	for _, vpc := range output.Vpcs {
		// Extract VPC ID
		vpcID := ""
		if vpc.VpcId != nil {
			vpcID = *vpc.VpcId
		}

		// Extract Name from Tags
		name := ""
		for _, tag := range vpc.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		// Extract CIDR Block
		cidrBlock := ""
		if vpc.CidrBlock != nil {
			cidrBlock = *vpc.CidrBlock
		}

		// Extract State
		state := string(vpc.State)

		// Extract IsDefault
		isDefault := "false"
		if vpc.IsDefault != nil && *vpc.IsDefault {
			isDefault = "true"
		}

		var findings []domain.Finding
		switch state {
		case "pending":
			findings = []domain.Finding{{Code: CodeVPCStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
		}

		r := resource.Resource{
			ID:   vpcID,
			Name: name,
			Fields: map[string]string{
				"vpc_id":     vpcID,
				"name":       name,
				"cidr_block": cidrBlock,
				"state":      state,
				"is_default": isDefault,
			},
			Findings:  findings,
			RawStruct: vpc,
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
