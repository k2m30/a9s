// iam_policies_related.go contains IAM Policy related-resource checker functions.
package aws

import (
	"context"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// policyEntitiesCache is a request-scoped, TTL-based cache for ListEntitiesForPolicy results.
// Each entry stores the output and the time it was fetched. Entries older than
// policyEntitiesTTL are evicted on the next read.
var (
	policyEntitiesCache   sync.Map
	policyEntitiesTTL     = 5 * time.Second
	iamListEntitiesAPIForTest IAMListEntitiesForPolicyAPI
)

type policyEntitiesCacheEntry struct {
	out       *iam.ListEntitiesForPolicyOutput
	err       error
	fetchedAt time.Time
}

// listAllPolicyEntities issues ONE unfiltered ListEntitiesForPolicy call and returns
// the full output. Results are cached per policyARN for policyEntitiesTTL.
func listAllPolicyEntities(ctx context.Context, api IAMListEntitiesForPolicyAPI, policyARN string) (*iam.ListEntitiesForPolicyOutput, error) {
	if v, ok := policyEntitiesCache.Load(policyARN); ok {
		entry := v.(policyEntitiesCacheEntry)
		if time.Since(entry.fetchedAt) < policyEntitiesTTL {
			return entry.out, entry.err
		}
		policyEntitiesCache.Delete(policyARN)
	}

	out, err := api.ListEntitiesForPolicy(ctx, &iam.ListEntitiesForPolicyInput{
		PolicyArn: &policyARN,
	})
	policyEntitiesCache.Store(policyARN, policyEntitiesCacheEntry{
		out:       out,
		err:       err,
		fetchedAt: time.Now(),
	})
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
		return resource.RelatedCheckResult{TargetType: "role", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
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
func checkPolicyUser(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "iam-user", Count: 0}
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
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
func checkPolicyGroup(ctx context.Context, clients any, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: -1}
	}
	policyARN := policyARNFromResource(res)
	if policyARN == "" {
		return resource.RelatedCheckResult{TargetType: "iam-group", Count: 0}
	}
	out, err := listAllPolicyEntities(ctx, resolveIAMAPI(c), policyARN)
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
