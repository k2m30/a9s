// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
	"github.com/k2m30/a9s/v3/core/resource"
)

// vpcePolicyExposure evaluates an endpoint policy document and reports the
// Attention rows for an unrestricted grant. All three of a wildcard
// principal, a wildcard action and a wildcard resource must meet in ONE
// unconditioned statement: that combination, and only it, is the full-access
// policy AWS attaches when the endpoint is created without one. A policy that
// names concrete actions, confines the grant to named resources, or scopes
// the wildcard principal with a condition to fixed values (no '*', '?' or
// policy variable) is a deliberate grant and is not a finding. An
// unparseable document is unknown, not open.
//
// Each Allow statement is evaluated beside the document's Deny statements:
// the aggregate Exposure merges the actions of every public statement, so one
// statement granting "*" on a bucket and another granting s3:GetObject on "*"
// together read as a full-access policy neither of them is.
func vpcePolicyExposure(policyDocument string) ([]domain.DetailRow, bool) {
	if policyDocument == "" {
		return nil, false
	}
	doc, err := iampolicy.Parse(policyDocument)
	if err != nil {
		return nil, false
	}
	var denies []iampolicy.Statement
	for _, st := range doc.Statement {
		if st.Effect == "Deny" {
			denies = append(denies, st)
		}
	}
	for _, st := range doc.Statement {
		if st.Effect != "Allow" || !slices.Contains(st.Resource, "*") {
			continue
		}
		ex := iampolicy.EvaluateEndpoint(iampolicy.Document{Statement: append([]iampolicy.Statement{st}, denies...)})
		if !ex.Public || !slices.ContainsFunc(ex.PublicActions, isWildcardAction) {
			continue
		}
		return append([]domain.DetailRow{
			{Label: "Principal", Value: "*", Tier: "~"},
			{Label: "Actions", Value: strings.Join(ex.PublicActions, ", "), Tier: "~"},
		}, publicConditionRows(ex, "~")...), true
	}
	return nil, false
}

// isWildcardAction reports whether an IAM action string grants every action
// the endpoint can reach. An endpoint only ever fronts one service, so
// "s3:*" withholds nothing that "*" would have granted through it.
func isWildcardAction(action string) bool {
	_, verb, hasService := strings.Cut(action, ":")
	if hasService {
		return verb == "*"
	}
	return action == "*"
}

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

		rows, open := vpcePolicyExposure(aws.ToString(vpce.PolicyDocument))
		policyExposure := "scoped"
		if open {
			policyExposure = "open"
		}

		findings := vpceFindings(state, policyExposure)

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if hasFinding(findings, CodeVPCEPolicyOpen) {
			attentionDetails = map[domain.FindingCode]domain.AttentionDetail{
				CodeVPCEPolicyOpen: {Rows: rows},
			}
		}

		r := resource.Resource{
			ID:   vpceID,
			Name: serviceName,
			Fields: map[string]string{
				"vpce_id":      vpceID,
				"service_name": serviceName,
				"type":         endpointType,
				"state":        state,
				"vpc_id":       vpcID,
				// The evaluation's verdict as a word, never the document.
				"policy_exposure": policyExposure,
			},
			Findings:         findings,
			AttentionDetails: attentionDetails,
			RawStruct:        vpce,
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
func vpceFindings(state, policyExposure string) []domain.Finding {
	var posture []domain.Finding
	if policyExposure == "open" {
		posture = append(posture, wave1Finding(CodeVPCEPolicyOpen))
	}
	return withState("vpce", state, posture)
}
