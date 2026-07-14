// iam_users_related.go contains IAM User related-resource checker functions.
package aws

import (
	"context"

	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
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
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: 0}
	}
	out, err := c.IAM.ListGroupsForUser(ctx, &iam.ListGroupsForUserInput{
		UserName: &userName,
	})
	if err != nil {
		return resource.ErrorRelated("iam-group", err)
	}
	var ids []string
	for _, g := range out.Groups {
		if g.GroupName != nil {
			ids = append(ids, *g.GroupName)
		}
	}
	return relatedResult("iam-group", ids)
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
		return resource.RelatedCheckResult{TargetType: "policy", Count: 0}
	}
	out, err := c.IAM.ListAttachedUserPolicies(ctx, &iam.ListAttachedUserPoliciesInput{
		UserName: &userName,
	})
	if err != nil {
		return resource.ErrorRelated("policy", err)
	}
	ids := attachedPolicyNames(out.AttachedPolicies)
	return relatedResult("policy", ids)
}

// checkIAMUserCtEvents scans the ct-events cache for CloudTrail events where
// the Username field matches this IAM user's name.
// Pattern C: cache scan matching Fields["user"] to username.
func checkIAMUserCtEvents(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	userName := res.ID
	if userName == "" {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: 0}
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
	result := relatedResult("ct-events", ids)
	result.FetchFilter = fetchFilter
	return result
}
