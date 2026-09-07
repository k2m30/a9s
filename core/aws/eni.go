// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchNetworkInterfacesPage fetches a single page of network interfaces.
func FetchNetworkInterfacesPage(ctx context.Context, api EC2DescribeNetworkInterfacesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeNetworkInterfacesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeNetworkInterfaces(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching network interfaces: %w", err)
	}

	var resources []resource.Resource

	for _, eni := range output.NetworkInterfaces {
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

		r := resource.Resource{
			ID:   eniID,
			Name: name,
			// Status intentionally unset — lifecycle state is emitted as a Finding.
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
		}

		// emit canonical Findings for non-healthy ENI states.
		// in-use → healthy (no Finding). available → SevWarn (potential cost waste,
		// except for requester-managed which are managed by AWS services).
		// attaching / detaching → SevWarn (transitional).
		r.Findings = eniFindings(status, requesterManaged)

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

// eniFindings is the one predicate for a network interface. An unattached
// interface is wasted spend unless AWS itself owns it: requester-managed
// interfaces (VPC endpoints, ELB NICs, EFS mount targets) are "available" by
// design, and InterfaceType alone cannot tell them apart. colorENI runs this
// over Fields for rows built outside the fetcher.
func eniFindings(status, requesterManaged string) []domain.Finding {
	switch status {
	case "available":
		if requesterManaged != "true" {
			return []domain.Finding{{Code: CodeENIStateAvailable, Phrase: "available", Detail: catalog.Detail(CodeENIStateAvailable), Severity: domain.SevWarn, Source: "wave1"}}
		}
	case "attaching":
		return []domain.Finding{{Code: CodeENIStateAttaching, Phrase: "attaching", Detail: catalog.Detail(CodeENIStateAttaching), Severity: domain.SevWarn, Source: "wave1"}}
	case "detaching":
		return []domain.Finding{{Code: CodeENIStateDetaching, Phrase: "detaching", Detail: catalog.Detail(CodeENIStateDetaching), Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
