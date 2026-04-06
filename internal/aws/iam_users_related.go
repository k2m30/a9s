// iam_users_related.go contains IAM User related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"

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
