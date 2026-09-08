// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchTransitGatewaysPage fetches a single page of transit gateways.
func FetchTransitGatewaysPage(ctx context.Context, api EC2DescribeTransitGatewaysAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeTransitGatewaysInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeTransitGateways(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching transit gateways: %w", err)
	}

	var resources []resource.Resource

	for _, tgw := range output.TransitGateways {
		tgwID := ""
		if tgw.TransitGatewayId != nil {
			tgwID = *tgw.TransitGatewayId
		}

		// Extract Name from Tags
		name := ""
		for _, tag := range tgw.Tags {
			if tag.Key != nil && *tag.Key == "Name" {
				if tag.Value != nil {
					name = *tag.Value
				}
				break
			}
		}

		state := string(tgw.State)

		ownerID := ""
		if tgw.OwnerId != nil {
			ownerID = *tgw.OwnerId
		}

		description := ""
		if tgw.Description != nil {
			description = *tgw.Description
		}

		autoAccept := "no"
		if tgw.Options != nil && tgw.Options.AutoAcceptSharedAttachments == ec2types.AutoAcceptSharedAttachmentsValueEnable {
			autoAccept = "yes"
		}

		findings := tgwFindings(state, autoAccept)

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if hasFinding(findings, CodeTGWAutoAccept) {
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
				CodeTGWAutoAccept: {Rows: []domain.DetailRow{
					{Label: "Auto-accept shared attachments", Value: "enabled", Tier: "~"},
				}},
			}
		}

		r := resource.Resource{
			ID:   tgwID,
			Name: name,
			Fields: map[string]string{
				"tgw_id":   tgwID,
				"name":     name,
				"state":    state,
				"owner_id": ownerID,
				// The option as a word: colorTGW's fallback runs tgwFindings
				// over Fields, so a row stripped of its findings has to be able
				// to recover this one. Same reason as subnet's auto_public_ip.
				"auto_accept": autoAccept,
				"description": description,
			},
			Findings:         findings,
			AttentionDetails: attentionDetails,
			RawStruct:        tgw,
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

// tgwFindings is the one predicate for a transit gateway: its state, then
// whether it accepts shared attachments without review. A gateway on its way
// out cannot accept anything, so the posture finding is suppressed there.
// colorTGW runs this over Fields for rows built outside the fetcher.
func tgwFindings(state, autoAccept string) []domain.Finding {
	var findings []domain.Finding
	switch state {
	case "pending":
		findings = []domain.Finding{wave1Finding(CodeTGWStatePending)}
	case "modifying":
		findings = []domain.Finding{wave1Finding(CodeTGWStateModifying)}
	case "deleting":
		findings = []domain.Finding{wave1Finding(CodeTGWStateDeleting)}
	case "failed":
		findings = []domain.Finding{wave1Finding(CodeTGWStateFailed)}
	case "deleted":
		findings = []domain.Finding{wave1Finding(CodeTGWStateDeleted)}
	}
	if autoAccept == "yes" && state != "deleting" && state != "deleted" {
		findings = append(findings, wave1Finding(CodeTGWAutoAccept))
	}
	return findings
}
