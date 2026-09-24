// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_users_related.go contains IAM User related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkUserGroup uses the IAM ListGroupsForUser API to return the groups
// this IAM user belongs to.
func checkUserGroup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("iam-group")
	}
	userName := res.ID
	if userName == "" {
		return keyMissing("iam-group", "userName")
	}
	groups, complete, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, marker *string) ([]iamtypes.Group, *string, error) {
		out, err := c.IAM.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
			UserName: &userName,
			Marker:   marker,
		})
		if err != nil {
			return nil, nil, err
		}
		return out.Groups, iamNextMarker(out.IsTruncated, out.Marker), nil
	})
	read := pagedRead(complete, err)
	for _, g := range groups {
		read.ids = append(read.ids, aws.ToString(g.GroupName))
	}
	return relatedAnswer("iam-group", read)
}

// checkUserPolicy uses the IAM ListAttachedUserPolicies API to return the
// managed policies attached to this IAM user.
func checkUserPolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return NotRead("policy")
	}
	userName := res.ID
	if userName == "" {
		return keyMissing("policy", "userName")
	}
	attached, complete, err := listAttachedUserPolicies(ctx, c.IAM, userName)
	read := pagedRead(complete, err)
	read.ids = attachedPolicyIDs(attached)
	return relatedAnswer("policy", read)
}
