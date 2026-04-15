package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// sensitivePorts is the set of ports that are considered security-sensitive
// when exposed to the internet (0.0.0.0/0 or ::/0).
var sensitivePorts = map[int32]bool{
	22:    true, // SSH
	3389:  true, // RDP
	3306:  true, // MySQL
	5432:  true, // PostgreSQL
	1433:  true, // MSSQL
	6379:  true, // Redis
	9200:  true, // Elasticsearch
	27017: true, // MongoDB
}

// coversSensitivePort reports whether the given IpPermission covers any
// sensitive port when open to the public internet.
func coversSensitivePort(p ec2types.IpPermission) bool {
	// All-protocols rule: covers every port.
	if aws.ToString(p.IpProtocol) == "-1" {
		return true
	}
	proto := aws.ToString(p.IpProtocol)
	if proto != "tcp" && proto != "udp" {
		return false
	}
	from := p.FromPort
	to := p.ToPort
	if from == nil || to == nil {
		return false
	}
	for port := range sensitivePorts {
		if *from <= port && port <= *to {
			return true
		}
	}
	return false
}

// isInternetFacing reports whether the IpPermission is open to the public
// internet via 0.0.0.0/0 or ::/0.
func isInternetFacing(p ec2types.IpPermission) bool {
	for _, r := range p.IpRanges {
		if aws.ToString(r.CidrIp) == "0.0.0.0/0" {
			return true
		}
	}
	for _, r := range p.Ipv6Ranges {
		if aws.ToString(r.CidrIpv6) == "::/0" {
			return true
		}
	}
	return false
}

// computeSGRiskFields inspects the ingress rules of a security group and
// returns (dangerous_open_count, wide_open) as strings ready for Fields.
func computeSGRiskFields(perms []ec2types.IpPermission) (string, string) {
	dangerousCount := 0
	wideOpen := false
	for _, p := range perms {
		if !isInternetFacing(p) {
			continue
		}
		// wide_open: all-protocols rule open to internet.
		if aws.ToString(p.IpProtocol) == "-1" {
			wideOpen = true
		}
		// dangerous_open_count: sensitive port open to internet.
		if coversSensitivePort(p) {
			dangerousCount++
		}
	}
	wideOpenStr := "false"
	if wideOpen {
		wideOpenStr = "true"
	}
	return strconv.Itoa(dangerousCount), wideOpenStr
}

func init() {
	resource.RegisterFieldKeys("sg", []string{"group_id", "group_name", "vpc_id", "description", "dangerous_open_count", "wide_open"})

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

		dangerousCount, wideOpen := computeSGRiskFields(sg.IpPermissions)

		r := resource.Resource{
			ID:     groupID,
			Name:   groupName,
			Status: "", // SGs have no status field
			Fields: map[string]string{
				"group_id":             groupID,
				"group_name":           groupName,
				"vpc_id":               vpcID,
				"description":          description,
				"dangerous_open_count": dangerousCount,
				"wide_open":            wideOpen,
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

