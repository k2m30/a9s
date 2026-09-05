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

		var findings []domain.Finding
		switch state {
		case "pending":
			findings = []domain.Finding{{Code: CodeTGWStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
		case "modifying":
			findings = []domain.Finding{{Code: CodeTGWStateModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1"}}
		case "deleting":
			findings = []domain.Finding{{Code: CodeTGWStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
		case "failed":
			findings = []domain.Finding{{Code: CodeTGWStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
		case "deleted":
			findings = []domain.Finding{{Code: CodeTGWStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"}}
		}

		// A gateway on its way out cannot accept anything; a posture finding
		// on it is noise an operator can do nothing about.
		lifecycleEnded := state == "deleting" || state == "deleted"

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if !lifecycleEnded && tgw.Options != nil && tgw.Options.AutoAcceptSharedAttachments == ec2types.AutoAcceptSharedAttachmentsValueEnable {
			findings = append(findings, domain.Finding{
				Code:     CodeTGWAutoAccept,
				Phrase:   TGWAutoAcceptPhrase,
				Detail:   TGWAutoAcceptDetail,
				Severity: domain.SevWarn,
				Source:   "wave1",
			})
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
				"tgw_id":      tgwID,
				"name":        name,
				"state":       state,
				"owner_id":    ownerID,
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
