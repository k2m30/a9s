// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_groups_related.go contains IAM Group related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkGroupUser uses the IAM GetGroup API to return the users in this IAM group.
func checkGroupUser(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("iam-user")
	}
	groupName := res.ID
	if groupName == "" {
		return keyMissing("iam-user", "groupName")
	}
	users, complete, err := iamGroupUsers(ctx, c.IAM, groupName)
	if err != nil {
		return ReadFailed("iam-user", err)
	}
	var ids []string
	for _, u := range users {
		ids = append(ids, aws.ToString(u.UserName))
	}
	return relatedResultTrunc("iam-user", ids, !complete)
}

// checkGroupPolicy returns the combined count of managed and inline policies for this IAM group.
func checkGroupPolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("policy")
	}
	groupName := res.ID
	if groupName == "" {
		return keyMissing("policy", "groupName")
	}
	attached, attachedComplete, err := iamGroupAttachedPolicies(ctx, c.IAM, groupName)
	inline, inlineComplete, err2 := iamGroupInlinePolicies(ctx, c.IAM, groupName)
	if err != nil && err2 != nil && len(attached) == 0 && len(inline) == 0 {
		return ReadFailed("policy", err)
	}
	// A walk that failed or stopped at the cap leaves ids a proven subset of
	// the group's policies, not the exhaustive answer.
	ids := attachedPolicyIDs(attached)
	for _, name := range inline {
		ids = append(ids, inlinePolicyID(groupName, name))
	}
	return relatedResultTrunc("policy", ids, !attachedComplete || !inlineComplete)
}

// iamGroupUsers walks GetGroup's member pages. complete is false when the
// walk failed or stopped at the page cap; users holds every page read.
func iamGroupUsers(ctx context.Context, api IAMGetGroupAPI, groupName string) ([]iamtypes.User, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.User, *string, error) {
		out, err := api.GetGroup(ctx, &iam.GetGroupInput{GroupName: aws.String(groupName), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.Users, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}

// iamGroupAttachedPolicies walks ListAttachedGroupPolicies; see iamGroupUsers.
func iamGroupAttachedPolicies(ctx context.Context, api IAMListAttachedGroupPoliciesAPI, groupName string) ([]iamtypes.AttachedPolicy, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.AttachedPolicy, *string, error) {
		out, err := api.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{GroupName: aws.String(groupName), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.AttachedPolicies, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}

// iamGroupInlinePolicies walks ListGroupPolicies; see iamGroupUsers.
func iamGroupInlinePolicies(ctx context.Context, api IAMListGroupPoliciesAPI, groupName string) ([]string, bool, error) {
	return PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]string, *string, error) {
		out, err := api.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: aws.String(groupName), Marker: marker})
		if err != nil {
			return nil, nil, err
		}
		return out.PolicyNames, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
}
