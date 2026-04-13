package aws

import (
	"context"
	"fmt"

	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// PolicyEnriched wraps iamtypes.Policy with an optional Document field
// populated by the enricher. Embedding promotes all SDK fields so
// YAML/JSON views render the full policy metadata alongside the document.
type PolicyEnriched struct {
	iamtypes.Policy
	Document any `json:"Document,omitempty" yaml:"Document,omitempty"`
}

func init() {
	resource.RegisterEnricher("policy", enrichPolicy)
}

// enrichPolicy fetches the policy document for a top-level IAM policy.
// All top-level policies are managed (Scope=Local), so only the managed
// document path is needed.
func enrichPolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return res, fmt.Errorf("invalid clients")
	}

	policy, ok := res.RawStruct.(iamtypes.Policy)
	if !ok {
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	if policy.Arn == nil {
		return res, fmt.Errorf("policy has no ARN")
	}
	policyArn := *policy.Arn

	cacheKey := ManagedKey(policyArn)
	if cached := c.PolicyDocCache.Get(cacheKey); cached != nil {
		res.RawStruct = PolicyEnriched{Policy: policy, Document: cached}
		return res, nil
	}

	getPolicyAPI, ok1 := c.IAM.(IAMGetPolicyAPI)
	getPolicyVersionAPI, ok2 := c.IAM.(IAMGetPolicyVersionAPI)
	if !ok1 || !ok2 {
		return res, fmt.Errorf("IAM client does not support GetPolicy/GetPolicyVersion")
	}

	doc, err := FetchManagedPolicyDocument(ctx, getPolicyAPI, getPolicyVersionAPI, policyArn)
	if err != nil {
		return res, err
	}

	c.PolicyDocCache.Set(cacheKey, doc)
	res.RawStruct = PolicyEnriched{Policy: policy, Document: doc}
	return res, nil
}
