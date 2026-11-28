// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// JudgeRolePolicies adds to each row of a role's policy list the verdict of
// its own document, from principalPolicyVerdict over that one policy:
// administrator access when it allows everything, broad power when it grants a
// privilege-escalation combination. The documents are the session cache's,
// the same the role row and the policy detail read. A document that could not
// be read is a failure of the list, since a row has no not-inspected mark.
func JudgeRolePolicies(ctx context.Context, clients *ServiceClients, res resource.FetchResult) (resource.FetchResult, error) {
	var failures []Failure
	for i, r := range res.Resources {
		row, _ := assertStruct[RolePolicyRow](r.RawStruct)
		set := policySet{kind: holderRole, holder: r.Fields["role_name"]}
		if row.PolicyType == "Inline" {
			set.inline = []string{row.PolicyName}
		} else {
			set.attached = []iamtypes.AttachedPolicy{{PolicyName: aws.String(row.PolicyName), PolicyArn: aws.String(row.PolicyArn)}}
		}
		v := principalPolicyVerdict(ctx, clients, []policySet{set}, "", false)
		for _, err := range v.errs {
			failures = append(failures, FailedCall(r.ID, err))
		}
		switch {
		case v.admin != nil:
			res.Resources[i] = withWave1(r, wave1Finding(CodeRolePolicyAdministrator))
		case len(v.privEsc) > 0:
			res.Resources[i] = withWave1(r, wave1Finding(CodeRolePolicyBroadPower))
		}
	}
	return res, AggregateFailures("role policy documents", failures, len(res.Resources))
}

// inlineRolePolicyFindings returns the wave1 finding for an inline role
// policy — always dim to distinguish from managed-policy rows.
func inlineRolePolicyFindings() []domain.Finding {
	return []domain.Finding{wave1Finding(CodeRolePolicyInline)}
}

// RolePolicyRow is the RawStruct for each role policy row.
// It holds policy data for detail/YAML view rendering.
type RolePolicyRow struct {
	PolicyName string
	PolicyArn  string
	PolicyType string
	Document   any `json:"Document,omitempty" yaml:"Document,omitempty"`
}

// FetchRolePolicies calls ListAttachedRolePolicies and ListRolePolicies to
// retrieve both managed and inline policies attached to the given IAM role.
// A single API call is made for each list per invocation. Managed policies
// appear first, followed by inline policies. If either list has more pages,
// IsTruncated is set to true. (IAM role policies are limited to 20 managed +
// 10 inline per role so truncation is extremely rare in practice.)
func FetchRolePolicies(
	ctx context.Context,
	attachedAPI IAMListAttachedRolePoliciesAPI,
	inlineAPI IAMListRolePoliciesAPI,
	parentCtx map[string]string,
	continuationToken string,
) (resource.FetchResult, error) {
	roleName := parentCtx["role_name"]

	attachedOutput, err := attachedAPI.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing attached policies for %s: %w", roleName, err)
	}

	var managed []resource.Resource
	for _, p := range attachedOutput.AttachedPolicies {
		policyName := ""
		if p.PolicyName != nil {
			policyName = *p.PolicyName
		}
		policyArn := ""
		if p.PolicyArn != nil {
			policyArn = *p.PolicyArn
		}

		managed = append(managed, withWave1(resource.Resource{
			ID:   policyArn,
			Name: policyName,
			Fields: map[string]string{
				"policy_name": policyName,
				"policy_arn":  policyArn,
				"policy_type": "Managed",
				"role_name":   roleName,
			},
			RawStruct: RolePolicyRow{
				PolicyName: policyName,
				PolicyArn:  policyArn,
				PolicyType: "Managed",
			},
		}))
	}

	inlineOutput, err := inlineAPI.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing inline policies for %s: %w", roleName, err)
	}

	var inline []resource.Resource
	for _, name := range inlineOutput.PolicyNames {
		inline = append(inline, withWave1(resource.Resource{
			ID:   name,
			Name: name,
			Fields: map[string]string{
				"policy_name": name,
				"policy_arn":  "",
				"policy_type": "Inline",
				"role_name":   roleName,
			},
			RawStruct: RolePolicyRow{
				PolicyName: name,
				PolicyArn:  "",
				PolicyType: "Inline",
			},
		}, inlineRolePolicyFindings()...))
	}

	resources := make([]resource.Resource, 0, len(managed)+len(inline))
	resources = append(resources, managed...)
	resources = append(resources, inline...)

	isTruncated := attachedOutput.IsTruncated || inlineOutput.IsTruncated

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
