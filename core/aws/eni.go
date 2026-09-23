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

// FetchNetworkInterfacesPage fetches a single page of network interfaces.
func FetchNetworkInterfacesPage(ctx context.Context, api EC2DescribeNetworkInterfacesAPI, continuationToken string) (resource.FetchResult, error) {
	// Service-managed interfaces (NAT gateways, VPC endpoints, RDS, Lambda)
	// are hidden by default once an account opts into hidden managed-resource
	// visibility; without them a security group in use by one of them reads as
	// unused.
	input := &ec2.DescribeNetworkInterfacesInput{
		MaxResults:              aws.Int32(DefaultPageSize),
		IncludeManagedResources: aws.Bool(true),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching network interfaces: %w", err)
	}

	resources := make([]resource.Resource, 0, len(output.NetworkInterfaces))
	for _, eni := range output.NetworkInterfaces {
		resources = append(resources, eniToResource(eni))
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

// FetchNetworkInterfacesByIDs fetches specific network interfaces by ID, for
// an exact-ID drill to an interface past the list's first page. Missing IDs
// are handled by fetchEC2ByIDs.
func FetchNetworkInterfacesByIDs(ctx context.Context, api EC2DescribeNetworkInterfacesAPI, ids []string) ([]resource.Resource, error) {
	return fetchEC2ByIDs(ctx, "eni FetchByIDs", "InvalidNetworkInterfaceID.NotFound", "eni-", ids, func(ids []string) ([]resource.Resource, error) {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeNetworkInterfacesOutput, error) {
			return api.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{NetworkInterfaceIds: ids, IncludeManagedResources: aws.Bool(true)})
		})
		if err != nil {
			return nil, err
		}
		resources := make([]resource.Resource, 0, len(out.NetworkInterfaces))
		for _, eni := range out.NetworkInterfaces {
			resources = append(resources, eniToResource(eni))
		}
		return resources, nil
	})
}

// eniToResource builds the row of one network interface.
func eniToResource(eni ec2types.NetworkInterface) resource.Resource {
	eniID := ""
	if eni.NetworkInterfaceId != nil {
		eniID = *eni.NetworkInterfaceId
	}

	// Extract Name from TagSet (NetworkInterface uses TagSet, not Tags)
	name := ""
	for _, tag := range eni.TagSet {
		if tag.Key != nil && *tag.Key == "Name" {
			if tag.Value != nil {
				name = *tag.Value
			}
			break
		}
	}

	status := string(eni.Status)
	interfaceType := string(eni.InterfaceType)

	vpcID := ""
	if eni.VpcId != nil {
		vpcID = *eni.VpcId
	}

	privateIP := ""
	if eni.PrivateIpAddress != nil {
		privateIP = *eni.PrivateIpAddress
	}

	requesterManaged := "false"
	if eni.RequesterManaged != nil && aws.ToBool(eni.RequesterManaged) {
		requesterManaged = "true"
	}

	description := ""
	if eni.Description != nil {
		description = *eni.Description
	}

	requesterID := ""
	if eni.RequesterId != nil {
		requesterID = *eni.RequesterId
	}

	securityGroupIDs := make([]string, 0, len(eni.Groups))
	for _, g := range eni.Groups {
		if g.GroupId != nil && *g.GroupId != "" {
			securityGroupIDs = append(securityGroupIDs, *g.GroupId)
		}
	}

	return resource.Resource{
		ID:   eniID,
		Name: name,
		Fields: map[string]string{
			"eni_id":            eniID,
			"name":              name,
			"status":            status,
			"type":              interfaceType,
			"vpc_id":            vpcID,
			"private_ip":        privateIP,
			"requester_managed": requesterManaged,
			// description/requester_id — required for the lambda:eni
			// related-panel pivot (checkLambdaENI matches Description
			// prefix "AWS Lambda VPC ENI-<FunctionName>-" and
			// RequesterId=="AWS Lambda VPC ENI" per docs/resources/lambda.md).
			"description":  description,
			"requester_id": requesterID,
			// security_groups — required for the ecs-task:sg related-panel
			// pivot (checkECSTaskSG chains Task -> ENI -> SG via this field).
			"security_groups": strings.Join(securityGroupIDs, ","),
		},
		RawStruct: eni,
		Findings:  eniFindings(status, requesterManaged),
	}
}

// eniFindings is the one predicate for a network interface. An unattached
// interface is wasted spend unless AWS itself owns it: requester-managed
// interfaces (VPC endpoints, ELB NICs, EFS mount targets) are "available" by
// design, and InterfaceType alone cannot tell them apart. colorENI runs this
// over Fields for rows built outside the fetcher.
func eniFindings(status, requesterManaged string) []domain.Finding {
	switch status {
	case "available":
		if requesterManaged != "true" {
			return []domain.Finding{wave1Finding(CodeENIStateAvailable)}
		}
	case "attaching":
		return []domain.Finding{wave1Finding(CodeENIStateAttaching)}
	case "detaching":
		return []domain.Finding{wave1Finding(CodeENIStateDetaching)}
	}
	return nil
}
