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
// Attention rows for an unrestricted grant. Only a wildcard principal holding
// a wildcard action counts: that is the full-access policy AWS attaches when
// the endpoint is created without one. A policy that names concrete actions,
// or scopes the wildcard principal with a condition, is a deliberate grant
// and is not a finding. An unparseable document is unknown, not open.
//
// ownAccount is empty here: only Exposure.Public is consulted, and that
// verdict does not depend on which account owns the endpoint.
func vpcePolicyExposure(policyDocument string) ([]domain.DetailRow, bool) {
	if policyDocument == "" {
		return nil, false
	}
	doc, err := iampolicy.Parse(policyDocument)
	if err != nil {
		return nil, false
	}
	ex := iampolicy.Evaluate(doc, "")
	if !ex.Public || !slices.ContainsFunc(ex.PublicActions, isWildcardAction) {
		return nil, false
	}
	return []domain.DetailRow{
		{Label: "Principal", Value: "*", Tier: "~"},
		{Label: "Actions", Value: strings.Join(ex.PublicActions, ", "), Tier: "~"},
	}, true
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

		var findings []domain.Finding
		switch state {
		case "PendingAcceptance":
			findings = []domain.Finding{{Code: CodeVPCEStatePendingAcceptance, Phrase: "pending acceptance", Severity: domain.SevWarn, Source: "wave1"}}
		case "Pending":
			findings = []domain.Finding{{Code: CodeVPCEStatePending, Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"}}
		case "Deleting":
			findings = []domain.Finding{{Code: CodeVPCEStateDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
		case "Failed":
			findings = []domain.Finding{{Code: CodeVPCEStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
		case "Rejected":
			findings = []domain.Finding{{Code: CodeVPCEStateRejected, Phrase: "rejected", Severity: domain.SevBroken, Source: "wave1"}}
		case "Expired":
			findings = []domain.Finding{{Code: CodeVPCEStateExpired, Phrase: "expired", Severity: domain.SevBroken, Source: "wave1"}}
		case "Partial":
			findings = []domain.Finding{{Code: CodeVPCEStatePartial, Phrase: "partial", Severity: domain.SevBroken, Source: "wave1"}}
		case "Deleted":
			findings = []domain.Finding{{Code: CodeVPCEStateDeleted, Phrase: "deleted", Severity: domain.SevDim, Source: "wave1"}}
		}

		// An endpoint being torn down carries no live exposure; a posture
		// finding on it is noise an operator can do nothing about.
		lifecycleEnded := state == "Deleting" || state == "Deleted"

		var attentionDetails map[domain.FindingCode]domain.AttentionDetail
		if rows, open := vpcePolicyExposure(aws.ToString(vpce.PolicyDocument)); open && !lifecycleEnded {
			findings = append(findings, domain.Finding{
				Code:     CodeVPCEPolicyOpen,
				Phrase:   VPCEPolicyOpenPhrase,
				Detail:   VPCEPolicyOpenDetail,
				Severity: domain.SevWarn,
				Source:   "wave1",
			})
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
