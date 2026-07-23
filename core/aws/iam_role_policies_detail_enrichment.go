// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/k2m30/a9s/v3/core/resource"
)

// enrichRolePolicy fetches the policy document for a role_policies resource
// and returns an enriched copy with Document set on RawStruct. Session-cached
// via the PolicyDocs cache provided on DetailEnrichmentCtx — inline and
// managed policies use distinct cache-key namespaces (InlineKey/ManagedKey)
// since a role's inline PolicyName and a managed PolicyArn are drawn from
// different identifier spaces.
func enrichRolePolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[RolePolicyRow, any]{
		unwrap: unwrapEnriched(func(w RolePolicyRow) RolePolicyRow { return w }),
		// id is a pure validation gate here — inline vs. managed cache keys
		// and API calls are drawn from different identifier spaces
		// (res.Fields["role_name"]/PolicyName vs. PolicyArn), so cacheKey and
		// fetch below re-branch on row.PolicyType instead of consuming id.
		id: func(row RolePolicyRow, res resource.Resource) (string, error) {
			if row.PolicyType == "Inline" {
				if res.Fields["role_name"] == "" {
					return "", fmt.Errorf("missing role_name for inline policy")
				}
				return "", nil
			}
			if row.PolicyArn == "" {
				return "", fmt.Errorf("missing policy ARN for managed policy")
			}
			return "", nil
		},
		cache: policyDocsCache,
		cacheKey: func(_ string, row RolePolicyRow, res resource.Resource) string {
			if row.PolicyType == "Inline" {
				return InlineKey(res.Fields["role_name"], row.PolicyName)
			}
			return ManagedKey(row.PolicyArn)
		},
		fetch: func(ctx context.Context, c *ServiceClients, _ string, row RolePolicyRow, res resource.Resource) (any, error) {
			if row.PolicyType == "Inline" {
				api, ok := c.IAM.(IAMGetRolePolicyAPI)
				if !ok {
					return nil, fmt.Errorf("IAM client does not support GetRolePolicy")
				}
				return FetchInlinePolicyDocument(ctx, api, res.Fields["role_name"], row.PolicyName)
			}
			getPolicyAPI, ok1 := c.IAM.(IAMGetPolicyAPI)
			getPolicyVersionAPI, ok2 := c.IAM.(IAMGetPolicyVersionAPI)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("IAM client does not support GetPolicy/GetPolicyVersion")
			}
			return FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, row.PolicyArn)
		},
		wrap: func(row RolePolicyRow, doc any) any {
			row.Document = doc
			return row
		},
	})
}
