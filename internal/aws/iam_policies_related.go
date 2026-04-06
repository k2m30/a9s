// iam_policies_related.go contains IAM Policy related-resource checker functions.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkPolicyRole uses the IAM ListEntitiesForPolicy API to return the IAM roles
// attached to this policy.
func checkPolicyRole(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	out, err := c.IAM.ListEntitiesForPolicy(context.Background(), &iam.ListEntitiesForPolicyInput{
		PolicyArn:    &policyARN,
		EntityFilter: iamtypes.EntityTypeRole,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "role", Count: -1, Err: err}
	}
	var ids []string
	for _, r := range out.PolicyRoles {
		if r.RoleName != nil {
			ids = append(ids, *r.RoleName)
		}
	}
	return relatedResult("role", ids)
}

// checkPolicyUser uses the IAM ListEntitiesForPolicy API to return the IAM users
// attached to this policy.
func checkPolicyUser(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}
	out, err := c.IAM.ListEntitiesForPolicy(context.Background(), &iam.ListEntitiesForPolicyInput{
		PolicyArn:    &policyARN,
		EntityFilter: iamtypes.EntityTypeUser,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1, Err: err}
	}
	var ids []string
	for _, u := range out.PolicyUsers {
		if u.UserName != nil {
			ids = append(ids, *u.UserName)
		}
	}
	return relatedResult("iam-user", ids)
}

// checkPolicyGroup uses the IAM ListEntitiesForPolicy API to return the IAM groups
// attached to this policy.
func checkPolicyGroup(_ context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: 0}
	}
	out, err := c.IAM.ListEntitiesForPolicy(context.Background(), &iam.ListEntitiesForPolicyInput{
		PolicyArn:    &policyARN,
		EntityFilter: iamtypes.EntityTypeGroup,
	})
	if err != nil {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: -1, Err: err}
	}
	var ids []string
	for _, g := range out.PolicyGroups {
		if g.GroupName != nil {
			ids = append(ids, *g.GroupName)
		}
	}
	return relatedResult("iam-group", ids)
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
