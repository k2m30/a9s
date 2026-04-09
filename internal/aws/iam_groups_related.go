// iam_groups_related.go contains IAM Group related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkGroupUser uses the IAM GetGroup API to return the users in this IAM group.
func checkGroupUser(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	groupName := res.ID
	if groupName == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}
	out, err := c.IAM.GetGroup(ctx, &iam.GetGroupInput{
		GroupName: &groupName,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1, Err: err}
	}
	var ids []string
	for _, u := range out.Users {
		if u.UserName != nil {
			ids = append(ids, *u.UserName)
		}
	}
	return relatedResult("iam-user", ids)
}

// checkGroupPolicy returns the combined count of managed and inline policies for this IAM group.
func checkGroupPolicy(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1}
	}
	groupName := res.ID
	if groupName == "" {
		return resource.RelatedCheckResult{TargetType: "policy", Count: 0}
	}
	var ids []string
	// Attached managed policies
	attached, err := c.IAM.ListAttachedGroupPolicies(ctx, &iam.ListAttachedGroupPoliciesInput{GroupName: &groupName})
	if err == nil {
		ids = append(ids, customerManagedAttachedPolicyNames(attached.AttachedPolicies)...)
	}
	// Inline policies
	inline, err2 := c.IAM.ListGroupPolicies(ctx, &iam.ListGroupPoliciesInput{GroupName: &groupName})
	if err2 == nil {
		ids = append(ids, inline.PolicyNames...)
	}
	if err != nil && err2 != nil {
		return resource.RelatedCheckResult{TargetType: "policy", Count: -1, Err: err}
	}
	return relatedResult("policy", ids)
}
