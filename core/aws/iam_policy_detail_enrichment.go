// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// PolicyEnriched wraps iamtypes.Policy with an optional Document field
// populated by the enricher. Embedding promotes all SDK fields so
// YAML/JSON views render the full policy metadata alongside the document.
type PolicyEnriched struct {
	iamtypes.Policy
	Document any `json:"Document,omitempty" yaml:"Document,omitempty"`
}

// enrichPolicy fetches the policy document for a top-level IAM policy.
// All top-level policies are managed (Scope=Local), so only the managed
// document path is needed. Session-cached via PolicyDocs.
func enrichPolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	return enrichDetail(ctx, clients, res, detailEnrichSpec[iamtypes.Policy, any]{
		unwrap: unwrapEnriched(func(w PolicyEnriched) iamtypes.Policy { return w.Policy }),
		id: func(policy iamtypes.Policy, _ resource.Resource) (string, error) {
			if policy.Arn == nil || *policy.Arn == "" {
				return "", fmt.Errorf("policy has no ARN")
			}
			return *policy.Arn, nil
		},
		cache: policyDocsCache,
		cacheKey: func(id string, _ iamtypes.Policy, _ resource.Resource) string {
			return ManagedKey(id)
		},
		fetch: func(ctx context.Context, c *ServiceClients, id string, _ iamtypes.Policy, _ resource.Resource) (any, error) {
			getPolicyAPI, ok1 := c.IAM.(IAMGetPolicyAPI)
			getPolicyVersionAPI, ok2 := c.IAM.(IAMGetPolicyVersionAPI)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("IAM client does not support GetPolicy/GetPolicyVersion")
			}
			return FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, id)
		},
		wrap: func(policy iamtypes.Policy, doc any) any {
			return PolicyEnriched{Policy: policy, Document: doc}
		},
	})
}
