// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// iam_users_related.go contains IAM User related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// checkUserGroup uses the IAM ListGroupsForUser API to return the groups
// this IAM user belongs to.
func checkUserGroup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("iam-group")
	}
	userName := res.ID
	if userName == "" {
		return resource.ProvenZero("iam-group", "userName")
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
	if err != nil {
		return resource.ErrorRelated("iam-group", err)
	}
	var ids []string
	for _, g := range groups {
		ids = append(ids, aws.ToString(g.GroupName))
	}
	return relatedResultTrunc("iam-group", ids, !complete)
}

// checkUserPolicy uses the IAM ListAttachedUserPolicies API to return the
// managed policies attached to this IAM user.
func checkUserPolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.UnknownRelated("policy")
	}
	userName := res.ID
	if userName == "" {
		return resource.ProvenZero("policy", "userName")
	}
	attached, complete, err := listAttachedUserPolicies(ctx, c.IAM, userName)
	if err != nil {
		return resource.ErrorRelated("policy", err)
	}
	return relatedResultTrunc("policy", attachedPolicyNames(attached), !complete)
}

// checkIAMUserCtEvents scans the ct-events cache for CloudTrail events where
// the Username field matches this IAM user's name.
func checkIAMUserCtEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	userName := res.ID
	if userName == "" {
		return resource.ProvenZero("ct-events", "userName")
	}

	eventList, truncated, err := relatedResourcesFor(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.ErrorRelated("ct-events", err)
	}
	if eventList == nil {
		return resource.UnknownRelated("ct-events")
	}

	var ids []string
	for _, eventRes := range eventList {
		raw, ok := assertStruct[cloudtrailtypes.Event](eventRes.RawStruct)
		if ok {
			if raw.Username != nil && *raw.Username == userName {
				ids = append(ids, eventRes.ID)
			}
			continue
		}
		if eventRes.Fields["user"] == userName {
			ids = append(ids, eventRes.ID)
		}
	}
	fetchFilter := map[string]string{"Username": userName}
	if truncated {
		// Cache is partial — the filtered fetch will determine the real count.
		return resource.DeferredRelated("ct-events", fetchFilter)
	}
	return relatedResultTrunc("ct-events", ids, false).WithFetchFilter(fetchFilter)
}
