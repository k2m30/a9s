package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// enrichRolePolicy fetches the policy document for a role_policies resource
// and returns an enriched copy with Document set on RawStruct.
// Uses the session-scoped PolicyDocs cache provided on DetailEnrichmentCtx.
func enrichRolePolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	dctx, ok := clients.(*DetailEnrichmentCtx)
	if !ok || dctx == nil || dctx.Clients == nil || dctx.PolicyDocs == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}
	c := dctx.Clients
	cache := dctx.PolicyDocs

	row, ok := res.RawStruct.(RolePolicyRow)
	if !ok {
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	var (
		doc any
		err error
	)

	if row.PolicyType == "Inline" {
		roleName := res.Fields["role_name"]
		if roleName == "" {
			return res, fmt.Errorf("missing role_name for inline policy")
		}
		cacheKey := InlineKey(roleName, row.PolicyName)

		if cached := cache.Get(cacheKey); cached != nil {
			doc = cached
		} else {
			getRolePolicyAPI, ok := c.IAM.(IAMGetRolePolicyAPI)
			if !ok {
				return res, fmt.Errorf("IAM client does not support GetRolePolicy")
			}
			doc, err = FetchInlinePolicyDocument(ctx, getRolePolicyAPI, roleName, row.PolicyName)
			if err != nil {
				return res, err
			}
			cache.Set(cacheKey, doc)
		}
	} else {
		if row.PolicyArn == "" {
			return res, fmt.Errorf("missing policy ARN for managed policy")
		}
		cacheKey := ManagedKey(row.PolicyArn)

		if cached := cache.Get(cacheKey); cached != nil {
			doc = cached
		} else {
			getPolicyAPI, ok1 := c.IAM.(IAMGetPolicyAPI)
			getPolicyVersionAPI, ok2 := c.IAM.(IAMGetPolicyVersionAPI)
			if !ok1 || !ok2 {
				return res, fmt.Errorf("IAM client does not support GetPolicy/GetPolicyVersion")
			}
			doc, err = FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, row.PolicyArn)
			if err != nil {
				return res, err
			}
			cache.Set(cacheKey, doc)
		}
	}

	row.Document = doc
	res.RawStruct = row
	return res, nil
}
