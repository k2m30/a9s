package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

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
// returns (dangerous_open_count, wide_open, risk_summary).
//
//	risk_summary is a short human-readable label:
//	  ""              — no internet exposure on dangerous ports
//	  "WIDE_OPEN"     — at least one rule with all-protocols (-1) open to 0.0.0.0/0
//	  "PORTS:22,3306" — specific dangerous ports open to 0.0.0.0/0
//	When both wide-open and specific ports are present, "WIDE_OPEN" wins
//	(it's the more severe signal).
func computeSGRiskFields(perms []ec2types.IpPermission) (string, string, string) {
	dangerousCount := 0
	wideOpen := false
	portSet := make(map[int32]struct{})
	for _, p := range perms {
		if !isInternetFacing(p) {
			continue
		}
		if aws.ToString(p.IpProtocol) == "-1" {
			wideOpen = true
			continue
		}
		if coversSensitivePort(p) {
			dangerousCount++
			// Capture the specific port(s) covered.
			if p.FromPort != nil && p.ToPort != nil {
				from, to := *p.FromPort, *p.ToPort
				if to < from {
					from, to = to, from
				}
				// Always enumerate the dangerous ports that fall inside [from, to].
				// We iterate the sensitive-port set (small, constant) rather than the
				// range itself, so a 1-65535 rule stays O(|sensitivePorts|) and
				// correctly surfaces every dangerous port the rule actually exposes.
				for sp := range sensitivePorts {
					if sp >= from && sp <= to {
						portSet[sp] = struct{}{}
					}
				}
			}
		}
	}
	wideOpenStr := "false"
	if wideOpen {
		wideOpenStr = "true"
	}
	riskSummary := ""
	switch {
	case wideOpen:
		riskSummary = "WIDE_OPEN"
	case dangerousCount > 0:
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, int(p))
		}
		sort.Ints(ports)
		parts := make([]string, len(ports))
		for i, p := range ports {
			parts[i] = strconv.Itoa(p)
		}
		if len(parts) > 0 {
			riskSummary = "PORTS:" + strings.Join(parts, ",")
		} else {
			// dangerousCount > 0 but no specific port captured (large-range case).
			riskSummary = "PORTS:?"
		}
	}
	return strconv.Itoa(dangerousCount), wideOpenStr, riskSummary
}

// FetchSecurityGroups calls the EC2 DescribeSecurityGroups API and returns all
// pages of security groups. Used by tests; the production path uses the per-page fetcher for pagination.
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

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeSecurityGroupsOutput, error) {
		return api.DescribeSecurityGroups(ctx, input)
	})
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

		dangerousCount, wideOpen, riskSummary := computeSGRiskFields(sg.IpPermissions)

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
				"risk_summary":         riskSummary,
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
