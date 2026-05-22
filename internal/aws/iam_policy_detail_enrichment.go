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

// enrichPolicy fetches the policy document for a top-level IAM policy.
// All top-level policies are managed (Scope=Local), so only the managed
// document path is needed.
func enrichPolicy(ctx context.Context, clients any, res resource.Resource) (resource.Resource, error) {
	dctx, ok := clients.(*DetailEnrichmentCtx)
	if !ok || dctx == nil || dctx.Clients == nil || dctx.PolicyDocs == nil {
		return res, fmt.Errorf("invalid detail-enrichment context")
	}
	c := dctx.Clients
	cache := dctx.PolicyDocs

	// Accept both the original SDK type and an already-enriched wrapper
	// (re-enrichment happens when detail→YAML/JSON each trigger enrichment).
	var policy iamtypes.Policy
	switch raw := res.RawStruct.(type) {
	case iamtypes.Policy:
		policy = raw
	case PolicyEnriched:
		policy = raw.Policy
	default:
		return res, fmt.Errorf("unexpected RawStruct type: %T", res.RawStruct)
	}

	if policy.Arn == nil || *policy.Arn == "" {
		return res, fmt.Errorf("policy has no ARN")
	}
	policyArn := *policy.Arn

	cacheKey := ManagedKey(policyArn)
	if cached := cache.Get(cacheKey); cached != nil {
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

	cache.Set(cacheKey, doc)
	res.RawStruct = PolicyEnriched{Policy: policy, Document: doc}
	return res, nil
}
