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

// FetchVPCEndpointsPage fetches a single page of VPC endpoints.
func FetchVPCEndpointsPage(ctx context.Context, api EC2DescribeVpcEndpointsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVpcEndpointsInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeVpcEndpoints(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching VPC endpoints: %w", err)
	}

	var resources []resource.Resource

	for _, vpce := range output.VpcEndpoints {
		vpceID := ""
		if vpce.VpcEndpointId != nil {
			vpceID = *vpce.VpcEndpointId
		}

		serviceName := ""
		if vpce.ServiceName != nil {
			serviceName = *vpce.ServiceName
		}

		endpointType := string(vpce.VpcEndpointType)
		state := string(vpce.State)

		vpcID := ""
		if vpce.VpcId != nil {
			vpcID = *vpce.VpcId
		}

		policy := readPolicy("vpce", aws.ToString(vpce.PolicyDocument), "", "")
		findings := vpceFindings(state, policy)

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if hasFinding(findings, CodeVPCEPolicyOpen) {
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
				CodeVPCEPolicyOpen: policy.detail(),
			}
		}

		r := withWave1(resource.Resource{
			ID:   vpceID,
			Name: serviceName,
			Fields: map[string]string{
				"vpce_id":      vpceID,
				"service_name": serviceName,
				"type":         endpointType,
				"state":        state,
				"vpc_id":       vpcID,
			},
			AttentionDetails: attentionDetails,
			RawStruct:        vpce,
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

// vpceStates is the VpcEndpoint State list as DescribeVpcEndpoints answers it
// (Amazon EC2 API reference, VpcEndpoint: "Valid Values: PendingAcceptance |
// Pending | Available | Deleting | Deleted | Rejected | Failed | Expired |
// Partial", sent lower-camel on the wire).
var vpceStates = []string{ //nolint:gochecknoglobals // static AWS status domain
	"pendingAcceptance", "pending", "available", "deleting", "deleted",
	"rejected", "failed", "expired", "partial",
}

// vpceFindings is the one predicate for an endpoint: its state, then whether
// its policy lets anyone in. DescribeVpcEndpoints sends the state lower-camel
// ("pendingAcceptance"), not in the SDK enum constants' spelling
// ("PendingAcceptance"), so the table declares the documented API list.
func vpceFindings(state string, policy policyReading) []domain.Finding {
	var posture []domain.Finding
	if policy.public() {
		posture = append(posture, wave1Finding(CodeVPCEPolicyOpen))
	}
	return withState("vpce", state, policy.withUnread(posture))
}
