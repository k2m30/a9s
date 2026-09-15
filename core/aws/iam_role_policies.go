// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rolePolicyFindings returns the wave1 findings for one attached managed
// policy.
//
// The ARN decides, not the name: a customer-managed policy may be named
// AdministratorAccess while carrying whatever its own document allows, and
// AWS's own policies sit in the partition the session is connected to.
func rolePolicyFindings(policyARN string) []domain.Finding {
	if awsManagedPolicyIn(policyARN, adminManagedPolicyResources) {
		return []domain.Finding{wave1Finding(CodeRolePolicyAdministrator)}
	}
	if awsManagedPolicyIn(policyARN, broadPowerManagedPolicyResources) {
		return []domain.Finding{wave1Finding(CodeRolePolicyBroadPower)}
	}
	return nil
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

	// Fetch one page of managed (attached) policies
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

		managed = append(managed, resource.Resource{
			ID:       policyArn,
			Name:     policyName,
			Findings: rolePolicyFindings(policyArn),
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
		})
	}

	// Fetch one page of inline policies
	inlineOutput, err := inlineAPI.ListRolePolicies(ctx, &iam.ListRolePoliciesInput{
		RoleName: &roleName,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing inline policies for %s: %w", roleName, err)
	}

	var inline []resource.Resource
	for _, name := range inlineOutput.PolicyNames {
		inline = append(inline, resource.Resource{
			ID:       name,
			Name:     name,
			Findings: inlineRolePolicyFindings(),
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
		})
	}

	// Managed first, then inline
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
