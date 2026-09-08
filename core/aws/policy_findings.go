// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// policy_findings.go owns the vocabulary every policy-derived finding shares:
// the one definition of "an AWS-managed policy that hands out
// administrator-equivalent power" (so the role, user and group
// ListAttached*Policies sweeps can never disagree), and the supporting rows
// a public or cross-account resource policy renders.
package aws

import (
	"context"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/iampolicy"
)

// adminManagedPolicyResources are the AWS-managed policies whose attachment
// makes the principal effectively an account administrator, i.e. whose
// document allows every action on every resource.
// They are named by their resource path rather than their full ARN: AWS
// returns the ARN in the partition the session is connected to, so a literal
// commercial ARN reports every China and GovCloud administrator unprivileged.
//
// AdministratorAccess is the only member. PowerUserAccess is deliberately not
// one: AWS's document allows "*" on "*" only with a NotAction that excludes
// IAM, Organizations and Account, so its holder can neither grant itself
// permissions nor touch the account. Calling it administrator-equivalent
// reported every developer as an admin.
var adminManagedPolicyResources = []string{ //nolint:gochecknoglobals // static AWS-managed policy set
	"policy/AdministratorAccess",
}

// listAttachedRolePolicies walks every page of a role's attached managed
// policies. IAM caps a principal at 20 managed policies today, which fits one
// page, but a quota is not a contract and the group sweep already paginates
// its equivalent — one shape for all three principals.
func listAttachedRolePolicies(ctx context.Context, api IAMListAttachedRolePoliciesAPI, roleName string) ([]iamtypes.AttachedPolicy, error) {
	var all []iamtypes.AttachedPolicy
	var marker *string
	for range PerParentPageCap {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListAttachedRolePoliciesOutput, error) {
			return api.ListAttachedRolePolicies(ctx, &iam.ListAttachedRolePoliciesInput{
				RoleName: aws.String(roleName),
				Marker:   marker,
			})
		})
		if err != nil {
			return all, err
		}
		all = append(all, out.AttachedPolicies...)
		if !out.IsTruncated {
			return all, nil
		}
		marker = out.Marker
	}
	return all, nil
}

// listAttachedUserPolicies is listAttachedRolePolicies for a user.
func listAttachedUserPolicies(ctx context.Context, api IAMListAttachedUserPoliciesAPI, userName string) ([]iamtypes.AttachedPolicy, error) {
	var all []iamtypes.AttachedPolicy
	var marker *string
	for range PerParentPageCap {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*iam.ListAttachedUserPoliciesOutput, error) {
			return api.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
				UserName: aws.String(userName),
				Marker:   marker,
			})
		})
		if err != nil {
			return all, err
		}
		all = append(all, out.AttachedPolicies...)
		if !out.IsTruncated {
			return all, nil
		}
		marker = out.Marker
	}
	return all, nil
}

// AdminAttachedPolicyNameForTest is an exported test-only wrapper for the
// unexported adminAttachedPolicyName — production code does not call it.
// Lives outside _test.go because tests in tests/unit/ are package unit and
// can't see same-package test helpers.
func AdminAttachedPolicyNameForTest(attached []iamtypes.AttachedPolicy) string {
	return adminAttachedPolicyName(attached)
}

// adminAttachedPolicyName returns the name of the first admin-equivalent
// managed policy in the attachment list, or "" when none is attached.
func adminAttachedPolicyName(attached []iamtypes.AttachedPolicy) string {
	for _, p := range attached {
		if a, awsOwned := awsManagedPolicy(aws.ToString(p.PolicyArn)); awsOwned &&
			slices.Contains(adminManagedPolicyResources, a.Resource) {
			name := aws.ToString(p.PolicyName)
			if name == "" {
				name = aws.ToString(p.PolicyArn)
			}
			return name
		}
	}
	return ""
}

// adminAttachedPhrase is the S4 cause for every admin-attached finding. It
// names the class rather than one policy, so the set can grow without the
// phrase becoming a lie; the Policy row says which one is attached.
const adminAttachedPhrase = "has an administrator policy"

// adminAttachedRows is the supporting Attention row shared by the role, user
// and group admin-attached findings.
func adminAttachedRows(policyName string) []domain.DetailRow {
	return []domain.DetailRow{{Label: "Policy", Value: policyName, Tier: "~"}}
}

// publicPolicyRows renders the supporting rows for a "policy is open to
// anyone" finding, shared by every resource-policy check so the KMS key, the
// secret and the next one all describe the exposure identically.
func publicPolicyRows(ex iampolicy.Exposure) []domain.DetailRow {
	rows := []domain.DetailRow{{Label: "Principal", Value: "*", Tier: "!"}}
	if len(ex.PublicActions) > 0 {
		rows = append(rows, domain.DetailRow{
			Label: "Actions", Value: strings.Join(ex.PublicActions, ", "), Tier: "!",
		})
	}
	return rows
}

// crossAccountPolicyRows renders the supporting rows for a "policy grants
// another account" finding.
func crossAccountPolicyRows(ex iampolicy.Exposure) []domain.DetailRow {
	return []domain.DetailRow{{Label: "Accounts", Value: strings.Join(ex.CrossAccount, ", "), Tier: "~"}}
}
