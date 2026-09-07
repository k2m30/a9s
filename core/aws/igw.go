// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchInternetGatewaysPage fetches a single page of internet gateways.
func FetchInternetGatewaysPage(ctx context.Context, api EC2DescribeInternetGatewaysAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeInternetGatewaysInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
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

		findings := igwFindings(state, len(igw.Attachments))

		r := resource.Resource{
			ID:   igwID,
			Name: name,
			Fields: map[string]string{
				"igw_id":            igwID,
				"name":              name,
				"vpc_id":            vpcID,
				"state":             state,
				"attachments_count": fmt.Sprintf("%d", len(igw.Attachments)),
			},
			Findings:  findings,
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

// igwFindings is the one predicate for an internet gateway: its attachment
// state, then whether it is attached to anything at all. colorIGW runs it over
// Fields for rows built outside the fetcher.
func igwFindings(state string, attachmentsCount int) []domain.Finding {
	switch state {
	case "attaching":
		return []domain.Finding{{Code: CodeIGWStateAttaching, Phrase: "attaching", Detail: catalog.Detail(CodeIGWStateAttaching), Severity: domain.SevWarn, Source: "wave1"}}
	case "detaching":
		return []domain.Finding{{Code: CodeIGWStateDetaching, Phrase: "detaching", Detail: catalog.Detail(CodeIGWStateDetaching), Severity: domain.SevWarn, Source: "wave1"}}
	}
	if attachmentsCount == 0 {
		return []domain.Finding{{Code: CodeIGWNoAttachments, Phrase: "no VPC attachments", Detail: catalog.Detail(CodeIGWNoAttachments), Severity: domain.SevWarn, Source: "wave1"}}
	}
	return nil
}
