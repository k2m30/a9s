// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// stampVPCSubnetIDs writes each VPC's subnet ids onto its row, which the flow
// log check reads: a flow log attaches to a VPC, a subnet or an interface, and
// asking only about the VPC's own id calls a fully covered VPC uncovered.
//
// One DescribeSubnets for the page, not one per VPC. Reached by type assertion
// so the narrow DescribeVpcs fakes this fetcher is exercised with keep working
// — they simply produce no subnet ids, and the check then reads what it read
// before. A failure is silent for the same reason: the subnet list is context
// for another check, not a fact this page promises.
func stampVPCSubnetIDs(ctx context.Context, api EC2DescribeVpcsAPI, resources []resource.Resource) {
	subnetAPI, ok := api.(EC2DescribeSubnetsAPI)
	if !ok || len(resources) == 0 {
		return
	}
	vpcIDs := make([]string, 0, len(resources))
	for _, r := range resources {
		vpcIDs = append(vpcIDs, r.ID)
	}
	byVPC := map[string][]string{}
	var nextToken *string
	for range PerParentPageCap {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeSubnetsOutput, error) {
			return subnetAPI.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
				Filters:   []ec2types.Filter{{Name: aws.String("vpc-id"), Values: vpcIDs}},
				NextToken: nextToken,
			})
		})
		if err != nil {
			return
		}
		for _, sn := range out.Subnets {
			byVPC[aws.ToString(sn.VpcId)] = append(byVPC[aws.ToString(sn.VpcId)], aws.ToString(sn.SubnetId))
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}
	for i := range resources {
		if ids := byVPC[resources[i].ID]; len(ids) > 0 {
			resources[i].Fields["subnet_ids"] = strings.Join(ids, ",")
		}
	}
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

		findings := vpcStateFindings(state)

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

	stampVPCSubnetIDs(ctx, api, resources)

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

// vpcStateFindings is the one predicate for a VPC's state. available reports
// nothing. colorVPC runs it over Fields for rows built outside the fetcher.
func vpcStateFindings(state string) []domain.Finding {
	if state == "pending" {
		return []domain.Finding{wave1Finding(CodeVPCStatePending)}
	}
	return nil
}
