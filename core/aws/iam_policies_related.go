// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_policies_related.go contains IAM Policy related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

var iamListEntitiesAPIForTest IAMListEntitiesForPolicyAPI

// listAllPolicyEntities walks the unfiltered ListEntitiesForPolicy pages and
// returns them merged into one output, IsTruncated when the walk stopped at
// the page cap.
func listAllPolicyEntities(ctx context.Context, api IAMListEntitiesForPolicyAPI, policyARN string) (*iam.ListEntitiesForPolicyOutput, error) {
	out := &iam.ListEntitiesForPolicyOutput{}
	pages, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]*iam.ListEntitiesForPolicyOutput, *string, error) {
		page, err := api.ListEntitiesForPolicy(ctx, &iam.ListEntitiesForPolicyInput{
			PolicyArn: &policyARN,
			Marker:    marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return []*iam.ListEntitiesForPolicyOutput{page}, iamNextMarker(page.IsTruncated, page.Marker), nil
	})
	for _, page := range pages {
		out.PolicyRoles = append(out.PolicyRoles, page.PolicyRoles...)
		out.PolicyUsers = append(out.PolicyUsers, page.PolicyUsers...)
		out.PolicyGroups = append(out.PolicyGroups, page.PolicyGroups...)
	}
	out.IsTruncated = !complete
	return out, err
}

// resolveIAMAPI returns the IAM API to use: the test override if set, otherwise c.IAM.
func resolveIAMAPI(c *ServiceClients) IAMListEntitiesForPolicyAPI {
	if iamListEntitiesAPIForTest != nil {
		return iamListEntitiesAPIForTest
	}
	return c.IAM
}

// checkPolicyRole uses the IAM ListEntitiesForPolicy API to return the IAM roles
// attached to this policy.
func checkPolicyRole(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("role")
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return foundNone("role", "policyARN")
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
	if err != nil {
		return ReadFailed("role", err)
	}
	var ids []string
	for _, r := range out.PolicyRoles {
		if r.RoleName != nil {
			ids = append(ids, *r.RoleName)
		}
	}
	return relatedResultTrunc("role", ids, out.IsTruncated)
}

// checkPolicyUser uses the IAM ListEntitiesForPolicy API to return the IAM users
// attached to this policy.
func checkPolicyUser(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("iam-user")
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return foundNone("iam-user", "policyARN")
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
	if err != nil {
		return ReadFailed("iam-user", err)
	}
	var ids []string
	for _, u := range out.PolicyUsers {
		if u.UserName != nil {
			ids = append(ids, *u.UserName)
		}
	}
	return relatedResultTrunc("iam-user", ids, out.IsTruncated)
}

// checkPolicyGroup uses the IAM ListEntitiesForPolicy API to return the IAM groups
// attached to this policy.
func checkPolicyGroup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("iam-group")
	}
	if groupName, ok := inlinePolicyGroup(res); ok {
		return relatedResultTrunc("iam-group", []string{groupName}, false)
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return foundNone("iam-group", "policyARN")
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
	if err != nil {
		return ReadFailed("iam-group", err)
	}
	var ids []string
	for _, g := range out.PolicyGroups {
		if g.GroupName != nil {
			ids = append(ids, *g.GroupName)
		}
	}
	return relatedResultTrunc("iam-group", ids, out.IsTruncated)
}

// policyARNFromResource extracts the policy ARN from Fields or RawStruct.
func policyARNFromResource(res resource.Resource) string {
	if arn := res.Fields["arn"]; arn != "" {
		return arn
	}
	if p, ok := assertStruct[iamtypes.Policy](res.RawStruct); ok && p.Arn != nil {
		return *p.Arn
	}
	return ""
}
