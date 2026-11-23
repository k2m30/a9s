// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

		autoPublicIP := "no"
		if subnet.MapPublicIpOnLaunch != nil && *subnet.MapPublicIpOnLaunch {
			autoPublicIP = "yes"
		}

		findings := subnetFindings(state, autoPublicIP)

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if autoPublicIP == "yes" {
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
				CodeSubnetAutoPublicIP: {Rows: []domain.DetailRow{
					{Label: "Public address on launch", Value: "enabled", Tier: "~"},
				}},
			}
		}

		r := withWave1("subnet", resource.Resource{
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
				"auto_public_ip":    autoPublicIP,
			},
			AttentionDetails: attentionDetails,
			RawStruct:        subnet,
		}, findings...)

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

func subnetFindings(state, autoPublicIP string) []domain.Finding {
	var posture []domain.Finding
	if autoPublicIP == "yes" {
		posture = append(posture, wave1Finding(CodeSubnetAutoPublicIP))
	}
	return withState("subnet", state, posture)
}
