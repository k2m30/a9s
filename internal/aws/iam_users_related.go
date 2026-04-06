// iam_users_related.go contains IAM User related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkUserGroup uses the IAM ListGroupsForUser API to return the groups
// this IAM user belongs to.
func checkUserGroup(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: -1}
	}
	userName := res.ID
	if userName == "" {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: 0}
	}
	out, err := c.IAM.ListGroupsForUser(context.Background(), &iam.ListGroupsForUserInput{
		UserName: &userName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: -1, Err: err}
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
func checkUserPolicy(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1}
	}
	userName := res.ID
	if userName == "" {
		return resource.RelatedCheckResult{TargetType: "policy", Count: 0}
	}
	out, err := c.IAM.ListAttachedUserPolicies(context.Background(), &iam.ListAttachedUserPoliciesInput{
		UserName: &userName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1, Err: err}
	}
	var ids []string
	for _, p := range out.AttachedPolicies {
		if p.PolicyArn != nil {
			ids = append(ids, *p.PolicyArn)
		}
	}
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

	eventList, truncated, err := iamUserRelatedResources(ctx, clients, cache, "ct-events")
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1, Err: err}
	}
	if eventList == nil {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
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
	if len(ids) == 0 && truncated {
		return resource.RelatedCheckResult{TargetType: "ct-events", Count: -1}
	}
	return relatedResult("ct-events", ids)
}

// iamUserRelatedResources returns the resource list for target from cache or by
// fetching the first page via the registered paginated fetcher.
func iamUserRelatedResources(ctx context.Context, clients any, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
	resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
	if err != nil {
		if _, ok := clients.(*ServiceClients); !ok {
			return nil, false, nil
		}
	}
	return resources, isTruncated, err
}
